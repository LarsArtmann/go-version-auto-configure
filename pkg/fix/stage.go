package fix

// Staging for gated go.mod rewrites. A rewrite whose verdict comes from the
// go command (`go mod tidy -diff`, a caller's gate) must not be judged on
// the live file: written first and reverted on rejection, the candidate is
// visible to everything else reading the module meanwhile (BuildFlow#124:
// concurrent go generate and govalid fail with "updates to go.mod needed"
// while the normalizer probes a downgrade, and the auto-commit daemon can
// commit the candidate). The candidate is staged into a private copy of the
// module's mod files instead, the go command reads that copy through
// -modfile, and the live go.mod is written exactly once, after every gate
// approves.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	atomicwrite "github.com/larsartmann/go-atomic-write"
)

// stageDirPattern is the os.MkdirTemp pattern of a staged copy's directory.
const stageDirPattern = "go-version-auto-configure-stage-"

// ExtraGate is a caller-supplied check on the candidate content of one
// module's go.mod, run before the rewrite is committed: dir is the module
// directory, modFile the staged copy carrying the candidate, meant for `go
// <check> -modfile=modFile` run in dir. A non-nil error withholds the
// rewrite. Go refuses -modfile in workspace mode, so a gate that shells out
// sets GOWORK=off.
type ExtraGate func(ctx context.Context, dir, modFile string) error

// stagedGoMod is a private copy of one module's go.mod and go.sum carrying
// the candidate rewrite, outside the module directory. The go command takes
// it through -modfile: the module root stays the module's own directory
// (packages resolve from there, relative replace directives stay relative
// to it), only the mod files are swapped.
type stagedGoMod struct {
	// dir is the temp directory owning the copies.
	dir string
	// modPath is the candidate go.mod.
	modPath string
	// sumPath is the copied go.sum; empty when the module has none.
	sumPath string
}

// rewriteGates names the checks a candidate go.mod must pass before commit.
type rewriteGates struct {
	// dependencyFloor runs `go mod tidy -diff` on the candidate. Only a
	// go-line downgrade can invalidate module resolution, so only it pays.
	dependencyFloor bool
	// extra is the caller's gate on the candidate; nil runs none.
	extra ExtraGate
}

// rewriteVerdict reports the gates' judgement of one candidate go.mod.
type rewriteVerdict struct {
	// tidy is the dependency-floor gate's output; zero when it did not run.
	tidy gateResult
	// extraErr is the caller gate's error; nil when it approved or did not run.
	extraErr error
}

// rejected reports whether any gate refused the candidate.
func (v rewriteVerdict) rejected() bool {
	return v.tidy.Dirty() || v.extraErr != nil
}

// gateGoModRewrite stages candidate (a rewrite of goModPath's original
// content), runs the requested gates against the staged copy, and commits
// it to goModPath exactly once when every gate approves. A rejected
// candidate never reaches the live file. The commit is verified: the live
// go.mod changing while the gates ran means the verdict was earned for
// content that no longer exists, so the write is refused with
// atomicwrite.ErrConcurrentModification and the other writer's file stands.
func gateGoModRewrite(
	ctx context.Context,
	goModPath string,
	original, candidate []byte,
	gates rewriteGates,
	run SplitRunner,
) (rewriteVerdict, error) {
	staged, err := stageGoMod(goModPath, candidate)
	if err != nil {
		return rewriteVerdict{}, err
	}

	defer func() { _ = staged.Close() }()

	verdict := rewriteVerdict{}
	dir := filepath.Dir(goModPath)

	if gates.dependencyFloor {
		if verdict.tidy = staged.tidyDiffGate(ctx, dir, run); verdict.tidy.Dirty() {
			return verdict, nil
		}
	}

	if gates.extra != nil {
		if verdict.extraErr = gates.extra(ctx, dir, staged.modPath); verdict.extraErr != nil {
			return verdict, nil
		}
	}

	if err = atomicwrite.WriteVerified(
		goModPath, candidate, atomicwrite.FingerprintFromBytes(original),
	); err != nil {
		return rewriteVerdict{}, fmt.Errorf("commit %s: %w", goModPath, err)
	}

	return verdict, nil
}

// stageGoMod writes candidate as the staged go.mod next to a copy of the
// module's go.sum (when the module has one). Nothing is written into the
// module directory.
func stageGoMod(goModPath string, candidate []byte) (*stagedGoMod, error) {
	goSumPath := filepath.Join(filepath.Dir(goModPath), "go.sum")

	// A module without go.sum (ErrNotExist) has nothing to copy; liveSum
	// then stays nil.
	liveSum, err := os.ReadFile(goSumPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", goSumPath, err)
	}

	dir, err := os.MkdirTemp("", stageDirPattern)
	if err != nil {
		return nil, fmt.Errorf("create stage directory for %s: %w", goModPath, err)
	}

	staged := &stagedGoMod{dir: dir, modPath: filepath.Join(dir, "go.mod")}

	if err = os.WriteFile(staged.modPath, candidate, 0o600); err != nil {
		_ = staged.Close()

		return nil, fmt.Errorf("stage candidate %s: %w", goModPath, err)
	}

	if liveSum != nil {
		staged.sumPath = filepath.Join(dir, "go.sum")

		if err = os.WriteFile(staged.sumPath, liveSum, 0o600); err != nil {
			_ = staged.Close()

			return nil, fmt.Errorf("stage %s: %w", goSumPath, err)
		}
	}

	return staged, nil
}

// Close removes the staged copies. A missing directory is not an error.
func (s *stagedGoMod) Close() error {
	return os.RemoveAll(s.dir)
}

// tidyDiffGate runs `go mod tidy -diff` with GOWORK=off in dir against the
// staged mod files. A clean result is exit 0 with empty stdout; a diff on
// stdout or a non-zero exit (floor conflict, resolve error) means the
// candidate is not dependency-floor safe. Stderr is informational only:
// tidy prints the diff on stdout while benign notices ("all" matched no
// packages) land on stderr with exit 0, so only stdout may mark the gate
// dirty. Nothing is written: -diff prints what tidy would change and leaves
// both staged files alone.
func (s *stagedGoMod) tidyDiffGate(ctx context.Context, dir string, run SplitRunner) gateResult {
	gateCtx, cancel := context.WithTimeout(ctx, tidyGateTimeout)
	defer cancel()

	stdout, stderr, err := run(gateCtx, dir, "mod", "tidy", "-diff", "-modfile="+s.modPath)

	return gateResult{
		Stdout: stdout,
		Detail: strings.TrimSpace(strings.TrimSpace(stdout) + "\n" + strings.TrimSpace(stderr)),
		Err:    err,
	}
}
