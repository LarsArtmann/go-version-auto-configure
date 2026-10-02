package fix

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// ModuleVersion is a released module version as listed by `go list -m`,
// e.g. "v1.10.0". A named type keeps module versions distinct from Go
// toolchain versions (GoVersion), which parse differently.
type ModuleVersion string

// DependencyFloor is one dependency with the `go` floor it declares: the
// row shape of both who-forces lists — a poisoned row's PoisonerFloors
// (dependencies forcing the floor ABOVE the directive) and an at-parity
// row's ParityFloors (dependencies holding the directive exactly where it
// is).
type DependencyFloor struct {
	// Module is the dependency's module path.
	Module surface.ModulePath `json:"module"`
	// Version is the dependency's released version, e.g. "v1.10.0".
	Version ModuleVersion `json:"version"`
	// Floor is the `go` directive the dependency declares.
	Floor surface.GoVersion `json:"floor"`
}

// FloorSource names the authority a row's floors were resolved from — the
// provenance of the numbers, additive on wire schema 2.
type FloorSource string

const (
	// FloorSourceList marks rows resolved via `go list -m`.
	FloorSourceList FloorSource = "list"
	// FloorSourceVendor marks rows resolved via vendor/modules.txt
	// annotations after a skewed vendor tree refused both listings.
	FloorSourceVendor FloorSource = "vendor"
)

// ModuleFloors is one module's dependency-floor matrix row: the floor the
// dependencies collectively force, and which of them carry it. The json
// tags are the stable machine contract for the who-forces --json output.
type ModuleFloors struct {
	// Path is the go.mod path relative to the repository root.
	Path string `json:"path"`
	// Kind is "go.mod" or "go.work"; go.work rows declare no dependencies
	// and carry no floor analysis.
	Kind surface.DirectiveKind `json:"kind"`
	// Module is the module path declared in the go.mod.
	Module surface.ModulePath `json:"module"`
	// Directive is the declared `go` directive ("" when none).
	Directive surface.GoVersion `json:"directive,omitempty"`
	// MaxDepFloor is the highest `go` floor any dependency declares
	// ("" when no dependency declares one).
	MaxDepFloor surface.GoVersion `json:"maxDepFloor,omitempty"`
	// PoisonerFloors lists every dependency whose own floor exceeds the
	// directive, each with its floor, sorted highest floor first; empty
	// unless Poisoned. Schema 2 of the wire contract removed the flat
	// `poisoners` list; every carrier is named here with its floor.
	PoisonerFloors []DependencyFloor `json:"poisonerFloors,omitempty"`
	// ParityFloors lists every dependency whose floor EQUALS the directive
	// when the row sits at parity (directive == MaxDepFloor): the carriers
	// holding the module at its current floor — strip the directive and
	// tidy re-raises it to exactly these. Empty unless at parity; Poisoned
	// stays false and the exit contract is unchanged.
	ParityFloors []DependencyFloor `json:"parityFloors,omitempty"`
	// Source names the authority the floors were resolved from ("list" or
	// "vendor"); empty when the row errored and nothing resolved.
	Source FloorSource `json:"source,omitempty"`
	// Poisoned reports whether `go mod tidy` would re-raise the directive:
	// the dependency floor exceeds the declared directive.
	Poisoned bool `json:"poisoned"`
	// Error is non-empty when the module's dependency graph could not be
	// listed; the other fields except Path and Module are unreliable then.
	Error FailureCause `json:"error,omitempty"`
}

// AnalyzeFloors resolves, for every go.mod under root, the highest `go`
// floor its dependencies declare and the dependencies carrying it — the
// poisoner matrix behind dep-forced fixes. Workspace files appear as marked
// rows (Kind go.work) without floor analysis: they declare no dependencies.
// go list failures are recorded per module in Error; a failed Discover
// aborts.
func AnalyzeFloors(ctx context.Context, root string, run GoCommandRunner) ([]ModuleFloors, error) {
	if run == nil {
		run = EditRunner()
	}

	s, _, err := surface.Discover(root)
	if err != nil {
		return nil, fmt.Errorf("fix: discover %q: %w", root, err)
	}

	rows := make([]ModuleFloors, 0, len(s.Modules))

	for _, m := range s.Modules {
		if m.Kind != surface.KindGoMod {
			rows = append(rows, ModuleFloors{Path: m.Path, Kind: m.Kind, Directive: m.Version})

			continue
		}

		rows = append(rows, floorsForModule(ctx, root, m, run))
	}

	return rows, nil
}

