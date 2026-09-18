package surface

import (
	"fmt"
	"strconv"
	"strings"
)

// majorMinor is a parsed major.minor toolchain version.
type majorMinor struct {
	Major int
	Minor int
}

// parseMajorMinor parses "1.26", "1.26.7", or "1.26.0" into its major.minor
// components. Patch components are accepted and ignored: policy treats the
// directive floor as major.minor only.
func parseMajorMinor(v string) (majorMinor, error) {
	parts := strings.Split(strings.TrimPrefix(v, "go"), ".")
	if len(parts) < 2 {
		return majorMinor{}, fmt.Errorf("version %q has no minor component", v)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return majorMinor{}, fmt.Errorf("version %q has non-numeric major: %w", v, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return majorMinor{}, fmt.Errorf("version %q has non-numeric minor: %w", v, err)
	}
	if major <= 0 || minor < 0 {
		return majorMinor{}, fmt.Errorf("version %q is not a positive major.minor", v)
	}
	return majorMinor{Major: major, Minor: minor}, nil
}

// hasPatch reports whether the version string carries a patch component
// (three or more dot-separated numeric parts). RC and expression forms are
// rejected: callers only pass module-parsed versions.
func hasPatch(v string) bool {
	return len(strings.Split(v, ".")) > 2
}

// GreaterVersion reports whether version a is a strictly higher Go version
// than b, comparing every dotted component (missing components are zero):
// "1.26.7" exceeds "1.26", and "1.27" exceeds "1.26.7". Unlike
// parseMajorMinor the patch component matters here, because the go tool
// lifts a directive to a dependency's exact patch floor. Versions that do
// not parse never exceed anything.
func GreaterVersion(a, b string) bool {
	aParts, aOK := versionParts(a)
	bParts, bOK := versionParts(b)

	if !aOK || !bOK {
		return false
	}

	for i := range max(len(aParts), len(bParts)) {
		ai, bi := partAt(aParts, i), partAt(bParts, i)

		if ai != bi {
			return ai > bi
		}
	}

	return false
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

// partAt returns the version component at index i, zero past the end.
func partAt(parts []int, i int) int {
	if i >= len(parts) {
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
	if v == "" || strings.Contains(v, "$") || strings.ContainsAny(v, "^~*x<>") || strings.ContainsFunc(v, func(r rune) bool {
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

// parseNixPin extracts a comparable major.minor from a nixpkgs Go
// reference: an attribute name (go_1_26) or a builder function name
// (buildGo126Module, 3-digit encoding of major.minor).
func parseNixPin(token string) (majorMinor, bool) {
	if digits, ok := strings.CutPrefix(token, "buildGo"); ok {
		digits = strings.TrimSuffix(digits, "Module")
		if len(digits) != 3 {
			return majorMinor{}, false
		}
		return parseTwoParts(digits[:1], digits[1:])
	}
	if rest, ok := strings.CutPrefix(token, "go_"); ok {
		parts := strings.Split(rest, "_")
		if len(parts) != 2 {
			return majorMinor{}, false
		}
		return parseTwoParts(parts[0], parts[1])
	}
	return majorMinor{}, false
}

// parseTwoParts converts two digit runs into a majorMinor.
func parseTwoParts(majorStr, minorStr string) (majorMinor, bool) {
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return majorMinor{}, false
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return majorMinor{}, false
	}
	return majorMinor{Major: major, Minor: minor}, true
}
