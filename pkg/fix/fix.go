// Package fix applies the mechanically safe repairs that surface.Analyze
// reports: normalizing go.mod and go.work `go` directives to major.minor
// form. Repairs run through `go mod edit` / `go work edit` (never text
// rewriting), and every applied fix is verified by re-reading the file —
// a fix is only counted when the directive actually changed.
package fix

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// Options controls how fixes are applied.
type Options struct {
	// DryRun reports what would change without touching files.
	DryRun bool
}

// Result summarizes one Apply run.
type Result struct {
	// Applied lists fixes whose directive now equals the target and
	// survived tidy, verified by re-reading the file.
	Applied []surface.Fix
	// HeldBack lists fixes skipped because of dry-run.
	HeldBack []surface.Fix
	// DepForced lists fixes that tidy reverts because dependencies force a
	// higher floor — supply-side re-tags are the actual fix.
	DepForced []DepForced
	// Failures lists fixes whose rewrite or verification failed.
	Failures []Failure
}

// DepForced is a form fix that `go mod tidy` reverts: the module's
// dependency floor is above the stripped directive.
type DepForced struct {
	Fix surface.Fix
	// Floor is the directive value tidy enforced (the highest dep floor).
	Floor string
	// Poisoners names the dependencies carrying that floor; nil when they
	// could not be resolved.
	Poisoners []string
}

// Failure is one fix that could not be applied or verified.
type Failure struct {
	Fix   surface.Fix
	Cause string
}

// Report renders the human-readable run summary.
func (r *Result) Report() string {
	if r == nil {
		return "no fixes"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "applied %d, dep-forced %d, held back %d, failed %d",
		len(r.Applied), len(r.DepForced), len(r.HeldBack), len(r.Failures))

	for _, f := range r.Applied {
		fmt.Fprintf(&b, "\n  ok:         %s", f.Describe())
	}

	for _, d := range r.DepForced {
		fmt.Fprintf(&b, "\n  dep-forced: %s", d.Fix.Describe())

		if len(d.Poisoners) > 0 {
			fmt.Fprintf(&b, "\n              floor go %s is forced by: %s", d.Floor, strings.Join(d.Poisoners, ", "))
			fmt.Fprintf(
				&b,
				"\n              fix supply-side: re-tag those modules with a major.minor-only go directive, then bump consumers",
			)
		} else {
			fmt.Fprintf(
				&b,
				"\n              floor go %s is forced by dependencies (poisoner resolution unavailable)",
				d.Floor,
			)
		}
	}

	for _, f := range r.HeldBack {
		fmt.Fprintf(&b, "\n  dry-run:    %s", f.Describe())
	}

	for _, f := range r.Failures {
		fmt.Fprintf(&b, "\n  FAILED:     %s: %s", f.Fix.Describe(), f.Cause)
	}

	return b.String()
}

// GoCommandRunner shells out to the go binary and returns its combined
// output; overridable in tests.
type GoCommandRunner func(ctx context.Context, dir string, args ...string) (string, error)

// SelfCheck sentinel error.
var errEmptyGoVersion = errors.New("fix: go env GOVERSION returned empty output")

// SelfCheck verifies the `go` binary is available: directive edits shell
// out to it, so a missing toolchain means repairs cannot run.
func SelfCheck(ctx context.Context) error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("fix: go binary not found in PATH: %w", err)
	}

	out, err := exec.CommandContext(ctx, goBin, "env", "GOVERSION").Output()
	if err != nil {
		return fmt.Errorf("fix: go env GOVERSION: %w", err)
	}

	if strings.TrimSpace(string(out)) == "" {
		return errEmptyGoVersion
	}

	return nil
}

// applyOne sentinel errors.
var (
	errUnknownDirectiveKind = errors.New("unknown directive kind")
	errDirectiveMismatch    = errors.New("verify after edit: directive mismatch")
)

// EditRunner returns the production runner: `go <args…>` executed in dir.
// Module edits and module-graph listings run with GOWORK=off so a workspace
// file cannot redirect them to a different module; workspace edits need the
// opposite — they edit the go.work next to dir and GOWORK=off makes
// `go work edit` fail with "no go.work file found".
func EditRunner() GoCommandRunner {
	return func(ctx context.Context, dir string, args ...string) (string, error) {
		goBin, err := exec.LookPath("go")
		if err != nil {
			return "", fmt.Errorf("find go binary: %w", err)
		}

		cmd := exec.CommandContext(ctx, goBin, args...)
		cmd.Dir = dir

		if moduleScoped(args) {
			cmd.Env = append(os.Environ(), "GOWORK=off")
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(
					out,
				), fmt.Errorf(
					"go %s: %s: %w",
					strings.Join(args, " "),
					strings.TrimSpace(string(out)),
					err,
				)
		}

		return string(out), nil
	}
}

// moduleScoped reports whether a go invocation must ignore any enclosing
// workspace: module edits and module listings resolve the module under dir,
// never the workspace above it.
func moduleScoped(args []string) bool {
	return len(args) > 0 && (args[0] == "mod" || args[0] == "list")
}

