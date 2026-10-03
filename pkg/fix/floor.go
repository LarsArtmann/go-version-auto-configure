package fix

// Floor resolution for a gate-rejected go.mod rewrite when `go list -m`
// cannot (or will not) name the carrier: the tidy diff itself names the
// forced directive, the go tool's diagnostics name the forcer, and a
// vendored tree carries its floors in vendor/modules.txt annotations.
// This file owns the shared floor model and the authority walk every
// floor consumer (fix classification, who-forces) reduces from.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// addedGoLine matches a unified-diff line ADDING a go directive, as emitted
// by `go mod tidy -diff` when it would re-raise the go line:
//
//	+go 1.27.1
//
// The `+++ file` header cannot match (go must be followed by a version).
var addedGoLine = regexp.MustCompile(`(?m)^\+go[ \t]+(\S+)$`)

// versionToken restrains what counts as a go directive version; tidy and the
// vendor annotations write canonical forms, and refusing garbage keeps the
// comparisons below honest.
var versionToken = regexp.MustCompile(`^\d+\.\d+(\.\d+)?([a-z0-9]*)$`)

// forcerMention matches the go tool's diagnostic naming what forces a floor:
//
//	go: module ../../../go-output requires go >= 1.27.1; switching to go1.27.1
//	go: example.com/dep@v4.4.0 requires go >= 1.27.1; switching to go1.27.1
var forcerMention = regexp.MustCompile(
	`(?m)^(?:go:[ \t]+)?(?:module[ \t]+)?(\S+)[ \t]+requires[ \t]+go[ \t]+>=[ \t]+([0-9][0-9A-Za-z.\-]*)(?:;.*)?$`,
)

// modulesTxtHeaderFields is the field count of a modules.txt module header
// (`# path version`); headers with other shapes carry no usable identity.
const modulesTxtHeaderFields = 3

// forcedFloorFromTidyDiff extracts the go directive tidy would write from the
// stdout diff of `go mod tidy -diff`. A raised directive above the fix target
// means the rewrite is externally forced even when no listed module carries
// the floor: a replaced local module, or the standard library's own floor
// (e.g. encoding/json/v2 requiring the toolchain patch).
func forcedFloorFromTidyDiff(diff string) surface.GoVersion {
	var forced surface.GoVersion

	for _, match := range addedGoLine.FindAllStringSubmatch(diff, -1) {
		candidate := match[1]
		if !versionToken.MatchString(candidate) {
			continue
		}

		if forced == "" || surface.GreaterVersion(candidate, string(forced)) {
			forced = surface.GoVersion(candidate)
		}
	}

	return forced
}

// gateForcerMentions extracts the go tool diagnostics naming what requires a
// floor, keeping those matching the forced version. They identify the forcer
// when `go list -m` cannot: replace targets, vendored dependencies, standard
// library modules.
func gateForcerMentions(detail string, forced surface.GoVersion) []string {
	var mentions []string

	for _, match := range forcerMention.FindAllStringSubmatch(detail, -1) {
		if surface.GoVersion(match[2]) != forced {
			continue
		}

		mentions = append(mentions, match[1])
	}

	return mentions
}

// depForcedCause renders the cause of a forced floor that no listed
// dependency carries: the go tool's own diagnostics when they name the forcer
// (a replaced local module, a vendored dependency), the standard-library
// floor as the residual explanation otherwise.
func depForcedCause(detail string, forced surface.GoVersion) string {
	mentions := gateForcerMentions(detail, forced)

	if len(mentions) > 0 {
		return fmt.Sprintf(
			"go mod tidy re-raises the directive to go %s: forced by %s "+
				"(a replace target, vendored module, or standard-library requirement, not a listed dependency floor)",
			forced,
			strings.Join(mentions, "; "),
		)
	}

	return fmt.Sprintf(
		"go mod tidy re-raises the directive to go %s: no listed dependency carries the floor; "+
			"the standard library (e.g. encoding/json/v2) is the likely forcer",
		forced,
	)
}

// ModuleVersion is a released module version as listed by `go list -m`,
// e.g. "v1.10.0". A named type keeps module versions distinct from Go
// toolchain versions (GoVersion), which parse differently.
type ModuleVersion string

// DependencyFloor is one dependency with the `go` floor it declares: the
// shared row shape every floor authority (go list -m, vendor/modules.txt
// annotations) resolves into, and the row of both who-forces lists — a
// poisoned row's PoisonerFloors (dependencies forcing the floor ABOVE the
// directive) and an at-parity row's ParityFloors (dependencies holding the
// directive exactly where it is).
type DependencyFloor struct {
	// Module is the dependency's module path.
	Module surface.ModulePath `json:"module"`
	// Version is the dependency's released version, e.g. "v1.10.0".
	Version ModuleVersion `json:"version"`
	// Floor is the `go` directive the dependency declares.
	Floor surface.GoVersion `json:"floor"`
}

// develVersion marks a `go list -m` record pinned to a local working copy
// by a replace directive: it carries a floor tidy still enforces, but
// names no published version to re-tag.
const develVersion ModuleVersion = "(devel)"

