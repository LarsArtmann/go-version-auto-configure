package surface

import (
	"fmt"
	"os"
	"strings"
)

// parseReleaseVersion validates and normalizes one release version as
// written in VERSION files and CHANGELOG headings: an optional v prefix,
// major.minor with an optional patch and prerelease ("v1.2", "0.7.0",
// "1.0.0-rc.1"). The v prefix is stripped so the two sources compare
// equal regardless of decoration. The bool is false for anything else:
// dynamic stamps ("dev", commit hashes) are legal release-file content
// and must stay silent rather than fire findings.
func parseReleaseVersion(raw string) (ReleaseVersion, bool) {
	normalized := strings.TrimPrefix(strings.TrimSpace(raw), "v")

	release, prerelease, hasPrerelease := strings.Cut(normalized, "-")

	parts := strings.Split(release, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", false
	}

	for _, part := range parts {
		if !allDigits(part) {
			return "", false
		}
	}

	if hasPrerelease && prerelease == "" {
		return "", false
	}

	return ReleaseVersion(normalized), true
}

// allDigits reports whether s is a non-empty run of ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// parseVersionStamp reads a root VERSION file and returns its release
// version. Unparseable content returns false (see parseReleaseVersion).
// Read errors also return false: WalkDir has just visited the file, so a
// failed read is a race not worth a finding.
func parseVersionStamp(path string) (ReleaseVersion, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	version, ok := parseReleaseVersion(string(content))
	if !ok {
		return "", false
	}

	return version, true
}

// parseChangelogTop reads a root CHANGELOG.md and returns the version of
// its topmost RELEASED section: the first `## [...]` heading whose bracket
// carries a recognizable version, skipping `[Unreleased]` and unversioned
// headings. The int is the 1-based line of that heading. A changelog
// without any versioned section returns false (the file is legal; there is
// simply nothing to compare).
func parseChangelogTop(path string) (ReleaseVersion, int, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", 0, false
	}

	for line, text := range strings.Split(string(content), "\n") {
		version, ok := changelogHeadingVersion(text)
		if !ok {
			continue
		}

		return version, line + 1, true
	}

	return "", 0, false
}

// changelogHeadingVersion extracts the release version from one
// Keep-a-Changelog section heading ("## [0.2.4] - 2026-10-03",
// "## [v4.16.0] — 2026-10-03"). Unversioned brackets ("[Unreleased]",
// "[0.2.4-rc1]" shapes that do not parse) do not match.
func changelogHeadingVersion(heading string) (ReleaseVersion, bool) {
	rest, ok := strings.CutPrefix(heading, "## [")
	if !ok {
		return "", false
	}

	bracket, _, closed := strings.Cut(rest, "]")
	if !closed {
		return "", false
	}

	return parseReleaseVersion(bracket)
}

// release lazily initializes the surface's release authority so either
// release file can be absorbed in walk order.
func (s *Surface) release() *ReleaseAuthority {
	if s.Release == nil {
		s.Release = &ReleaseAuthority{}
	}

	return s.Release
}

// releaseAuthorityIssues reports drift between the repository's pure-file
// release authorities. Both sources must carry a version: a repo that
// stamps only one (or neither) has nothing to reconcile, and the planned
// git-tag second pass — not this rule — owns the three-way comparison.
func releaseAuthorityIssues(surf *Surface) []Issue {
	ra := surf.Release
	if ra == nil || ra.VersionFile == "" || ra.ChangelogTop == "" {
		return nil
	}

	if ra.VersionFile == ra.ChangelogTop {
		return nil
	}

	return []Issue{{
		Rule: RuleReleaseAuthorityDrift,
		Message: fmt.Sprintf(
			"VERSION stamps %s but CHANGELOG.md's top released section is %s",
			ra.VersionFile,
			ra.ChangelogTop,
		),
		File: "VERSION",
		Line: 1,
		Suggestion: "decide which source is authoritative and update the other; " +
			"neither side is a mechanical rewrite (release policy is per-repo)",
	}}
}
