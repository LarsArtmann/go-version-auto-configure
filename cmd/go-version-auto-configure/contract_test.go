package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	v4 "github.com/larsartmann/cmdguard/v4/pkg/cmdguard/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExitCodeMatrix locks the process contract across every command:
// clean → 0, findings/failed repairs → 1, hard errors → 2.
func TestExitCodeMatrix(t *testing.T) {
	t.Parallel()

	drifted := seedRepo(t)
	clean := seedCleanRepo(t)

	var out strings.Builder

	assert.Equal(t, exitOK, run([]string{"check", clean}, &out), "check clean → 0")
	assert.Equal(t, exitFindings, run([]string{"check", drifted}, &out), "check findings → 1")
	assert.Equal(
		t,
		exitError,
		run([]string{"check", filepath.Join(t.TempDir(), "gone")}, &out),
		"check hard error → 2",
	)
	assert.Equal(t, exitOK, run([]string{"fix", clean}, &out), "fix clean → 0")
	assert.Equal(
		t,
		exitError,
		run([]string{"fix", "--dry-run", filepath.Join(t.TempDir(), "gone")}, &out),
		"fix hard error → 2",
	)
	assert.Equal(t, exitOK, run([]string{"who-forces", clean}, &out), "who-forces clean → 0")
}

// TestHelpExitsClean pins the fang convention: --help/-h render help and
// exit 0 (the pre-cmdguard skeleton exited 2; that change is documented in
// CHANGELOG.md).
func TestHelpExitsClean(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	assert.Equal(t, exitOK, run([]string{"--help"}, &out))
	assert.Equal(t, exitOK, run([]string{"check", "-h"}, &out))
}

// TestNoRawFlagSkeleton bans the hand-rolled skeleton class the cmdguard
// migration deleted: the flag package and raw cobra must not reappear in
// this package.
func TestNoRawFlagSkeleton(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	require.NoError(t, err)

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		data, err := os.ReadFile(name)
		require.NoError(t, err)

		src := string(data)
		assert.NotContains(t, src, `"flag"`, "%s: use cmdguard flag structs, not the flag package", name)
		assert.NotContains(t, src, "spf13/cobra", "%s: cmdguard owns cobra wiring", name)
	}
}

// TestSharedFlagContract asserts the shared flags carry identical names,
// defaults, and help text across the three analysis commands, so no
// subcommand's help can drift from the others'.
func TestSharedFlagContract(t *testing.T) {
	t.Parallel()

	type flagShape struct {
		name, def, help string
	}

	want := map[string]flagShape{
		"json":     {"json", "false", "emit machine-readable JSON"},
		"parallel": {"parallel", "0", "max repositories analyzed concurrently (0 = auto: CPU count)"},
		"quiet": {
			"quiet",
			"false",
			"exit-code-only: suppress the human report (JSON is still emitted with --json)",
		},
	}

	for name, flags := range map[string]any{
		"check":      &checkFlags{},
		"fix":        &fixFlags{},
		"who-forces": &whoForcesFlags{},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tags, err := v4.ParseFlagTags(flags)
			require.NoError(t, err)

			got := make(map[string]v4.FlagTag, len(tags))

			for _, tag := range tags {
				got[tag.Name] = tag
			}

			for wantName, wantShape := range want {
				tag, ok := got[wantName]
				require.True(t, ok, "%s: shared flag --%s missing", name, wantName)
				assert.Equal(t, wantShape.def, tag.Default, "--%s default drifted", wantName)
				assert.Equal(t, wantShape.help, tag.Help, "--%s help drifted", wantName)
			}
		})
	}
}

// jsonGolden runs one --json command on a single root and returns the parsed
// document with the temp-root path normalized to <ROOT>, so the golden
// comparison locks every field name and shape without temp-path noise.
func jsonGolden(t *testing.T, args ...string) map[string]any {
	t.Helper()

	root := args[len(args)-1]

	var out strings.Builder

	code := run(args, &out)
	require.NotEqual(t, exitError, code, "golden scenario must not hard-fail: %v", args)

	var doc map[string]any
	require.NoError(
		t,
		json.Unmarshal([]byte(strings.ReplaceAll(out.String(), root, "<ROOT>")), &doc),
		"wire format must be valid JSON",
	)

	return doc
}

