package fix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ciWorkflow is the desire-secrets class in miniature: five below-floor pins
// spread across jobs, with quoting and neighboring YAML around them.
const ciWorkflow = "name: CI\n\njobs:\n  lint:\n    steps:\n" +
	"      - uses: actions/setup-go@v5\n        with:\n" +
	"          go-version: \"1.22\"\n          cache: true\n\n" +
	"  build:\n    steps:\n      - uses: actions/setup-go@v5\n        with:\n" +
	"          go-version: 1.22\n          cache: true\n"

// TestApplyCIPinFix_QuoteStyles locks the byte-preserving rewrite for every
// value style the scanner records: the replacement inherits the original
// quoting and every other line stays byte-identical.
func TestApplyCIPinFix_QuoteStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		from     string
		to       string
		wantLine string
	}{
		{
			name:     "double-quoted",
			content:  "steps:\n  with:\n    go-version: \"1.22\"\n",
			from:     `"1.22"`,
			to:       "1.27",
			wantLine: `    go-version: "1.27"`,
		},
		{
			name:     "single-quoted",
			content:  "steps:\n  with:\n    go-version: '1.22'\n",
			from:     `'1.22'`,
			to:       "1.27",
			wantLine: `    go-version: '1.27'`,
		},
		{
			name:     "bare",
			content:  "steps:\n  with:\n    go-version: 1.22\n",
			from:     "1.22",
			to:       "1.27",
			wantLine: "    go-version: 1.27",
		},
		{
			name:     "flow-sequence-bracket",
			content:  "steps:\n  with:\n    go-version: [1.22]\n",
			from:     "1.22",
			to:       "1.27",
			wantLine: "    go-version: [1.27]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			rel := ".github/workflows/ci.yml"
			path := filepath.Join(root, rel)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, []byte(tt.content), 0o644))

			res, err := Apply(
				context.Background(),
				root,
				[]surface.Fix{{File: rel, Kind: surface.KindCIPin, From: surface.GoVersion(tt.from), To: surface.GoVersion(tt.to), Line: 3}},
				Options{},
				nil,
			)
			require.NoError(t, err)
			require.Len(t, res.Applied, 1)
			assert.Empty(t, res.Failures)

			data, err := os.ReadFile(path)
			require.NoError(t, err)

			lines := splitLines(string(data))
			require.Len(t, lines, len(splitLines(tt.content)))
			assert.Equal(t, tt.wantLine, lines[2], "the pin line carries the raised floor in the original quoting")
		})
	}
}

// TestApplyCIPinFix_MultiPinFileIsolatedRewrites proves one fix touches
// exactly its own line: fixing pin 8 of a five-pin workflow leaves the
// other four pins at 1.22, and the neighboring cache line is byte-identical.
func TestApplyCIPinFix_MultiPinFileIsolatedRewrites(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := ".github/workflows/ci.yml"
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(ciWorkflow), 0o644))

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: rel, Kind: surface.KindCIPin, From: `"1.22"`, To: "1.27", Line: 8}},
		Options{},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, res.Applied, 1)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	got := string(data)
	want := "name: CI\n\njobs:\n  lint:\n    steps:\n" +
		"      - uses: actions/setup-go@v5\n        with:\n" +
		"          go-version: \"1.27\"\n          cache: true\n\n" +
		"  build:\n    steps:\n      - uses: actions/setup-go@v5\n        with:\n" +
		"          go-version: 1.22\n          cache: true\n"
	assert.Equal(t, want, got, "only the targeted pin line changes")
}

// TestApplyCIPinFix_DetectFixVerify drives the full loop against one
// repository: Discover records the pins, Analyze emits mechanical fixes for
// every below-floor pin, Apply raises all of them, and a second
// Discover+Analyze reports a clean surface — a fix is checked with exactly
// the code that detected it.
func TestApplyCIPinFix_DetectFixVerify(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRepoFile(t, root, "go.mod", "module example.com/m\n\ngo 1.27\n")
	writeRepoFile(t, root, ".github/workflows/ci.yml", ciWorkflow)

	s, discovery, err := surface.Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discovery)
	require.Len(t, s.CIPins, 2, "both pins are comparable")

	issues := surface.Analyze(s)
	require.Len(t, issues, 2, "both pins trail the 1.27 floor")

	fixes := make([]surface.Fix, 0, len(issues))
	for _, issue := range issues {
		require.NotNil(t, issue.Fix)
		fixes = append(fixes, *issue.Fix)
	}

	res, err := Apply(context.Background(), root, fixes, Options{}, nil)
	require.NoError(t, err)
	require.Len(t, res.Applied, 2)
	assert.Empty(t, res.Failures)
	assert.Empty(t, res.DepForced)

	s2, discovery2, err := surface.Discover(root)
	require.NoError(t, err)
	assert.Empty(t, discovery2)
	assert.Empty(t, surface.Analyze(s2), "the raised surface is clean under the same detector")
}

// TestApplyCIPinFix_DriftGuardRefusesStaleDetection answers the rewrite
// safety question: when the recorded value no longer matches the disk (the
// file changed between detection and repair), the fix must fail untouched
// instead of silently rewriting a different pin.
func TestApplyCIPinFix_DriftGuardRefusesStaleDetection(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := ".github/workflows/ci.yml"
	path := filepath.Join(root, rel)
	stale := "steps:\n  with:\n    go-version: 1.22\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(stale), 0o644))

	// Detection saw "1.22"; the disk now carries "1.24".
	fx := surface.Fix{File: rel, Kind: surface.KindCIPin, From: "1.24", To: "1.27", Line: 3}

	res, err := Apply(context.Background(), root, []surface.Fix{fx}, Options{}, nil)
	require.NoError(t, err)
	require.Len(t, res.Failures, 1)
	assert.Empty(t, res.Applied)
	assert.Contains(t, string(res.Failures[0].Cause), "changed since detection", "the failure names the drift, not a generic error")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, stale, string(data), "a refused fix leaves the file untouched")
}

// TestApplyCIPinFix_MissingFileFailsClean reports a failure (not a panic or
// a hard error) when the workflow vanished between analysis and repair.
func TestApplyCIPinFix_MissingFileFailsClean(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: ".github/workflows/gone.yml", Kind: surface.KindCIPin, From: "1.22", To: "1.27", Line: 1}},
		Options{},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, res.Failures, 1)
	assert.Contains(t, string(res.Failures[0].Cause), "read")
}

// TestApply_DryRunHoldsCIFixBack locks the dry-run contract for the new
// kind: nothing is written, the fix lands in HeldBack.
func TestApply_DryRunHoldsCIFixBack(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := ".github/workflows/ci.yml"
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(ciWorkflow), 0o644))

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: rel, Kind: surface.KindCIPin, From: `"1.22"`, To: "1.27", Line: 8}},
		Options{DryRun: true},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, res.HeldBack, 1)
	assert.Empty(t, res.Applied)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, ciWorkflow, string(data), "dry-run must not touch the file")
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
