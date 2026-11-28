package main

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-finding"
	"github.com/larsartmann/go-version-auto-configure/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sarifGolden runs check --sarif on one root and returns the parsed SARIF
// document with the temp-root path and the tool version normalized, so the
// golden locks the document shape without environment noise. The tool
// version comes from the ldflags/VCS stamp and differs between build modes.
func sarifGolden(t *testing.T, root string) map[string]any {
	t.Helper()

	var out strings.Builder

	code := run([]string{"check", "--sarif", root}, &out)
	require.NotEqual(t, exitError, code, "sarif scenario must not hard-fail")

	normalized := strings.ReplaceAll(out.String(), root, "<ROOT>")
	normalized = strings.ReplaceAll(normalized, version.Version, "<VERSION>")

	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(normalized), &doc), "SARIF output must be valid JSON")

	return doc
}

// TestSARIFWireContractGolden locks the SARIF 2.1.0 export shape (plan M24):
// the document envelope, the tool descriptor, the run propertyBag stamping
// (scanRoots, schemaVersion), and the per-finding rule/ruleId/level triple
// for a known fixture. Any diff is a contract change and must be deliberate.
func TestSARIFWireContractGolden(t *testing.T) {
	t.Parallel()

	drifted := seedRepo(t)

	doc := sarifGolden(t, drifted)

	assert.Equal(t, "2.1.0", doc["version"], "SARIF schema version")
	runs, ok := doc["runs"].([]any)
	require.True(t, ok, "runs array present")
	require.Len(t, runs, 1, "one run for one root")

	run := runs[0].(map[string]any)

	tool, ok := run["tool"].(map[string]any)
	require.True(t, ok, "run.tool present")

	driver, ok := tool["driver"].(map[string]any)
	require.True(t, ok, "run.tool.driver present")
	assert.Equal(t, "go-version-auto-configure", driver["name"], "tool descriptor name")
	assert.Equal(t, "<VERSION>", driver["version"], "tool descriptor version stamped from the fleet version kit")

	props, ok := run["properties"].(map[string]any)
	require.True(t, ok, "run propertyBag present")
	assert.Equal(t, []any{"<ROOT>"}, props["scanRoots"], "scanRoots stamped")
	assert.InDelta(
		t,
		float64(jsonSchemaVersion),
		props["schemaVersion"],
		1e-9,
		"schemaVersion ties the export to the --json contract",
	)
	assert.NotContains(t, props, "failedRoots", "no failed roots in a clean scenario")

	results, ok := run["results"].([]any)
	require.True(t, ok, "results array present")
	require.NotEmpty(t, results, "the drifted fixture must produce findings")

	first := results[0].(map[string]any)
	assert.NotEmpty(t, first["ruleId"], "result carries ruleId")
	assert.NotEmpty(t, first["message"], "result carries message")

	location, ok := first["locations"].([]any)
	require.True(t, ok, "result carries locations")
	require.NotEmpty(t, location, "result carries at least one location")

	loc := location[0].(map[string]any)
	artifact := loc["physicalLocation"].(map[string]any)["artifactLocation"].(map[string]any)
	assert.Contains(t, artifact["uri"], "<ROOT>", "finding location is absolute against its root")
}

// TestSARIFRoundTrip proves the export is consumable, not just shapely: the
// emitted document parses back through go-finding's SARIF reader into the
// same finding population (count and rule set) the CLI reported.
func TestSARIFRoundTrip(t *testing.T) {
	t.Parallel()

	drifted := seedRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(drifted, ".github", "workflows"), 0o755))
	writeFile(t, drifted,
		".github/workflows/ci.yml",
		"steps:\n  - uses: actions/setup-go@v5\n    with:\n      go-version: \"1.22\"\n",
	)

	var out strings.Builder

	code := run([]string{"check", "--sarif", drifted}, &out)
	require.NotEqual(t, exitError, code)

	findings, err := finding.FindingsFromSARIF(context.Background(), []byte(out.String()))
	require.NoError(t, err, "the exported document must round-trip")
	require.NotEmpty(t, findings, "round-trip preserves the findings")

	rules := make(map[string]bool, len(findings))
	for _, f := range findings {
		rules[string(f.Rule)] = true
	}

	assert.Contains(t, rules, "go-directive-patch-form", "the fixture's patch-form finding survives the round-trip")
	assert.Contains(t, rules, "ci-pin-below-floor", "the fixture's CI-pin finding survives the round-trip")
}

// TestSARIFCleanRepoAndExitCode pins the format's policy stance: a clean
// repo emits an empty results array, exit codes stay the findings contract
// (0 clean / 1 findings) — SARIF is an output format, not a gate change.
func TestSARIFCleanRepoAndExitCode(t *testing.T) {
	t.Parallel()

	clean := seedCleanRepo(t)

	var out strings.Builder

	code := run([]string{"check", "--sarif", clean}, &out)
	assert.Equal(t, exitOK, code, "clean repo exits 0 under --sarif")

	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.String()), &doc))

	runs := doc["runs"].([]any)
	results := runs[0].(map[string]any)["results"].([]any)
	assert.Empty(t, results, "clean repo emits no results")

	drifted := seedRepo(t)
	code = run([]string{"check", "--sarif", drifted}, &strings.Builder{})
	assert.Equal(t, exitFindings, code, "drifted repo exits 1 under --sarif")
}
