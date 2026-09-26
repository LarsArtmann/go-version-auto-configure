package fix

// Floor resolution for a gate-rejected go.mod rewrite when `go list -m`
// cannot (or will not) name the carrier: the tidy diff itself names the
// forced directive, the go tool's diagnostics name the forcer, and a
// vendored tree carries its floors in vendor/modules.txt annotations.

import (
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
			"go mod tidy re-raises the directive to go %s: forced by %s (a replace target, vendored module, or standard-library requirement, not a listed dependency floor)",
			forced,
			strings.Join(mentions, "; "),
		)
	}

	return fmt.Sprintf(
		"go mod tidy re-raises the directive to go %s: no listed dependency carries the floor; the standard library (e.g. encoding/json/v2) is the likely forcer",
		forced,
	)
}

// vendorAnnotationFloor extracts the highest dependency go floor recorded in
// a vendor/modules.txt, together with the vendored modules carrying it. Each
// stanza names the module on a `# path version` header and, for explicit
// requirements, the floor on a `## explicit; go X` annotation:
//
//	# github.com/x/dep v4.4.0
//	## explicit; go 1.27.1
//
// The annotations record exactly what the vendored graph requires, so they
// resolve the floor when a skewed vendor tree makes `go list -m` refuse to
// load the module graph at all.
func vendorAnnotationFloor(content string) (surface.GoVersion, []string) {
	var (
		floor    surface.GoVersion
		carriers = make([]string, 0, 2)
		path     string
		version  string
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

			switch {
			case floor == "" || surface.GreaterVersion(annotated, string(floor)):
				floor, carriers = surface.GoVersion(annotated), []string{path + "@" + version}
			case surface.GoVersion(annotated) == floor:
				carriers = append(carriers, path+"@"+version)
			}
		}
	}

	return floor, carriers
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

// readVendorAnnotationFloor resolves the dependency floor from the
// vendor/modules.txt under dir. ok is false when the file is absent or
// records no go annotations, leaving the verdict to the caller.
func readVendorAnnotationFloor(dir string) (floor surface.GoVersion, carriers []string, ok bool) {
	data, err := os.ReadFile(filepath.Join(dir, "vendor", "modules.txt"))
	if err != nil {
		return "", nil, false
	}

	floor, carriers = vendorAnnotationFloor(string(data))

	return floor, carriers, floor != ""
}
