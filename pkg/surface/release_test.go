package surface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// releaseDriftIssues filters issues down to the release-authority rule.
func releaseDriftIssues(issues []Issue) []Issue {
	var matched []Issue

	for _, issue := range issues {
		if issue.Rule == RuleReleaseAuthorityDrift {
			matched = append(matched, issue)
		}
	}

	return matched
}

func TestParseReleaseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw      string
		want     ReleaseVersion
		wantJSON bool
	}{
		{raw: "0.7.0", want: "0.7.0", wantJSON: true},
		{raw: "v0.7.0", want: "0.7.0", wantJSON: true},
		{raw: " 1.2\n", want: "1.2", wantJSON: true},
		{raw: "4.16.0", want: "4.16.0", wantJSON: true},
		{raw: "1.0.0-rc.1", want: "1.0.0-rc.1", wantJSON: true},
		{raw: "dev", wantJSON: false},
		{raw: "", wantJSON: false},
		{raw: "1", wantJSON: false},
		{raw: "1.2.3.4", wantJSON: false},
		{raw: "v1.x", wantJSON: false},
		{raw: "1.2-", wantJSON: false},
	}

	for _, test := range tests {
		got, ok := parseReleaseVersion(test.raw)
		assert.Equal(t, test.wantJSON, ok, "parse %q", test.raw)
		assert.Equal(t, test.want, got, "normalize %q", test.raw)
	}
}

func TestDiscoverAndAnalyze_ReleaseAuthorityDriftFires(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"VERSION": "0.7.0",
		"CHANGELOG.md": "# Changelog\n\n" +
			"## [Unreleased]\n\n- something\n\n" +
			"## [v0.6.3] - 2026-09-01\n\n- older release\n",
		"go.mod": "module example.com/root\n\ngo 1.27\n",
	})

	surf, discoverIssues, err := Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discoverIssues)

	require.NotNil(t, surf.Release)
	assert.Equal(t, ReleaseVersion("0.7.0"), surf.Release.VersionFile)
	assert.Equal(t, ReleaseVersion("0.6.3"), surf.Release.ChangelogTop)
	assert.Equal(t, 7, surf.Release.ChangelogLine)

	issues := Analyze(surf)
	drift := releaseDriftIssues(issues)
	require.Len(t, drift, 1)

	assert.Equal(t, "VERSION", drift[0].File)
	assert.Equal(t, 1, drift[0].Line)
	assert.Nil(t, drift[0].Fix, "release drift is suggest-only")
	assert.Contains(t, drift[0].Message, "0.7.0")
	assert.Contains(t, drift[0].Message, "0.6.3")
	assert.NotEmpty(t, drift[0].Suggestion)
}

func TestDiscoverAndAnalyze_ReleaseAuthorityAgreementIsSilent(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"VERSION": "0.7.0",
		"CHANGELOG.md": "## [Unreleased]\n\n" +
			"## [v0.7.0] — 2026-10-03\n\n- the v prefix normalizes away\n",
		"go.mod": "module example.com/root\n\ngo 1.27\n",
	})

	surf, _, err := Discover(root)
	require.NoError(t, err)

	require.NotNil(t, surf.Release)
	assert.Equal(t, ReleaseVersion("0.7.0"), surf.Release.ChangelogTop)

	assert.Empty(t, releaseDriftIssues(Analyze(surf)))
}

func TestDiscoverAndAnalyze_SingleReleaseSourceIsSilent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		files map[string]string
	}{
		"version file only": {
			files: map[string]string{
				"VERSION": "0.7.0",
				"go.mod":  "module example.com/root\n\ngo 1.27\n",
			},
		},
		"changelog only": {
			files: map[string]string{
				"CHANGELOG.md": "## [Unreleased]\n\n## [0.6.3] - 2026-09-01\n",
				"go.mod":       "module example.com/root\n\ngo 1.27\n",
			},
		},
		"dynamic stamp": {
			files: map[string]string{
				"VERSION":      "dev\n",
				"CHANGELOG.md": "## [0.6.3] - 2026-09-01\n",
				"go.mod":       "module example.com/root\n\ngo 1.27\n",
			},
		},
		"unversioned changelog": {
			files: map[string]string{
				"VERSION":      "0.7.0",
				"CHANGELOG.md": "# Notes\n\nno sections at all\n",
				"go.mod":       "module example.com/root\n\ngo 1.27\n",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			surf, _, err := Discover(writeRepo(t, test.files))
			require.NoError(t, err)

			assert.Empty(t, releaseDriftIssues(Analyze(surf)))
		})
	}
}

func TestDiscover_NestedReleaseFilesIgnored(t *testing.T) {
	t.Parallel()

	root := writeRepo(t, map[string]string{
		"VERSION":      "0.7.0",
		"CHANGELOG.md": "## [0.7.0] - 2026-10-03\n",
		"docs/VERSION": "9.9.9",
		"sub/go.mod":   "module example.com/sub\n\ngo 1.27\n",
		"go.mod":       "module example.com/root\n\ngo 1.27\n",
		"go.work":      "go 1.27\n\nuse .\nuse ./sub\n",
	})

	surf, _, err := Discover(root)
	require.NoError(t, err)

	require.NotNil(t, surf.Release)
	assert.Equal(t, ReleaseVersion("0.7.0"), surf.Release.VersionFile)
	assert.Equal(t, ReleaseVersion("0.7.0"), surf.Release.ChangelogTop)
	assert.Empty(t, releaseDriftIssues(Analyze(surf)))
}
