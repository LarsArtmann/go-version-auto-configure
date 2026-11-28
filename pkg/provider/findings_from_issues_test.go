package provider

import (
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindingsFromIssues(t *testing.T) {
	t.Parallel()

	t.Run("converts every issue", func(t *testing.T) {
		t.Parallel()

		findings, err := FindingsFromIssues(ToolName, []surface.Issue{
			{Rule: surface.RuleGoModUnparseable, Message: "go.mod: unparseable", File: "go.mod", Line: 3},
			{
				Rule:    surface.RuleGoDirectivePatchForm,
				Message: "go directive carries a patch version",
				File:    "go.mod",
				Line:    5,
			},
		})
		require.NoError(t, err)
		require.Len(t, findings, 2)
	})

	t.Run("an issue the bridge refuses fails the conversion instead of vanishing", func(t *testing.T) {
		t.Parallel()

		findings, err := FindingsFromIssues(ToolName, []surface.Issue{{File: "go.mod", Line: 3}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "convert go.mod:3")
		assert.Nil(t, findings)
	})
}
