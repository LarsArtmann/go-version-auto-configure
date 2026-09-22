package fix

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleScoped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"mod", "edit", "-go=1.26"}, want: true},
		{args: []string{"list", "-m", "all"}, want: true},
		{args: []string{"work", "edit", "-go=1.26"}, want: false},
		{args: nil, want: false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, moduleScoped(tt.args), "args: %v", tt.args)
	}
}

func TestDepForcedErrorMessage(t *testing.T) {
	t.Parallel()

	withPoisoners := &DepForcedError{
		Floor:     "1.26.7",
		Poisoners: []string{"github.com/larsartmann/go-finding", "github.com/cespare/xxhash"},
	}
	msg := withPoisoners.Error()
	assert.Contains(t, msg, "go 1.26.7")
	assert.Contains(t, msg, "go-finding")
	assert.Contains(t, msg, "re-tagging")

	withoutPoisoners := &DepForcedError{Floor: "1.27"}
	assert.Contains(
		t,
		withoutPoisoners.Error(),
		"a dependency forces this floor",
		"unresolved poisoners still name the forced floor",
	)

	withCause := &DepForcedError{Floor: "1.27", Cause: "poisoner resolution failed: boom"}
	assert.Equal(t, "poisoner resolution failed: boom", withCause.Error())
}

func TestReport_RendersEverySection(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "no fixes", (*Result)(nil).Report())

	dryRunFix := surface.Fix{
		Kind: surface.KindGoWork,
		File: "go.work",
		From: "1.26.7",
		To:   "1.26",
		Line: 1,
	}
	res := &Result{
		Applied: []surface.Fix{
			{Kind: surface.KindGoMod, File: "go.mod", From: "1.26.7", To: "1.26", Line: 3},
		},
		DepForced: []DepForced{
			{
				Fix: surface.Fix{
					Kind: surface.KindGoMod,
					File: "sub/go.mod",
					From: "1.26.7",
					To:   "1.26",
					Line: 3,
				},
				Floor:     "1.26.7",
				Poisoners: []string{"github.com/larsartmann/go-finding"},
			},
			{
				Fix: surface.Fix{
					Kind: surface.KindGoMod,
					File: "other/go.mod",
					From: "1.26.7",
					To:   "1.26",
					Line: 3,
				},
				Floor: "1.27",
			},
		},
		HeldBack: []surface.Fix{dryRunFix},
		Failures: []Failure{
			{Fix: dryRunFix, Cause: "verify after edit: directive mismatch"},
		},
	}

	report := res.Report()
	assert.Contains(t, report, "applied 1, dep-forced 2, held back 1, failed 1")
	assert.Contains(t, report, "ok:         rewrite go.mod directive")
	assert.Contains(t, report, "dep-forced: rewrite go.mod directive in sub/go.mod")
	assert.Contains(t, report, "floor go 1.26.7 is forced by: github.com/larsartmann/go-finding")
	assert.Contains(t, report, "re-tag those modules")
	assert.Contains(t, report, "poisoner resolution unavailable")
	assert.Contains(t, report, "dry-run:    rewrite go.work directive")
	assert.Contains(
		t,
		report,
		"FAILED:     rewrite go.work directive in go.work: go 1.26.7 → go 1.26: verify after edit: directive mismatch",
	)
}

func TestSelfCheck_FindsLocalToolchain(t *testing.T) {
	t.Parallel()

	require.NoError(
		t,
		SelfCheck(context.Background()),
		"the test environment always has a go binary",
	)
}

// TestApplyOne_RealTool runs one directive rewrite through the production
// runners against a dependency-free temp module: it covers the text
// surgery, re-parse verification, and the tidy -diff gate (clean for a
// dependency-free module) without network access.
func TestApplyOne_RealTool(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	require.NoError(t, os.WriteFile(goMod, []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644))

	fx := surface.Fix{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}
	require.NoError(t, applyOne(context.Background(), root, fx, EditRunner(), ExecSplitRunner()))

	data, err := os.ReadFile(goMod)
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26\n")
}

// TestEditRunner_WrapsFailure verifies the production runner reports command
// failures with the command and its output, and that a missing go binary
// surfaces as a wrapped error.
func TestEditRunner_WrapsFailure(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go binary in PATH")
	}

	run := EditRunner()

	_, err := run(context.Background(), t.TempDir(), "mod", "edit", "-go=not.a.version")
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "go mod edit"), "the error names the command")

	// A directory without a go.mod makes `go mod edit` fail: the failure
	// carries the command output for diagnosis.
	_, err = run(context.Background(), t.TempDir(), "mod", "tidy")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go.mod", "the error carries the go tool's output")
}

// TestEditRunner_WorkEditNeedsWorkspace covers the documented asymmetry:
// `go work edit` requires workspace discovery, so GOWORK=off must NOT be set
// for work commands (policy: dispatch on args[0]).
func TestEditRunner_WorkEditNeedsWorkspace(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go binary in PATH")
	}

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.26.7\n\nuse .\n"), 0o644),
	)

	run := EditRunner()

	_, err := run(context.Background(), root, "work", "edit", "-go=1.26")
	require.NoError(t, err, "work commands must run with workspace discovery enabled")

	data, err := os.ReadFile(filepath.Join(root, "go.work"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26\n")
}

func TestApply_UnknownDirectiveKindFails(t *testing.T) {
	t.Parallel()

	res, err := Apply(
		context.Background(),
		t.TempDir(),
		[]surface.Fix{{File: "go.mod", Kind: "weird", From: "1.26.7", To: "1.26", Line: 3}},
		Options{},
		fakeRunner(func(_ string, _ []string) (string, error) { return "", nil }),
	)
	require.NoError(t, err)
	require.Len(t, res.Failures, 1)
	assert.NotEmpty(t, res.Failures[0].Cause, "failure carries a cause")
}

func TestModuleScopedExtraEnv(t *testing.T) {
	t.Run("local pin is normalized to auto", func(t *testing.T) {
		t.Setenv("GOTOOLCHAIN", "local")
		assert.Equal(t, []string{"GOTOOLCHAIN=auto"}, moduleScopedExtraEnv())
	})

	t.Run("unset is normalized to auto", func(t *testing.T) {
		t.Setenv("GOTOOLCHAIN", "")
		assert.Equal(t, []string{"GOTOOLCHAIN=auto"}, moduleScopedExtraEnv())
	})

	t.Run("explicit non-local pin is inherited untouched", func(t *testing.T) {
		t.Setenv("GOTOOLCHAIN", "go1.27.1")
		assert.Nil(t, moduleScopedExtraEnv())
	})
}

// TestModuleScopedListSurvivesLocalOlderShell verifies the WP-I scenario from
// the 2026-09-22 plan end to end: a parent shell pinned to an OLDER local
// toolchain must not stop `go list -m` from reading a module whose floor is
// newer. The child gets GOTOOLCHAIN=auto, resolves the newer toolchain (cached
// in this environment), and reports the true floor. Skipped when no newer
// toolchain can be resolved (offline or unpinned CI without the cache).
func TestModuleScopedListSurvivesLocalOlderShell(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary unavailable")
	}

	t.Setenv("GOTOOLCHAIN", "local")

	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("go.mod", "module example.com/floorcheck\n\ngo 1.27\n")
	write("main.go", "package main\n\nfunc main() {}\n")

	run := EditRunner()
	out, err := run(context.Background(), dir, "list", "-m")
	require.NoError(t, err, "module-scoped go list must survive a local older shell: %s", out)
	assert.Contains(t, out, "example.com/floorcheck")
}
