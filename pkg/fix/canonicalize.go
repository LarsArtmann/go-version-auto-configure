package fix

// Canonicalization of one go.mod's toolchain directives: the byte-preserving
// counterpart to surface.Fix application. Where Apply rewrites one directive
// recorded by Analyze, CanonicalizeGoMod derives the changes itself from the
// file (patch-form go line, toolchain line) and guards the downgrade with a
// `go mod tidy -diff` dependency-floor gate. The verdict is earned on a
// staged copy of the mod files (stage.go), never on the live file: a
// rejected downgrade is simply not committed.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	atomicwrite "github.com/larsartmann/go-atomic-write"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// errNoDirectiveToRewrite is wrapped when the go-line regex finds nothing.
var errNoDirectiveToRewrite = errors.New("no go directive found to rewrite")

// SplitRunner shells out to the go binary and returns stdout and stderr
// separately, overridable in tests. The split matters for the dependency
// gate: `go mod tidy -diff` prints the diff on stdout while benign notices
// ("all" matched no packages) land on stderr with exit 0, so only stdout
// may mark a gate dirty.
type SplitRunner func(ctx context.Context, dir string, args ...string) (stdout string, stderr string, err error)

// tidyGateTimeout bounds the `go mod tidy -diff` dependency-floor gate; a
// wedged gate must not stall the caller past its own deadline.
const tidyGateTimeout = 2 * time.Minute

// execWaitDelay bounds cmd.Wait after context cancellation: a wedged child
// holding a pipe cannot block Wait forever.
const execWaitDelay = 5 * time.Second

// ExecSplitRunner returns the production runner: `go <args…>` executed in
// dir, with GOWORK=off for module-scoped subcommands (mod, list) so an
// enclosing workspace cannot redirect module resolution.
func ExecSplitRunner() SplitRunner {
	return func(ctx context.Context, dir string, args ...string) (string, string, error) {
		goBin, err := exec.LookPath("go")
		if err != nil {
			return "", "", fmt.Errorf("find go binary: %w", err)
		}

		cmd := exec.CommandContext(ctx, goBin, args...)
		cmd.Dir = dir
		cmd.WaitDelay = execWaitDelay

		if moduleScoped(args) {
			cmd.Env = append(append(os.Environ(), "GOWORK=off"), moduleScopedExtraEnv()...)
		}

		var stdoutBuf, stderrBuf strings.Builder

		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		err = cmd.Run()

		return stdoutBuf.String(), stderrBuf.String(),
			hintToolchainDownload(stdoutBuf.String()+stderrBuf.String(), err)
	}
}

// gateResult captures the outcome of the tidy-diff dependency-floor gate.
type gateResult struct {
	Stdout string // raw stdout: a non-empty stdout means tidy wants changes
	Detail string // trimmed stdout+stderr combined, for diagnostics
	Err    error  // non-nil when the command failed (floor conflict, resolve error)
}

// Dirty reports whether the gate forbids the edit: tidy wants changes (diff
// on stdout) or the command failed outright.
func (r gateResult) Dirty() bool {
	return r.Err != nil || strings.TrimSpace(r.Stdout) != ""
}

// goDirectiveLine matches a top-level go directive line, capturing the
// version token and any trailing content (e.g. a comment). Anchored to line
// start so module paths like "go-something" inside require blocks cannot
// match.
var goDirectiveLine = regexp.MustCompile(`(?m)^go[ \t]+(\S+)(.*)$`)

// toolchainDirectiveLine matches a full toolchain directive line including
// its trailing newline, so removal does not leave a blank line behind.
var toolchainDirectiveLine = regexp.MustCompile(`(?m)^toolchain[ \t]+\S+[^\n]*\n?`)

