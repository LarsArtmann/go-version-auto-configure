package fix

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
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

// floorsForModule resolves one module's dependency floors through the
// shared authority walk and extracts its floor matrix row. Unreleased
// records ((devel) replacements) carry no floor here: the matrix answers
// which published tags force the directive, and there is nothing to re-tag
// in a local working copy.
func floorsForModule(ctx context.Context, root string, m surface.ModuleDirective, run GoCommandRunner) ModuleFloors {
	row := ModuleFloors{Path: m.Path, Kind: m.Kind, Module: m.Module, Directive: m.Version}

	dir := filepath.Join(root, filepath.Dir(m.Path))

	resolved, source, err := resolveDependencyFloors(ctx, dir, m.Module, run)
	if err != nil {
		row.Error = FailureCause(err.Error())

		return row
	}

	var forcers, parity []DependencyFloor

	for _, entry := range resolved {
		if !entry.published() {
			continue
		}

		accumulateFloor(&row, &forcers, &parity, entry.Floor, entry.Module, entry.Version)
	}

	row.Source = source

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
