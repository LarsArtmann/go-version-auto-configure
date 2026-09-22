package fix

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner records invocations and rewrites files instead of shelling out
// to go. rewrite receives the working dir and the go args; when it returns
// output, that output is handed to the caller (used for `go list`).
func fakeRunner(rewrite func(dir string, args []string) (string, error)) GoCommandRunner {
	return func(_ context.Context, dir string, args ...string) (string, error) {
		return rewrite(dir, args)
	}
}

// fakeGate adapts a directory-keyed function into a SplitRunner for the
// dependency gate.
func fakeGate(fn func(dir string, args []string) (string, string, error)) SplitRunner {
	return func(_ context.Context, dir string, args ...string) (string, string, error) {
		return fn(dir, args)
	}
}

// noopGate is the always-clean dependency gate: exit 0, no diff.
func noopGate() SplitRunner {
	return fakeGate(func(string, []string) (string, string, error) { return "", "", nil })
}

func TestApply_DryRunHoldsBack(t *testing.T) {
	t.Parallel()

	fixes := []surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}}

	res, err := Apply(context.Background(), t.TempDir(), fixes, Options{DryRun: true}, nil)
	require.NoError(t, err)
	require.Len(t, res.HeldBack, 1)
	assert.Empty(t, res.Applied)
}

func TestApply_VerificationCatchesNoOpEdit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	// The file drifted since detection: the fix targets 1.26 but the
	// directive on disk is still 1.26.7. Rewriting anyway would silently
	// apply a different change than the one Analyze recorded.
	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26", To: "1.25", Line: 3}},
		Options{},
		nil,
	)
	require.NoError(t, err, "a failed fix is reported in the result, not as an error")
	assert.Empty(t, res.Applied)
	require.Len(t, res.Failures, 1)
	assert.Contains(t, res.Failures[0].Cause, "changed since detection: got go 1.26.7, want go 1.26")
}

func TestApply_SuccessWhenFileActuallyChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	original := "module example.com/m\n\ngo 1.26.7\n\nrequire foo v1.0.0 // kept byte-for-byte\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o644))

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{Gate: noopGate()},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)
	assert.Empty(t, res.Failures)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26\n")
	assert.Contains(t, string(data), "// kept byte-for-byte",
		"the rewrite is byte-preserving text surgery, not a modfile reformat")
}

func TestApply_DepForcedFloorIsNamed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goModPath := filepath.Join(root, "go.mod")
	original := "module example.com/m\n\ngo 1.26.7\n"
	require.NoError(t, os.WriteFile(goModPath, []byte(original), 0o644))

	run := fakeRunner(func(string, []string) (string, error) {
		return "example.com/m (devel) 1.26\n" +
			"github.com/larsartmann/go-finding v1.10.0 1.26.7\n" +
			"github.com/x/other v1.0.0 1.25\n", nil
	})

	// The gate rejects the downgrade: tidy -diff wants changes (the
	// dependency floor re-raises the directive).
	gate := fakeGate(func(string, []string) (string, string, error) {
		return "\ndiff of what tidy would change\n", "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{Gate: gate},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.DepForced, 1)
	assert.Empty(t, res.Applied)

	d := res.DepForced[0]
	assert.Equal(t, surface.GoVersion("1.26.7"), d.Floor)
	assert.Equal(
		t,
		[]string{"github.com/larsartmann/go-finding"},
		d.Poisoners,
		"only the published dep whose floor matches is named; the (devel) main module is not",
	)

	data, readErr := os.ReadFile(goModPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data), "a rejected fix is reverted, not left half-applied")

	report := res.Report()
	assert.Contains(t, report, "dep-forced 1")
	assert.Contains(t, report, "forced by: github.com/larsartmann/go-finding")
	assert.Contains(t, report, "re-tag")
}

func TestApply_GoWorkEdit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workPath := filepath.Join(root, "go.work")
	require.NoError(t, os.WriteFile(workPath, []byte("go 1.26.5\n\nuse .\n"), 0o644))

	run := fakeRunner(func(dir string, args []string) (string, error) {
		if len(args) >= 2 && args[1] == "edit" {
			require.Equal(t, "work", args[0])
			require.NoError(t, os.WriteFile(filepath.Join(dir, "go.work"), []byte("go 1.26\n\nuse .\n"), 0o644))
		}

		return "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.work", Kind: surface.KindGoWork, From: "1.26.5", To: "1.26", Line: 1}},
		Options{},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)
}

func TestApply_SubdirectoryModuleRunsInItsDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := filepath.Join("sub", "api", "go.mod")
	abs := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte("module example.com/sub/api\n\ngo 1.27.1\n"), 0o644))

	var gotDir string

	// The dependency gate is the only subprocess a go.mod fix needs: it
	// must run inside the module's own directory, never the repo root.
	gate := fakeGate(func(dir string, _ []string) (string, string, error) {
		gotDir = dir

		return "", "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: rel, Kind: surface.KindGoMod, From: "1.27.1", To: "1.27", Line: 3}},
		Options{Gate: gate},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)
	assert.Equal(t, filepath.Join(root, "sub", "api"), gotDir)
}

// errFakeGo is the canned go failure used in runner fakes.
var errFakeGo = errors.New("go: exit 1")

func TestApply_RunnerErrorBecomesFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goModPath := filepath.Join(root, "go.mod")
	original := "module example.com/m\n\ngo 1.26.7\n"
	require.NoError(t, os.WriteFile(goModPath, []byte(original), 0o644))

	run := fakeRunner(func(string, []string) (string, error) { return "", errFakeGo })
	gate := fakeGate(func(string, []string) (string, string, error) {
		return "", "", errFakeGo
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{Gate: gate},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.Failures, 1)
	assert.Contains(t, res.Failures[0].Cause, "exit 1")

	data, readErr := os.ReadFile(goModPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data), "a failed fix restores the original content")
}

func TestApply_UnknownKindFails(t *testing.T) {
	t.Parallel()

	run := fakeRunner(func(string, []string) (string, error) { return "", nil })
	res, err := Apply(
		context.Background(),
		t.TempDir(),
		[]surface.Fix{{File: "x", Kind: "weird", From: "1", To: "2", Line: 1}},
		Options{},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.Failures, 1)
	assert.Contains(t, res.Failures[0].Cause, "unknown directive kind")
}

func TestReport_Empty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "no fixes", (*Result)(nil).Report())
	assert.True(t, strings.HasPrefix((&Result{}).Report(), "applied 0"))
}
