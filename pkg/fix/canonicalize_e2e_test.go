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

// TestCanonicalizeGoMod_E2E_RealGateJudgesStagedCandidate drives the
// production dependency gate over a staged candidate for both verdicts.
func TestCanonicalizeGoMod_E2E_RealGateJudgesStagedCandidate(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: needs the real go toolchain")
	}

	t.Parallel()

	installed := surface.GoVersion(probeInstalledVersion(t))

	t.Run("dep-forced downgrade is withheld and the live file never moves", func(t *testing.T) {
		t.Parallel()

		root := seedRealGateFixture(t, "1.27.1")
		consumer := filepath.Join(root, "consumer")
		goModPath := filepath.Join(consumer, "go.mod")
		original := readFile(t, goModPath)
		seen := gateObservation{}
		real := ExecSplitRunner()

		gate := observingGate(goModPath, &seen, func(dir string, args []string) (string, string, error) {
			return real(context.Background(), dir, args...)
		})

		res, err := CanonicalizeGoMod(context.Background(), goModPath, CanonicalizeOptions{
			InstalledToolchain: installed,
		}, gate)
		require.NoError(t, err)

		assert.True(t, res.HeldBackByDepFloor)
		assert.False(t, res.Changed)
		assert.Contains(t, res.GateDetail, "+go 1.27.1",
			"the real gate reports the floor the replace target forces; a staged copy outside the module still resolves the relative replace")
		assert.Equal(t, original, seen.live, "the real gate ran while the live file held the original directive")
		assert.Equal(t, original, readFile(t, goModPath))
		assert.Equal(t, []string{"go.mod", "main.go"}, dirEntryNames(t, consumer))
		assertStagedCopyGone(t, seen.modFile)
	})

	t.Run("floor-safe downgrade is committed", func(t *testing.T) {
		t.Parallel()

		root := seedRealGateFixture(t, "1.27.1")
		consumer := filepath.Join(root, "consumer")
		goModPath := filepath.Join(consumer, "go.mod")

		// Drop the poisoner: an independent module whose floor nothing forces.
		require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/m\n\ngo 1.27.1\n"), 0o644))
		require.NoError(
			t,
			os.WriteFile(filepath.Join(consumer, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644),
		)

		seen := gateObservation{}
		real := ExecSplitRunner()

		gate := observingGate(goModPath, &seen, func(dir string, args []string) (string, string, error) {
			return real(context.Background(), dir, args...)
		})

		res, err := CanonicalizeGoMod(context.Background(), goModPath, CanonicalizeOptions{
			InstalledToolchain: installed,
		}, gate)
		require.NoError(t, err)

		assert.True(t, res.Changed)
		assert.False(t, res.HeldBackByDepFloor)
		assert.Equal(t, "module example.com/m\n\ngo 1.27.1\n", seen.live, "even the real gate judged the staged copy")
		assert.Equal(t, "module example.com/m\n\ngo 1.27\n", readFile(t, goModPath))
		assert.Equal(t, []string{"go.mod", "main.go"}, dirEntryNames(t, consumer))
		assertStagedCopyGone(t, seen.modFile)
	})
}
