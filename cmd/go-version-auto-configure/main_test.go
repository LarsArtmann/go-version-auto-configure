package main

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedRepo writes a minimal drifted repository: a patch-form directive plus
// an aligned flake pin.
func seedRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.26.7\n")
	writeFile(t, root, "flake.nix", "{ x = pkgs.go_1_26; }\n")

	return root
}

// seedCleanRepo writes a minimal policy-conforming repository.
func seedCleanRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.26\n")

	return root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, rel)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// readGoMod returns a repo's go.mod content.
func readGoMod(t *testing.T, root string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)

	return string(data)
}

func TestRun_VersionAndUsage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, exitOK, run([]string{"version"}, &strings.Builder{}))
	assert.Equal(t, exitError, run(nil, &strings.Builder{}))
	assert.Equal(t, exitError, run([]string{"nonsense"}, &strings.Builder{}))
}

func TestRun_CheckFindsDriftAndFixClears(t *testing.T) {
	t.Parallel()

	root := seedRepo(t)

	var out strings.Builder

	require.Equal(t, exitFindings, run([]string{"check", root}, &out))
	require.Equal(t, exitOK, run([]string{"fix", root}, &out))
	assert.Contains(t, readGoMod(t, root), "go 1.26\n", "fix normalizes the directive")
	assert.Equal(t, exitOK, run([]string{"check", root}, &out), "check is clean after fix")
}

func TestRun_FixDryRunLeavesFiles(t *testing.T) {
	t.Parallel()

	root := seedRepo(t)

	var out strings.Builder

	require.Equal(t, exitOK, run([]string{"fix", "--dry-run", root}, &out))
	assert.Contains(t, readGoMod(t, root), "go 1.26.7\n", "dry-run must not touch the file")
}

// checkDoc mirrors the check --json contract the tests rely on.
type checkDoc struct {
	Repos []struct {
		Root   string `json:"root"`
		Error  string `json:"error"`
		Clean  bool   `json:"clean"`
		Counts struct {
			Total      int `json:"total"`
			Mechanical int `json:"mechanical"`
		} `json:"counts"`
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	} `json:"repos"`
}

// fixDoc mirrors the fix --json contract the tests rely on.
type fixDoc struct {
	Repos []struct {
		Root    string `json:"root"`
		Applied []struct {
			From string `json:"from"`
		} `json:"applied"`
		HeldBack []struct {
			From string `json:"from"`
		} `json:"heldBack"`
	} `json:"repos"`
}

func decodeJSON[D any](t *testing.T, out string) D {
	t.Helper()

	var doc D

	require.NoError(t, json.Unmarshal([]byte(out), &doc))

	return doc
}

func TestRun_CheckJSONReportsEachRepo(t *testing.T) {
	t.Parallel()

	drifted, clean := seedRepo(t), seedCleanRepo(t)

	var out strings.Builder

	code := run([]string{"check", "--json", drifted, clean}, &out)
	require.Equal(t, exitFindings, code, "findings in any repo exit 1")

	doc := decodeJSON[checkDoc](t, out.String())
	require.Len(t, doc.Repos, 2)

	assert.Negative(t, strings.Compare(doc.Repos[0].Root, doc.Repos[1].Root), "repos are sorted by root")

	var cleanRepo, driftedRepo *struct {
		Root   string `json:"root"`
		Error  string `json:"error"`
		Clean  bool   `json:"clean"`
		Counts struct {
			Total      int `json:"total"`
			Mechanical int `json:"mechanical"`
		} `json:"counts"`
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	}

	for i := range doc.Repos {
		if doc.Repos[i].Clean {
			cleanRepo = &doc.Repos[i]
		} else {
			driftedRepo = &doc.Repos[i]
		}
	}

	require.NotNil(t, cleanRepo)
	require.NotNil(t, driftedRepo)

	assert.Equal(t, 0, cleanRepo.Counts.Total)
	assert.Empty(t, cleanRepo.Findings)
	assert.Equal(t, 1, driftedRepo.Counts.Mechanical)
	require.Len(t, driftedRepo.Findings, 1)
	assert.Equal(t, "go-directive-patch-form", driftedRepo.Findings[0].Rule)
}

func TestRun_CheckJSONReportsMissingRoot(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	code := run([]string{"check", "--json", filepath.Join(t.TempDir(), "gone")}, &out)
	require.Equal(t, exitError, code)

	doc := decodeJSON[checkDoc](t, out.String())
	require.Len(t, doc.Repos, 1)
	assert.NotEmpty(t, doc.Repos[0].Error, "a failed root is reported, not dropped")
}

func TestRun_FixMultiRootSkipsCleanRepos(t *testing.T) {
	t.Parallel()

	drifted, clean := seedRepo(t), seedCleanRepo(t)
	before := readGoMod(t, clean)

	var out strings.Builder

	code := run([]string{"fix", drifted, clean}, &out)
	require.Equal(t, exitOK, code)

	assert.Contains(t, readGoMod(t, drifted), "go 1.26\n", "drifted repo is normalized")
	assert.Equal(t, before, readGoMod(t, clean), "clean repo is not touched (fast path)")
}

func TestRun_FixDryRunJSONHoldsBack(t *testing.T) {
	t.Parallel()

	root := seedRepo(t)

	var out strings.Builder

	code := run([]string{"fix", "--dry-run", "--json", root}, &out)
	require.Equal(t, exitOK, code)

	doc := decodeJSON[fixDoc](t, out.String())
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].HeldBack, 1)
	assert.Equal(t, "1.26.7", doc.Repos[0].HeldBack[0].From)
	assert.Empty(t, doc.Repos[0].Applied)
	assert.Contains(t, readGoMod(t, root), "go 1.26.7\n")
}

func TestRun_WhoForcesCleanModule(t *testing.T) {
	t.Parallel()

	root := seedCleanRepo(t)

	var out strings.Builder

	code := run([]string{"who-forces", root}, &out)
	require.Equal(t, exitOK, code)
	assert.Contains(t, out.String(), "clean: no dependency forces a higher floor")
}

func TestRun_WhoForcesJSONIncludesModule(t *testing.T) {
	t.Parallel()

	root := seedCleanRepo(t)

	var out strings.Builder

	code := run([]string{"who-forces", "--json", root}, &out)
	require.Equal(t, exitOK, code)

	doc := decodeJSON[struct {
		Repos []struct {
			Root    string `json:"root"`
			Modules []struct {
				Path     string `json:"path"`
				Module   string `json:"module"`
				Poisoned bool   `json:"poisoned"`
			} `json:"modules"`
		} `json:"repos"`
	}](t, out.String())

	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Modules, 1)
	assert.Equal(t, "go.mod", doc.Repos[0].Modules[0].Path)
	assert.Equal(t, "example.com/m", doc.Repos[0].Modules[0].Module)
	assert.False(t, doc.Repos[0].Modules[0].Poisoned)
}

func TestRun_WhoForcesFailsClosed(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	code := run([]string{"who-forces", filepath.Join(t.TempDir(), "gone")}, &out)
	assert.Equal(t, exitError, code)
}
