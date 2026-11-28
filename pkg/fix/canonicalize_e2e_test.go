package fix

// End-to-end coverage of the staged gate against the real go toolchain:
// `go mod tidy -diff -modfile=<staged>` judges the candidate exactly as it
// judged the live file before staging, relative replace directives still
// resolve against the module directory, and the live go.mod never leaves
// its original state before the verdict. It needs the go binary on PATH
// and skips under -short.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realObservingGate is the production dependency gate, recording what it
// saw of the live go.mod and which staged copy it judged when it ran.
func realObservingGate(goModPath string, seen *gateObservation) SplitRunner {
	production := ExecSplitRunner()

	return observingGate(goModPath, seen, func(dir string, args []string) (string, string, error) {
		return production(context.Background(), dir, args...)
	})
}

// TestCanonicalizeGoMod_E2E_RealGateWithholdsDepForcedDowngrade drives the
// production dependency gate over a staged candidate a replace target's
// floor rejects: the diff names that floor (so the staged copy outside the
// module still resolved the relative replace), and the live go.mod never
// leaves its original state.
func TestCanonicalizeGoMod_E2E_RealGateWithholdsDepForcedDowngrade(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: needs the real go toolchain")
	}

	t.Parallel()

	root := seedRealGateFixture(t)
	consumer := filepath.Join(root, "consumer")
	goModPath := filepath.Join(consumer, "go.mod")
	original := readFile(t, goModPath)
	seen := gateObservation{}

	res, err := CanonicalizeGoMod(context.Background(), goModPath, CanonicalizeOptions{
		InstalledToolchain: surface.GoVersion(probeInstalledVersion(t)),
	}, realObservingGate(goModPath, &seen))
	require.NoError(t, err)

	assert.True(t, res.HeldBackByDepFloor)
	assert.False(t, res.Changed)
	assert.Contains(t, res.GateDetail, "+go 1.27.1",
		"the real gate reports the floor the replace target forces")
	assert.Equal(t, original, seen.live, "the real gate ran while the live file held the original directive")
	assert.Equal(t, original, readFile(t, goModPath))
	assert.Equal(t, []string{"go.mod", "main.go"}, dirEntryNames(t, consumer))
	assertStagedCopyGone(t, seen.modFile)
}

// TestCanonicalizeGoMod_E2E_RealGateCommitsFloorSafeDowngrade proves the
// green path against the real toolchain: with no dependency above the
// target, the staged candidate passes and is committed to the live file.
func TestCanonicalizeGoMod_E2E_RealGateCommitsFloorSafeDowngrade(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: needs the real go toolchain")
	}

	t.Parallel()

	root := seedRealGateFixture(t)
	consumer := filepath.Join(root, "consumer")
	goModPath := filepath.Join(consumer, "go.mod")

	// Drop the poisoner: an independent module whose floor nothing forces.
	require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/m\n\ngo 1.27.1\n"), 0o644))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(consumer, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644),
	)

	seen := gateObservation{}

	res, err := CanonicalizeGoMod(context.Background(), goModPath, CanonicalizeOptions{
		InstalledToolchain: surface.GoVersion(probeInstalledVersion(t)),
	}, realObservingGate(goModPath, &seen))
	require.NoError(t, err)

	assert.True(t, res.Changed)
	assert.False(t, res.HeldBackByDepFloor)
	assert.Equal(t, "module example.com/m\n\ngo 1.27.1\n", seen.live, "even the real gate judged the staged copy")
	assert.Equal(t, "module example.com/m\n\ngo 1.27\n", readFile(t, goModPath))
	assert.Equal(t, []string{"go.mod", "main.go"}, dirEntryNames(t, consumer))
	assertStagedCopyGone(t, seen.modFile)
}
