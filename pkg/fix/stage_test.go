package fix

// Staged mod files: the candidate lives outside the module directory, the
// module's go.sum travels with it, and Close leaves nothing behind.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dirEntryNames lists a directory's entry names in os.ReadDir order.
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}

func TestStageGoMod_CopiesSumAndLeavesModuleDirectoryAlone(t *testing.T) {
	t.Parallel()

	module := t.TempDir()
	goModPath := filepath.Join(module, "go.mod")
	liveSum := "example.com/dep v1.0.0 h1:abc=\n"

	require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/m\n\ngo 1.27.1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(module, "go.sum"), []byte(liveSum), 0o644))

	before := dirEntryNames(t, module)
	candidate := []byte("module example.com/m\n\ngo 1.27\n")

	staged, err := stageGoMod(goModPath, candidate)
	require.NoError(t, err)

	assert.Equal(t, before, dirEntryNames(t, module), "nothing is staged inside the module directory")
	assert.NotEqual(t, module, staged.dir)

	stagedMod, readErr := os.ReadFile(staged.modPath)
	require.NoError(t, readErr)
	assert.Equal(t, string(candidate), string(stagedMod))

	require.NotEmpty(t, staged.sumPath, "the module's go.sum travels with the candidate")

	stagedSum, sumErr := os.ReadFile(staged.sumPath)
	require.NoError(t, sumErr)
	assert.Equal(t, liveSum, string(stagedSum))

	require.NoError(t, staged.Close())

	_, statErr := os.Stat(staged.dir)

	assert.ErrorIs(t, statErr, os.ErrNotExist, "Close removes the staged copies")
}

func TestStageGoMod_ModuleWithoutGoSumStagesNone(t *testing.T) {
	t.Parallel()

	module := t.TempDir()
	goModPath := filepath.Join(module, "go.mod")

	require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/m\n\ngo 1.27\n"), 0o644))

	staged, err := stageGoMod(goModPath, []byte("module example.com/m\n\ngo 1.27\n"))
	require.NoError(t, err)
	require.Empty(t, staged.sumPath, "a module without go.sum stages no sum file")

	names := dirEntryNames(t, staged.dir)
	assert.Equal(t, []string{"go.mod"}, names)

	require.NoError(t, staged.Close())
	assert.NoError(t, staged.Close(), "Close is safe to repeat")
}

func TestStageGoMod_UnreadableSumStagesNothing(t *testing.T) {
	t.Parallel()

	module := t.TempDir()
	goModPath := filepath.Join(module, "go.mod")

	require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/m\n\ngo 1.27.1\n"), 0o644))

	// A directory named go.sum cannot be read as the checksum file.
	require.NoError(t, os.Mkdir(filepath.Join(module, "go.sum"), 0o755))

	staged, err := stageGoMod(goModPath, []byte("module example.com/m\n\ngo 1.27\n"))

	assert.Nil(t, staged)
	require.Error(t, err)
	assert.Contains(t, err.Error(), filepath.Join(module, "go.sum"))
}
