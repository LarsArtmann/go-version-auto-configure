package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-finding"
	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	autoconfigure "github.com/larsartmann/linter-autoconfigure-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errUnreachableListing is the canned listing failure proving the
// annotation path never runs where it must not.
var errUnreachableListing = errors.New("go: exit 1: unreachable")

// errFakeListFailure is the canned go list failure for degraded-annotation
// fixtures.
var errFakeListFailure = errors.New("go: exit 1: missing go.sum entry")

// fakeFloorsList answers every `go list` with the given records, one per
// line in the GoVersion/Path/Version tab format the floor resolution
// requests; the production path shells out through fix.EditRunner instead.
func fakeFloorsList(out string) fix.GoCommandRunner {
	return func(_ context.Context, _ string, _ ...string) (string, error) {
		return out, nil
	}
}

// failingFloorsList rejects every listing: the dep-forced annotation must
// degrade to a plain finding, never an error.
func failingFloorsList(err error) fix.GoCommandRunner {
	return func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", err
	}
}

// requirePatchFormIssue returns the single go-directive-patch-form issue.
func requirePatchFormIssue(
	t *testing.T,
	issues []autoconfigure.ConfigIssue,
) autoconfigure.ConfigIssue {
	t.Helper()

	for _, issue := range issues {
		if string(issue.Rule) == "go-directive-patch-form" {
			return issue
		}
	}

	t.Fatal("no go-directive-patch-form issue")

	return autoconfigure.ConfigIssue{}
}

// TestAnalyze_HeldPatchFormExplainsItself pins the BuildFlow-facing answer
// to "why was this not auto-fixed": the finding names the dependencies
// holding the floor and suggests the supply-side remediation instead of a
// rewrite tidy would silently revert.
func TestAnalyze_HeldPatchFormExplainsItself(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := fakeFloorsList(strings.Join([]string{
		"1.26.7\texample.com/m\t(devel)",
		"1.26.7\tgithub.com/x/dep\tv1.0.0",
	}, "\n"))

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, run)
	require.NoError(t, err)

	issue := requirePatchFormIssue(t, issues)
	assert.Contains(t, issue.Message, "not auto-fixed: go mod tidy reverts the strip")
	assert.Contains(t, issue.Message, "github.com/x/dep@v1.0.0")
	assert.Contains(t, issue.Suggestion, "re-tag github.com/x/dep@v1.0.0")
	assert.NotContains(t, issue.Suggestion, "rewrite", "the withheld rewrite must not pose as the fix")
	assert.Equal(t, finding.SeverityWarning, issue.Severity, "advisory findings stay warning-severity")
}

func TestAnalyze_HeldPoisonedPatchFormNamesForcer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := fakeFloorsList(strings.Join([]string{
		"1.26.7\texample.com/m\t(devel)",
		"1.27.1\tgithub.com/x/dep\tv1.0.0",
	}, "\n"))

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, run)
	require.NoError(t, err)

	issue := requirePatchFormIssue(t, issues)
	assert.Contains(t, issue.Message, "dependencies force go 1.27.1 above the declared directive")
	assert.Contains(t, issue.Message, "github.com/x/dep@v1.0.0 (go 1.27.1)")
	assert.Contains(t, issue.Suggestion, "go mod edit -go=1.27.1")
}

func TestAnalyze_UnheldPatchFormKeepsRewriteSuggestion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := fakeFloorsList(strings.Join([]string{
		"1.26.7\texample.com/m\t(devel)",
		"1.26\tgithub.com/x/dep\tv1.0.0",
	}, "\n"))

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, run)
	require.NoError(t, err)

	issue := requirePatchFormIssue(t, issues)
	assert.NotContains(t, issue.Message, "not auto-fixed", "nothing holds the strip, the fix applies")
	assert.Contains(t, issue.Suggestion, "rewrite", "the mechanical rewrite stays the suggestion")
}

func TestAnalyze_FloorListingFailureKeepsPlainFinding(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, failingFloorsList(errFakeListFailure))
	require.NoError(t, err, "a failed enrichment must not red detection")

	issue := requirePatchFormIssue(t, issues)
	assert.NotContains(t, issue.Message, "not auto-fixed")
	assert.Contains(t, issue.Suggestion, "rewrite")
}

func TestAnalyze_CleanRepoNeverListsFloors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644),
	)

	ctx := finding.WithWorkingDir(context.Background(), root)
	issues, err := analyzeWith(ctx, failingFloorsList(errUnreachableListing))
	require.NoError(t, err)
	assert.Empty(t, issues, "a clean repo pays nothing for the annotation path")
}
