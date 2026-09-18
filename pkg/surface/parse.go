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
