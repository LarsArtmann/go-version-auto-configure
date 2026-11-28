package fix

// Apply's go.mod rewrite goes through the same staged gate as
// CanonicalizeGoMod (stage.go): the dependency gate never sees the live
// file hold the candidate, and an approved fix is committed with a
// verified write.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// patchFormStrip is the one fix these tests apply: a patch-form go line
// stripped to its minor form.
var patchFormStrip = surface.Fix{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}

// listingRunner is a `go list -m` answer naming one published carrier of
// the patch-form floor, so a rejected strip classifies as dep-forced.
func listingRunner() GoCommandRunner {
	return fakeRunner(func(string, []string) (string, error) {
		return "1.26\texample.com/m\t(devel)\n" +
			"1.26.7\tgithub.com/larsartmann/go-finding\tv1.10.0\n", nil
	})
}

// TestApply_LiveFileUntouchedWhileGateRuns is the BuildFlow#124 regression
// for the fix path: the dependency gate runs while the live go.mod still
// holds the original directive.
func TestApply_LiveFileUntouchedWhileGateRuns(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goModPath := filepath.Join(root, "go.mod")
	original := "module example.com/m\n\ngo 1.26.7\n"

	require.NoError(t, os.WriteFile(goModPath, []byte(original), 0o644))

	seen := gateObservation{}

	gate := observingGate(goModPath, &seen, func(string, []string) (string, string, error) {
		return "\ndiff current/go.mod tidy/go.mod\n-go 1.26\n+go 1.26.7\n", "", nil
	})

	res, err := Apply(context.Background(), root, []surface.Fix{patchFormStrip}, Options{Gate: gate}, listingRunner())
	require.NoError(t, err)

	require.Len(t, res.DepForced, 1)
	assert.Empty(t, res.Applied)
	assert.Equal(t, original, seen.live, "the gate runs while the live file still holds the original directive")
	assert.Equal(t, "module example.com/m\n\ngo 1.26\n", seen.candidate, "the gate judged the staged candidate")
	assert.Equal(t, original, readFile(t, goModPath), "a rejected fix is withheld, not reverted")
	assertStagedCopyGone(t, seen.modFile)
}

func TestApply_ApprovedFixIsCommitted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goModPath := filepath.Join(root, "go.mod")

	require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644))

	res, err := Apply(context.Background(), root, []surface.Fix{patchFormStrip}, Options{}, listingRunner())
	require.NoError(t, err)

	require.Len(t, res.Applied, 1)
	assert.Empty(t, res.DepForced)
	assert.Empty(t, res.Failures)
	assert.Equal(t, "module example.com/m\n\ngo 1.26\n", readFile(t, goModPath))
	assert.Equal(t, []string{"go.mod"}, dirEntryNames(t, root), "the module directory never holds a staged copy")
}

// TestApply_LiveFileChangedDuringGateIsNotClobbered pins the verified
// commit: an approved fix is not written over a go.mod someone else
// rewrote while the gate ran; it is reported as a failure instead.
func TestApply_LiveFileChangedDuringGateIsNotClobbered(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	goModPath := filepath.Join(root, "go.mod")
	otherWriter := "module example.com/m\n\ngo 1.26.7\n\n// grouped by a concurrent go mod tidy\n"

	require.NoError(t, os.WriteFile(goModPath, []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644))

	gate := fakeGate(func(string, []string) (string, string, error) {
		require.NoError(t, os.WriteFile(goModPath, []byte(otherWriter), 0o644))

		return "", "", nil
	})

	res, err := Apply(context.Background(), root, []surface.Fix{patchFormStrip}, Options{Gate: gate}, listingRunner())
	require.NoError(t, err)

	require.Len(t, res.Failures, 1)
	assert.Contains(t, res.Failures[0].Cause, "modified concurrently")
	assert.Empty(t, res.Applied)
	assert.Equal(t, otherWriter, readFile(t, goModPath), "the other writer's file stands")
}
