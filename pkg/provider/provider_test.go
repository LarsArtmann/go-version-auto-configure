package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-finding"
	toolsdk "github.com/larsartmann/go-finding/toolsdk"
	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	autoconfigure "github.com/larsartmann/linter-autoconfigure-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errUnreachableListing is the canned listing failure proving the
// annotation path never runs where it must not.
var errUnreachableListing = errors.New("go: exit 1: unreachable")

// errFakeListFailure is the canned go list failure for degraded-annotation
// fixtures.
var errFakeListFailure = errors.New("go: exit 1: missing go.sum entry")

func TestProviderRegistersItself(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, Provider.Name)
	assert.Equal(t, toolName, Provider.Name)
	assert.NotNil(t, Provider.Detect, "the provider must detect")
	assert.NotNil(t, Provider.Repair, "the provider must repair")
	assert.Equal(t, "go", Provider.Trigger.Language, "trigger must scope to Go projects")
	assert.Contains(t, Provider.Trigger.Files, "go.mod")

	all := toolsdk.All()
	found := false

	for _, spec := range all {
		if spec.Name == toolName {
			found = true
		}
	}

	assert.True(t, found, "Provider must be in the process-global registry for BuildFlow discovery")
}

func TestDetect_ReportsPatchForm(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, findings)

	rules := map[string]bool{}

	for _, f := range findings {
		assert.Equal(t, finding.ToolName(toolName), f.ToolName, "every finding is stamped with the tool name")
		rules[string(f.Rule)] = true
	}

	assert.Contains(t, rules, "go-directive-patch-form")
}

func TestDetect_CleanRepoIsEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644))

	ctx := finding.WithWorkingDir(context.Background(), root)
	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

func TestDetect_ReportsUnparseableGoWork(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644),
	)
	// `toolchain local` (rejected by the go tool) or any unparseable go
	// line must surface as a finding, never vanish silently.
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.work"), []byte("go not.a.version\n\nuse .\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, findings)

	rules := map[string]bool{}

	for _, f := range findings {
		rules[string(f.Rule)] = true
	}

	assert.Contains(t, rules, "go-work-unparseable")
}

func TestRepair_RewritesDirective(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	require.NoError(t, os.WriteFile(path, []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644))

	ctx := finding.WithWorkingDir(context.Background(), root)
	result, err := Provider.Repair.Repair(ctx)
	require.NoError(t, err)
	assert.Contains(t, result.Description, "go.mod")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26\n", "repair must normalize the directive to major.minor")
}

func TestRepair_DryRunLeavesFileUntouched(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "go.mod")
	require.NoError(t, os.WriteFile(path, []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644))

	ctx := finding.WithWorkingDir(context.Background(), root)
	ctx = toolsdk.WithDryRun(ctx, true)
	result, err := Provider.Repair.Repair(ctx)
	require.NoError(t, err)
	assert.Contains(t, result.Description, "dry-run")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26.7\n", "dry-run must not modify the file")
}

func TestDetect_ReportsStaleToolchain(t *testing.T) {
	t.Parallel()

	// Provider tests predate toolchain modeling: a toolchain below the same
	// file's go directive must surface as toolchain-below-directive.
	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(root, "go.mod"),
			[]byte("module example.com/m\n\ngo 1.27\n\ntoolchain go1.25.0\n"),
			0o644,
		),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)

	rules := map[string]bool{}
	for _, f := range findings {
		rules[string(f.Rule)] = true
	}

	assert.Contains(t, rules, "toolchain-below-directive")
}

func TestDetect_ToolchainRaisingFloorFlagsTrailingNixPin(t *testing.T) {
	t.Parallel()

	// A toolchain naming a newer minor raises the effective floor: a flake
	// pinned to the go directive's minor trails it and must be flagged.
	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(root, "go.mod"),
			[]byte("module example.com/m\n\ngo 1.26\n\ntoolchain go1.27.1\n"),
			0o644,
		),
	)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "flake.nix"), []byte("{ x = pkgs.go_1_26; }\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)

	rules := map[string]bool{}
	for _, f := range findings {
		rules[string(f.Rule)] = true
	}

	assert.Contains(t, rules, "nix-pin-below-floor")
}

func TestRepair_CleanRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644))

	ctx := finding.WithWorkingDir(context.Background(), root)
	result, err := Provider.Repair.Repair(ctx)
	require.NoError(t, err)
	assert.Contains(t, result.Description, "no mechanical")
}

// probeInstalledVersion resolves the test environment's installed Go
// version (full form, e.g. "1.27.1") through the fix package's probe so the
// option tests build fixtures the local toolchain satisfies.
func probeInstalledVersion(t *testing.T) string {
	t.Helper()

	installed, err := fix.InstalledToolchain(context.Background(), t.TempDir())
	require.NoError(t, err, "the test environment always has a go binary")

	return strings.TrimPrefix(string(installed), "go")
}

func TestProviderDeclaresRespectPatchFloorOption(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, Provider.Options, "the spec must declare its tool options")

	for _, opt := range Provider.Options {
		if opt.Name != providerOptionRespectPatchFloor {
			continue
		}

		assert.Equal(t, toolsdk.OptionKindBool, opt.Kind)
		assert.Equal(t, false, opt.Default)

		return
	}

	t.Fatalf("spec does not declare option %q", providerOptionRespectPatchFloor)
}

