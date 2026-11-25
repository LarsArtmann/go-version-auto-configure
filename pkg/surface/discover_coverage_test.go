package surface

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscoverCoverage pins the DetectorV2 evidence contract (plan M25):
// FilesScanned counts exactly the surface files read (the six recognized
// classes; flake.lock is examined but deliberately unread), and vendored
// module trees are named in SkippedModules so absent modules are provably
// not scanned files.
func TestDiscoverCoverage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	write := func(rel, content string) {
		t.Helper()

		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}

	write("go.mod", "module example.com/m\n\ngo 1.26\n")
	write("flake.nix", "{ x = pkgs.go_1_26; }\n")
	write("flake.lock", "{\"lock\":true}\n")
	write("VERSION", "0.1.0\n")
	write("vendor/example.com/dep/go.mod", "module example.com/dep\n\ngo 1.25\n")

	surf, _, err := Discover(root)
	require.NoError(t, err)

	// go.mod, flake.nix, VERSION are read; flake.lock is deliberately
	// unread (T9); the vendored go.mod is skipped, not scanned.
	assert.Equal(t, 3, surf.FilesScanned)
	assert.Equal(t, []FilePath{"vendor"}, surf.SkippedModules)
}
