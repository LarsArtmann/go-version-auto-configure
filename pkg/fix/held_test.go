package fix

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errUnreachable is the sentinel a fixture runner returns only where the
// annotation path must never list the module graph.
var errUnreachable = errors.New("unreachable")

func TestHeldStrip_ReasonAtParity(t *testing.T) {
	t.Parallel()

	strip := HeldStrip{
		Floor: "1.27.1",
		Carriers: []DependencyFloor{
			{Module: "github.com/LarsArtmann/go-daemon", Version: "v0.4.0", Floor: "1.27.1"},
			{Module: "github.com/larsartmann/go-sse/sseparse", Version: "v0.2.0", Floor: "1.27.1"},
		},
	}

	assert.Equal(
		t,
		"not auto-fixed: go mod tidy reverts the strip, go 1.27.1 is forced by "+
			"github.com/LarsArtmann/go-daemon@v0.4.0, github.com/larsartmann/go-sse/sseparse@v0.2.0",
		strip.Reason(),
	)
}

func TestHeldStrip_ReasonPoisonedNamesForcerFloors(t *testing.T) {
	t.Parallel()

	strip := HeldStrip{
		Floor:    "1.28",
		Poisoned: true,
		Carriers: []DependencyFloor{
			{Module: "example.com/high", Version: "v2.0.0", Floor: "1.28"},
			{Module: "example.com/low", Version: "v1.0.0", Floor: "1.27.2"},
		},
	}

	assert.Equal(
		t,
		"not auto-fixed: dependencies force go 1.28 above the declared directive "+
			"(example.com/high@v2.0.0 (go 1.28), example.com/low@v1.0.0 (go 1.27.2))",
		strip.Reason(),
	)
}

func TestHeldStrip_Remediation(t *testing.T) {
	t.Parallel()

	parity := HeldStrip{
		Floor: "1.27.1",
		Carriers: []DependencyFloor{
			{Module: "github.com/LarsArtmann/go-daemon", Version: "v0.4.0", Floor: "1.27.1"},
		},
	}
	assert.Equal(
		t,
		"fix supply-side: re-tag github.com/LarsArtmann/go-daemon@v0.4.0 with a major.minor-only go directive, "+
			"then bump this repo; who-forces reports the full matrix",
		parity.Remediation(),
	)

	poisoned := HeldStrip{Floor: "1.28", Poisoned: true, Carriers: parity.Carriers}
	assert.Equal(
		t,
		"raise the go directive to the forced floor (go mod edit -go=1.28) or re-tag the forcers with lower floors; "+
			"who-forces reports the full matrix",
		poisoned.Remediation(),
	)
}

func TestExplainHeldStrips_ParityNamesCarriers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27.1")

	run := fakeList(strings.Join([]string{
		"1.27.1\texample.com/m\t(devel)",
		"1.27.1\tgithub.com/LarsArtmann/go-daemon\tv0.4.0",
		"1.27.1\tgithub.com/larsartmann/go-sse/sseparse\tv0.2.0",
		"1.26\tgithub.com/gofrs/flock\tv0.13.1",
	}, "\n"))

	held := ExplainHeldStrips(context.Background(), root, run, []string{"go.mod"})
	require.Len(t, held, 1)

	strip := held["go.mod"]
	assert.False(t, strip.Poisoned)
	assert.Equal(t, surface.GoVersion("1.27.1"), strip.Floor)
	require.Len(t, strip.Carriers, 2, "only the dependencies holding the directive's exact floor carry it")
	assert.Equal(t, surface.ModulePath("github.com/LarsArtmann/go-daemon"), strip.Carriers[0].Module)
	assert.Equal(t, surface.ModulePath("github.com/larsartmann/go-sse/sseparse"), strip.Carriers[1].Module)
}

func TestExplainHeldStrips_PoisonedUsesForcers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.26.7")

	run := fakeList(strings.Join([]string{
		"1.26.7\texample.com/m\t(devel)",
		"1.27.1\tgithub.com/larsartmann/go-finding\tv1.12.0",
	}, "\n"))

	held := ExplainHeldStrips(context.Background(), root, run, []string{"go.mod"})
	require.Len(t, held, 1)

	strip := held["go.mod"]
	assert.True(t, strip.Poisoned)
	assert.Equal(t, surface.GoVersion("1.27.1"), strip.Floor)
	require.Len(t, strip.Carriers, 1)
	assert.Equal(t, surface.ModulePath("github.com/larsartmann/go-finding"), strip.Carriers[0].Module)
}

func TestExplainHeldStrips_StripBelowFloorsNotHeld(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27.1")

	run := fakeList(strings.Join([]string{
		"1.27.1\texample.com/m\t(devel)",
		"1.27\tgithub.com/larsartmann/go-output\tv0.38.5",
		"1.26.7\tgithub.com/larsartmann/go-finding\tv1.12.0",
	}, "\n"))

	assert.Nil(
		t,
		ExplainHeldStrips(context.Background(), root, run, []string{"go.mod"}),
		"no dependency carries the patch floor, so the strip is auto-fixable",
	)
}

func TestExplainHeldStrips_NoFilesSkipsListing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27")

	listed := false
	run := fakeRunner(func(_ string, _ []string) (string, error) {
		listed = true

		return "", errUnreachable
	})

	assert.Nil(t, ExplainHeldStrips(context.Background(), root, run, nil))
	assert.False(t, listed, "a clean repo must not pay for a module-graph listing")
}

func TestExplainHeldStrips_ListFailureDegradesToNil(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27.1")

	run := fakeRunner(func(_ string, _ []string) (string, error) {
		return "", errFakeList
	})

	assert.Nil(
		t,
		ExplainHeldStrips(context.Background(), root, run, []string{"go.mod"}),
		"a failed listing leaves the finding plain instead of guessing",
	)
}

func TestExplainHeldStrips_OnlyRequestedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27.1")

	nested := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	seedGoMod(t, nested, "1.27.1")

	run := fakeList(strings.Join([]string{
		"1.27.1\texample.com/m\t(devel)",
		"1.27.1\tgithub.com/LarsArtmann/go-daemon\tv0.4.0",
	}, "\n"))

	held := ExplainHeldStrips(context.Background(), root, run, []string{"sub/go.mod"})
	require.Len(t, held, 1)
	assert.Contains(t, held, "sub/go.mod")
}