// TestDetect_RespectPatchFloorOption pins the DAG-side plumbing end to end:
// with the tool option set through the options context, a patch pin at or
// below the INSTALLED toolchain stops being a finding. The fixture pins the
// installed minor at patch .0, so the pin is at or below it on every host
// that satisfies the module floor.
func TestDetect_RespectPatchFloorOption(t *testing.T) {
	t.Parallel()

	installed := probeInstalledVersion(t)
	pin := string(surface.MinorForm(surface.GoVersion(installed))) + ".0"

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo "+pin+"\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	ctx = toolsdk.WithOptions(ctx, toolsdk.OptionValues{providerOptionRespectPatchFloor: true})

	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)

	for _, f := range findings {
		assert.NotEqual(t, "go-directive-patch-form", string(f.Rule),
			"pin %s is at or below installed %s and must be respected", pin, installed)
	}
}

// TestDetect_RespectPatchFloorOptionUnsetStillFlags pins the default: the
// same fixture without the option keeps reporting the patch form, so the
// policy change is opt-in per repo.
func TestDetect_RespectPatchFloorOptionUnsetStillFlags(t *testing.T) {
	t.Parallel()

	installed := probeInstalledVersion(t)
	pin := string(surface.MinorForm(surface.GoVersion(installed))) + ".0"

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo "+pin+"\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)

	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)

	rules := map[string]bool{}
	for _, f := range findings {
		rules[string(f.Rule)] = true
	}

	assert.True(t, rules["go-directive-patch-form"], "without the option the pin stays drift")
}

// fakeFloorsList answers every `go list` with the given records, one per
// line in the GoVersion/Path/Version tab format the floor resolution
// requests; the production path shells out through fix.EditRunner instead.
func fakeFloorsList(out string) fix.GoCommandRunner {
	return func(_ context.Context, _ string, _ ...string) (string, error) {
		return out, nil
	}
}

// failingFloorsList rejects every listing: the dep-forced annotation must
// degrade to a plain finding, never an error.
func failingFloorsList(err error) fix.GoCommandRunner {
	return func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", err
	}
}

// requirePatchFormIssue returns the single go-directive-patch-form issue.
func requirePatchFormIssue(
	t *testing.T,
	issues []autoconfigure.ConfigIssue,
) autoconfigure.ConfigIssue {
	t.Helper()

	for _, issue := range issues {
		if string(issue.Rule) == "go-directive-patch-form" {
			return issue
		}
	}

	t.Fatal("no go-directive-patch-form issue")

	return autoconfigure.ConfigIssue{}
}

// TestAnalyze_HeldPatchFormExplainsItself pins the BuildFlow-facing answer
// to "why was this not auto-fixed": the finding names the dependencies
// holding the floor and suggests the supply-side remediation instead of a
// rewrite tidy would silently revert.
func TestAnalyze_HeldPatchFormExplainsItself(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := fakeFloorsList(strings.Join([]string{
		"1.26.7\texample.com/m\t(devel)",
		"1.26.7\tgithub.com/x/dep\tv1.0.0",
	}, "\n"))

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, run)
	require.NoError(t, err)

	issue := requirePatchFormIssue(t, issues)
	assert.Contains(t, issue.Message, "not auto-fixed: go mod tidy reverts the strip")
	assert.Contains(t, issue.Message, "github.com/x/dep@v1.0.0")
	assert.Contains(t, issue.Suggestion, "re-tag github.com/x/dep@v1.0.0")
	assert.NotContains(t, issue.Suggestion, "rewrite", "the withheld rewrite must not pose as the fix")
	assert.Equal(t, finding.SeverityWarning, issue.Severity, "advisory findings stay warning-severity")
}

func TestAnalyze_HeldPoisonedPatchFormNamesForcer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := fakeFloorsList(strings.Join([]string{
		"1.26.7\texample.com/m\t(devel)",
		"1.27.1\tgithub.com/x/dep\tv1.0.0",
	}, "\n"))

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, run)
	require.NoError(t, err)

	issue := requirePatchFormIssue(t, issues)
	assert.Contains(t, issue.Message, "dependencies force go 1.27.1 above the declared directive")
	assert.Contains(t, issue.Message, "github.com/x/dep@v1.0.0 (go 1.27.1)")
	assert.Contains(t, issue.Suggestion, "go mod edit -go=1.27.1")
}

func TestAnalyze_UnheldPatchFormKeepsRewriteSuggestion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := fakeFloorsList(strings.Join([]string{
		"1.26.7\texample.com/m\t(devel)",
		"1.26\tgithub.com/x/dep\tv1.0.0",
	}, "\n"))

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, run)
	require.NoError(t, err)

	issue := requirePatchFormIssue(t, issues)
	assert.NotContains(t, issue.Message, "not auto-fixed", "nothing holds the strip, the fix applies")
	assert.Contains(t, issue.Suggestion, "rewrite", "the mechanical rewrite stays the suggestion")
}

func TestAnalyze_FloorListingFailureKeepsPlainFinding(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, failingFloorsList(errFakeListFailure))
	require.NoError(t, err, "a failed enrichment must not red detection")

	issue := requirePatchFormIssue(t, issues)
	assert.NotContains(t, issue.Message, "not auto-fixed")
	assert.Contains(t, issue.Suggestion, "rewrite")
}

func TestAnalyze_CleanRepoNeverListsFloors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, failingFloorsList(errUnreachableListing))
	require.NoError(t, err)
	assert.Empty(t, issues, "a clean repo pays nothing for the annotation path")
}
