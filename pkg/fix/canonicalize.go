// Canonicalization of one go.mod's toolchain directives: the byte-preserving
// counterpart to surface.Fix application. Where Apply rewrites one directive
// recorded by Analyze, CanonicalizeGoMod derives the changes itself from the
// file (patch-form go line, toolchain line) and guards the downgrade with a
// `go mod tidy -diff` dependency-floor gate, reverting atomically when the
// floor forbids it.
package fix

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
			cmd.Env = append(os.Environ(), "GOWORK=off")
		}

		var stdoutBuf, stderrBuf strings.Builder

		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		err = cmd.Run()

		return stdoutBuf.String(), stderrBuf.String(), err
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

// runTidyDiffGate runs `go mod tidy -diff` with GOWORK=off in dir. A clean
// result is exit 0 with empty stdout; a diff on stdout or a non-zero exit
// (floor conflict, resolve error) means the edit is not dependency-floor
// safe. Stderr is informational only.
func runTidyDiffGate(ctx context.Context, dir string, run SplitRunner) gateResult {
	gateCtx, cancel := context.WithTimeout(ctx, tidyGateTimeout)
	defer cancel()

	stdout, stderr, err := run(gateCtx, dir, "mod", "tidy", "-diff")

	return gateResult{
		Stdout: stdout,
		Detail: strings.TrimSpace(strings.TrimSpace(stdout) + "\n" + strings.TrimSpace(stderr)),
		Err:    err,
	}
}

// goDirectiveLine matches a top-level go directive line, capturing the
// version token and any trailing content (e.g. a comment). Anchored to line
// start so module paths like "go-something" inside require blocks cannot
// match.
//
//nolint:gochecknoglobals // compiled once, read-only
var goDirectiveLine = regexp.MustCompile(`(?m)^go[ \t]+(\S+)(.*)$`)

// toolchainDirectiveLine matches a full toolchain directive line including
// its trailing newline, so removal does not leave a blank line behind.
//
//nolint:gochecknoglobals // compiled once, read-only
var toolchainDirectiveLine = regexp.MustCompile(`(?m)^toolchain[ \t]+\S+[^\n]*\n?`)

