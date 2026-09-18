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
	// Module is the module path declared in the go.mod.
	Module string `json:"module"`
	// Directive is the declared `go` directive ("" when none).
	Directive string `json:"directive,omitempty"`
	// MaxDepFloor is the highest `go` floor any dependency declares
	// ("" when no dependency declares one).
	MaxDepFloor string `json:"max_dep_floor,omitempty"`
	// Poisoners names the dependencies carrying MaxDepFloor when that floor
	// exceeds the directive (Poisoned); empty otherwise.
	Poisoners []string `json:"poisoners,omitempty"`
	// Poisoned reports whether `go mod tidy` would re-raise the directive:
	// the dependency floor exceeds the declared directive.
	Poisoned bool `json:"poisoned"`
	// Error is non-empty when the module's dependency graph could not be
	// listed; the other fields except Path and Module are unreliable then.
	Error string `json:"error,omitempty"`
}

// AnalyzeFloors resolves, for every go.mod under root, the highest `go`
// floor its dependencies declare and the dependencies carrying it — the
// poisoner matrix behind dep-forced fixes. Workspace files are skipped:
// they declare no dependencies. go list failures are recorded per module
// in Error; a failed Discover aborts.
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
			continue
		}

		rows = append(rows, floorsForModule(ctx, root, m, run))
	}

	return rows, nil
}

// floorsForModule lists one module's dependency graph and extracts its
// floor matrix row.
func floorsForModule(ctx context.Context, root string, m surface.ModuleDirective, run GoCommandRunner) ModuleFloors {
	row := ModuleFloors{Path: m.Path, Module: m.Module, Directive: m.Version}

	dir := filepath.Join(root, filepath.Dir(m.Path))

	out, err := run(ctx, dir, "list", "-m", "-f", "{{.GoVersion}}\t{{.Path}}\t{{.Version}}", "all")

	if err != nil {
		row.Error = err.Error()

		return row
	}

	floors := map[string][]string{}

	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		floor, _, entry, ok := parseFloorLine(line, m.Module)

		if !ok {
			continue
		}

		floors[floor] = append(floors[floor], entry)

		if row.MaxDepFloor == "" || surface.GreaterVersion(floor, row.MaxDepFloor) {
			row.MaxDepFloor = floor
		}
	}

	row.Poisoned = surface.GreaterVersion(row.MaxDepFloor, row.Directive)

	if row.Poisoned {
		row.Poisoners = floors[row.MaxDepFloor]
		slices.Sort(row.Poisoners)
	}

	return row
}

// parseFloorLine splits one `go list -m` line into its dependency floor,
// the module carrying it, and a printable "path@version" entry. The main
// module itself and unreplaced development versions carry no floor here.
func parseFloorLine(line, module string) (floor, carry, entry string, ok bool) {
	fields := strings.Split(line, "\t")

	if len(fields) != 3 || fields[0] == "" || fields[1] == "" || fields[1] == module {
		return "", "", "", false
	}

	if fields[2] == "" || fields[2] == "(devel)" {
		return "", "", "", false
	}

	return fields[0], fields[1], fields[1] + "@" + fields[2], true
}
