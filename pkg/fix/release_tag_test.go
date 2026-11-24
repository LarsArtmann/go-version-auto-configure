package fix

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

func TestNewestReleaseTag(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	newestFirst := "v0.7.0\nv0.2.0\nv0.1.0\n"

	tests := []struct {
		name    string
		run     GitCommandRunner
		want    surface.ReleaseVersion
		wantErr bool
	}{
		{
			name: "newest parseable tag wins",
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return newestFirst, nil
			},
			want: "0.7.0",
		},
		{
			name: "unparseable decoration is skipped",
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "release-notes\nv2026-snapshot\nv0.3.0\n", nil
			},
			want: "0.3.0",
		},
		{
			name: "no v* tags at all",
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", nil
			},
			want: "",
		},
		{
			name: "git failure is an error",
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", errors.New("not a git repository")
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		got, err := NewestReleaseTag(ctx, "/repo", test.run)
		if test.wantErr {
			assert.Error(t, err, test.name)

			continue
		}

		require.NoError(t, err, test.name)
		assert.Equal(t, test.want, got, test.name)
	}
}

func TestReleaseAuthorityTagIssues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	both := &surface.Surface{Release: &surface.ReleaseAuthority{
		VersionFile:  "0.7.0",
		ChangelogTop: "0.7.0",
	}}

	tests := []struct {
		name      string
		surf      *surface.Surface
		run       GitCommandRunner
		wantFires bool
	}{
		{
			name: "drift fires: consensus files, trailing tag",
			surf: both,
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "v0.2.0\n", nil
			},
			wantFires: true,
		},
		{
			name: "silent: tag agrees",
			surf: both,
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "v0.7.0\n", nil
			},
			wantFires: false,
		},
		{
			name: "silent: no release sources on the surface",
			surf: &surface.Surface{},
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "v0.2.0\n", nil
			},
			wantFires: false,
		},
		{
			name: "silent: git fails (degrade, not error)",
			surf: both,
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", errors.New("fatal: not a git repository")
			},
			wantFires: false,
		},
		{
			name: "silent: no parseable tags",
			surf: both,
			run: func(ctx context.Context, dir string, args ...string) (string, error) {
				return "", nil
			},
			wantFires: false,
		},
	}

	for _, test := range tests {
		issues := ReleaseAuthorityTagIssues(ctx, "/repo", test.surf, test.run)
		if test.wantFires {
			assert.Len(t, issues, 1, test.name)
			if len(issues) == 1 {
				assert.Equal(t, surface.RuleReleaseTagDrift, issues[0].Rule, test.name)
			}

			continue
		}

		assert.Empty(t, issues, test.name)
	}
}
