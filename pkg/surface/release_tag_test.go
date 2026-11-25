package surface

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReleaseTagIssues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		versionFile  ReleaseVersion
		changelogTop ReleaseVersion
		tag          ReleaseVersion
		wantFires    bool
	}{
		{
			name:         "live case: files agree, tag trails",
			versionFile:  "0.7.0",
			changelogTop: "0.7.0",
			tag:          "0.2.0",
			wantFires:    true,
		},
		{
			name:         "files agree, tag ahead (pre-release tags moved on)",
			versionFile:  "0.7.0",
			changelogTop: "0.7.0",
			tag:          "0.8.0",
			wantFires:    true,
		},
		{
			name:         "all three agree",
			versionFile:  "0.7.0",
			changelogTop: "0.7.0",
			tag:          "0.7.0",
			wantFires:    false,
		},
		{
			name:         "files disagree: first pass owns it",
			versionFile:  "0.7.0",
			changelogTop: "0.2.0",
			tag:          "0.1.0",
			wantFires:    false,
		},
		{
			name:         "missing tag source",
			versionFile:  "0.7.0",
			changelogTop: "0.7.0",
			tag:          "",
			wantFires:    false,
		},
		{
			name:         "missing version file",
			versionFile:  "",
			changelogTop: "0.7.0",
			tag:          "0.2.0",
			wantFires:    false,
		},
		{
			name:         "missing changelog",
			versionFile:  "0.7.0",
			changelogTop: "",
			tag:          "0.2.0",
			wantFires:    false,
		},
		{
			name:         "two-component equivalence is not drift",
			versionFile:  "0.7",
			changelogTop: "0.7.0",
			tag:          "0.7.0",
			wantFires:    false,
		},
	}

	for _, test := range tests {
		issues := ReleaseTagIssues(test.versionFile, test.changelogTop, test.tag)
		if test.wantFires {
			assert.Len(t, issues, 1, "%s: expected exactly one finding", test.name)

			if len(issues) == 1 {
				assert.Equal(t, RuleReleaseTagDrift, issues[0].Rule, test.name)
				assert.Nil(t, issues[0].Fix, "%s: must stay suggest-only", test.name)
				assert.Contains(
					t,
					issues[0].Message,
					string(test.versionFile),
					"%s: message names the file stamp",
					test.name,
				)
				assert.Contains(t, issues[0].Message, string(test.tag), "%s: message names the tag", test.name)
			}

			continue
		}

		assert.Empty(t, issues, test.name)
	}
}
