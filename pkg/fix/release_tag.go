package fix

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// GitCommandRunner mirrors GoCommandRunner for git invocations: `git <args…>`
// executed in dir, combined output returned. A separate type keeps the two
// runner contracts honest — module-scoped environment surgery (GOWORK,
// GOTOOLCHAIN) makes no sense for git, and tests fake each independently.
type GitCommandRunner func(ctx context.Context, dir string, args ...string) (string, error)

// GitTagRunner returns the production git runner: `git <args…>` executed in
// dir with the inherited environment. Unlike the go commands, git needs no
// GOWORK/GOTOOLCHAIN surgery.
func GitTagRunner() GitCommandRunner {
	return func(ctx context.Context, dir string, args ...string) (string, error) {
		gitBin, err := exec.LookPath("git")
		if err != nil {
			return "", fmt.Errorf("find git binary: %w", err)
		}

		cmd := exec.CommandContext(ctx, gitBin, args...)
		cmd.Dir = dir

		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(
					out,
				), fmt.Errorf(
					"git %s: %s: %w",
					strings.Join(args, " "),
					strings.TrimSpace(string(out)),
					err,
				)
		}

		return string(out), nil
	}
}

// newestTagCommand lists `v*` tags newest-first by version. version:refname
// sorts semantically, so the first parseable line IS the newest parseable
// tag — later lines can only be older or unparseable decoration.
const newestTagCommand = "for-each-ref"

// NewestReleaseTag returns the repository's newest release tag: the first
// `v*` tag (sorted semantically) whose name parses through
// surface.ParseReleaseVersion — the same validator VERSION stamps and
// CHANGELOG headings go through, so all three sources compare through
// identical normalization. No parseable tag yields ("", nil): a repo
// without v* tags has no third source, which is silence, not an error.
func NewestReleaseTag(ctx context.Context, root string, run GitCommandRunner) (surface.ReleaseVersion, error) {
	out, err := run(ctx, root, newestTagCommand, "refs/tags/v*", "--sort=-version:refname", "--format=%(refname:short)")
	if err != nil {
		return "", fmt.Errorf("list release tags in %s: %w", root, err)
	}

	for line := range strings.SplitSeq(out, "\n") {
		tag, ok := surface.ParseReleaseVersion(strings.TrimSpace(line))
		if ok {
			return tag, nil
		}
	}

	return "", nil
}

// ReleaseAuthorityTagIssues is the second release-authority pass (the
// git-tag third source): it runs where Discover's pure walk cannot. The
// pure-file sources come from the already-discovered Surface; the tag comes
// from git. Like every read-only enrichment in this package it degrades
// silently when its input is unavailable — a git failure (no repo, no git
// binary) yields no findings rather than an error: the first pass already
// covers the pure-file half, and a missing third source is not drift.
func ReleaseAuthorityTagIssues(
	ctx context.Context,
	root string,
	surf *surface.Surface,
	run GitCommandRunner,
) []surface.Issue {
	ra := surf.Release
	if ra == nil || ra.VersionFile == "" || ra.ChangelogTop == "" {
		return nil
	}

	tag, err := NewestReleaseTag(ctx, root, run)
	if err != nil || tag == "" {
		return nil
	}

	return surface.ReleaseTagIssues(ra.VersionFile, ra.ChangelogTop, tag)
}
