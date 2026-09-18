package surface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMajorMinor(t *testing.T) {
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

func TestMajorMinorOrdering(t *testing.T) {
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
	assert.True(t, hasPatch("1.26.7"))
	assert.True(t, hasPatch("1.26.0"))
	assert.False(t, hasPatch("1.26"))
}

func TestParseCIPin(t *testing.T) {
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
			got, ok := parseCIPin(tt.raw)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseNixPin(t *testing.T) {
	tests := []struct {
		name   string
		token  string
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
			got, ok := parseNixPin(tt.token)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