// published reports whether a resolved floor names a released version — a
// re-taggable carrier. Development versions and empty versions are replaced
// local working copies or unresolvable records: their floors may still
// count toward enforcement, but they are never named as poisoners.
func (d DependencyFloor) published() bool {
	return d.Version != "" && d.Version != develVersion
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

// resolveDependencyFloors walks the floor authorities in order for one
// module directory: `go list -m` (plain, then error-tolerant for the
// untidy trees a rejected fix or a mid-fix consumer presents), then the
// vendor/modules.txt annotations a skewed vendor tree still answers with.
// The main module's own record never carries a dependency floor.
func resolveDependencyFloors(
	ctx context.Context,
	dir string,
	main surface.ModulePath,
	run GoCommandRunner,
) ([]DependencyFloor, FloorSource, error) {
	out, err := listDependencyFloors(ctx, dir, run)
	if err == nil {
		return parseDependencyFloors(out, main), FloorSourceList, nil
	}

	if vendored, ok := readVendorModuleFloors(dir); ok {
		return vendored, FloorSourceVendor, nil
	}

	return nil, "", fmt.Errorf("list dependency floors: %w", err)
}

// listDependencyFloors runs `go list -m` for every dependency's declared
// floor, one record per line in the GoVersion/Path/Version tab format. A
// plain list refuses an untidy graph — exactly the tree a rejected fix
// leaves behind — so the listing retries with -e, which tolerates load
// errors and still reports every module's recorded floor.
func listDependencyFloors(ctx context.Context, dir string, run GoCommandRunner) (string, error) {
	const format = "{{.GoVersion}}\t{{.Path}}\t{{.Version}}"

	out, err := run(ctx, dir, "list", "-m", "-f", format, "all")
	if err != nil {
		return run(ctx, dir, "list", "-m", "-e", "-f", format, "all")
	}

	return out, nil
}

// parseDependencyFloors splits `go list -m` output into dependency floors.
// Malformed lines, empty floors, and the main module's own record carry
// nothing a dependency forced. Development versions stay: a replaced local
// module's floor is real (tidy enforces it), so callers decide how devel
// rows reduce — counted-but-unnamed for fix classification, dropped for
// the who-forces matrix.
func parseDependencyFloors(out string, main surface.ModulePath) []DependencyFloor {
	floors := make([]DependencyFloor, 0)

	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			continue
		}

		floor, path, version := fields[0], fields[1], fields[2]
		if floor == "" || path == "" || path == string(main) {
			continue
		}

		floors = append(floors, DependencyFloor{
			Floor:   surface.GoVersion(floor),
			Module:  surface.ModulePath(path),
			Version: ModuleVersion(version),
		})
	}

	return floors
}

// maxDependencyFloor reduces resolved floors to the enforced floor and the
// dependencies carrying it, in resolution order.
func maxDependencyFloor(floors []DependencyFloor) (surface.GoVersion, []DependencyFloor) {
	var floor surface.GoVersion

	for _, entry := range floors {
		if floor == "" || surface.GreaterVersion(string(entry.Floor), string(floor)) {
			floor = entry.Floor
		}
	}

	carriers := make([]DependencyFloor, 0, len(floors))

	for _, entry := range floors {
		if entry.Floor == floor {
			carriers = append(carriers, entry)
		}
	}

	return floor, carriers
}

// readModulePath parses the module path out of the go.mod in dir. A missing
// or unparseable file yields "": the main-module record then simply stays
// in the listing, which only ever over-reports a floor the module itself
// already declares.
func readModulePath(dir string) surface.ModulePath {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}

	path, err := surface.ParseModulePath(surface.KindGoMod, data)
	if err != nil {
		return ""
	}

	return path
}

// vendorModuleFloors extracts every dependency go floor recorded in a
// vendor/modules.txt. Each stanza names the module on a `# path version`
// header and, for explicit requirements, the floor on a
// `## explicit; go X` annotation:
//
//	# github.com/x/dep v4.4.0
//	## explicit; go 1.27.1
//
// The annotations record exactly what the vendored graph requires, so they
// resolve floors when a skewed vendor directory makes `go list -m` refuse
// to load the module graph at all ("inconsistent vendoring").
func vendorModuleFloors(content string) []DependencyFloor {
	var (
		floors  = make([]DependencyFloor, 0)
		path    string
		version string
	)

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "# "):
			if fields := strings.Fields(line); len(fields) == modulesTxtHeaderFields {
				path, version = fields[1], fields[2]
			}
		case strings.HasPrefix(line, "## "):
			annotated, ok := annotationGoVersion(line)
			if !ok || path == "" {
				continue
			}

			floors = append(floors, DependencyFloor{
				Module:  surface.ModulePath(path),
				Version: ModuleVersion(version),
				Floor:   surface.GoVersion(annotated),
			})
		}
	}

	return floors
}

// annotationGoVersion pulls the `go X` suffix off a modules.txt annotation
// line (`## explicit; go 1.27.1`), reporting false when the annotation
// carries no version.
func annotationGoVersion(annotation string) (string, bool) {
	_, rest, found := strings.Cut(annotation, "; go ")
	if !found {
		return "", false
	}

	version := strings.TrimSpace(rest)
	if !versionToken.MatchString(version) {
		return "", false
	}

	return version, true
}

// readVendorModuleFloors reads the vendored dependency floors from the
// vendor/modules.txt under dir. ok is false when the file is absent or
// records no go annotations, leaving the verdict to the caller.
func readVendorModuleFloors(dir string) ([]DependencyFloor, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "vendor", "modules.txt"))
	if err != nil {
		return nil, false
	}

	floors := vendorModuleFloors(string(data))

	return floors, len(floors) > 0
}
