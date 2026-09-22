package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-finding"
	toolsdk "github.com/larsartmann/go-finding/toolsdk"
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
