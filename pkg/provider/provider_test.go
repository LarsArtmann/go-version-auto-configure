package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-finding"
	toolsdk "github.com/larsartmann/go-finding/toolsdk"
	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// TestProviderDeclaresFullVersionSurface pins the read/write surface every
// file class Discover dispatches on must be declared: Inputs (bridge-derived
// from ConfigFiles) so BuildFlow re-runs this tool on flake.nix, CI-pin,
// VERSION and CHANGELOG.md drift — the under-trigger regression the
// library-utilization run named its top finding — and Outputs so hosts
// order producers/consumers around the repair write contract.
func TestProviderDeclaresFullVersionSurface(t *testing.T) {
	t.Parallel()

	expectedInputs := []string{
		"go.mod",
		"go.work",
		"flake.nix",
		".github/workflows/*.yml",
		".github/workflows/*.yaml",
		"VERSION",
		"CHANGELOG.md",
	}
	assert.ElementsMatch(t, expectedInputs, Provider.Inputs,
		"Inputs must cover every file class surface.Discover reads")

	expectedOutputs := []string{
		"go.mod",
		"go.work",
		".github/workflows/*.yml",
		".github/workflows/*.yaml",
	}
	assert.ElementsMatch(t, expectedOutputs, Provider.Outputs,
		"Outputs must cover exactly the files fix.Apply rewrites")

	for _, readOnly := range []string{"flake.nix", "VERSION", "CHANGELOG.md"} {
		assert.NotContains(t, Provider.Outputs, readOnly,
			"%s is a read-only surface; the tool never moves that pin", readOnly)
	}
}

// TestDetect_FlakeOnlyDrift exercises the under-trigger scenario end to end
// at the provider boundary: a repo whose ONLY drift is a flake.nix pin
// trailing the module floor still reports through Detect, the path BuildFlow
// calls once Inputs gates it in.
func TestDetect_FlakeOnlyDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644),
	)
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "flake.nix"),
		[]byte("{ pkgs }: pkgs.mkShell { buildInputs = [ (pkgs.go_1_25) ]; }\n"),
		0o644,
	))

	ctx := finding.WithWorkingDir(context.Background(), root)
	findings, err := Provider.Detect.Detect(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, findings, "flake-only drift must not be silent")

	rules := map[string]bool{}
	for _, f := range findings {
		rules[string(f.Rule)] = true
	}

	assert.Contains(t, rules, "nix-pin-below-floor")
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

// TestRepairStructured_FactsAndModifiedFiles pins the sdk-v0.9.0 provenance
// channel (plan M7b): the repair result names the rewritten file and stamps
// the run's counts as Facts, so hosts measure repairs without parsing the
// prose description.
func TestRepairStructured_FactsAndModifiedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	result, err := Provider.Repair.Repair(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"go.mod"}, result.ModifiedFiles)

	facts := make(map[string]string, len(result.Facts))
	for _, fact := range result.Facts {
		facts[fact.Key] = fact.Value
	}

	assert.Equal(t, "1", facts["applied-count"], "exactly the go.mod rewrite applied")
	assert.Equal(t, "false", facts["dry-run"])
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

	structured := make(map[string]string, len(result.Facts))
	for _, fact := range result.Facts {
		structured[fact.Key] = fact.Value
	}

	assert.Equal(t, "true", structured["dry-run"], "the Facts channel reports dry-run too")
	assert.Empty(t, result.ModifiedFiles, "dry-run rewrites nothing, so no ModifiedFiles")
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

// TestProvider_DetectV2CarriesCoverage pins the sdk-v0.9.0 evidence channel
// (plan M7b): RunDetect prefers DetectV2, and the registered detector now
// reports what it scanned, so a host can prove a clean answer.
func TestProvider_DetectV2CarriesCoverage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644))

	ctx := finding.WithWorkingDir(context.Background(), root)
	result, err := finding.RunDetect(ctx, Provider.Detect)
	require.NoError(t, err)
	assert.Positive(t, result.FilesScanned, "the fixture's go.mod was scanned")
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

// TestRespectPatchFloorOption_WrongTypeErrors pins the loud-misconfig
// contract: a wrong-typed option value (e.g. the string "true" from a
// config typo) is an error, never a silent fall-through to the default
// policy — the raw map assertion this replaces treated it as unset.
func TestRespectPatchFloorOption_WrongTypeErrors(t *testing.T) {
	t.Parallel()

	ctx := toolsdk.WithOptions(context.Background(), toolsdk.OptionValues{
		providerOptionRespectPatchFloor: "true",
	})

	_, err := respectPatchFloorOptions(ctx, ".")
	require.Error(t, err)
	require.ErrorIs(t, err, toolsdk.ErrOptionKindMismatch)
	assert.Contains(t, err.Error(), providerOptionRespectPatchFloor,
		"the error must name the misconfigured option")
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
