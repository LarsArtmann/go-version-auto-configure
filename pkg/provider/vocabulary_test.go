package provider

import (
	"testing"

	"github.com/larsartmann/go-finding"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
)

// TestVocabularyFor pins the rule→Category/Tag mapping (plan M26): every
// consumer rule classifies into exactly one go-finding category and carries
// its consumer taxonomy class as a tag, so the M7b bridge wiring has one
// tested decision point instead of an ad-hoc switch at adoption time.
func TestVocabularyFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rule      surface.Rule
		category  finding.Category
		class_tag finding.Tag
	}{
		{surface.RuleGoDirectivePatchForm, finding.CategoryStyle, tagForm},
		{surface.RuleWorkDirectivePatchForm, finding.CategoryStyle, tagForm},
		{surface.RuleCIPinPatchForm, finding.CategoryStyle, tagForm},
		{surface.RuleToolchainBelowDirective, finding.CategoryStyle, tagForm},
		{surface.RuleToolchainNonVersion, finding.CategoryStyle, tagForm},
		{surface.RuleNixPinBelowFloor, finding.CategoryConfiguration, tagAlignment},
		{surface.RuleCIPinBelowFloor, finding.CategoryConfiguration, tagAlignment},
		{surface.RuleGoWorkBelowFloor, finding.CategoryConfiguration, tagAlignment},
		{surface.RuleMinorExceedsExpectation, finding.CategoryConfiguration, tagAlignment},
		{surface.RuleReleaseAuthorityDrift, finding.CategoryBestPractice, tagDrift},
		{surface.RuleReleaseTagDrift, finding.CategoryBestPractice, tagDrift},
		{surface.RuleGoModUnparseable, finding.CategoryCorrectness, tagDiscovery},
		{surface.RuleGoWorkUnparseable, finding.CategoryCorrectness, tagDiscovery},
	}

	for _, test := range tests {
		category, tags := vocabularyFor(test.rule)

		assert.Equal(t, test.category, category, "category for %s", test.rule)
		assert.Contains(t, tags, test.class_tag, "taxonomy tag for %s", test.rule)
		assert.Len(t, tags, 2, "every rule carries exactly its category tag + class tag: %s", test.rule)
	}
}