// Apply executes every mechanical fix under root.
func Apply(ctx context.Context, root string, fixes []surface.Fix, opts Options, run GoCommandRunner) (*Result, error) {
	if run == nil {
		run = EditRunner()
	}

	res := &Result{}

	for _, fx := range fixes {
		if opts.DryRun {
			res.HeldBack = append(res.HeldBack, fx)

			continue
		}

		if err := applyOne(ctx, root, fx, run); err != nil {
			if depForced, ok := errors.AsType[*DepForcedError](err); ok {
				res.DepForced = append(res.DepForced, DepForced{
					Fix:       fx,
					Floor:     depForced.Floor,
					Poisoners: depForced.Poisoners,
				})

				continue
			}

			res.Failures = append(res.Failures, Failure{Fix: fx, Cause: err.Error()})

			continue
		}

		res.Applied = append(res.Applied, fx)
	}

	return res, nil
}

// applyOne rewrites one directive via the go tool and verifies the result
// survives `go mod tidy` (for go.mod: since Go 1.21 the directive must stay
// at or above the highest dependency floor, so a stripped directive that
// tidy re-raises is dep-forced, not mechanically fixable). Command success
// alone proves nothing; the file is re-parsed as the oracle.
func applyOne(ctx context.Context, root string, fx surface.Fix, run GoCommandRunner) error {
	abs := filepath.Join(root, fx.File)
	dir := filepath.Dir(abs)

	switch fx.Kind {
	case surface.KindGoMod:
		if _, err := run(ctx, dir, "mod", "edit", "-go="+fx.To); err != nil {
			return err
		}
	case surface.KindGoWork:
		if _, err := run(ctx, dir, "work", "edit", "-go="+fx.To); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: %q", errUnknownDirectiveKind, fx.Kind)
	}

	got, err := currentDirective(abs, fx.Kind)
	if err != nil {
		return fmt.Errorf("verify after edit: %w", err)
	}

	if got != fx.To {
		return fmt.Errorf("%w: got go %s, want go %s", errDirectiveMismatch, got, fx.To)
	}

	if fx.Kind == surface.KindGoMod {
		if err := ensureTidyStable(ctx, dir, fx, run); err != nil {
			return err
		}
	}

	return nil
}

// ensureTidyStable re-runs tidy and re-reads the directive: when tidy
// raises it back above the target, the floor is dep-forced. The poisoners
// — dependencies whose own go.mod floor equals the raised directive — are
// resolved with `go list -m` and named in the error so the fix surfaces as
// an actionable supply-side re-tag instead of a silently reverted edit.
func ensureTidyStable(ctx context.Context, dir string, fx surface.Fix, run GoCommandRunner) error {
	if _, err := run(ctx, dir, "mod", "tidy"); err != nil {
		return fmt.Errorf("tidy stability check: %w", err)
	}

	got, err := currentDirective(filepath.Join(dir, "go.mod"), surface.KindGoMod)
	if err != nil {
		return fmt.Errorf("verify tidy stability: %w", err)
	}

	if got == fx.To {
		return nil
	}

	poisoners, err := resolvePoisoners(ctx, dir, run, got)
	if err != nil {
		return &DepForcedError{
			Fix:       fx,
			Floor:     got,
			Poisoners: nil,
			Cause: fmt.Sprintf(
				"go mod tidy re-raises the directive to go %s (poisoner resolution failed: %v)",
				got,
				err,
			),
		}
	}

	return &DepForcedError{Fix: fx, Floor: got, Poisoners: poisoners}
}

// resolvePoisoners lists modules whose go floor equals the forced floor
// (the value tidy enforces, which is the highest dependency floor).
func resolvePoisoners(ctx context.Context, dir string, run GoCommandRunner, floor string) ([]string, error) {
	out, err := run(ctx, dir, "list", "-m", "-f", "{{.Path}} {{.Version}} {{.GoVersion}}", "all")
	if err != nil {
		return nil, fmt.Errorf("list dependency floors: %w", err)
	}

	var poisoners []string

	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 3 || fields[2] != floor {
			continue
		}

		if fields[1] == "(devel)" || fields[1] == "" {
			continue
		}

		poisoners = append(poisoners, fields[0])
	}

	return poisoners, nil
}

// DepForcedError reports a form fix that tidy reverts because a dependency
// forces a higher floor. Poisoners names the dependencies carrying that
// floor (nil when resolution failed; Floor still names the forced value).
type DepForcedError struct {
	Fix       surface.Fix
	Floor     string
	Poisoners []string
	Cause     string
}

func (e *DepForcedError) Error() string {
	if len(e.Poisoners) == 0 {
		if e.Cause != "" {
			return e.Cause
		}

		return fmt.Sprintf("go mod tidy re-raises the directive to go %s: a dependency forces this floor", e.Floor)
	}

	return fmt.Sprintf(
		"go mod tidy re-raises the directive to go %s: dependency floor forced by %s — "+
			"fix supply-side by re-tagging with a major.minor-only go directive, then bump",
		e.Floor,
		strings.Join(e.Poisoners, ", "),
	)
}

// currentDirective re-parses the file and returns the current `go` directive.
func currentDirective(abs string, kind surface.DirectiveKind) (string, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", abs, err)
	}

	parsed, _, err := surface.ParseDirective(kind, data)
	if err != nil {
		return "", fmt.Errorf("parse directive: %w", err)
	}

	return parsed, nil
}