// rewriteGoDirective rewrites the go directive to the given target version.
// Text-level surgery preserves the rest of the file byte-for-byte;
// `go mod edit` would reflow the whole file and churn unrelated lines.
func rewriteGoDirective(content, target string) (string, error) {
	loc := goDirectiveLine.FindStringSubmatchIndex(content)
	if loc == nil {
		return "", fmt.Errorf("no go directive found to rewrite (target go %s)", target)
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
	// line rewrite. Stripping needs no gate of its own (toolchain
	// directives do not participate in module resolution); a
	// downgrade+strip combination reverts together when the gate fails.
	StripToolchain bool
	// InstalledToolchain is the installed Go version, e.g. "1.27.1" (the
	// "go" prefix is tolerated). Empty resolves it via `go env GOVERSION`.
	InstalledToolchain surface.GoVersion
}

// CanonicalizeResult summarizes one CanonicalizeGoMod run.
type CanonicalizeResult struct {
	// Changed reports whether the file was rewritten.
	Changed bool
	// Changes describes each applied rewrite, e.g. "go line 1.26.7 -> 1.26".
	Changes []string
	// HeldBack reports a dry-run: Changes lists what would be applied.
	HeldBack bool
	// HeldBackByDepFloor reports that the gate rejected the downgrade and
	// the original content was restored: a dependency forces the floor.
	HeldBackByDepFloor bool
	// GateDetail carries the `go mod tidy -diff` output for a rejected
	// downgrade.
	GateDetail string
	// Skipped reports that nothing was attempted; SkipReason says why.
	Skipped bool
	// SkipReason explains a Skipped result, e.g. an unparseable go.mod or
	// a go line above the installed toolchain. Empty when Skipped is false.
	SkipReason string
}

// CanonicalizeGoMod canonicalizes the toolchain directives of one go.mod:
//
//	go 1.26.7          ->  go 1.26
//	toolchain go1.26.7 ->  (removed, when StripToolchain is set)
//
// The go-line downgrade is dependency-floor aware: after the edit, `go mod
// tidy -diff` (GOWORK=off) must come back clean. A non-clean result means a
// dependency's go line (or pre-existing untidiness) forbids the downgrade;
// the original content is restored atomically and HeldBackByDepFloor is
// reported: dep-forced floors are a legitimate state, not an error.
//
// Unparseable go.mod files are Skipped, not failed: the go-line rewrite is
// regex-based, but the surrounding passes own the broken file. gate is the
// SplitRunner for the dependency gate; nil uses the production exec runner.
func CanonicalizeGoMod(ctx context.Context, goModPath string, opts CanonicalizeOptions, gate SplitRunner) (CanonicalizeResult, error) {
	if gate == nil {
		gate = ExecSplitRunner()
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return CanonicalizeResult{}, fmt.Errorf("canonicalize: read %s: %w", goModPath, err)
	}

	dir := filepath.Dir(goModPath)
	res := CanonicalizeResult{}

	goVersion, _, parseErr := surface.ParseDirective(surface.KindGoMod, content)
	switch {
	case errors.Is(parseErr, surface.ErrNoDirective):
		res.Skipped, res.SkipReason = true, "no go directive"

		return res, nil
	case parseErr != nil:
		res.Skipped, res.SkipReason = true, fmt.Sprintf("unparseable go.mod: %v", parseErr)

		return res, nil
	}

	toolchainVersion, _, toolErr := surface.ParseToolchain(surface.KindGoMod, content)
	hasToolchain := toolErr == nil && toolchainVersion != ""

	if !surface.HasPatch(goVersion) && !(hasToolchain && opts.StripToolchain) {
		res.Skipped, res.SkipReason = true, "directives already canonical"

		return res, nil
	}

	installed, err := resolveInstalledToolchain(ctx, dir, opts.InstalledToolchain, gate)
	if err != nil {
		return CanonicalizeResult{}, err
	}

	// The installed toolchain guards pointless rewrites: when the module's
	// minor floor exceeds it, the gate could not resolve anything anyway.
	// Comparison at minor granularity; an unparseable installed version
	// compares as "not exceeding", leaving the verdict to the gate.
	if surface.GreaterVersion(string(surface.MinorForm(goVersion)), string(surface.MinorForm(installed))) {
		res.Skipped, res.SkipReason = true, fmt.Sprintf(
			"go line %s exceeds installed toolchain %s; keeping patch floor",
			goVersion, installed,
		)

		return res, nil
	}

	minor := surface.MinorForm(goVersion)

	if surface.HasPatch(goVersion) {
		res.Changes = append(res.Changes, "go line "+string(goVersion)+" -> "+string(minor))
	}

	if hasToolchain && opts.StripToolchain {
		res.Changes = append(res.Changes, "remove toolchain directive "+string(toolchainVersion))
	}

	if opts.DryRun {
		res.HeldBack = true

		return res, nil
	}

	updated := string(content)

	if surface.HasPatch(goVersion) {
		updated, err = rewriteGoDirective(updated, string(minor))
		if err != nil {
			return CanonicalizeResult{}, fmt.Errorf("canonicalize %s: %w", goModPath, err)
		}
	}

	if hasToolchain && opts.StripToolchain {
		updated = stripToolchainDirective(updated)
	}

	if writeErr := atomicwrite.Write(goModPath, []byte(updated)); writeErr != nil {
		return CanonicalizeResult{}, fmt.Errorf("canonicalize: write %s: %w", goModPath, writeErr)
	}

	// Only the go-line downgrade can invalidate module resolution, so only
	// it pays for the gate; a toolchain strip alone needs none. The gate
	// covers the combination: a failure reverts the whole file.
	if surface.HasPatch(goVersion) {
		g := runTidyDiffGate(ctx, dir, gate)
		if g.Dirty() {
			if revertErr := atomicwrite.Write(goModPath, content); revertErr != nil {
				return CanonicalizeResult{}, fmt.Errorf(
					"canonicalize %s: downgrade was not dependency-floor-safe AND revert failed: %w",
					goModPath, revertErr,
				)
			}

			res.HeldBackByDepFloor = true
			res.GateDetail = g.Detail

			return res, nil
		}
	}

	res.Changed = true

	return res, nil
}

// resolveInstalledToolchain returns the installed Go version: the explicit
// option when set, otherwise a `go env GOVERSION` probe through the gate
// runner. The result keeps or drops the "go" prefix as written.
func resolveInstalledToolchain(ctx context.Context, dir string, explicit surface.GoVersion, gate SplitRunner) (surface.GoVersion, error) {
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
