package provider

import (
	"github.com/larsartmann/go-finding"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// The consumer taxonomy classes, as finding tags. go-finding's Tag is a
// string type and custom tags are legal vocabulary, so the domain classes
// ride beside the predefined ones.
const (
	tagForm      finding.Tag = "form"
	tagAlignment finding.Tag = "alignment"
	tagDrift     finding.Tag = "drift"
	tagDiscovery finding.Tag = "discovery"
)

// vocabulary maps the consumer's rule taxonomy onto go-finding's Category
// and Tag vocabulary (plan M26). The bridge's ConfigIssue carries neither
// field yet (plan M7 upstream ask), so the mapping lives here as the single
// decision point M7b wires into toConfigIssue once the fields exist — the
// taxonomy decision should not wait on, or be scattered across, the bridge
// work.
//
// Mapping policy: form rules are style (the value is a normalized shape,
// not a behavioral defect); alignment rules are configuration (which side
// of a floor moves is environment configuration); drift rules are
// best-practice (release bookkeeping); unparseable files are correctness
// (the file contradicts its own format). Every finding is additionally
// tagged with its consumer taxonomy class (form/alignment/drift/discovery)
// so fleet consumers can slice by the domain classification, not just the
// generic category.
func vocabularyFor(rule surface.Rule) (finding.Category, []finding.Tag) {
	var tags []finding.Tag

	switch rule {
	case surface.RuleGoDirectivePatchForm, surface.RuleWorkDirectivePatchForm,
		surface.RuleCIPinPatchForm, surface.RuleToolchainBelowDirective,
		surface.RuleToolchainNonVersion:
		tags = []finding.Tag{finding.TagStyle, tagForm}

		return finding.CategoryStyle, tags
	case surface.RuleNixPinBelowFloor, surface.RuleCIPinBelowFloor,
		surface.RuleGoWorkBelowFloor, surface.RuleMinorExceedsExpectation:
		tags = []finding.Tag{finding.TagBuild, tagAlignment}

		return finding.CategoryConfiguration, tags
	case surface.RuleReleaseAuthorityDrift, surface.RuleReleaseTagDrift:
		tags = []finding.Tag{finding.TagDocumentation, tagDrift}

		return finding.CategoryBestPractice, tags
	case surface.RuleGoModUnparseable, surface.RuleGoWorkUnparseable:
		tags = []finding.Tag{finding.TagCorrectness, tagDiscovery}

		return finding.CategoryCorrectness, tags
	}

	return finding.CategoryConfiguration, tags
}
