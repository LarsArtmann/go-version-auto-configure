package fix

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForcedFloorFromTidyDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		diff string
		want surface.GoVersion
	}{
		{
			name: "tidy re-raises the stripped patch form",
			diff: "diff current/go.mod tidy/go.mod\n" +
				"--- current/go.mod\n" +
				"+++ tidy/go.mod\n" +
				"@@ -1,6 +1,6 @@\n" +
				" module example.com/m\n" +
				"\n" +
				"-go 1.27\n" +
				"+go 1.27.1\n" +
				"\n" +
				"require (\n",
			want: "1.27.1",
		},
		{
			name: "file headers are not directives",
			diff: "+++ tidy/go.mod\n+++ other\n",
			want: "",
		},
		{
			name: "highest raise wins",
			diff: "+go 1.26.3\n+go 1.27.1\n",
			want: "1.27.1",
		},
		{
			name: "garbage tokens are ignored",
			diff: "+go bananas\n+go 1.27.1\n",
			want: "1.27.1",
		},
		{
			name: "no raise means no forced floor",
			diff: "-go 1.27.1\n+go 1.27\n",
			want: "1.27",
		},
		{
			name: "empty diff",
			diff: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, forcedFloorFromTidyDiff(tt.diff))
		})
	}
}

func TestGateForcerMentions(t *testing.T) {
	t.Parallel()

	detail := "diff current/go.mod tidy/go.mod\n" +
		"-go 1.27\n" +
		"+go 1.27.1\n" +
		"go: module ../../../go-output requires go >= 1.27.1; switching to go1.27.1\n" +
		"go: example.com/dep/sub@v4.4.0 requires go >= 1.27.1; switching to go1.27.1\n" +
		"go: example.com/older@v1.0.0 requires go >= 1.26.0; switching to go1.26.0\n"

	assert.Equal(
		t,
		[]string{"../../../go-output", "example.com/dep/sub@v4.4.0"},
		gateForcerMentions(detail, "1.27.1"),
		"only diagnostics matching the forced floor name the forcer",
	)
	assert.Empty(t, gateForcerMentions(detail, "1.28.0"))
}

func TestDepForcedCause(t *testing.T) {
	t.Parallel()

	t.Run("gate diagnostic names the forcer", func(t *testing.T) {
		t.Parallel()

		cause := depForcedCause("go: module ../../go-output requires go >= 1.27.1\n", "1.27.1")
		assert.Contains(t, cause, "go 1.27.1")
		assert.Contains(t, cause, "../../go-output")
		assert.Contains(t, cause, "not a listed dependency")
	})

	t.Run("no diagnostic falls back to the standard library", func(t *testing.T) {
		t.Parallel()

		cause := depForcedCause("diff current/go.mod tidy/go.mod\n+go 1.27.1\n", "1.27.1")
		assert.Contains(t, cause, "no listed dependency carries the floor")
		assert.Contains(t, cause, "standard library")
	})
}

func TestVendorModuleFloors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modules string
		want    []vendorModuleFloor
	}{
		{
			name: "skewed vendor tree carries the cqrs family floor",
			modules: "# github.com/larsartmann/go-sse v0.6.1\n" +
				"## explicit; go 1.27.1\n" +
				"github.com/larsartmann/go-sse\n" +
				"# github.com/larsartmann/go-cqrs-lite/record/v4 v4.5.1\n" +
				"## explicit; go 1.27.1\n" +
				"github.com/larsartmann/go-cqrs-lite/record/v4\n" +
				"# github.com/charmbracelet/x/ansi v0.1.2\n" +
				"## explicit; go 1.24.0\n",
			want: []vendorModuleFloor{
				{Module: "github.com/larsartmann/go-sse", Version: "v0.6.1", Floor: "1.27.1"},
				{Module: "github.com/larsartmann/go-cqrs-lite/record/v4", Version: "v4.5.1", Floor: "1.27.1"},
				{Module: "github.com/charmbracelet/x/ansi", Version: "v0.1.2", Floor: "1.24.0"},
			},
		},
		{
			name:    "annotations without go versions carry no floor",
			modules: "# example.com/dep v1.0.0\n## explicit\n",
			want:    nil,
		},
		{
			name:    "malformed versions are ignored",
			modules: "# example.com/dep v1.0.0\n## explicit; go bananas\n",
			want:    nil,
		},
		{
			name:    "annotation before any module header is ignored",
			modules: "## explicit; go 1.27.1\n# example.com/dep v1.0.0\n",
			want:    nil,
		},
		{
			name:    "empty file",
			modules: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, vendorModuleFloors(tt.modules))
		})
	}
}

func TestMaxVendorFloor(t *testing.T) {
	t.Parallel()

	floor, carriers := maxVendorFloor([]vendorModuleFloor{
		{Module: "example.com/low", Version: "v1.0.0", Floor: "1.24.0"},
		{Module: "example.com/a", Version: "v2.0.0", Floor: "1.27.1"},
		{Module: "example.com/b", Version: "v0.6.1", Floor: "1.27.1"},
	})

	assert.Equal(t, surface.GoVersion("1.27.1"), floor)
	assert.Equal(t, []string{"example.com/a@v2.0.0", "example.com/b@v0.6.1"}, carriers)
}

func TestReadVendorModuleFloors(t *testing.T) {
	t.Parallel()

	t.Run("absent vendor tree is not ok", func(t *testing.T) {
		t.Parallel()

		_, ok := readVendorModuleFloors(t.TempDir())
		assert.False(t, ok)
	})

	t.Run("annotations resolve floors", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(
			t,
			os.MkdirAll(filepath.Join(dir, "vendor"), 0o755),
		)
		require.NoError(
			t,
			os.WriteFile(
				filepath.Join(dir, "vendor", "modules.txt"),
				[]byte("# example.com/dep v2.0.0\n## explicit; go 1.27.1\n"),
				0o644,
			),
		)

		floors, ok := readVendorModuleFloors(dir)
		require.True(t, ok)
		assert.Equal(
			t,
			[]vendorModuleFloor{{Module: "example.com/dep", Version: "v2.0.0", Floor: "1.27.1"}},
			floors,
		)
	})
}

