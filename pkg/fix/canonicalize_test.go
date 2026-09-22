package fix

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

func TestRewriteGoDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		target      string
		expected    string
		expectError bool
	}{
		{
			name:     "rewrites patch pin to minor",
			content:  "module example.com/t\n\ngo 1.26.7\n\nrequire foo v1.0.0\n",
			target:   "1.26",
			expected: "module example.com/t\n\ngo 1.26\n\nrequire foo v1.0.0\n",
		},
		{
			name:     "idempotent when already minor-only",
			content:  "module example.com/t\n\ngo 1.26\n",
			target:   "1.26",
			expected: "module example.com/t\n\ngo 1.26\n",
		},
		{
			name:     "preserves trailing comment on the go line",
			content:  "go 1.26.7 // pinned by hand\n",
			target:   "1.26",
			expected: "go 1.26 // pinned by hand\n",
		},
		{
			name:     "module paths starting with go- cannot match",
			content:  "module example.com/t\n\ngo 1.26.7\n\nrequire go-something v1.0.0\n",
			target:   "1.26",
			expected: "module example.com/t\n\ngo 1.26\n\nrequire go-something v1.0.0\n",
		},
		{
			name:        "missing go directive is an error",
			content:     "module example.com/t\n",
			target:      "1.26",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := rewriteGoDirective(tt.content, tt.target)

			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestStripToolchainDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "removes line and preceding blank line",
			content:  "module example.com/t\n\ngo 1.26\n\ntoolchain go1.26.7\n\nrequire foo v1.0.0\n",
			expected: "module example.com/t\n\ngo 1.26\n\nrequire foo v1.0.0\n",
		},
		{
			name:     "removes line directly after go line",
			content:  "go 1.26\ntoolchain go1.26.7\nrequire foo v1.0.0\n",
			expected: "go 1.26\nrequire foo v1.0.0\n",
		},
		{
			name:     "removes line at EOF without trailing newline",
			content:  "go 1.26\n\ntoolchain go1.26.7",
			expected: "go 1.26\n",
		},
		{
			name:     "content without toolchain line is unchanged",
			content:  "module example.com/t\n\ngo 1.26\n\nrequire foo v1.0.0\n",
			expected: "module example.com/t\n\ngo 1.26\n\nrequire foo v1.0.0\n",
		},
		{
			name:     "require paths containing toolchain do not match",
			content:  "module example.com/t\n\ngo 1.26\n\nrequire example.com/toolchain-helper v1.0.0\n",
			expected: "module example.com/t\n\ngo 1.26\n\nrequire example.com/toolchain-helper v1.0.0\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, stripToolchainDirective(tt.content))
		})
	}
}

// writeGoModFixture materializes a go.mod and returns its path.
func writeGoModFixture(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "go.mod")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	return path
}

func TestCanonicalizeGoMod_DowngradesPatchPinAndStripsToolchain(t *testing.T) {
	t.Parallel()

	installed := probeInstalledMinor(t)

	path := writeGoModFixture(t,
		"module example.com/norm\n\ngo "+installed+"\n\ntoolchain go"+installed+"\n")

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		StripToolchain:     true,
		InstalledToolchain: surface.GoVersion(installed),
	}, noopGate())
	require.NoError(t, err)
	assert.True(t, res.Changed)
	assert.Equal(t,
		[]string{
			"go line " + installed + " -> " + surface.MinorForm(surface.GoVersion(installed)).String(),
			"remove toolchain directive go" + installed,
		},
		res.Changes)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t,
		"module example.com/norm\n\ngo "+surface.MinorForm(surface.GoVersion(installed)).String()+"\n",
		string(data))
}

func TestCanonicalizeGoMod_ToolchainStripOnMinorLineNeedsNoGate(t *testing.T) {
	t.Parallel()

	installed := probeInstalledMinor(t)

	path := writeGoModFixture(t,
		"module example.com/norm\n\ngo "+surface.MinorForm(surface.GoVersion(installed)).String()+"\n\ntoolchain go"+installed+"\n")

	gate := fakeGate(func(string, []string) (string, string, error) {
		return "", "", errors.New("the gate must not run for a toolchain-only strip")
	})

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		StripToolchain:     true,
		InstalledToolchain: surface.GoVersion(installed),
	}, gate)
	require.NoError(t, err)
	assert.True(t, res.Changed)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t,
		"module example.com/norm\n\ngo "+surface.MinorForm(surface.GoVersion(installed)).String()+"\n",
		string(data))
}

