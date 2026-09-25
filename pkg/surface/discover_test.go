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

func issueRules(issues []Issue) []Rule {
	rules := make([]Rule, 0, len(issues))
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
		".github/workflows/ci.yml": "jobs:\n  lint:\n    steps:\n" +
			"      - uses: actions/setup-go@v5\n        with:\n          go-version: 1.26.7\n",
	})

	surf, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	require.Len(t, surf.Modules, 3, "vendor must be skipped, go.work counted")

	byPath := map[string]ModuleDirective{}
	for _, m := range surf.Modules {
		byPath[m.Path] = m
	}

	assert.Equal(t, GoVersion("1.26.7"), byPath["go.mod"].Version)
	assert.Equal(t, KindGoMod, byPath["go.mod"].Kind)
	assert.Equal(t, GoVersion("1.26"), byPath["sub/api/go.mod"].Version)
	assert.Equal(t, GoVersion("1.26.5"), byPath["go.work"].Version)
	assert.Equal(t, KindGoWork, byPath["go.work"].Kind)

	require.Len(t, surf.CIPins, 1)
	assert.Equal(t, GoVersion("1.26"), surf.CIPins[0].Version, "Version is the normalized floor")
	assert.Equal(t, GoVersion("1.26.7"), surf.CIPins[0].Raw, "Raw preserves the patch component")

	issues := Analyze(surf)
	rules := issueRules(issues)
	assert.Contains(t, rules, RuleGoDirectivePatchForm)
	assert.Contains(t, rules, RuleGoWorkBelowFloor,
		"go.work 1.26.5 is below the full module floor 1.26.7")
	assert.NotContains(t, rules, RuleWorkDirectivePatchForm,
		"the patch-form strip is not offered: the floor requires at least 1.26.7")
	assert.Contains(t, rules, RuleCIPinPatchForm)
	assert.NotContains(t, rules, RuleCIPinBelowFloor)

	var goModFix, workFix *Fix

	for _, issue := range issues {
		if issue.Rule == RuleGoDirectivePatchForm {
			require.NotNil(t, issue.Fix)
			goModFix = issue.Fix
		}

		if issue.Rule == RuleGoWorkBelowFloor {
			require.NotNil(t, issue.Fix)
			workFix = issue.Fix
		}
	}

	require.NotNil(t, goModFix)
	assert.Equal(t, "go.mod", goModFix.File)
	assert.Equal(t, GoVersion("1.26.7"), goModFix.From)
	assert.Equal(t, GoVersion("1.26"), goModFix.To)

	require.NotNil(t, workFix)
	assert.Equal(t, GoVersion("1.26.5"), workFix.From)
	assert.Equal(
		t,
		GoVersion("1.26.7"),
		workFix.To,
		"the below-floor fix raises to the full module floor",
	)
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

// TestDiscoverAndAnalyze_NixPinInCommentIgnored pins the regression found on
// go-health 2026-09-25: a comment explaining why the pin IS go_1_27 mentions
// go_1_26 and was reported as the pin itself (phantom nix-pin-below-floor).
func TestDiscoverAndAnalyze_NixPinInCommentIgnored(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod": "module example.com/root\n\ngo 1.27\n",
		"flake.nix": "{\n" +
			"  # GOTOOLCHAIN=local cannot satisfy the directive under go_1_26,\n" +
			"  # so the toolchain is nixpkgs' go_1_27.\n" +
			"  goPkg = pkgs.go_1_27;\n" +
			"}\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	require.Len(t, s.NixPins, 1, "only the real pin counts; comment mentions are not pins")
	assert.Equal(t, GoVersion("1.27"), s.NixPins[0].Version)
	assert.Equal(t, 4, s.NixPins[0].Line)

	assert.Empty(t, Analyze(s), "clean: the real pin meets the floor")
}

// TestDiscoverAndAnalyze_NixPinBlockCommentAndStrings covers block comments
// spanning lines (pins inside ignored, code after the close kept) and `#`
// inside a double-quoted string (not a comment start; the rest of the line
// stays scannable).
func TestDiscoverAndAnalyze_NixPinBlockCommentAndStrings(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"go.mod": "module example.com/root\n\ngo 1.27\n",
		"flake.nix": "{\n" +
			"  /* block comment start\n" +
			"     mentions go_1_26 and buildGo126Module\n" +
			"  end */ goPkg = pkgs.go_1_27;\n" +
			"  url = \"https://example.com/a#fragment\"; other = pkgs.go_1_27;\n" +
			"}\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	require.Len(t, s.NixPins, 2, "pins after a closed block comment and past an in-string # both count")
	assert.Equal(t, GoVersion("1.27"), s.NixPins[0].Version)
	assert.Equal(t, 4, s.NixPins[0].Line)
	assert.Equal(t, GoVersion("1.27"), s.NixPins[1].Version)
	assert.Equal(t, 5, s.NixPins[1].Line)

	assert.Empty(t, Analyze(s))
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

func TestAnalyze_GoWorkBelowFloorRestoresFullFloor(t *testing.T) {
	t.Parallel()

	// go-finding shape: the root module is dep-forced at go 1.27.1 while
	// go.work sits below it. The fix must restore the FULL patch floor
	// (1.27.1): raising only to the minor form (1.27) would leave the
	// workspace unable to resolve its own modules ("module X listed in
	// go.work file requires go >= 1.27.1, but go.work lists go 1.27").
	// This is the go-finding 2026-09-20 outage shape (BuildFlow gotcha
	// #169) reproduced as a rule.
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
		if issue.Rule == RuleGoWorkBelowFloor {
			require.NotNil(t, issue.Fix)
			workFix = issue.Fix
		}
	}

	require.NotNil(t, workFix)
	assert.Equal(t, GoVersion("1.26.7"), workFix.From)
	assert.Equal(
		t,
		GoVersion("1.27.1"),
		workFix.To,
		"go.work must cover the full patch floor, not just the minor",
	)
}

