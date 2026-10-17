package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	v4 "github.com/larsartmann/cmdguard/v4/pkg/cmdguard/v4"
	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/larsartmann/go-version-auto-configure/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJSONDepForcedCauseGolden golden-locks the additive `cause` field on a
// dep-forced fix entry (wire schema stays 2 per the additive-fields policy).
// The sibling golden in contract_test.go covers the cause-ABSENT shape (a
// listed dependency carries the floor); this one covers cause-PRESENT, the
// state only a replace target, a vendored module, or the standard library
// can produce — deterministically unreachable offline through the real go
// command, so the emission is exercised directly. Without this lock the field
// could be renamed or dropped and schema 2 would drift silently.
func TestJSONDepForcedCauseGolden(t *testing.T) {
	t.Parallel()

	const cause = "go mod tidy re-raises the directive to go 1.27.1: no listed " +
		"dependency carries the floor; the standard library (e.g. encoding/json/v2) " +
		"is the likely forcer"

	outcome := fixOutcome{
		root: "<ROOT>",
		result: &fix.Result{
			DepForced: []fix.DepForced{{
				Fix:   surface.Fix{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3},
				Floor: "1.27.1",
				Cause: cause,
			}},
		},
	}

	var out strings.Builder

	require.Equal(t, exitOK, emitFixJSON(&out, []fixOutcome{outcome}))

	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.String()), &doc))

	assert.Equal(t, map[string]any{
		"schema": float64(2),
		"repos": []any{
			map[string]any{
				"root":     "<ROOT>",
				"applied":  []any{},
				"heldBack": []any{},
				"depForced": []any{
					map[string]any{
						"fix": map[string]any{
							"file": "go.mod", "kind": "go.mod",
							"from": "1.26.7", "to": "1.26", "line": float64(3),
						},
						"floor": "1.27.1",
						"cause": cause,
					},
				},
				"failures":  []any{},
				"suggested": []any{},
				"discovery": []any{},
			},
		},
	}, doc, "dep-forced cause present: field name, casing, and presence are locked")
}

// TestExitFromOutcomesContractTable pins the full outcome-combination matrix
// of the fix exit contract: dep-forced/held-back/applied-only runs are
// successes, discovery findings and failed repairs exit 1, hard errors
// dominate and exit 2.
func TestExitFromOutcomesContractTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		outcomes []fixOutcome
		want     int
	}{
		{"no outcomes", nil, exitOK},
		{"clean outcome", []fixOutcome{{root: "/ok"}}, exitOK},
		{
			"applied fixes only",
			[]fixOutcome{{root: "/ok", result: &fix.Result{Applied: []surface.Fix{{}}}}},
			exitOK,
		},
		{
			"held-back fixes only",
			[]fixOutcome{{root: "/ok", result: &fix.Result{HeldBack: []surface.Fix{{}}}}},
			exitOK,
		},
		{
			"dep-forced only",
			[]fixOutcome{{root: "/ok", result: &fix.Result{DepForced: []fix.DepForced{{}}}}},
			exitOK,
		},
		{
			"failed repair",
			[]fixOutcome{{root: "/bad", result: &fix.Result{Failures: []fix.Failure{{}}}}},
			exitFindings,
		},
		{
			"discovery finding",
			[]fixOutcome{{root: "/bad", discovery: []surface.Issue{{Rule: surface.RuleGoModUnparseable}}}},
			exitFindings,
		},
		{"hard error", []fixOutcome{{root: "/err", err: assertError{}}}, exitError},
		{
			"hard error dominates discovery and failures",
			[]fixOutcome{
				{root: "/bad", discovery: []surface.Issue{{Rule: surface.RuleGoModUnparseable}}},
				{root: "/err", err: assertError{}},
			},
			exitError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, exitFromOutcomes(tc.outcomes))
		})
	}
}

// TestRunSortedContract locks the worker-pool helper's boundary behavior:
// zero items yield an empty (non-panicking) result, results are sorted by
// key, every item is visited exactly once, and the parallel width never
// changes the output.
func TestRunSortedContract(t *testing.T) {
	t.Parallel()

	t.Run("zero items yields an empty result", func(t *testing.T) {
		t.Parallel()

		calls := 0

		got := runSorted([]int{}, 0, func(string) string { return "" }, func(i int) string {
			calls++

			return fmt.Sprintf("%d", i)
		})

		assert.Empty(t, got)
		assert.Zero(t, calls, "no item may be visited")
	})

	t.Run("parallel 1 and 100 produce identical sorted results", func(t *testing.T) {
		t.Parallel()

		items := make([]int, 0, 64)
		for i := range 64 {
			items = append(items, i)
		}

		fn := func(i int) string { return fmt.Sprintf("r%03d", i) }
		key := func(s string) string { return s }

		one := runSorted(items, 1, key, fn)
		many := runSorted(items, 100, key, fn)

		require.Len(t, one, 64)
		assert.Equal(t, one, many, "pool width must not change the output")
		assert.Equal(t, fn(0), one[0], "sorted ascending by key")
		assert.Equal(t, fn(63), one[len(one)-1])
	})
}

// TestVersionOutputGolden locks the byte-exact version line for the
// subcommand and both aliases (the pre-cmdguard byte-compat contract). The
// stamp is "dev" under plain `go test` (no ldflags), so the line is
// deterministic.
func TestVersionOutputGolden(t *testing.T) {
	t.Parallel()

	want := "go-version-auto-configure " + version.Version + "\n"

	for _, args := range [][]string{{"version"}, {"--version"}, {"-version"}} {
		var out strings.Builder

		require.Equal(t, exitOK, run(args, &out))
		assert.Equal(t, want, out.String(), "args %q", args)
	}
}

// TestFlagHelpProperty asserts the property every declared flag must satisfy:
// a non-empty name, non-empty help text, and no duplicates within a command.
// A flag without help is an undocumented surface; a duplicate would silently
// shadow.
func TestFlagHelpProperty(t *testing.T) {
	t.Parallel()

	for name, flags := range map[string]any{
		"check":      &checkFlags{},
		"fix":        &fixFlags{},
		"who-forces": &whoForcesFlags{},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tags, err := v4.ParseFlagTags(flags)
			require.NoError(t, err)
			require.NotEmpty(t, tags, "%s declares no flags", name)

			seen := make(map[string]bool, len(tags))

			for _, tag := range tags {
				assert.NotEmpty(t, tag.Name, "%s: flag with an empty name", name)
				assert.NotEmpty(t, tag.Help, "%s: --%s is undocumented", name, tag.Name)
				assert.False(t, seen[tag.Name], "%s: duplicate flag --%s", name, tag.Name)

				seen[tag.Name] = true
			}
		})
	}
}

// BenchmarkApplyAll_CleanFastPath measures the fleet-sweep fast path: clean
// repositories skip the go tool entirely, so sweep cost scales with the
// number of drifted repos, not repo count. The real-gate path (which shells
// out to `go mod tidy -diff`) is exercised by tests, not benchmarked here to
// keep the benchmark hermetic and deterministic.
func BenchmarkApplyAll_CleanFastPath(b *testing.B) {
	roots := make([]repoAnalysis, 0, 32)
	for range 32 {
		roots = append(roots, repoAnalysis{root: "/clean", report: &report{}})
	}

	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		_ = applyAll(ctx, roots, fix.Options{}, 8)
	}
}
