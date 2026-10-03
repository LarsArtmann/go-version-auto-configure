package fix

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Apply tests with injected fake gates prove classification logic; these
// end-to-end tests run the production runners (ExecSplitRunner for
// `go mod tidy -diff`, EditRunner for `go list -m`) against seeded fixture
// repositories, so the claim "the gate rejects what tidy reverts" is tested
// against the real go toolchain, not a replay of its output shape.
//
// They need the go binary on PATH and skip under -short.

// seedRealGateFixture writes a consumer module requiring a local poisoner
// through a directory replace (offline-safe: no module downloads, no
// go.sum), plus a .go file so the module has packages to resolve.
func seedRealGateFixture(t *testing.T, directive string) string {
	t.Helper()

	root := t.TempDir()

	poisoner := filepath.Join(root, "poisoner")
	require.NoError(t, os.MkdirAll(poisoner, 0o755))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(poisoner, "go.mod"), []byte("module example.com/poisoner\n\ngo 1.27.1\n"), 0o644),
	)
	require.NoError(t, os.WriteFile(filepath.Join(poisoner, "poisoner.go"), []byte("package poisoner\n"), 0o644))

	consumer := filepath.Join(root, "consumer")
	require.NoError(t, os.MkdirAll(consumer, 0o755))

	goMod := "module example.com/m\n\ngo " + directive + "\n\n" +
		"require example.com/poisoner v0.0.0\n\n" +
		"replace example.com/poisoner => ../poisoner\n"
	require.NoError(t, os.WriteFile(filepath.Join(consumer, "go.mod"), []byte(goMod), 0o644))
	mainGo := "package main\n\nimport _ \"example.com/poisoner\"\n\nfunc main() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(consumer, "main.go"), []byte(mainGo), 0o644))

	return root
}

// TestApply_E2E_RealTidyGateRejectsDepForcedDowngrade drives the full
// production path: the patch-form strip is written, the real
// `go mod tidy -diff` gate sees the dependency floor re-raise, the rewrite
// is reverted, and the classification names the floor and the replace
// target carrying it.
func TestApply_E2E_RealTidyGateRejectsDepForcedDowngrade(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: needs the real go toolchain")
	}

	t.Parallel()

	root := seedRealGateFixture(t, "1.27.1")
	goModPath := filepath.Join(root, "consumer", "go.mod")

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "consumer/go.mod", Kind: surface.KindGoMod, From: "1.27.1", To: "1.27", Line: 3}},
		Options{},
		nil,
	)
	require.NoError(t, err)

	require.Len(t, res.DepForced, 1, "the real gate rejects the strip the local floor forces back")
	assert.Empty(t, res.Applied)
	assert.Empty(t, res.Failures)

	dep := res.DepForced[0]
	assert.Equal(t, surface.GoVersion("1.27.1"), dep.Floor, "the enforced floor is the replace target's directive")

	data, readErr := os.ReadFile(goModPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "go 1.27.1", "the rejected rewrite is reverted on disk")
}

// TestApply_E2E_RealTidyGateAcceptsCleanStrip proves the green path against
// the real toolchain: with no dependency above the target, the strip
// survives `go mod tidy -diff` and lands on disk.
func TestApply_E2E_RealTidyGateAcceptsCleanStrip(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: needs the real go toolchain")
	}

	t.Parallel()

	root := seedRealGateFixture(t, "1.27.1")
	goModPath := filepath.Join(root, "consumer", "go.mod")

	// Drop the poisoner: an independent module whose floor nothing forces.
	require.NoError(
		t,
		os.WriteFile(
			goModPath,
			[]byte("module example.com/m\n\ngo 1.27.1\n"),
			0o644,
		),
	)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "consumer", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644),
	)

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "consumer/go.mod", Kind: surface.KindGoMod, From: "1.27.1", To: "1.27", Line: 3}},
		Options{},
		nil,
	)
	require.NoError(t, err)

	require.Len(t, res.Applied, 1, "the real gate keeps the floor-safe strip")
	assert.Empty(t, res.DepForced)
	assert.Empty(t, res.Failures)

	data, readErr := os.ReadFile(goModPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "go 1.27\n", "the strip lands on disk")
	assert.NotContains(t, string(data), "go 1.27.1", "the patch form is gone")
}