// rewriteGoDirective rewrites the go directive to the given target version.
// Text-level surgery preserves the rest of the file byte-for-byte;
// `go mod edit` would reflow the whole file and churn unrelated lines.
func rewriteGoDirective(content, target string) (string, error) {
	loc := goDirectiveLine.FindStringSubmatchIndex(content)
	if loc == nil {
		return "", fmt.Errorf("%w: target go %s", errNoDirectiveToRewrite, target)
	}

	newGoLine := "go " + target + content[loc[4]:loc[5]]

	return content[:loc[0]] + newGoLine + content[loc[1]:], nil
}

// stripToolchainDirective removes a toolchain directive line from go.mod
// content. Fleet policy: go.mod files carry no toolchain line — under
// GOTOOLCHAIN=local it is a hard floor that fails the build when the
// toolchain lags, and under GOTOOLCHAIN=auto it forces a toolchain download.
// A blank line directly before the removed line is consumed too, so the go
// block does not grow a double blank line. Content without a toolchain line
// is returned unchanged.
func stripToolchainDirective(content string) string {
	loc := toolchainDirectiveLine.FindStringIndex(content)
	if loc == nil {
		return content
	}

	start := loc[0]

	if start >= 2 && content[start-1] == '\n' && content[start-2] == '\n' {
		start--
	}

	return content[:start] + content[loc[1]:]
}

// CanonicalizeOptions controls CanonicalizeGoMod.
type CanonicalizeOptions struct {
	// DryRun reports what would change without touching files.
	DryRun bool
	// StripToolchain removes a `toolchain` directive line alongside the go
	// line rewrite. Stripping needs no dependency gate of its own
	// (toolchain directives do not participate in module resolution); a
	// downgrade+strip combination is withheld together when a gate rejects
	// the candidate.
	StripToolchain bool
	// InstalledToolchain is the installed Go version, e.g. "1.27.1" (the
	// "go" prefix is tolerated). Empty resolves it via `go env GOVERSION`.
	InstalledToolchain surface.GoVersion
	// RespectPatchFloor keeps a patch-form go directive whose full version
	// does not exceed the installed toolchain: fleet hosts pin an exact
	// patch, and a pin the local toolchain already satisfies is deliberate
	// policy, not drift. The toolchain-strip path is independent: a strip
	// requested explicitly still happens.
	RespectPatchFloor bool
	// ExtraGate is a caller-supplied check on the candidate go.mod, run
	// after the dependency-floor gate and before the commit; nil runs none.
	// Every rewrite pays it, a toolchain-only strip included: a stripped
	// toolchain line can change which toolchain builds the module.
	ExtraGate ExtraGate
}

// CanonicalizeResult summarizes one CanonicalizeGoMod run.
type CanonicalizeResult struct {
	// Changed reports whether the file was rewritten.
	Changed bool
	// Changes describes each applied rewrite, e.g. "go line 1.26.7 -> 1.26".
	Changes []string
	// HeldBack reports a dry-run: Changes lists what would be applied.
	HeldBack bool
	// HeldBackByDepFloor reports that the dependency-floor gate rejected the
	// downgrade, so the file was never written: a dependency forces the
	// floor.
	HeldBackByDepFloor bool
	// HeldBackByExtraGate reports that the caller gate rejected the rewrite,
	// so the file was never written. Never set together with
	// HeldBackByDepFloor: the dependency-floor gate runs first and a
	// rejection there skips the caller gate.
	HeldBackByExtraGate bool
	// GateDetail carries the rejecting gate's output: the `go mod tidy
	// -diff` diff for a dependency-floor rejection, the caller gate's error
	// text for an extra-gate rejection.
	GateDetail string
	// Skipped reports that nothing was attempted; SkipReason says why.
	Skipped bool
	// SkipReason explains a Skipped result, e.g. an unparseable go.mod or
	// a go line above the installed toolchain. Empty when Skipped is false.
	SkipReason string
}

// canonicalizePlan is the parsed rewrite plan for one go.mod: which
// directives change and why, before any filesystem or gate work.
type canonicalizePlan struct {
	goVersion        surface.GoVersion
	minor            surface.GoVersion
	toolchainVersion surface.GoVersion
	hasToolchain     bool
	patchPinned      bool
	stripToolchain   bool
	// skipReason is non-empty when nothing should be attempted.
	skipReason string
}