func TestAnalyze_GoWorkPatchFormRequiredByFloorIsSilent(t *testing.T) {
	t.Parallel()

	// A dep-forced module floor of go 1.27.1 REQUIRES go.work to carry the
	// patch: stripping it to go 1.27 would invalidate the workspace. That
	// state is correct, so neither go.work rule may report it. The module's
	// own patch form still fires (fleet policy: major.minor only); the
	// tidy-dep-forced classification is the repairer's concern.
	root := writeRepo(t, map[string]string{
		"go.mod":  "module example.com/root\n\ngo 1.27.1\n",
		"go.work": "go 1.27.1\n\nuse .\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	rules := issueRules(Analyze(s))
	assert.NotContains(t, rules, RuleWorkDirectivePatchForm)
	assert.NotContains(t, rules, RuleGoWorkBelowFloor)
}

func TestSurface_FullModuleFloor(t *testing.T) {
	t.Parallel()

	s := &Surface{Modules: []ModuleDirective{
		{Path: "a/go.mod", Kind: KindGoMod, Version: "1.26"},
		{Path: "b/go.mod", Kind: KindGoMod, Version: "1.27.1"},
		{Path: "c/go.mod", Kind: KindGoMod, Version: "1.27"},
		{Path: "go.work", Kind: KindGoWork, Version: "1.28.9"},
	}}

	floor, ok := s.FullModuleFloor()
	require.True(t, ok)
	assert.Equal(t, GoVersion("1.27.1"), floor,
		"the full floor is the max module directive at patch granularity, go.work excluded")

	_, ok = (&Surface{}).FullModuleFloor()
	assert.False(t, ok)
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
			assert.Equal(t, GoVersion("1.26"), issue.Fix.To)
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

	assert.Equal(t, GoVersion("go1.26.7"), byPath["go.mod"].Version, "Version keeps the go prefix")
	assert.Equal(t, KindGoMod, byPath["go.mod"].Kind)
	assert.Equal(t, GoVersion("go1.25.2"), byPath["go.work"].Version)
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

func TestFixDescribe(t *testing.T) {
	t.Parallel()

	fx := Fix{Kind: KindGoMod, File: "go.mod", From: "1.26.7", To: "1.26"}
	assert.Equal(t, "rewrite go.mod directive in go.mod: go 1.26.7 → go 1.26", fx.Describe())
}

func TestAnalyze_ToolchainDefaultIsSurfaced(t *testing.T) {
	t.Parallel()

	// `toolchain default` is valid per the go tool but names no explicit
	// toolchain version: it must surface as informational instead of being
	// silently excluded from floor analysis.
	root := writeRepo(t, map[string]string{
		"go.mod": "module example.com/root\n\ngo 1.26\n\ntoolchain default\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	issues := Analyze(s)
	require.Len(t, issues, 1)
	assert.Equal(t, RuleToolchainNonVersion, issues[0].Rule)
	assert.Equal(t, 5, issues[0].Line, "toolchain is on line 5")
	assert.Empty(
		t,
		issues[0].Suggestion,
		"a non-version toolchain is informational, not actionable",
	)
}

func TestDiscover_ToolchainLocalMakesGoWorkUnparseable(t *testing.T) {
	t.Parallel()

	// The go tool rejects `toolchain local` ("must match format go1.23.0
	// or default"), so the whole go.work fails to parse. That must surface
	// as a discovery issue, not silently drop the file's go directive.
	root := writeRepo(t, map[string]string{
		"go.mod":  "module example.com/root\n\ngo 1.26\n",
		"go.work": "go 1.26\n\ntoolchain local\n\nuse .\n",
	})

	_, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	require.Len(t, discoverIssues, 1)
	assert.Equal(t, RuleGoWorkUnparseable, discoverIssues[0].Rule)
	assert.Equal(t, "go.work", discoverIssues[0].File)
	assert.Contains(t, discoverIssues[0].Message, "invalid toolchain version 'local'")
}

func TestAnalyze_ExpectMinorFlagsSurfacesAbovePolicy(t *testing.T) {
	t.Parallel()

	// ADR-0001 shape: the fleet minor is 1.27. A directive, toolchain, and
	// flake pin at 1.28 exceed the policy; a 1.27 directive with a patch
	// (dep-forced) and the aligned 1.27 pin stay silent.
	root := writeRepo(t, map[string]string{
		"go.mod":    "module example.com/m\n\ngo 1.28.1\n\ntoolchain go1.28.3\n",
		"flake.nix": "{ x = pkgs.go_1_28; }\n",
	})

	surf, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	opt, optErr := WithExpectedMinor("1.27")
	require.NoError(t, optErr)

	hits := map[Rule]int{}
	notes := map[Rule]string{}

	for _, issue := range Analyze(surf, opt) {
		hits[issue.Rule]++

		if notes[issue.Rule] == "" {
			notes[issue.Rule] = issue.Message
		}
	}

	assert.Equal(
		t,
		3,
		hits[RuleMinorExceedsExpectation],
		"directive, toolchain, and flake pin all exceed 1.27 at minor granularity",
	)

	rules := issueRules(Analyze(surf, opt))
	assert.Contains(t, rules, RuleMinorExceedsExpectation)
	assert.NotContains(
		t,
		notes[RuleMinorExceedsExpectation],
		"1.27.",
		"the aligned 1.27 flake pin never exceeds the policy",
	)
	assert.Contains(t, notes[RuleMinorExceedsExpectation], "1.28")
}

func TestAnalyze_ExpectMinorBelowFloorStillFires(t *testing.T) {
	t.Parallel()

	// The enforcement direction: a repo living on go 1.28 while the fleet
	// policy is 1.27 is flagged even though every pin is aligned, because
	// the expectation is absolute policy, not a floor comparison.
	root := writeRepo(t, map[string]string{
		"go.mod":    "module example.com/m\n\ngo 1.28\n",
		"flake.nix": "{ x = pkgs.go_1_28; }\n",
	})

	surf, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	opt, optErr := WithExpectedMinor("1.27")
	require.NoError(t, optErr)

	fired := 0

	for _, issue := range Analyze(surf, opt) {
		if issue.Rule == RuleMinorExceedsExpectation {
			fired++
		}
	}

	assert.Equal(t, 2, fired, "directive and flake pin both sit above the policy minor")
}

func TestWithExpectedMinor_RejectsNonVersions(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"banana", "1", "one.two"} {
		_, err := WithExpectedMinor(bad)
		require.Error(t, err, "value %q must be rejected", bad)
		assert.Contains(t, err.Error(), bad, "the error names the offending value")
	}
}

func TestAnalyze_GoWorkPatchRequiredByZeroPatchFloorIsSilent(t *testing.T) {
	t.Parallel()

	// The go tool ranks the bare minor BELOW every patch form of the same
	// minor: go 1.26 does not cover a module floor of go 1.26.0 (probed
	// 2026-09-22: "module m listed in go.work file requires go >= 1.26.0,
	// but go.work lists go 1.26"). So a go.work at go 1.26.7 covering a
	// module at go 1.26.0 must NOT be offered a strip to go 1.26, and it
	// is not below-floor either (1.26.7 covers 1.26.0).
	root := writeRepo(t, map[string]string{
		"go.mod":  "module example.com/m\n\ngo 1.26.0\n",
		"go.work": "go 1.26.7\n\nuse .\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	rules := issueRules(Analyze(s))
	assert.NotContains(
		t,
		rules,
		RuleWorkDirectivePatchForm,
		"stripping go.work to go 1.26 would break the workspace against the 1.26.0 module floor",
	)
	assert.NotContains(t, rules, RuleGoWorkBelowFloor, "go 1.26.7 covers go 1.26.0")
}

func TestAnalyze_GoWorkBelowFloorFiresOnZeroPatchGap(t *testing.T) {
	t.Parallel()

	// Same go-tool ranking, other direction: go.work at the bare minor
	// under a module floor at .0 patch IS broken and gets the mechanical
	// restoration to the FULL floor.
	root := writeRepo(t, map[string]string{
		"go.mod":  "module example.com/m\n\ngo 1.26.0\n",
		"go.work": "go 1.26\n\nuse .\n",
	})

	s, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	issues := Analyze(s)

	var workFix *Fix

	for _, issue := range issues {
		if issue.Rule == RuleGoWorkBelowFloor {
			require.NotNil(t, issue.Fix)
			workFix = issue.Fix
		}
	}

	require.NotNil(t, workFix, "go 1.26 does not cover the module floor go 1.26.0 under the go tool's rules")
	assert.Equal(t, GoVersion("1.26.0"), workFix.To, "the fix restores the FULL patch floor")
}

func TestReadLines(t *testing.T) {
	t.Parallel()

	t.Run("normal file returns lines", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "go.mod")
		require.NoError(t, os.WriteFile(path, []byte("module m\n\ngo 1.26\n"), 0o644))

		assert.Equal(t, []string{"module m", "", "go 1.26", ""}, readLines(path))
	})

	t.Run("unreadable path yields nil without panicking", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, readLines(filepath.Join(t.TempDir(), "missing.txt")))
	})

	t.Run("directory yields nil without panicking", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, readLines(t.TempDir()))
	})
}
