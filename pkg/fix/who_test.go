package fix

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errFakeList is the canned go list failure used in runner fakes.
var errFakeList = errors.New("go: exit 1: missing go.sum entry")

// seedGoMod writes one go.mod for example.com/m with the given directive.
func seedGoMod(t *testing.T, dir, directive string) {
	t.Helper()

	content := "module example.com/m\n\ngo " + directive + "\n"
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
	seedGoMod(t, root, "1.26")

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
	assert.Equal(t, surface.KindGoMod, row.Kind)
	assert.Equal(t, surface.ModulePath("example.com/m"), row.Module)
	assert.Equal(t, surface.GoVersion("1.26"), row.Directive)
	assert.Equal(t, surface.GoVersion("1.26.7"), row.MaxDepFloor)
	require.Len(t, row.PoisonerFloors, 1, "the single floor-carrying dependency is named with its floor")
	assert.Equal(t, surface.ModulePath("github.com/larsartmann/go-finding"), row.PoisonerFloors[0].Module)
	assert.Equal(t, ModuleVersion("v1.12.0"), row.PoisonerFloors[0].Version)
	assert.Equal(t, surface.GoVersion("1.26.7"), row.PoisonerFloors[0].Floor)
	assert.True(t, row.Poisoned, "tidy re-raises go 1.26 to the 1.26.7 dependency floor")
	assert.Empty(t, row.ParityFloors, "a poisoned row carries poisoners, not parity carriers")
	assert.Equal(t, FloorSourceList, row.Source)
	assert.Empty(t, row.Error)
}

func TestAnalyzeFloors_DirectiveAboveFloorsIsClean(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27")

	run := fakeList(strings.Join([]string{
		"1.27\texample.com/m\t(devel)",
		"1.26.7\tgithub.com/larsartmann/go-finding\tv1.12.0",
	}, "\n"))

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.Equal(t, surface.GoVersion("1.26.7"), rows[0].MaxDepFloor)
	assert.False(t, rows[0].Poisoned, "the 1.26.7 floor is below the 1.27 directive")
	assert.Empty(t, rows[0].PoisonerFloors, "only dependencies forcing above the directive are listed")
	assert.Empty(t, rows[0].ParityFloors, "a directive above every floor sits at parity with nobody")
}

// TestAnalyzeFloors_AtParityListsCarriers answers the 2026-10-02 incident
// question ("who holds me at parity?") without raw `go list -m`: when the
// directive equals the max dependency floor, the carriers are named, the
// row stays clean, and no exit semantics change.
func TestAnalyzeFloors_AtParityListsCarriers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27.1")

	run := fakeList(strings.Join([]string{
		"1.27.1\texample.com/m\t(devel)",
		"1.27.1\tgithub.com/larsartmann/go-output\tv0.38.2",
		"1.27\tgithub.com/larsartmann/go-finding\tv1.13.0",
	}, "\n"))

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.False(t, row.Poisoned, "at parity is clean: no dependency forces a HIGHER floor")
	assert.Equal(t, surface.GoVersion("1.27.1"), row.MaxDepFloor)
	assert.Empty(t, row.PoisonerFloors)
	require.Len(t, row.ParityFloors, 1, "the dependency holding the directive at its value is named")
	assert.Equal(t, DependencyFloor{
		Module:  "github.com/larsartmann/go-output",
		Version: "v0.38.2",
		Floor:   "1.27.1",
	}, row.ParityFloors[0])
	assert.Equal(t, FloorSourceList, row.Source)
}

func TestAnalyzeFloors_NoDependencies(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.26")

	run := fakeList("1.26\texample.com/m\t(devel)\n")

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.Empty(t, rows[0].MaxDepFloor)
	assert.False(t, rows[0].Poisoned)
	assert.Empty(t, rows[0].PoisonerFloors)
}

