package surface

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRepo materializes a temp repository from a map of relative path to
// file content and returns its root.
func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	}

	return root
}

func issueRules(issues []Issue) []string {
	rules := make([]string, 0, len(issues))
	for _, issue := range issues {
		rules = append(rules, issue.Rule)
	}

	return rules
}

func TestDiscoverAndAnalyze_PatchFormAcrossModules(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod":                      "module example.com/root\n\ngo 1.26.7\n",
		"sub/api/go.mod":              "module example.com/sub/api\n\ngo 1.26\n",
		"vendor/example.com/x/go.mod": "module x\n\ngo 1.21\n",
		"go.work":                     "go 1.26.5\n\nuse .\n\tuse ./sub/api\n",
		".github/workflows/ci.yml":    "jobs:\n  lint:\n    steps:\n      - uses: actions/setup-go@v5\n        with:\n          go-version: 1.26.7\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	require.Len(t, s.Modules, 3, "vendor must be skipped, go.work counted")

	byPath := map[string]ModuleDirective{}
	for _, m := range s.Modules {
		byPath[m.Path] = m
	}

	assert.Equal(t, "1.26.7", byPath["go.mod"].Version)
	assert.Equal(t, KindGoMod, byPath["go.mod"].Kind)
	assert.Equal(t, "1.26", byPath["sub/api/go.mod"].Version)
	assert.Equal(t, "1.26.5", byPath["go.work"].Version)
	assert.Equal(t, KindGoWork, byPath["go.work"].Kind)

	require.Len(t, s.CIPins, 1)
	assert.Equal(t, "1.26", s.CIPins[0].Version, "Version is the normalized floor")
	assert.Equal(t, "1.26.7", s.CIPins[0].Raw, "Raw preserves the patch component")

	issues := Analyze(s)
	rules := issueRules(issues)
	assert.Contains(t, rules, RuleGoDirectivePatchForm)
	assert.Contains(t, rules, RuleWorkDirectivePatchForm)
	assert.Contains(t, rules, RuleCIPinPatchForm)
	assert.NotContains(t, rules, RuleCIPinBelowFloor)

	var goModFix *Fix

	for _, issue := range issues {
		if issue.Rule == RuleGoDirectivePatchForm {
			require.NotNil(t, issue.Fix)
			goModFix = issue.Fix
		}
	}

	require.NotNil(t, goModFix)
	assert.Equal(t, "go.mod", goModFix.File)
	assert.Equal(t, "1.26.7", goModFix.From)
	assert.Equal(t, "1.26", goModFix.To)
}

func TestDiscoverAndAnalyze_NixPinBelowFloor(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod":    "module example.com/root\n\ngo 1.27\n",
		"flake.nix": "{\n  buildGoModule = pkgs.go_1_26;\n  x = buildGo126Module;\n}\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	floor, ok := s.Floor()
	require.True(t, ok)
	assert.Equal(t, "1.27", floor.String())
	require.Len(t, s.NixPins, 2)

	issues := Analyze(s)
	require.Len(t, issues, 2, "both the attribute and the builder pin are below the floor")

	for _, issue := range issues {
		assert.Equal(t, RuleNixPinBelowFloor, issue.Rule)
		assert.Nil(t, issue.Fix, "alignment issues are suggest-only")
		assert.NotEmpty(t, issue.Suggestion)
	}
}

func TestDiscoverAndAnalyze_CIPinBelowFloor(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod":                   "module example.com/root\n\ngo 1.26\n",
		".github/workflows/ci.yml": "steps:\n  - uses: actions/setup-go@v5\n    with:\n      go-version: 1.23\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	issues := Analyze(s)
	require.Len(t, issues, 1)
	assert.Equal(t, RuleCIPinBelowFloor, issues[0].Rule)
	assert.Equal(t, ".github/workflows/ci.yml", issues[0].File)
	assert.Equal(t, 4, issues[0].Line, "go-version is on line 4")
	assert.NotEmpty(t, issues[0].Suggestion)
}

func TestDiscoverAndAnalyze_CleanSurface(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod":                   "module example.com/root\n\ngo 1.26\n",
		"go.work":                  "go 1.26\n\nuse .\n",
		"flake.nix":                "{ buildGoModule = pkgs.go_1_26; }\n",
		".github/workflows/ci.yml": "with:\n  go-version: 1.26\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)
	assert.Empty(t, Analyze(s), "aligned surface must produce zero issues")
}

func TestDiscover_UnparseableGoModBecomesIssue(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod":        "module example.com/root\n\ngo 1.26\n",
		"broken/go.mod": "this is not a go.mod at all {{{",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Len(t, s.Modules, 1)
	require.Len(t, discoverIssues, 1)
	assert.Equal(t, RuleGoModUnparseable, discoverIssues[0].Rule)
	assert.Equal(t, filepath.Join("broken", "go.mod"), discoverIssues[0].File)
}

