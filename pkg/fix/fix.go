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

	atomicwrite "github.com/larsartmann/go-atomic-write"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// Options controls how fixes are applied.
type Options struct {
	// DryRun reports what would change without touching files.
	DryRun bool
	// Gate is the SplitRunner for the `go mod tidy -diff` dependency-floor
	// gate on go.mod rewrites; nil uses the production exec runner.
	Gate SplitRunner
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
	Floor surface.GoVersion
	// Poisoners names the dependencies carrying that floor; nil when they
	// could not be resolved.
	Poisoners []string
}

// FailureCause explains why one fix could not be applied or verified.
type FailureCause string

// Failure is one fix that could not be applied or verified.
type Failure struct {
	Fix   surface.Fix
	Cause FailureCause
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
	errDirectiveDrifted     = errors.New("directive changed since detection")
	errGateNotClean         = errors.New("go mod tidy -diff is not clean after the rewrite")
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
			cmd.Env = append(append(os.Environ(), "GOWORK=off"), moduleScopedExtraEnv()...)
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
// moduleScopedExtraEnv returns the extra environment module-scoped commands
// need beyond GOWORK=off. GOTOOLCHAIN=auto is appended when the parent shell
// pins "local" (or nothing): a `go list -m` must reflect the analyzed
// module's own floor, and a shell stuck on an older local toolchain would
// otherwise fail the listing with "go.mod requires go >= X" — the exact
// drift who-forces exists to surface. An explicit non-local parent pin (the
// fleet standard, e.g. GOTOOLCHAIN=go1.27.1) is inherited untouched.
func moduleScopedExtraEnv() []string {
	switch os.Getenv("GOTOOLCHAIN") {
	case "", "local":
		return []string{"GOTOOLCHAIN=auto"}
	default:
		return nil
	}
}

func moduleScoped(args []string) bool {
	return len(args) > 0 && (args[0] == "mod" || args[0] == "list")
}

// gateRunner resolves the dependency-gate runner for these options: the
// injected one when set, the production exec runner otherwise.
func gateRunner(opts Options) SplitRunner {
	if opts.Gate != nil {
		return opts.Gate
	}

	return ExecSplitRunner()
}

// Apply executes every mechanical fix under root.
func Apply(ctx context.Context, root string, fixes []surface.Fix, opts Options, run GoCommandRunner) (*Result, error) {
	if run == nil {
		run = EditRunner()
	}

	gate := gateRunner(opts)

	res := &Result{}

	for _, fx := range fixes {
		if opts.DryRun {
			res.HeldBack = append(res.HeldBack, fx)

			continue
		}

		if err := applyOne(ctx, root, fx, run, gate); err != nil {
			if depForced, ok := errors.AsType[*DepForcedError](err); ok {
				res.DepForced = append(res.DepForced, DepForced{
					Fix:       fx,
					Floor:     depForced.Floor,
					Poisoners: depForced.Poisoners,
				})

				continue
			}

			res.Failures = append(res.Failures, Failure{Fix: fx, Cause: FailureCause(err.Error())})

			continue
		}

		res.Applied = append(res.Applied, fx)
	}

	return res, nil
}

// applyOne rewrites one directive and verifies the result survives the
// `go mod tidy -diff` dependency-floor gate (for go.mod: since Go 1.21 the
// directive must stay at or above the highest dependency floor, so a
// stripped directive the gate rejects is dep-forced, not mechanically
// fixable). go.mod rewrites are byte-preserving text surgery — `go mod
// edit` would reflow the whole file and churn unrelated lines — and the
// gate is non-mutating, so a rejected fix leaves the file untouched. go.work
// rewrites go through `go work edit`, which needs workspace discovery. The
// file is re-parsed as the oracle on every path.
func applyOne(ctx context.Context, root string, fx surface.Fix, run GoCommandRunner, gate SplitRunner) error {
	abs := filepath.Join(root, fx.File)
	dir := filepath.Dir(abs)

	switch fx.Kind {
	case surface.KindGoMod:
		return applyGoModFix(ctx, abs, fx, run, gate)
	case surface.KindGoWork:
		if _, err := run(ctx, dir, "work", "edit", "-go="+string(fx.To)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: %q", errUnknownDirectiveKind, fx.Kind)
	}

	got, err := currentDirective(surface.FilePath(abs), fx.Kind)
	if err != nil {
		return fmt.Errorf("verify after edit: %w", err)
	}

	if got != fx.To {
		return fmt.Errorf("%w: got go %s, want go %s", errDirectiveMismatch, got, fx.To)
	}

	return nil
}

// applyGoModFix performs one byte-preserving go-directive rewrite on a
// go.mod and classifies a dirty dependency gate: the module floor forced by
// the dependency graph (the highest dependency go line, which is what tidy
// enforces) above the fix target means dep-forced; anything else is a
// plain failure. The original content is restored before classifying.
func applyGoModFix(ctx context.Context, abs string, fx surface.Fix, run GoCommandRunner, gate SplitRunner) error {
	original, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read %s: %w", abs, err)
	}

	if err := verifyDriftGuard(abs, original, fx.From); err != nil {
		return err
	}

	updated, err := rewriteGoDirective(string(original), string(fx.To))
	if err != nil {
		return err
	}

	if err := atomicwrite.Write(abs, []byte(updated)); err != nil {
		return fmt.Errorf("write %s: %w", abs, err)
	}

	err = verifyDirectiveMatches(updated, fx.To)
	if err == nil {
		if g := runTidyDiffGate(ctx, filepath.Dir(abs), gate); g.Dirty() {
			err = classifyGateRejection(ctx, abs, fx, run, g)
		}
	}

	if err != nil {
		if revertErr := atomicwrite.Write(abs, original); revertErr != nil {
			return fmt.Errorf("%w (AND revert of %s failed: %w)", err, abs, revertErr)
		}
	}

	return err
}

// verifyDriftGuard rejects the fix when the directive on disk no longer
// matches the value Analyze recorded: rewriting anyway would silently apply
// a different change than the one detected.
func verifyDriftGuard(abs string, content []byte, want surface.GoVersion) error {
	got, _, err := surface.ParseDirective(surface.KindGoMod, content)
	switch {
	case errors.Is(err, surface.ErrNoDirective):
		return fmt.Errorf("%w: %s declares no go directive", surface.ErrNoDirective, abs)
	case err != nil:
		return fmt.Errorf("parse %s: %w", abs, err)
	case got != want:
		return fmt.Errorf("%w: got go %s, want go %s", errDirectiveDrifted, got, want)
	default:
		return nil
	}
}

// verifyDirectiveMatches re-parses rewritten content and reports a mismatch
// against the fix target.
func verifyDirectiveMatches(content string, want surface.GoVersion) error {
	now, _, err := surface.ParseDirective(surface.KindGoMod, []byte(content))
	if err != nil {
		return fmt.Errorf("verify after edit: %w", err)
	}

	if now != want {
		return fmt.Errorf("%w: got go %s, want go %s", errDirectiveMismatch, now, want)
	}

	return nil
}

// classifyGateRejection reverts-context classification of a dirty gate: it
// names the dependency floor and its carriers when the module is
// dep-forced, and falls back to the gate output otherwise.
func classifyGateRejection(ctx context.Context, abs string, fx surface.Fix, run GoCommandRunner, g gateResult) error {
	floor, poisoners, listErr := resolveDepFloor(ctx, filepath.Dir(abs), run)
	switch {
	case listErr != nil:
		return fmt.Errorf(
			"%w and the dependency graph could not be listed: %w; gate: %s",
			errGateNotClean,
			listErr,
			g.Detail,
		)
	case floor != "" && surface.GreaterVersion(string(floor), string(fx.To)):
		return &DepForcedError{Fix: fx, Floor: floor, Poisoners: poisoners}
	default:
		return fmt.Errorf("%w (dependency floor %s does not exceed the target): %s", errGateNotClean, floor, g.Detail)
	}
}

// resolveDepFloor lists the module's dependency graph and returns the
// highest dependency `go` floor — the value tidy enforces — together with
// the published dependencies carrying it. Replaced development versions
// ("(devel)") count toward the floor but are never named as poisoners,
// since there is nothing to re-tag.
func resolveDepFloor(ctx context.Context, dir string, run GoCommandRunner) (surface.GoVersion, []string, error) {
	out, err := run(ctx, dir, "list", "-m", "-f", "{{.Path}} {{.Version}} {{.GoVersion}}", "all")
	if err != nil {
		// The rewritten tree is untidy exactly when tidy is about to
		// revert the fix, and a plain list refuses to load that graph.
		// -e lists the module graph anyway, so the floor and the
		// dependencies carrying it stay nameable in the dep-forced report.
		out, err = run(ctx, dir, "list", "-m", "-e", "-f", "{{.Path}} {{.Version}} {{.GoVersion}}", "all")
		if err != nil {
			return "", nil, fmt.Errorf("list dependency floors: %w", err)
		}
	}

	var (
		floor     surface.GoVersion
		poisoners []string
	)

	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 3 || fields[0] == "" || fields[2] == "" {
			continue
		}

		if floor == "" || surface.GreaterVersion(fields[2], string(floor)) {
			floor = surface.GoVersion(fields[2])
			poisoners = nil
		}

		if fields[1] == "(devel)" || fields[1] == "" {
			continue
		}

		if fields[2] == string(floor) {
			poisoners = append(poisoners, fields[0])
		}
	}

	return floor, poisoners, nil
}

// DepForcedError reports a form fix that tidy reverts because a dependency
// forces a higher floor. Poisoners names the dependencies carrying that
// floor (nil when resolution failed; Floor still names the forced value).
type DepForcedError struct {
	Fix       surface.Fix
	Floor     surface.GoVersion
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
func currentDirective(abs surface.FilePath, kind surface.DirectiveKind) (surface.GoVersion, error) {
	data, err := os.ReadFile(string(abs))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", abs, err)
	}

	parsed, _, err := surface.ParseDirective(kind, data)
	if err != nil {
		return "", fmt.Errorf("parse directive: %w", err)
	}

	return parsed, nil
}