// gateForcedDiff is the pkg/domain shape (2026-09-25): tidy re-raises the
// directive because a replaced local module requires the patch floor, while
// `go list -m` resolves no module above the target.
const gateForcedDiff = "diff current/go.mod tidy/go.mod\n" +
	"--- current/go.mod\n" +
	"+++ tidy/go.mod\n" +
	"@@ -1,6 +1,6 @@\n" +
	" module example.com/m\n" +
	"\n" +
	"-go 1.27\n" +
	"+go 1.27.1\n" +
	"\n" +
	"require (\n"

func TestApply_GateForcedFloorWithoutListedCarrierIsDepForced(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.27.1\n"), 0o644),
	)

	run := fakeRunner(func(string, []string) (string, error) {
		return "example.com/m (devel) 1.27\nexample.com/x/dep v1.0.0 1.27\n", nil
	})

	gate := fakeGate(func(string, []string) (string, string, error) {
		return gateForcedDiff, "go: module ../../go-output requires go >= 1.27.1; switching to go1.27.1\n", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.27.1", To: "1.27", Line: 3}},
		Options{Gate: gate},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.DepForced, 1)
	assert.Empty(t, res.Applied)
	assert.Empty(t, res.Failures)

	dep := res.DepForced[0]
	assert.Equal(t, surface.GoVersion("1.27.1"), dep.Floor, "the tidy diff names the enforced floor")
	assert.Empty(t, dep.Poisoners, "no listed dependency carries it")
	assert.Contains(t, dep.Cause, "../../go-output")
	assert.Contains(t, dep.Cause, "go 1.27.1")

	report := res.Report()
	assert.Contains(t, report, "dep-forced 1")
	assert.Contains(t, report, "../../go-output")

	data, readErr := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "go 1.27.1", "a rejected fix is reverted")
}

func TestApply_StdLibraryFloorNamesLikelyForcer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.27.1\n"), 0o644),
	)

	run := fakeRunner(func(string, []string) (string, error) {
		return "example.com/m (devel) 1.27\nexample.com/x/dep v1.0.0 1.27\n", nil
	})

	gate := fakeGate(func(string, []string) (string, string, error) {
		return gateForcedDiff, "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.27.1", To: "1.27", Line: 3}},
		Options{Gate: gate},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.DepForced, 1)

	dep := res.DepForced[0]
	assert.Equal(t, surface.GoVersion("1.27.1"), dep.Floor)
	assert.Contains(t, dep.Cause, "standard library")
}

func TestApply_VendorSkewClassifiesDepForcedFromAnnotations(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.27.1\n"), 0o644),
	)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "vendor"), 0o755))
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(root, "vendor", "modules.txt"),
			[]byte(
				"# github.com/larsartmann/go-sse v0.6.1\n"+
					"## explicit; go 1.27.1\n"+
					"# github.com/charmbracelet/x/ansi v0.1.2\n"+
					"## explicit; go 1.24.0\n"+
					"# github.com/larsartmann/go-cqrs-lite/record/v4 v4.5.1\n"+
					"## explicit; go 1.27.1\n",
			),
			0o644,
		),
	)

	run := fakeRunner(func(string, []string) (string, error) {
		return "", assert.AnError
	})

	gate := fakeGate(func(string, []string) (string, string, error) {
		return gateForcedDiff, "go: github.com/larsartmann/go-cqrs-lite/record/v4@v4.5.0 requires go >= 1.27.1\n", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.27.1", To: "1.27", Line: 3}},
		Options{Gate: gate},
		run,
	)
	require.NoError(t, err)
	require.Len(t, res.DepForced, 1, "the vendored annotations resolve the floor the skewed graph hides")
	assert.Empty(t, res.Failures)

	dep := res.DepForced[0]
	assert.Equal(t, surface.GoVersion("1.27.1"), dep.Floor)
	assert.Equal(
		t,
		[]string{
			"github.com/larsartmann/go-sse@v0.6.1",
			"github.com/larsartmann/go-cqrs-lite/record/v4@v4.5.1",
		},
		dep.Poisoners,
		"vendored carriers are named with their versions",
	)

	report := res.Report()
	assert.Contains(t, report, "forced by: github.com/larsartmann/go-sse@v0.6.1")
	assert.Contains(t, report, "re-tag")
}

func TestApply_UntidyGateWithoutGoRaiseStillFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.7\n"), 0o644),
	)

	run := fakeRunner(func(string, []string) (string, error) {
		return "example.com/m (devel) 1.26\n", nil
	})

	gate := fakeGate(func(string, []string) (string, string, error) {
		return "diff current/go.mod tidy/go.mod\n" +
			"@@ -2,3 +2,4 @@\n" +
			" require example.com/missing v1.0.0\n" +
			"+require example.com/extra v1.2.3\n", "", nil
	})

	res, err := Apply(
		context.Background(),
		root,
		[]surface.Fix{{File: "go.mod", Kind: surface.KindGoMod, From: "1.26.7", To: "1.26", Line: 3}},
		Options{Gate: gate},
		run,
	)
	require.NoError(t, err)
	assert.Empty(t, res.DepForced)
	require.Len(t, res.Failures, 1, "a dirty gate with no directive raise stays a plain failure")
	assert.Contains(t, res.Failures[0].Cause, "dependency floor 1.26 does not exceed the target")
}