// TestJSONWireContractGoldens locks the stable machine contract (AGENTS:
// "stable machine contract") for all three emit paths: field names, casing,
// nesting, and schema version. Any diff here is a breaking change and must
// be updated deliberately.
func TestJSONWireContractGoldens(t *testing.T) {
	t.Parallel()

	drifted := seedRepo(t)

	assert.Equal(t, map[string]any{
		"schema": float64(2),
		"repos": []any{
			map[string]any{
				"root":  "<ROOT>",
				"clean": false,
				"counts": map[string]any{
					"total": float64(1), "mechanical": float64(1),
					"suggested": float64(0), "discovery": float64(0),
				},
				"findings": []any{
					map[string]any{
						"rule":    "go-directive-patch-form",
						"message": "go.mod declares go 1.26.7: the go directive is a floor and must be major.minor only; a patch component pins the toolchain to one exact patch and breaks trailing environments",
						"file":    "go.mod",
						"line":    float64(3),
						"fix": map[string]any{
							"file": "go.mod", "kind": "go.mod",
							"from": "1.26.7", "to": "1.26", "line": float64(3),
						},
					},
				},
			},
		},
	}, jsonGolden(t, "check", "--json", drifted), "check --json golden")

	assert.Equal(t, map[string]any{
		"schema": float64(2),
		"repos": []any{
			map[string]any{
				"root":    "<ROOT>",
				"applied": []any{},
				"heldBack": []any{
					map[string]any{
						"file": "go.mod",
						"kind": "go.mod",
						"from": "1.26.7",
						"to":   "1.26",
						"line": float64(3),
					},
				},
				"depForced": []any{},
				"failures":  []any{},
				"suggested": []any{},
				"discovery": []any{},
			},
		},
	}, jsonGolden(t, "fix", "--dry-run", "--json", drifted), "fix --json golden (held-back)")

	clean := seedCleanRepo(t)

	assert.Equal(t, map[string]any{
		"schema": float64(2),
		"repos": []any{
			map[string]any{
				"root": "<ROOT>",
				"modules": []any{
					map[string]any{
						"path": "go.mod", "module": "example.com/m",
						"kind": "go.mod", "directive": "1.26",
						"poisoned": false,
					},
				},
			},
		},
	}, jsonGolden(t, "who-forces", "--json", clean), "who-forces --json golden")
}

// TestRootsFromNormalizesPositionalArgs locks the root normalization:
// empty → working directory, relative → absolute, existing behavior.
func TestRootsFromNormalizesPositionalArgs(t *testing.T) {
	t.Parallel()

	t.Run("empty defaults to working directory", func(t *testing.T) {
		t.Parallel()

		wd, err := os.Getwd()
		require.NoError(t, err)
		assert.Equal(t, []string{wd}, rootsFrom(nil))
	})

	t.Run("relative paths become absolute", func(t *testing.T) {
		t.Parallel()

		roots := rootsFrom([]string{"pkg", "./cmd"})
		require.Len(t, roots, 2)
		assert.True(t, filepath.IsAbs(roots[0]), "pkg → %q", roots[0])
		assert.True(t, filepath.IsAbs(roots[1]), "./cmd → %q", roots[1])
	})

	t.Run("absolute paths pass through", func(t *testing.T) {
		t.Parallel()

		abs, err := filepath.Abs("some/repo")
		require.NoError(t, err)
		assert.Equal(t, []string{abs}, rootsFrom([]string{"some/repo"}))
	})
}

// TestWorkersFor locks the pool-sizing clamp.
func TestWorkersFor(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, workersFor(0, 0), "no work still yields one worker")
	assert.Equal(t, 3, workersFor(3, 0), "work below CPU count caps at work")
	assert.Equal(t, 1, workersFor(10, 1), "operator limit 1 is honored")
	assert.LessOrEqual(t, workersFor(10, -5), runtime.GOMAXPROCS(0), "negative limit means auto: CPU-count bound")
}