// floorsForModule lists one module's dependency graph and extracts its
// floor matrix row. A vendor directory skewed against go.mod ("inconsistent
// vendoring") makes `go list` refuse the graph; the modules.txt
// annotations still record every vendored floor and resolve the row then.
func floorsForModule(ctx context.Context, root string, m surface.ModuleDirective, run GoCommandRunner) ModuleFloors {
	row := ModuleFloors{Path: m.Path, Kind: m.Kind, Module: m.Module, Directive: m.Version}

	dir := filepath.Join(root, filepath.Dir(m.Path))

	var forcers, parity []DependencyFloor

	out, err := run(ctx, dir, "list", "-m", "-f", "{{.GoVersion}}\t{{.Path}}\t{{.Version}}", "all")
	if err != nil {
		vendored, ok := readVendorModuleFloors(dir)
		if !ok {
			row.Error = FailureCause(err.Error())

			return row
		}

		for _, v := range vendored {
			accumulateFloor(&row, &forcers, &parity, v.Floor, v.Module, v.Version)
		}

		row.Source = FloorSourceVendor

		return finalizeFloors(row, forcers, parity)
	}

	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		floor, dep, version, ok := parseFloorLine(line, m.Module)
		if !ok {
			continue
		}

		accumulateFloor(&row, &forcers, &parity, floor, dep, version)
	}

	row.Source = FloorSourceList

	return finalizeFloors(row, forcers, parity)
}

// accumulateFloor folds one dependency floor triple into the row's max
// floor, the forcer list (floors above the directive), and the parity list
// (floors equal to the directive — the carriers that hold it there).
func accumulateFloor(
	row *ModuleFloors,
	forcers *[]DependencyFloor,
	parity *[]DependencyFloor,
	floor surface.GoVersion,
	dep surface.ModulePath,
	version ModuleVersion,
) {
	if row.MaxDepFloor == "" || surface.GreaterVersion(string(floor), string(row.MaxDepFloor)) {
		row.MaxDepFloor = floor
	}

	entry := DependencyFloor{Module: dep, Version: version, Floor: floor}

	switch {
	case surface.GreaterVersion(string(floor), string(row.Directive)):
		*forcers = append(*forcers, entry)
	case floor == row.Directive:
		*parity = append(*parity, entry)
	}
}

// finalizeFloors marks poisoning, orders the forcers highest floor first,
// and — on a non-poisoned row sitting exactly at its max dependency floor —
// names the parity carriers so "who holds me here" has a one-command
// answer. The poisoned semantics and the exit contract are unchanged.
func finalizeFloors(row ModuleFloors, forcers []DependencyFloor, parity []DependencyFloor) ModuleFloors {
	row.Poisoned = surface.GreaterVersion(string(row.MaxDepFloor), string(row.Directive))

	if row.Poisoned {
		row.PoisonerFloors = forcers
		slices.SortFunc(row.PoisonerFloors, compareDependencyFloors)

		return row
	}

	if row.Directive != "" && row.MaxDepFloor == row.Directive && len(parity) > 0 {
		row.ParityFloors = parity
		slices.SortFunc(row.ParityFloors, compareDependencyFloors)
	}

	return row
}

// compareDependencyFloors orders poisoners by floor (highest first), then by
// module path for determinism.
func compareDependencyFloors(a, b DependencyFloor) int {
	switch {
	case surface.GreaterVersion(string(a.Floor), string(b.Floor)):
		return -1
	case surface.GreaterVersion(string(b.Floor), string(a.Floor)):
		return 1
	default:
		return strings.Compare(string(a.Module), string(b.Module))
	}
}

// parseFloorLine splits one `go list -m` line into its dependency floor,
// the module carrying it, and the module's released version. The main
// module itself and unreplaced development versions carry no floor here.
// Results, in order: floor, carrying module, version, ok.
func parseFloorLine(
	line string,
	module surface.ModulePath,
) (surface.GoVersion, surface.ModulePath, ModuleVersion, bool) {
	fields := strings.Split(line, "\t")

	if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[1] == string(module) {
		return "", "", "", false
	}

	if fields[2] == "" || fields[2] == "(devel)" {
		return "", "", "", false
	}

	return surface.GoVersion(fields[0]), surface.ModulePath(fields[1]), ModuleVersion(fields[2]), true
}
