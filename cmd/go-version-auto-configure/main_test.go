package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRepo writes a minimal drifted repository.
func seedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "flake.nix"), []byte("{ x = pkgs.go_1_26; }\n"), 0o644))
	return root
}

func TestRun_VersionAndUsage(t *testing.T) {
	assert.Equal(t, 0, run([]string{"version"}))
	assert.Equal(t, 2, run(nil))
	assert.Equal(t, 2, run([]string{"nonsense"}))
}

func TestRun_CheckFindsDrift(t *testing.T) {
	root := seedRepo(t)

	exit := captureStdout(t, func() int { return run([]string{"check", root}) })
	assert.Equal(t, 1, exit, "check exits 1 when drift exists")
}

func TestRun_FixNormalizesAndCheckClears(t *testing.T) {
	root := seedRepo(t)

	exit := captureStdout(t, func() int { return run([]string{"fix", root}) })
	assert.Equal(t, 0, exit, "fix succeeds when every mechanical fix applies")

	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26\n")

	exit = captureStdout(t, func() int { return run([]string{"check", root}) })
	assert.Equal(t, 0, exit, "check is clean after fix (flake pin 1.26 equals floor 1.26)")
}

func TestRun_FixDryRunLeavesFiles(t *testing.T) {
	root := seedRepo(t)

	exit := captureStdout(t, func() int { return run([]string{"fix", "--dry-run", root}) })
	assert.Equal(t, 0, exit)

	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "go 1.26.7\n", "dry-run must not touch the file")
}

func captureStdout(t *testing.T, fn func() int) int {
	t.Helper()
	orig := os.Stdout
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)
	os.Stdout = devNull
	defer func() {
		os.Stdout = orig
		_ = devNull.Close()
	}()
	return fn()
}