func TestAnalyzeFloors_MarksWorkspaceRowWithoutAnalysis(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.26")
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.26\n\nuse .\n"), 0o644))

	// The main-module record and a local (devel) replacement carry no floor.
	run := fakeList(strings.Join([]string{
		"1.26\texample.com/m\t(devel)",
		"1.26.7\texample.com/local\t(devel)",
	}, "\n"))

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 2, "go.work appears as a marked row: it declares no dependencies")

	var moduleRow, workRow *ModuleFloors

	for i := range rows {
		switch rows[i].Kind {
		case surface.KindGoMod:
			moduleRow = &rows[i]
		case surface.KindGoWork:
			workRow = &rows[i]
		}
	}

	require.NotNil(t, moduleRow)
	require.NotNil(t, workRow)
	assert.Empty(t, moduleRow.MaxDepFloor, "neither the main module nor (devel) replacements force a floor")
	assert.Equal(t, "go.work", workRow.Path)
	assert.False(t, workRow.Poisoned, "a workspace row is never poisoned: no dependency graph")
	assert.Empty(t, workRow.Error)
}

func TestAnalyzeFloors_DependencyFloorsCarryEachForcersFloor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.26")

	// Two dependencies force above the directive at different floors: the
	// matrix must carry each with its own floor, highest first, not only
	// the carriers of the single highest floor.
	run := fakeList(strings.Join([]string{
		"1.26\texample.com/m\t(devel)",
		"1.26.7\tgithub.com/larsartmann/go-finding\tv1.12.0",
		"1.27\tgithub.com/larsartmann/go-atomic-write\tv0.5.0",
		"1.25\tgithub.com/gofrs/flock\tv0.13.1",
	}, "\n"))

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.True(t, row.Poisoned)
	assert.Equal(t, surface.GoVersion("1.27"), row.MaxDepFloor)

	require.Len(t, row.PoisonerFloors, 2, "every forcing dependency is listed, not only the max carriers")
	assert.Equal(t, DependencyFloor{
		Module:  "github.com/larsartmann/go-atomic-write",
		Version: "v0.5.0",
		Floor:   "1.27",
	}, row.PoisonerFloors[0])
	assert.Equal(t, DependencyFloor{
		Module:  "github.com/larsartmann/go-finding",
		Version: "v1.12.0",
		Floor:   "1.26.7",
	}, row.PoisonerFloors[1])
}

func TestAnalyzeFloors_ListFailureRecordedPerModule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.26")

	run := fakeRunner(func(_ string, _ []string) (string, error) {
		return "", errFakeList
	})

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err, "a module-level go list failure must not abort the matrix")
	require.Len(t, rows, 1)
	assert.Contains(t, rows[0].Error, "missing go.sum entry")
	assert.False(t, rows[0].Poisoned, "an unanalyzable module is not claimed as poisoned")
}

// TestAnalyzeFloors_UntidyTreeResolvesViaErrorTolerantList pins the shared
// authority walk on the who-forces side: a tree a plain `go list -m`
// refuses (exactly what a rejected fix leaves behind) still resolves
// through the error-tolerant listing, with list provenance — the same
// walk fix classification has always used.
func TestAnalyzeFloors_UntidyTreeResolvesViaErrorTolerantList(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.26")

	run := fakeRunner(func(_ string, args []string) (string, error) {
		if !slices.Contains(args, "-e") {
			return "", errFakeList
		}

		return "1.26.7\tgithub.com/larsartmann/go-finding\tv1.12.0\n", nil
	})

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Empty(t, row.Error, "the -e retry resolves what the plain listing refuses")
	assert.True(t, row.Poisoned)
	assert.Equal(t, surface.GoVersion("1.26.7"), row.MaxDepFloor)
	assert.Equal(t, FloorSourceList, row.Source)
}