// planCanonicalization parses the go.mod content and decides what would
// change. A non-empty skipReason means the file is left untouched (already
// canonical, unparseable, or beyond the installed toolchain).
func planCanonicalization(content []byte, opts CanonicalizeOptions) canonicalizePlan {
	goVersion, _, parseErr := surface.ParseDirective(surface.KindGoMod, content)
	switch {
	case errors.Is(parseErr, surface.ErrNoDirective):
		return canonicalizePlan{skipReason: "no go directive"}
	case parseErr != nil:
		return canonicalizePlan{skipReason: fmt.Sprintf("unparseable go.mod: %v", parseErr)}
	}

	toolchainVersion, _, toolErr := surface.ParseToolchain(surface.KindGoMod, content)
	hasToolchain := toolErr == nil && toolchainVersion != ""

	plan := canonicalizePlan{
		goVersion:        goVersion,
		minor:            surface.MinorForm(goVersion),
		toolchainVersion: toolchainVersion,
		hasToolchain:     hasToolchain,
		patchPinned:      surface.HasPatch(goVersion),
		stripToolchain:   hasToolchain && opts.StripToolchain,
	}

	if !plan.patchPinned && !plan.stripToolchain {
		return canonicalizePlan{skipReason: "directives already canonical"}
	}

	return plan
}

// CanonicalizeGoMod canonicalizes the toolchain directives of one go.mod:
//
//	go 1.26.7          ->  go 1.26
//	toolchain go1.26.7 ->  (removed, when StripToolchain is set)
//
// The go-line downgrade is dependency-floor aware: `go mod tidy -diff`
// (GOWORK=off) must come back clean against the staged candidate. A
// non-clean result means a dependency's go line (or pre-existing
// untidiness) forbids the downgrade; HeldBackByDepFloor is reported and the
// live file stays as it was: dep-forced floors are a legitimate state, not
// an error. The gate runs while the live go.mod is untouched, so nothing
// else reading the module ever sees the candidate before it is committed
// (BuildFlow#124).
//
// Unparseable go.mod files are Skipped, not failed: the go-line rewrite is
// regex-based, but the surrounding passes own the broken file. gate is the
// SplitRunner for the dependency gate; nil uses the production exec runner.
func CanonicalizeGoMod(
	ctx context.Context,
	goModPath string,
	opts CanonicalizeOptions,
	gate SplitRunner,
) (CanonicalizeResult, error) {
	if gate == nil {
		gate = ExecSplitRunner()
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return CanonicalizeResult{}, fmt.Errorf("canonicalize: read %s: %w", goModPath, err)
	}

	plan := planCanonicalization(content, opts)
	res := CanonicalizeResult{}

	if plan.skipReason != "" {
		res.Skipped, res.SkipReason = true, plan.skipReason

		return res, nil
	}

	dir := filepath.Dir(goModPath)

	plan, err = canonicalizeGuard(ctx, dir, opts, plan, gate, &res)
	if err != nil {
		return CanonicalizeResult{}, err
	}

	if res.Skipped {
		return res, nil
	}

	res.Changes = plan.changes()

	if opts.DryRun {
		res.HeldBack = true

		return res, nil
	}

	updated := string(content)

	if plan.patchPinned {
		updated, err = rewriteGoDirective(updated, string(plan.minor))
		if err != nil {
			return CanonicalizeResult{}, fmt.Errorf("canonicalize %s: %w", goModPath, err)
		}
	}

	if plan.stripToolchain {
		updated = stripToolchainDirective(updated)
	}

	verdict, gateErr := gateGoModRewrite(ctx, goModPath, content, []byte(updated), rewriteGates{
		dependencyFloor: plan.patchPinned,
		extra:           opts.ExtraGate,
	}, gate)
	if gateErr != nil {
		if errors.Is(gateErr, atomicwrite.ErrConcurrentModification) {
			res.Skipped, res.SkipReason = true,
				"go.mod changed by another writer while the gates ran; left untouched"

			return res, nil
		}

		return CanonicalizeResult{}, fmt.Errorf("canonicalize: %w", gateErr)
	}

	switch {
	case verdict.tidy.Dirty():
		res.HeldBackByDepFloor, res.GateDetail = true, verdict.tidy.Detail
	case verdict.extraErr != nil:
		res.HeldBackByExtraGate, res.GateDetail = true, verdict.extraErr.Error()
	default:
		res.Changed = true
	}

	return res, nil
}

