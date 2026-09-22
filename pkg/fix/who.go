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

// PoisonerFloor is one dependency whose own `go` floor exceeds the module's
// declared directive: it forces the floor upward on tidy. Unlike Poisoners
// (which names only the carriers of the single highest floor), this list
// carries every forcing dependency with its own floor.
type PoisonerFloor struct {
	// Module is the dependency's module path.
	Module surface.ModulePath `json:"module"`
	// Version is the dependency's released version, e.g. "v1.10.0".
	Version ModuleVersion `json:"version"`
	// Floor is the `go` directive the dependency declares.
	Floor surface.GoVersion `json:"floor"`
}

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
	PoisonerFloors []PoisonerFloor `json:"poisonerFloors,omitempty"`
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
// floor matrix row.
func floorsForModule(ctx context.Context, root string, m surface.ModuleDirective, run GoCommandRunner) ModuleFloors {
	row := ModuleFloors{Path: m.Path, Kind: m.Kind, Module: m.Module, Directive: m.Version}

	dir := filepath.Join(root, filepath.Dir(m.Path))

	out, err := run(ctx, dir, "list", "-m", "-f", "{{.GoVersion}}\t{{.Path}}\t{{.Version}}", "all")
	if err != nil {
		row.Error = FailureCause(err.Error())

		return row
	}

	var forcers []PoisonerFloor

	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		floor, dep, version, ok := parseFloorLine(line, m.Module)

		if !ok {
			continue
		}

		if row.MaxDepFloor == "" || surface.GreaterVersion(string(floor), string(row.MaxDepFloor)) {
			row.MaxDepFloor = floor
		}

		if surface.GreaterVersion(string(floor), string(row.Directive)) {
			forcers = append(forcers, PoisonerFloor{Module: dep, Version: version, Floor: floor})
		}
	}

	row.Poisoned = surface.GreaterVersion(string(row.MaxDepFloor), string(row.Directive))

	if row.Poisoned {
		row.PoisonerFloors = forcers
		slices.SortFunc(row.PoisonerFloors, comparePoisonerFloors)
	}

	return row
}

// comparePoisonerFloors orders poisoners by floor (highest first), then by
// module path for determinism.
func comparePoisonerFloors(a, b PoisonerFloor) int {
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
