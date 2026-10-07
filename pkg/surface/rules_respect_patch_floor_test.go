package surface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// respectOption builds the respect-patch-floor option for tests; the value
// is a literal constant per case, so a parse failure is a test bug.
func respectOption(t *testing.T, installed string) AnalyzeOption {
	t.Helper()

	opt, err := WithRespectPatchFloor(installed)
	require.NoError(t, err)

	return opt
}

func TestWithRespectPatchFloor_ValidatesValue(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"1.27.1", "go1.27.1", "1.27"} {
		_, err := WithRespectPatchFloor(ok)
		assert.NoError(t, err, "version %q must parse", ok)
	}

	for _, bad := range []string{"", "go", "1", "x.y.z", "stable"} {
		_, err := WithRespectPatchFloor(bad)
		assert.Error(t, err, "version %q must be rejected", bad)
	}
}

// TestAnalyze_RespectPatchFloor covers the detect side of the patch-floor
// policy: with the installed toolchain declared, patch-form `go` directives
// at or below it stop being drift; above it they stay flagged — they break
// builds on exactly that host.
func TestAnalyze_RespectPatchFloor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		directive   GoVersion
		kind        DirectiveKind
		installed   string
		respect     bool
		wantFlagged bool
	}{
		{
			name:        "pin below installed is respected",
			directive:   "1.26.7",
			kind:        KindGoMod,
			installed:   "1.27.1",
			respect:     true,
			wantFlagged: false,
		},
		{
			name:        "pin equal installed is respected",
			directive:   "1.27.1",
			kind:        KindGoMod,
			installed:   "go1.27.1",
			respect:     true,
			wantFlagged: false,
		},
		{
			name:        "pin above installed at patch granularity stays drift",
			directive:   "1.27.1",
			kind:        KindGoMod,
			installed:   "1.27.0",
			respect:     true,
			wantFlagged: true,
		},
		{
			name:        "pin above installed at minor granularity stays drift",
			directive:   "1.28.1",
			kind:        KindGoMod,
			installed:   "1.27.1",
			respect:     true,
			wantFlagged: true,
		},
		{
			name:        "without the option every patch pin is drift",
			directive:   "1.26.7",
			kind:        KindGoMod,
			installed:   "1.27.1",
			wantFlagged: true,
		},
		{
			name:        "go.work patch form at or below installed is respected",
			directive:   "1.26.7",
			kind:        KindGoWork,
			installed:   "1.27.1",
			respect:     true,
			wantFlagged: false,
		},
		{
			name:        "go.work patch form above installed stays drift",
			directive:   "1.27.2",
			kind:        KindGoWork,
			installed:   "1.27.1",
			respect:     true,
			wantFlagged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := "go.mod"
			if tt.kind == KindGoWork {
				path = "go.work"
			}

			s := &Surface{
				Root: "/repo",
				Modules: []ModuleDirective{{
					Path:    path,
					Kind:    tt.kind,
					Module:  "example.com/t",
					Version: tt.directive,
					Line:    3,
				}},
			}

			var opts []AnalyzeOption
			if tt.respect {
				opts = append(opts, respectOption(t, tt.installed))
			}

			rules := issueRules(Analyze(s, opts...))

			assert.Equal(t, tt.wantFlagged,
				containsRule(rules, formRule(tt.kind)),
				"rules: %v (respect=%v installed=%s directive=%s)",
				rules, tt.respect, tt.installed, tt.directive)
		})
	}
}

// containsRule reports whether rules contains want; assert.Contains needs a
// comparable element type and Rule is a string alias, but an explicit helper
// keeps the failure message ours.
func containsRule(rules []Rule, want Rule) bool {
	for _, r := range rules {
		if r == want {
			return true
		}
	}

	return false
}

// TestAnalyze_RespectPatchFloor_KeepsOtherRules verifies the option scopes
// to the form rules: a Nix pin below the floor and a dep-forced go.work
// situation still report as usual.
func TestAnalyze_RespectPatchFloor_KeepsOtherRules(t *testing.T) {
	t.Parallel()

	s := &Surface{
		Root: "/repo",
		Modules: []ModuleDirective{{
			Path:    "go.mod",
			Kind:    KindGoMod,
			Module:  "example.com/t",
			Version: "1.26.7",
			Line:    3,
		}},
		NixPins: []Pin{{
			Path:    "flake.nix",
			Version: "1.26",
			Raw:     "go_1_26",
			Source:  PinNixFlake,
		}},
	}

	opt := respectOption(t, "1.27.1")

	rules := issueRules(Analyze(s, opt))

	assert.NotContains(t, rules, RuleGoDirectivePatchForm,
		"the patch pin at or below the installed toolchain is policy, not drift")
	assert.Contains(t, rules, RuleNixPinBelowFloor,
		"the option scopes to form rules; floor alignment still reports")
}

// TestAnalyze_RespectPatchFloor_GoWorkRequiredPatchStaysSilent pins the
// dep-forced interplay: a go.work patch form the module floor REQUIRES is
// silent with and without the option (existing rule), and a respect option
// pointing below the floor keeps the below-floor finding — the option never
// suppresses RuleGoWorkBelowFloor.
func TestAnalyze_RespectPatchFloor_GoWorkRequiredPatchStaysSilent(t *testing.T) {
	t.Parallel()

	s := &Surface{
		Root: "/repo",
		Modules: []ModuleDirective{
			{
				Path:    "go.mod",
				Kind:    KindGoMod,
				Module:  "example.com/t",
				Version: "1.26.7",
				Line:    3,
			},
			{
				Path:    "go.work",
				Kind:    KindGoWork,
				Version: "1.26.7",
				Line:    1,
			},
		},
	}

	opt := respectOption(t, "1.27.1")

	rules := issueRules(Analyze(s, opt))

	assert.NotContains(t, rules, RuleWorkDirectivePatchForm,
		"the floor requires the patch form; the strip was never offered")
	assert.NotContains(t, rules, RuleGoWorkBelowFloor,
		"go.work 1.26.7 covers the full module floor 1.26.7")
	assert.NotContains(t, rules, RuleGoDirectivePatchForm,
		"the go.mod patch pin is at or below the installed toolchain")
}