// changes lists the planned rewrites in application order.
func (p canonicalizePlan) changes() []string {
	var changes []string

	if p.patchPinned {
		changes = append(changes, "go line "+string(p.goVersion)+" -> "+string(p.minor))
	}

	if p.stripToolchain {
		changes = append(changes, "remove toolchain directive "+string(p.toolchainVersion))
	}

	return changes
}

// canonicalizeGuard resolves the installed toolchain and returns the
// possibly amended plan: under RespectPatchFloor a patch pin the installed
// toolchain already satisfies is dropped from the plan (patchPinned=false;
// the rewrite is skipped as "patch floor respected" when nothing else
// remains), independently of the toolchain-strip path. Without the flag it
// only skips the whole rewrite when the module's minor floor exceeds the
// installed toolchain (the gate could not resolve anything anyway).
// Comparison is at full directive granularity; an unparseable installed
// version compares as "not exceeding", leaving the verdict to the gate.
func canonicalizeGuard(
	ctx context.Context,
	dir string,
	opts CanonicalizeOptions,
	plan canonicalizePlan,
	gate SplitRunner,
	res *CanonicalizeResult,
) (canonicalizePlan, error) {
	installed, err := resolveInstalledToolchain(ctx, dir, opts.InstalledToolchain, gate)
	if err != nil {
		return plan, err
	}

	if surface.GreaterVersion(string(surface.MinorForm(plan.goVersion)), string(surface.MinorForm(installed))) {
		res.Skipped, res.SkipReason = true, fmt.Sprintf(
			"go line %s exceeds installed toolchain %s; keeping patch floor",
			plan.goVersion, installed,
		)

		return plan, nil
	}

	if opts.RespectPatchFloor && plan.patchPinned &&
		!surface.GreaterVersion(string(plan.goVersion), string(installed)) {
		plan.patchPinned = false

		if !plan.stripToolchain {
			res.Skipped, res.SkipReason = true, fmt.Sprintf(
				"patch floor respected: go %s is at or below installed toolchain %s",
				plan.goVersion, installed,
			)
		}
	}

	return plan, nil
}

// resolveInstalledToolchain returns the installed Go version: the explicit
// option when set, otherwise a `go env GOVERSION` probe through the gate
// runner. The result keeps or drops the "go" prefix as written.
func resolveInstalledToolchain(
	ctx context.Context,
	dir string,
	explicit surface.GoVersion,
	gate SplitRunner,
) (surface.GoVersion, error) {
	if explicit != "" {
		return explicit, nil
	}

	stdout, _, err := gate(ctx, dir, "env", "GOVERSION")
	if err != nil {
		return "", fmt.Errorf("canonicalize: resolve installed toolchain: %w", err)
	}

	installed := surface.GoVersion(strings.TrimSpace(stdout))

	if installed == "" {
		return "", errEmptyGoVersion
	}

	return installed, nil
}

// InstalledToolchain resolves the installed Go version via `go env GOVERSION`
// run in dir. Providers use it to feed WithRespectPatchFloor once per run
// instead of probing per module. The "go" prefix is kept as the tool
// reports it ("go1.27.1"); callers tolerate either form.
func InstalledToolchain(ctx context.Context, dir string) (surface.GoVersion, error) {
	return resolveInstalledToolchain(ctx, dir, "", ExecSplitRunner())
}
