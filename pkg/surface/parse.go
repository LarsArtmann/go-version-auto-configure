package surface

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// majorMinor is a parsed major.minor toolchain version.
type majorMinor struct {
	Major int
	Minor int
}

// Version-shape constants: a major.minor version has two dot-separated
// parts (a patch-form one has more) and the buildGoNNNModule encoding
// carries a three-digit minor.
const (
	majorMinorParts = 2
	patchPartIndex  = 2
	builderDigits   = 3
)

// parseMajorMinor sentinel errors, wrapped with the offending value.
var (
	errNoMinorComponent = errors.New("version has no minor component")
	errNonNumericMajor  = errors.New("version has non-numeric major")
	errNonNumericMinor  = errors.New("version has non-numeric minor")
	errNotPositive      = errors.New("version is not a positive major.minor")
)

// parseMajorMinor parses "1.26", "1.26.7", or "1.26.0" into its major.minor
// components. Patch components are accepted and ignored: policy treats the
// directive floor as major.minor only.
func parseMajorMinor(v string) (majorMinor, error) {
	parts := strings.Split(strings.TrimPrefix(v, "go"), ".")

	if len(parts) < majorMinorParts {
		return majorMinor{}, fmt.Errorf("%w: %q", errNoMinorComponent, v)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return majorMinor{}, fmt.Errorf("%w: %q", errNonNumericMajor, v)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return majorMinor{}, fmt.Errorf("%w: %q", errNonNumericMinor, v)
	}

	if major <= 0 || minor < 0 {
		return majorMinor{}, fmt.Errorf("%w: %q", errNotPositive, v)
	}

	return majorMinor{Major: major, Minor: minor}, nil
}

// hasPatch reports whether the version string carries a patch component
// (three or more dot-separated numeric parts). RC and expression forms are
// rejected: callers only pass module-parsed versions.
func hasPatch(v string) bool {
	return len(strings.Split(v, ".")) > majorMinorParts
}

// HasPatch reports whether the version string carries a patch component
// (three or more dot-separated numeric parts). RC and expression forms are
// rejected: callers only pass module-parsed versions.
func HasPatch(v GoVersion) bool {
	return hasPatch(string(v))
}

// MinorForm returns the major.minor form of the version ("1.26.7" and
// "1.26" both yield "1.26"). Unparseable versions are returned unchanged.
func MinorForm(v GoVersion) GoVersion {
	parsed, err := parseMajorMinor(string(v))
	if err != nil {
		return v
	}

	return GoVersion(parsed.String())
}

// GreaterVersion reports whether version a is a strictly higher Go version
// than b, ranked the way the go tool ranks directives: full patch
// granularity, with the bare minor ranking BELOW every patch form of the
// same minor (go 1.26 < go 1.26.0 < go 1.26.7 — probed against the go
// tool 2026-09-22). Unparseable versions never exceed anything.
func GreaterVersion(a, b string) bool {
	return CompareDirective(GoVersion(a), GoVersion(b)) > 0
}

// CompareDirective orders two Go version strings under the go tool's
// directive ranking. Returns -1 when a ranks below b, 0 when equal, +1
// when a ranks above b; comparisons involving unparseable inputs return 0.
func CompareDirective(a, b GoVersion) int {
	aParts, aOK := versionParts(string(a))
	bParts, bOK := versionParts(string(b))

	if !aOK || !bOK {
		return 0
	}

	for i := range max(len(aParts), len(bParts)) {
		ai, bi := directivePartAt(aParts, i), directivePartAt(bParts, i)

		if ai != bi {
			if ai > bi {
				return 1
			}

			return -1
		}
	}

	return 0
}

// versionParts parses a Go version into its dotted numeric components,
// ignoring the "go" prefix. ok is false for non-numeric versions.
func versionParts(v string) ([]int, bool) {
	fields := strings.Split(strings.TrimPrefix(v, "go"), ".")
	parts := make([]int, 0, len(fields))

	for _, field := range fields {
		n, err := strconv.Atoi(field)

		if err != nil || n < 0 {
			return nil, false
		}

		parts = append(parts, n)
	}

	return parts, true
}

// directivePartAt returns the version component at index i for go-tool
// directive ranking. The patch component (index 2) defaults to -1 when
// absent: the go tool ranks the bare minor (go 1.26) below go 1.26.0,
// because a directive of go 1.26.0 requires a toolchain that go 1.26 does
// not guarantee at patch granularity.
func directivePartAt(parts []int, i int) int {
	if i >= len(parts) {
		if i == patchPartIndex {
			return -1
		}

		return 0
	}

	return parts[i]
}

// greaterThan orders by major, then minor.
func (m majorMinor) greaterThan(o majorMinor) bool {
	if m.Major != o.Major {
		return m.Major > o.Major
	}

	return m.Minor > o.Minor
}

// lessThan orders by major, then minor.
func (m majorMinor) lessThan(o majorMinor) bool {
	return o.greaterThan(m)
}

// String renders "major.minor".
func (m majorMinor) String() string {
	return fmt.Sprintf("%d.%d", m.Major, m.Minor)
}

// parseCIPin extracts a comparable major.minor from a CI go-version value.
// Expressions (${{ … }}), ranges (1.26.x, ^1.26), and words (stable) return
// false: they are not comparable floors, so no alignment rule fires on them.
func parseCIPin(raw string) (majorMinor, bool) {
	v := strings.Trim(raw, `"' `+"`")
	if v == "" || strings.Contains(v, "$") || strings.ContainsAny(v, "^~*x<>") ||
		strings.ContainsFunc(v, func(r rune) bool {
			return (r < '0' || r > '9') && r != '.'
		}) {
		return majorMinor{}, false
	}

	parsed, err := parseMajorMinor(v)
	if err != nil {
		return majorMinor{}, false
	}

	return parsed, true
}

// nixGoRef is a nixpkgs reference to a Go toolchain: an attribute name
// (go_1_26) or a builder function name (buildGo126Module).
type nixGoRef string

// digitRun is a run of ASCII digits matched by one of the pin regexes.
type digitRun string

// parseNixPin extracts a comparable major.minor from a nixpkgs Go
// reference: an attribute name (go_1_26) or a builder function name
// (buildGo126Module, 3-digit encoding of major.minor).
func parseNixPin(token nixGoRef) (majorMinor, bool) {
	if digits, ok := strings.CutPrefix(string(token), "buildGo"); ok {
		digits = strings.TrimSuffix(digits, "Module")

		if len(digits) != builderDigits {
			return majorMinor{}, false
		}

		return parseTwoParts(digitRun(digits[:1]), digitRun(digits[1:]))
	}

	if rest, ok := strings.CutPrefix(string(token), "go_"); ok {
		parts := strings.Split(rest, "_")

		if len(parts) != majorMinorParts {
			return majorMinor{}, false
		}

		return parseTwoParts(digitRun(parts[0]), digitRun(parts[1]))
	}

	return majorMinor{}, false
}

// parseTwoParts converts two digit runs into a majorMinor.
func parseTwoParts(majorStr, minorStr digitRun) (majorMinor, bool) {
	major, err := strconv.Atoi(string(majorStr))
	if err != nil {
		return majorMinor{}, false
	}

	minor, err := strconv.Atoi(string(minorStr))
	if err != nil {
		return majorMinor{}, false
	}

	return majorMinor{Major: major, Minor: minor}, true
}
