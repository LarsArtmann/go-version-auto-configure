package surface

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDirective pins the single shared parsing entry point for both
// discovery and fix verification. The post-v0.38.3 fleet state is
// minor-form directives; the invariants below hold for both forms and for
// the exact byte shapes a fix's text surgery produces (trailing comment
// preserved, unrelated require lines untouched).
func TestParseDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		kind    DirectiveKind
		data    string
		want    GoVersion
		wantLn  int
		wantErr error
	}{
		{
			name: "go.mod minor form (the fleet target state)",
			kind: KindGoMod,
			data: "module example.com/m\n\ngo 1.27\n",
			want: "1.27",
			wantLn: 3,
		},
		{
			name: "go.mod patch form (the poisoner state)",
			kind: KindGoMod,
			data: "module example.com/m\n\ngo 1.27.1\n",
			want: "1.27.1",
			wantLn: 3,
		},
		{
			name: "post-surgery shape keeps the version and the comment",
			kind: KindGoMod,
			data: "module example.com/m\n\ngo 1.27 // floor\n",
			want: "1.27",
			wantLn: 3,
		},
		{
			name: "module paths containing go do not match",
			kind: KindGoMod,
			data: "module example.com/go-sse\n\ngo 1.26\n\nrequire example.com/gofrs v1.0.0\n",
			want: "1.26",
			wantLn: 3,
		},
		{
			name:    "well-formed go.mod without a directive is ErrNoDirective",
			kind:    KindGoMod,
			data:    "module example.com/m\n",
			wantErr: ErrNoDirective,
		},
		{
			name:    "unparseable go.mod is an error, not a missing directive",
			kind:    KindGoMod,
			data:    "module\n",
			wantErr: assert.AnError,
		},
		{
			name: "go.work directive and line",
			kind: KindGoWork,
			data: "go 1.27\n\nuse .\n",
			want: "1.27",
			wantLn: 1,
		},
		{
			name:    "go.work without a directive is ErrNoDirective",
			kind:    KindGoWork,
			data:    "use .\n",
			wantErr: ErrNoDirective,
		},
		{
			name:    "unknown kind is rejected",
			kind:    DirectiveKind("weird"),
			data:    "go 1.27\n",
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, line, err := ParseDirective(tt.kind, []byte(tt.data))
			if tt.wantErr != nil {
				require.Error(t, err)

				if !errors.Is(tt.wantErr, assert.AnError) {
					require.ErrorIs(t, err, tt.wantErr)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantLn, line, "the 1-based directive line feeds the fix's surgical rewrite")
		})
	}
}

func TestParseMajorMinor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    majorMinor
		wantErr bool
	}{
		{name: "plain minor", version: "1.26", want: majorMinor{Major: 1, Minor: 26}},
		{name: "with patch", version: "1.26.7", want: majorMinor{Major: 1, Minor: 26}},
		{name: "with zero patch", version: "1.26.0", want: majorMinor{Major: 1, Minor: 26}},
		{name: "go prefix", version: "go1.26.7", want: majorMinor{Major: 1, Minor: 26}},
		{name: "major only", version: "1", wantErr: true},
		{name: "empty", version: "", wantErr: true},
		{name: "non numeric", version: "one.twenty", wantErr: true},
		{name: "zero major", version: "0.26", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseMajorMinor(tt.version)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseToolchain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		kind    DirectiveKind
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "go.mod toolchain",
			kind:    KindGoMod,
			content: "module example.com/m\n\ngo 1.26\n\ntoolchain go1.26.7\n",
			want:    "go1.26.7",
		},
		{
			name:    "absent is normal",
			kind:    KindGoMod,
			content: "module example.com/m\n\ngo 1.26\n",
			want:    "",
		},
		{
			name:    "go.work toolchain",
			kind:    KindGoWork,
			content: "go 1.26\n\ntoolchain go1.25.2\n\nuse .\n",
			want:    "go1.25.2",
		},
		{name: "unparseable", kind: KindGoMod, content: "{{{", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, _, err := ParseToolchain(tt.kind, []byte(tt.content))
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, GoVersion(tt.want), got)
		})
	}
}

func TestParseModulePath(t *testing.T) {
	t.Parallel()

	path, err := ParseModulePath(KindGoMod, []byte("module example.com/sub/api\n\ngo 1.26\n"))
	require.NoError(t, err)
	assert.Equal(t, ModulePath("example.com/sub/api"), path)

	workPath, err := ParseModulePath(KindGoWork, []byte("go 1.26\n\nuse .\n"))
	require.NoError(t, err)
	assert.Empty(t, workPath, "go.work declares no module")
}

