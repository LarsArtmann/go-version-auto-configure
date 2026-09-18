package fix

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedGoMod writes one go.mod with the given module path and directive.
func seedGoMod(t *testing.T, dir, module, directive string) {
	t.Helper()

	content := "module " + module + "\n\ngo " + directive + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644))
}

// fakeList answers every `go list` with the given output, one record per
// line in the GoVersion/Path/Version tab format AnalyzeFloors requests.
func fakeList(out string) GoCommandRunner {
	return fakeRunner(func(_ string, _ []string) (string, error) {
		return out, nil
	})
}

func TestAnalyzeFloors_NamesPoisoners(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "example.com/m", "1.26")

	run := fakeList(strings.Join([]string{
		"1.26\texample.com/m\t(devel)",
		"1.26.7\tgithub.com/larsartmann/go-finding\tv1.12.0",
		"1.25\tgithub.com/gofrs/flock\tv0.13.1",
	}, "\n"))

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Equal(t, "go.mod", row.Path)
	assert.Equal(t, "example.com/m", row.Module)
	assert.Equal(t, "1.26", row.Directive)
	assert.Equal(t, "1.26.7", row.MaxDepFloor)
	assert.Equal(t, []string{"github.com/larsartmann/go-finding@v1.12.0"}, row.Poisoners)
	assert.True(t, row.Poisoned, "tidy re-raises go 1.26 to the 1.26.7 dependency floor")
	assert.Empty(t, row.Error)
}

func TestAnalyzeFloors_DirectiveAboveFloorsIsClean(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "example.com/m", "1.27")

	run := fakeList(strings.Join([]string{
		"1.27\texample.com/m\t(devel)",
		"1.26.7\tgithub.com/larsartmann/go-finding\tv1.12.0",
	}, "\n"))

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.Equal(t, "1.26.7", rows[0].MaxDepFloor)
	assert.False(t, rows[0].Poisoned, "the 1.26.7 floor is below the 1.27 directive")
	assert.Empty(t, rows[0].Poisoners, "only floor-carrying modules are poisoners")
}

func TestAnalyzeFloors_NoDependencies(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "example.com/m", "1.26")

	run := fakeList("1.26\texample.com/m\t(devel)\n")

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.Empty(t, rows[0].MaxDepFloor)
	assert.False(t, rows[0].Poisoned)
	assert.Empty(t, rows[0].Poisoners)
}

func TestAnalyzeFloors_SkipsWorkspaceAndMainModule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "example.com/m", "1.26")
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.26\n\nuse .\n"), 0o644))

	// The main-module record and a local (devel) replacement carry no floor.
	run := fakeList(strings.Join([]string{
		"1.26\texample.com/m\t(devel)",
		"1.26.7\texample.com/local\t(devel)",
	}, "\n"))

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1, "go.work declares no dependencies and is skipped")
	assert.Empty(t, rows[0].MaxDepFloor, "neither the main module nor (devel) replacements force a floor")
}

func TestAnalyzeFloors_ListFailureRecordedPerModule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "example.com/m", "1.26")

	run := fakeRunner(func(_ string, _ []string) (string, error) {
		return "", errors.New("go: exit 1: missing go.sum entry")
	})

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err, "a module-level go list failure must not abort the matrix")
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].Error, "missing go.sum entry")
	assert.False(t, rows[0].Poisoned, "an unanalyzable module is not claimed as poisoned")
}

func TestAnalyzeFloors_DiscoverFailureAborts(t *testing.T) {
	t.Parallel()

	run := fakeList("")

	_, err := AnalyzeFloors(context.Background(), filepath.Join(t.TempDir(), "gone"), run)
	require.Error(t, err)
}