func TestCanonicalizeGoMod_DepForcedFloorKeepsPatchPin(t *testing.T) {
	t.Parallel()

	path := writeGoModFixture(t, "module example.com/norm\n\ngo 1.26.7\n")
	original := "module example.com/norm\n\ngo 1.26.7\n"

	gate := fakeGate(func(string, []string) (string, string, error) {
		return "\ndiff: tidy would raise the directive\n", "", nil
	})

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		InstalledToolchain: "1.26.7",
	}, gate)
	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.True(t, res.HeldBackByDepFloor)
	assert.NotEmpty(t, res.GateDetail)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data), "a rejected downgrade is reverted atomically")
}

func TestCanonicalizeGoMod_DryRunDoesNotWrite(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26.7\n"
	path := writeGoModFixture(t, original)

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		DryRun:             true,
		InstalledToolchain: "1.26.7",
	}, noopGate())
	require.NoError(t, err)
	assert.False(t, res.Changed)
	assert.True(t, res.HeldBack)
	assert.Equal(t, []string{"go line 1.26.7 -> 1.26"}, res.Changes)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(data))
}

func TestCanonicalizeGoMod_SkipsCleanAndUnparseable(t *testing.T) {
	t.Parallel()

	t.Run("minor-only without toolchain is already canonical", func(t *testing.T) {
		t.Parallel()

		res, err := CanonicalizeGoMod(context.Background(), writeGoModFixture(t,
			"module example.com/norm\n\ngo 1.26\n"), CanonicalizeOptions{InstalledToolchain: "1.26.7"}, noopGate())
		require.NoError(t, err)
		assert.True(t, res.Skipped)
		assert.Equal(t, "directives already canonical", res.SkipReason)
	})

	t.Run("unparseable go.mod is skipped, not failed", func(t *testing.T) {
		t.Parallel()

		res, err := CanonicalizeGoMod(context.Background(), writeGoModFixture(t,
			"this is not a go.mod {{{"), CanonicalizeOptions{}, noopGate())
		require.NoError(t, err)
		assert.True(t, res.Skipped)
		assert.Contains(t, res.SkipReason, "unparseable")
	})

	t.Run("go line above the installed toolchain keeps the patch floor", func(t *testing.T) {
		t.Parallel()

		original := "module example.com/norm\n\ngo 1.99.1\n"
		path := writeGoModFixture(t, original)

		res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
			InstalledToolchain: "1.26.7",
		}, noopGate())
		require.NoError(t, err)
		assert.True(t, res.Skipped)
		assert.Contains(t, res.SkipReason, "exceeds installed toolchain")

		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, original, string(data))
	})
}

func TestCanonicalizeGoMod_MissingFileIsError(t *testing.T) {
	t.Parallel()

	_, err := CanonicalizeGoMod(context.Background(),
		filepath.Join(t.TempDir(), "go.mod"), CanonicalizeOptions{}, noopGate())
	require.Error(t, err)
}

// probeInstalledMinor resolves the test environment's installed Go version
// (minor form) so fixtures never exceed the local toolchain.
func probeInstalledMinor(t *testing.T) string {
	t.Helper()

	run := ExecSplitRunner()

	stdout, _, err := run(context.Background(), t.TempDir(), "env", "GOVERSION")
	require.NoError(t, err, "the test environment always has a go binary")

	return string(surface.MinorForm(surface.GoVersion(trimGoPrefix(stdout))))
}

func trimGoPrefix(v string) string {
	for len(v) > 0 && (v[0] == 'g' || v[0] == 'o' || v[0] == ' ' || v[0] == '\n' || v[0] == '\t') {
		v = v[1:]
	}

	return v
}