func TestGreaterVersion(t *testing.T) {
	t.Parallel()

	assert.True(t, GreaterVersion("1.26.7", "1.26"))
	assert.True(t, GreaterVersion("1.27", "1.26.7"))
	assert.False(t, GreaterVersion("1.26", "1.26.7"))
	assert.False(t, GreaterVersion("1.26", "1.26"))
	assert.False(t, GreaterVersion("garbage", "1.26"), "unparseable never exceeds")
	assert.False(t, GreaterVersion("1.26", "garbage"), "unparseable is never exceeded")
}

func TestMajorMinorOrdering(t *testing.T) {
	t.Parallel()

	a := majorMinor{Major: 1, Minor: 26}
	b := majorMinor{Major: 1, Minor: 27}
	c := majorMinor{Major: 2, Minor: 0}

	assert.True(t, b.greaterThan(a))
	assert.False(t, a.greaterThan(b))
	assert.True(t, a.lessThan(b))
	assert.True(t, c.greaterThan(b))
	assert.Equal(t, "1.26", a.String())
}

func TestHasPatch(t *testing.T) {
	t.Parallel()

	assert.True(t, hasPatch("1.26.7"))
	assert.True(t, hasPatch("1.26.0"))
	assert.False(t, hasPatch("1.26"))
}

func TestParseCIPin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		raw    string
		want   majorMinor
		wantOK bool
	}{
		{name: "exact", raw: "1.26", want: majorMinor{Major: 1, Minor: 26}, wantOK: true},
		{name: "quoted", raw: `"1.26"`, want: majorMinor{Major: 1, Minor: 26}, wantOK: true},
		{name: "patch", raw: "1.26.7", want: majorMinor{Major: 1, Minor: 26}, wantOK: true},
		{name: "matrix expression", raw: "${{ matrix.go }}", wantOK: false},
		{name: "range", raw: "1.26.x", wantOK: false},
		{name: "caret", raw: "^1.26", wantOK: false},
		{name: "stable", raw: "stable", wantOK: false},
		{name: "word", raw: "oldstable", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseCIPin(tt.raw)
			assert.Equal(t, tt.wantOK, ok)

			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseNixPin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		token  nixGoRef
		want   majorMinor
		wantOK bool
	}{
		{name: "attribute", token: "go_1_26", want: majorMinor{Major: 1, Minor: 26}, wantOK: true},
		{name: "builder", token: "buildGo126Module", want: majorMinor{Major: 1, Minor: 26}, wantOK: true},
		{name: "builder 127", token: "buildGo127Module", want: majorMinor{Major: 1, Minor: 27}, wantOK: true},
		{name: "garbage", token: "go_256", wantOK: false},
		{name: "unrelated", token: "nodejs_22", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseNixPin(tt.token)
			assert.Equal(t, tt.wantOK, ok)

			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCompareDirectiveRanksBareMinorBelowZeroPatch(t *testing.T) {
	t.Parallel()

	assert.Negative(t, CompareDirective("1.26", "1.26.0"), "go tool ranking: go 1.26 < go 1.26.0")
	assert.Negative(t, CompareDirective("1.26.0", "1.26.7"))
	assert.Positive(t, CompareDirective("1.27", "1.26.7"))
	assert.Equal(t, 0, CompareDirective("1.26", "1.26"))
	assert.Equal(t, 0, CompareDirective("banana", "1.26"), "unparseable input compares as equal")
}

func TestFullModuleFloorIsOrderIndependent(t *testing.T) {
	t.Parallel()

	// GreaterVersion's old zero-padding read go 1.26.0 and go 1.26 as
	// equal, so FullModuleFloor's max depended on discovery order and
	// could return go 1.26 when a module carries go 1.26.0 (the x/text
	// dep-forced shape). Both orders must yield the full floor.
	first, _ := (&Surface{Modules: []ModuleDirective{
		{Path: "go.mod", Kind: KindGoMod, Version: "1.26"},
		{Path: "bridge/go.mod", Kind: KindGoMod, Version: "1.26.0"},
	}}).FullModuleFloor()
	second, _ := (&Surface{Modules: []ModuleDirective{
		{Path: "bridge/go.mod", Kind: KindGoMod, Version: "1.26.0"},
		{Path: "go.mod", Kind: KindGoMod, Version: "1.26"},
	}}).FullModuleFloor()

	assert.Equal(t, GoVersion("1.26.0"), first)
	assert.Equal(t, GoVersion("1.26.0"), second)
}