func TestDiscover_MissingRoot(t *testing.T) {
	t.Parallel()

	_, _, err := Discover(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}

func TestFloor_NoModules(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"README.md": "nothing here\n",
	})
	s, _, err := Discover(root)
	require.NoError(t, err)

	_, ok := s.Floor()
	assert.False(t, ok)
	assert.Empty(t, Analyze(s))
}

func TestAnalyze_GoWorkTargetRespectsWorkspaceFloor(t *testing.T) {
	t.Parallel()

	// go-finding shape: root module drifted to 1.27 while go.work still
	// carries a patch of an older minor. The go.work rewrite must land on
	// the workspace floor (1.27), not the stripped directive (1.26) — the
	// latter would leave the workspace unable to resolve its own modules.
	root := writeRepo(t, map[string]string{
		"go.mod":  "module example.com/root\n\ngo 1.27.1\n",
		"go.work": "go 1.26.7\n\nuse .\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	issues := Analyze(s)

	var workFix *Fix

	for _, issue := range issues {
		if issue.Rule == RuleWorkDirectivePatchForm {
			require.NotNil(t, issue.Fix)
			workFix = issue.Fix
		}
	}

	require.NotNil(t, workFix)
	assert.Equal(t, "1.26.7", workFix.From)
	assert.Equal(t, "1.27", workFix.To, "go.work must cover the workspace floor")
}

func TestAnalyze_GoWorkTargetIsDirectiveWhenAboveFloor(t *testing.T) {
	t.Parallel()

	// All modules at 1.26, go.work carrying a patch: the rewrite strips to
	// 1.26 — the directive minor, not the floor, is what it already is.
	root := writeRepo(t, map[string]string{
		"go.mod":  "module example.com/root\n\ngo 1.26\n",
		"go.work": "go 1.26.7\n\nuse .\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	issues := Analyze(s)
	for _, issue := range issues {
		if issue.Rule == RuleWorkDirectivePatchForm {
			require.NotNil(t, issue.Fix)
			assert.Equal(t, "1.26", issue.Fix.To)
		}
	}
}

func TestDiscover_ToolchainDirectives(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod":       "module example.com/root\n\ngo 1.26\n\ntoolchain go1.26.7\n",
		"go.work":      "go 1.26\n\ntoolchain go1.25.2\n\nuse .\n",
		"plain/go.mod": "module example.com/plain\n\ngo 1.26\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	require.Len(t, s.Toolchains, 2, "only files declaring a toolchain are recorded")

	byPath := map[string]ToolchainDirective{}

	for _, tc := range s.Toolchains {
		byPath[tc.Path] = tc
	}

	assert.Equal(t, "go1.26.7", byPath["go.mod"].Version, "Version keeps the go prefix")
	assert.Equal(t, KindGoMod, byPath["go.mod"].Kind)
	assert.Equal(t, "go1.25.2", byPath["go.work"].Version)
	assert.Equal(t, KindGoWork, byPath["go.work"].Kind)
}

func TestAnalyze_ToolchainRaisesPinFloor(t *testing.T) {
	t.Parallel()

	// The go floor is 1.26, but toolchain go1.27.1 makes builds switch
	// toolchains: the flake pin go_1_26 now trails the effective floor.
	root := writeRepo(t, map[string]string{
		"go.mod":    "module example.com/root\n\ngo 1.26\n\ntoolchain go1.27.1\n",
		"flake.nix": "{ buildGoModule = pkgs.go_1_26; }\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	issues := Analyze(s)
	require.Len(t, issues, 1)
	assert.Equal(t, RuleNixPinBelowFloor, issues[0].Rule)
	assert.Nil(t, issues[0].Fix, "alignment issues are suggest-only")
	assert.Contains(t, issues[0].Message, "toolchain go1.27.1")
	assert.Contains(t, issues[0].Message, "1.27")
	assert.Contains(t, issues[0].Suggestion, "go_1_27")
}

func TestAnalyze_ToolchainBelowDirectiveIsStale(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod": "module example.com/root\n\ngo 1.27\n\ntoolchain go1.25.0\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	issues := Analyze(s)
	require.Len(t, issues, 1)
	assert.Equal(t, RuleToolchainBelowDirective, issues[0].Rule)
	assert.Equal(t, 5, issues[0].Line, "toolchain is on line 5")
	assert.Contains(t, issues[0].Suggestion, "go mod edit -toolchain=none")
}

func TestAnalyze_CurrentToolchainNotFlagged(t *testing.T) {
	t.Parallel()

	// toolchain go1.26.7 above go 1.26 is the normal post-`go get` shape:
	// it pins a patch within the same minor and must stay silent.
	root := writeRepo(t, map[string]string{
		"go.mod": "module example.com/root\n\ngo 1.26\n\ntoolchain go1.26.7\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)
	assert.Empty(t, Analyze(s))
}
