package fix

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeWorkspaceFixture materializes a go.work plus module go.mods from
// minor-line maps and returns the root.
func writeWorkspaceFixture(t *testing.T, goWork string, moduleGoLines map[string]string) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte(goWork), 0o644))

	for dirName, goLine := range moduleGoLines {
		dir := filepath.Join(root, dirName)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
			[]byte("module example.com/"+dirName+"\n\ngo "+goLine+"\n"), 0o644))
	}

	return root
}

func TestSyncGoWorkDirectives_RestoresDowngradedFloor(t *testing.T) {
	t.Parallel()

	// BuildFlow gotcha #169 shape: a workspace-mode `go get` rewrote the
	// go.work directive to the minor form while a module is dep-forced at
	// the patch. The sync must restore the FULL patch floor (the S82
	// regression): go.work 1.27 does not cover a module at 1.27.1.
	root := writeWorkspaceFixture(t, "go 1.27\n\nuse ./a\n", map[string]string{
		"a": "1.27.1",
	})

	res, err := SyncGoWorkDirectives(context.Background(), root, Options{}, nil)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)

	data, readErr := os.ReadFile(filepath.Join(root, "go.work"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "go 1.27.1",
		"the floor must be restored at full patch granularity")
}

func TestSyncGoWorkDirectives_StripsPatchWhenFloorSafe(t *testing.T) {
	t.Parallel()

	root := writeWorkspaceFixture(t, "go 1.26.7\n\nuse ./a\n\nuse ./b\n", map[string]string{
		"a": "1.26",
		"b": "1.26",
	})

	res, err := SyncGoWorkDirectives(context.Background(), root, Options{}, nil)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)

	data, readErr := os.ReadFile(filepath.Join(root, "go.work"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "go 1.26\n")
	assert.NotContains(t, string(data), "1.26.7")
}

func TestSyncGoWorkDirectives_ModuleFloorBlocksPatchStrip(t *testing.T) {
	t.Parallel()

	// go.work at 1.26.7 with a module dep-forced at 1.26.7: stripping to
	// 1.26 would invalidate the workspace, so no fix may be offered at
	// all (the patch form is required, not a violation).
	original := "go 1.26.7\n\nuse ./a\n"
	root := writeWorkspaceFixture(t, original, map[string]string{
		"a": "1.26.7",
	})

	res, err := SyncGoWorkDirectives(context.Background(), root, Options{}, nil)
	require.NoError(t, err)
	assert.Empty(t, res.Applied)
	assert.Empty(t, res.HeldBack)

	data, readErr := os.ReadFile(filepath.Join(root, "go.work"))
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data))
}

func TestSyncGoWorkDirectives_NoWorkYieldsEmptyResult(t *testing.T) {
	t.Parallel()

	res, err := SyncGoWorkDirectives(context.Background(), t.TempDir(), Options{}, nil)
	require.NoError(t, err)
	assert.Empty(t, res.Applied)
}

func TestSyncGoWorkDirectives_DryRunHoldsBack(t *testing.T) {
	t.Parallel()

	original := "go 1.27\n\nuse ./a\n"
	root := writeWorkspaceFixture(t, original, map[string]string{
		"a": "1.27.1",
	})

	res, err := SyncGoWorkDirectives(context.Background(), root, Options{DryRun: true}, nil)
	require.NoError(t, err)
	require.Len(t, res.HeldBack, 1)
	assert.Empty(t, res.Applied)

	data, readErr := os.ReadFile(filepath.Join(root, "go.work"))
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data))
}

func TestSyncGoWorkDirectives_HealthyFloorIsNoOp(t *testing.T) {
	t.Parallel()

	original := "go 1.27.1\n\nuse ./a\n"
	root := writeWorkspaceFixture(t, original, map[string]string{
		"a": "1.27.1",
	})

	res, err := SyncGoWorkDirectives(context.Background(), root, Options{}, nil)
	require.NoError(t, err)
	assert.Empty(t, res.Applied)

	data, readErr := os.ReadFile(filepath.Join(root, "go.work"))
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data))
}
