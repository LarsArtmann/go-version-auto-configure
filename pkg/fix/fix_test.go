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

// scriptRunner dispatches on the go subcommand, simulating tidy behavior.
type scriptRunner struct {
	dir     string
	onEdit  func(dir string) // after `mod edit` / `work edit`
	tidy    func(dir string) // after `mod tidy`; may rewrite the directive back
	listOut string           // output for `go list`
}

func (s *scriptRunner) call(_ context.Context, dir string, args ...string) (string, error) {
	switch {
	case len(args) >= 2 && args[0] == "mod" && args[1] == "edit":
		s.onEdit(dir)
	case len(args) >= 2 && args[0] == "work" && args[1] == "edit":
		s.onEdit(dir)
	case len(args) >= 2 && args[0] == "mod" && args[1] == "tidy":
		if s.tidy != nil {
			s.tidy(dir)
		}
	case len(args) >= 1 && args[0] == "list":
		return s.listOut, nil
	}

	return "", nil
}

func rewriteGoMod(dir, directive string) {
	path := filepath.Join(dir, "go.mod")
	content := "module example.com/m\n\ngo " + directive + "\n"
	_ = os.WriteFile(path, []byte(content), 0o644)
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

	run := fakeRunner(func(dir string, args []string) (string, error) {
		if len(args) >= 2 && args[1] == "tidy" {
			return "", nil
		}

		rewriteGoMod(dir, "1.26.7") // edit leaves the file untouched

		return "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{},
		run,
	)
	require.NoError(t, err, "a failed fix is reported in the result, not as an error")
	assert.Empty(t, res.Applied)
	require.Len(t, res.Failures, 1)
	assert.Contains(t, res.Failures[0].Cause, "directive mismatch: got go 1.26.7, want go 1.26")
}

func TestApply_SuccessWhenFileActuallyChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	require.NoError(t, os.WriteFile(path, []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644))

	run := fakeRunner(func(dir string, args []string) (string, error) {
		if len(args) >= 1 && args[0] == "mod" {
			rewriteGoMod(dir, "1.26")
		}

		return "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)
	assert.Empty(t, res.Failures)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26\n")
}

func TestApply_DepForcedFloorIsNamed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := &scriptRunner{
		dir:    root,
		onEdit: func(dir string) { rewriteGoMod(dir, "1.26") },
		tidy:   func(dir string) { rewriteGoMod(dir, "1.26.7") }, // tidy re-poisons
		listOut: "example.com/m (devel) 1.26.7\n" +
			"github.com/larsartmann/go-finding v1.10.0 1.26.7\n" +
			"github.com/x/other v1.0.0 1.25\n",
	}

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{},
		run.call,
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
		"only the dep whose floor matches is named",
	)

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

	run := fakeRunner(func(dir string, args []string) (string, error) {
		if len(args) >= 2 && args[1] == "edit" {
			gotDir = dir
			rewriteGoMod(dir, "1.27")
		}

		if len(args) >= 2 && args[1] == "tidy" {
			rewriteGoMod(dir, "1.27")
		}

		return "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: rel, Kind: surface.KindGoMod, From: "1.27.1", To: "1.27", Line: 3}},
		Options{},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)
	assert.Equal(t, filepath.Join(root, "sub", "api"), gotDir)
}

// errFakeGo is the canned go failure used in runner fakes.
var errFakeGo = errors.New("go: exit 1")

func TestApply_RunnerErrorBecomesFailure(t *testing.T) {
	t.Parallel()

	run := fakeRunner(func(_ string, _ []string) (string, error) { return "", errFakeGo })

	res, err := Apply(
		context.Background(),
		t.TempDir(),
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.Failures, 1)
	assert.Contains(t, res.Failures[0].Cause, "exit 1")
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