func TestCompareDependencyFloorsOrdersByFloorThenModule(t *testing.T) {
	t.Parallel()

	high, low := DependencyFloor{Module: "b", Floor: "1.27"}, DependencyFloor{Module: "a", Floor: "1.26.7"}
	sameFloorA, sameFloorB := DependencyFloor{Module: "z", Floor: "1.26"}, DependencyFloor{Module: "a", Floor: "1.26"}

	assert.Negative(t, compareDependencyFloors(high, low), "higher floor sorts first")
	assert.Positive(t, compareDependencyFloors(low, high))
	assert.Positive(
		t,
		compareDependencyFloors(sameFloorA, sameFloorB),
		"same floor orders by module path (z after a)",
	)
	assert.Negative(t, compareDependencyFloors(sameFloorB, sameFloorA))
	assert.Equal(t, 0, compareDependencyFloors(sameFloorB, sameFloorB))
}

func TestAnalyzeFloors_DiscoverFailureAborts(t *testing.T) {
	t.Parallel()

	run := fakeList("")

	_, err := AnalyzeFloors(context.Background(), filepath.Join(t.TempDir(), "gone"), run)
	require.Error(t, err)
}

func TestAnalyzeFloors_VendorSkewResolvesFromAnnotations(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "vendor"), 0o755))
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(root, "vendor", "modules.txt"),
			[]byte(
				"# github.com/larsartmann/go-sse v0.6.1\n"+
					"## explicit; go 1.27.1\n"+
					"# github.com/charmbracelet/x/ansi v0.1.2\n"+
					"## explicit; go 1.24.0\n"+
					"# github.com/larsartmann/go-cqrs-lite/record/v4 v4.5.1\n"+
					"## explicit; go 1.27.1\n",
			),
			0o644,
		),
	)

	run := fakeRunner(func(string, []string) (string, error) {
		return "", errFakeList
	})

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Empty(t, row.Error, "the vendored annotations resolve what the skewed graph hides")
	assert.Equal(t, surface.GoVersion("1.27.1"), row.MaxDepFloor)
	assert.True(t, row.Poisoned)
	require.Len(t, row.PoisonerFloors, 2)
	assert.Equal(
		t,
		surface.ModulePath("github.com/larsartmann/go-cqrs-lite/record/v4"),
		row.PoisonerFloors[0].Module,
		"equal floors tie-break by module path",
	)
	assert.Equal(t, surface.ModulePath("github.com/larsartmann/go-sse"), row.PoisonerFloors[1].Module)
	assert.Equal(t, FloorSourceVendor, row.Source, "the vendor fallback is visible as provenance")
}

func TestAnalyzeFloors_ListFailureWithoutVendorRecordsError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.26")

	run := fakeRunner(func(string, []string) (string, error) {
		return "", errFakeList
	})

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.NotEmpty(t, rows[0].Error, "without vendor annotations the per-module listing error stands")
	assert.False(t, rows[0].Poisoned)
	assert.Empty(t, rows[0].Source, "an unresolved row claims no provenance")
}

// TestAnalyzeFloors_VendorAtParityCarriesVendorSource pins the additive
// provenance on the at-parity path too: a vendored carrier holding the
// directive is named with source vendor, still clean.
func TestAnalyzeFloors_VendorAtParityCarriesVendorSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedGoMod(t, root, "1.27.1")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "vendor"), 0o755))
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(root, "vendor", "modules.txt"),
			[]byte("# github.com/larsartmann/go-output v0.38.2\n## explicit; go 1.27.1\n"),
			0o644,
		),
	)

	run := fakeRunner(func(string, []string) (string, error) {
		return "", errFakeList
	})

	rows, err := AnalyzeFloors(context.Background(), root, run)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	assert.Empty(t, row.Error)
	assert.False(t, row.Poisoned)
	assert.Equal(t, FloorSourceVendor, row.Source)
	require.Len(t, row.ParityFloors, 1)
	assert.Equal(t, surface.ModulePath("github.com/larsartmann/go-output"), row.ParityFloors[0].Module)
}
