package main

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
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

// TestExitFromOutcomes_DepForcedOnlyExitsZero pins the exit contract (owner
// decision 2026-09-25, verified live on a go-health-dashboard copy): a fix
// run whose only findings were reverted by the dependency-floor gate is a
// success — the drift is externally forced and the remediation is
// supply-side, not a fix failure.
func TestExitFromOutcomes_DepForcedOnlyExitsZero(t *testing.T) {
	t.Parallel()

	outcomes := []fixOutcome{{
		root: "drifted",
		result: &fix.Result{
			DepForced: []fix.DepForced{{}},
		},
	}}

	assert.Equal(t, exitOK, exitFromOutcomes(outcomes),
		"dep-forced-only runs exit 0: the state is forced by a dependency floor")

	outcomes[0].result.Failures = []fix.Failure{{}}
	assert.Equal(t, exitFindings, exitFromOutcomes(outcomes), "failed repairs still exit 1")
}

func TestRun_VersionAndUsage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, exitOK, run([]string{"version"}, &strings.Builder{}))
	assert.Equal(
		t,
		exitOK,
		run([]string{"--version"}, &strings.Builder{}),
		"--version aliases the version subcommand",
	)
	assert.Equal(t, exitOK, run([]string{"-version"}, &strings.Builder{}))
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

// TestRun_CheckNotesGatedFixability pins the honesty fix (2026-09-25): check
// must not promise unconditional auto-fixability for go.mod form fixes — fix
// runs the dependency-floor gate and can come back dep-forced.
func TestRun_CheckNotesGatedFixability(t *testing.T) {
	t.Parallel()

	root := seedRepo(t)

	var out strings.Builder

	require.Equal(t, exitFindings, run([]string{"check", root}, &out))
	assert.Contains(t, out.String(), "dependency-floor gate",
		"the summary must carry the gate caveat for patch-form findings")
}

// checkDoc mirrors the check --json contract the tests rely on.
type checkDoc struct {
	Schema int `json:"schema"`
	Repos  []struct {
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
	Schema int `json:"schema"`
	Repos  []struct {
		Root    string `json:"root"`
		Applied []struct {
			From string `json:"from"`
		} `json:"applied"`
		HeldBack []struct {
			From string `json:"from"`
		} `json:"heldBack"`
		Discovery []struct {
			Rule string `json:"rule"`
		} `json:"discovery"`
		Suggested []struct {
			Rule string `json:"rule"`
		} `json:"suggested"`
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

	assert.Negative(
		t,
		strings.Compare(doc.Repos[0].Root, doc.Repos[1].Root),
		"repos are sorted by root",
	)

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

func TestRun_WhoForcesJSONCarriesSchemaAndWorkspaceRow(t *testing.T) {
	t.Parallel()

	root := seedCleanRepo(t)
	writeFile(t, root, "go.work", "go 1.26\n\nuse .\n")

	var out strings.Builder

	code := run([]string{"who-forces", "--json", root}, &out)
	require.Equal(t, exitOK, code)

	doc := decodeJSON[struct {
		Schema int `json:"schema"`
		Repos  []struct {
			Modules []struct {
				Path     string `json:"path"`
				Kind     string `json:"kind"`
				Module   string `json:"module"`
				Poisoned bool   `json:"poisoned"`
			} `json:"modules"`
		} `json:"repos"`
	}](t, out.String())

	assert.Equal(t, 2, doc.Schema, "who-forces carries the same wire schema version")

	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Modules, 2, "the workspace row decodes alongside the module row")

	byKind := map[string]struct {
		Path   string
		Module string
	}{}

	for _, m := range doc.Repos[0].Modules {
		byKind[m.Kind] = struct {
			Path   string
			Module string
		}{m.Path, m.Module}
		assert.False(t, m.Poisoned, "a dependency-free seeded repo is never poisoned")
	}

	assert.Equal(t, "go.mod", byKind["go.mod"].Path)
	assert.Equal(t, "example.com/m", byKind["go.mod"].Module)
	assert.Equal(t, "go.work", byKind["go.work"].Path, "the go.work row is marked kind go.work on the wire")
}

func TestRun_CheckParallelZeroMatchesDefault(t *testing.T) {
	t.Parallel()

	root := seedRepo(t)

	var defaultOut strings.Builder

	require.Equal(t, exitFindings, run([]string{"check", "--json", root}, &defaultOut))
	defaultDoc := decodeJSON[checkDoc](t, defaultOut.String())

	var autoOut strings.Builder

	code := run([]string{"check", "--json", "--parallel", "0", root}, &autoOut)
	require.Equal(t, exitFindings, code, "--parallel 0 keeps the findings exit contract")
	autoDoc := decodeJSON[checkDoc](t, autoOut.String())

	assert.Equal(t, defaultDoc.Repos[0].Counts, autoDoc.Repos[0].Counts,
		"--parallel 0 (explicit auto) finds exactly what the default pool finds")
}

func TestRun_WhoForcesFailsClosed(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	code := run([]string{"who-forces", filepath.Join(t.TempDir(), "gone")}, &out)
	assert.Equal(t, exitError, code)
}

func TestRun_CheckQuietSuppressesOutput(t *testing.T) {
	t.Parallel()

	root := seedRepo(t)

	var out strings.Builder

	code := run([]string{"check", "--quiet", root}, &out)
	assert.Equal(t, exitFindings, code, "quiet keeps the exit contract")
	assert.Empty(t, out.String(), "quiet emits no human report")

	var jsonOut strings.Builder

	code = run([]string{"check", "--quiet", "--json", root}, &jsonOut)
	assert.Equal(t, exitFindings, code)
	assert.Contains(
		t,
		jsonOut.String(),
		`"schema"`,
		"--json still emits the document under --quiet",
	)
}

func TestRun_FixAndWhoForcesQuietSymmetry(t *testing.T) {
	t.Parallel()

	fixRoot := seedRepo(t)

	var fixOut strings.Builder

	code := run([]string{"fix", "--quiet", fixRoot}, &fixOut)
	assert.Equal(t, exitOK, code, "fix --quiet keeps the exit contract")
	assert.Empty(t, fixOut.String(), "fix --quiet emits no human report")

	var fixJSON strings.Builder

	code = run([]string{"fix", "--quiet", "--json", seedRepo(t)}, &fixJSON)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, fixJSON.String(), `"schema"`, "fix --json still emits under --quiet")

	whoRoot := seedCleanRepo(t)

	var whoOut strings.Builder

	code = run([]string{"who-forces", "--quiet", whoRoot}, &whoOut)
	assert.Equal(t, exitOK, code, "who-forces --quiet keeps the exit contract")
	assert.Empty(t, whoOut.String(), "who-forces --quiet emits no human report")

	var whoJSON strings.Builder

	code = run([]string{"who-forces", "--quiet", "--json", whoRoot}, &whoJSON)
	assert.Equal(t, exitOK, code)
	assert.Contains(t, whoJSON.String(), `"schema"`, "who-forces --json still emits under --quiet")
}

func TestRun_CheckJSONCarriesSchemaVersion(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	require.Equal(t, exitOK, run([]string{"check", "--json", seedCleanRepo(t)}, &out))

	doc := decodeJSON[checkDoc](t, out.String())
	assert.Equal(t, 2, doc.Schema, "the schema field pins the wire contract version (2: poisoners dropped)")
}

// seedBrokenRepo writes a repo whose go.mod cannot be parsed: a discovery
// finding with no mechanical fix.
func seedBrokenRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo not.a.version\n")

	return root
}

func TestRun_FixJSONSurfacesDiscoveryIssues(t *testing.T) {
	t.Parallel()

	root := seedBrokenRepo(t)

	var checkOut strings.Builder

	checkCode := run([]string{"check", root}, &checkOut)
	require.Equal(t, exitFindings, checkCode, "check reports the unparseable go.mod")

	var out strings.Builder

	code := run([]string{"fix", "--json", root}, &out)
	assert.Equal(
		t,
		exitFindings,
		code,
		"discovery findings keep fix from reporting clean, matching check",
	)

	doc := decodeJSON[fixDoc](t, out.String())
	require.Len(t, doc.Repos, 1)
	assert.Equal(t, 2, doc.Schema)
	require.Len(t, doc.Repos[0].Discovery, 1, "fix --json surfaces what check already reported")
	assert.Equal(t, "go-mod-unparseable", doc.Repos[0].Discovery[0].Rule)
}

func TestRun_CheckParallelLimitCoversMoreRootsThanWorkers(t *testing.T) {
	t.Parallel()

	const repoCount = 6

	roots := make([]string, 0, repoCount)
	for range repoCount {
		roots = append(roots, seedRepo(t))
	}

	args := append([]string{"check", "--json", "--parallel", "1"}, roots...)

	var out strings.Builder

	code := run(args, &out)
	require.Equal(t, exitFindings, code)

	doc := decodeJSON[checkDoc](t, out.String())
	require.Len(t, doc.Repos, repoCount, "every root is analyzed even with a single worker")

	for i := 1; i < len(doc.Repos); i++ {
		assert.Negative(
			t,
			strings.Compare(doc.Repos[i-1].Root, doc.Repos[i].Root),
			"output stays sorted by root",
		)
	}

	for _, repo := range doc.Repos {
		assert.Equal(
			t,
			1,
			repo.Counts.Mechanical,
			"each seeded repo reports its patch-form directive",
		)
	}
}

func TestExitFromFloors_AllowPartialDowngradesModuleErrors(t *testing.T) {
	t.Parallel()

	results := []floorsResult{{
		root: "/repo",
		rows: []fix.ModuleFloors{{Path: "go.mod", Error: "go list failed"}},
	}}

	assert.Equal(
		t,
		exitError,
		exitFromFloors(results, false),
		"default fails closed on module listing errors",
	)
	assert.Equal(
		t,
		exitFindings,
		exitFromFloors(results, true),
		"allow-partial downgrades module errors to findings",
	)
}

func TestSummarizeAndExitContracts(t *testing.T) {
	t.Parallel()

	clean := repoAnalysis{root: "/clean", report: &report{}}
	drifted := repoAnalysis{root: "/drifted", report: &report{
		mechanical: []surface.Issue{{Rule: surface.RuleGoDirectivePatchForm}},
	}}
	failed := repoAnalysis{root: "/failed", err: assertError{}}

	cleanN, findingsN, failedN := summarize([]repoAnalysis{clean, drifted, failed})
	assert.Equal(t, 1, cleanN)
	assert.Equal(t, 1, findingsN)
	assert.Equal(t, 1, failedN)

	assert.Equal(
		t,
		exitError,
		exitFromAnalyses([]repoAnalysis{drifted, failed}),
		"hard errors dominate",
	)
	assert.Equal(t, exitFindings, exitFromAnalyses([]repoAnalysis{drifted}))
	assert.Equal(t, exitOK, exitFromAnalyses([]repoAnalysis{clean}))

	outcomes := make([]fixOutcome, 0, 2)

	outcomes = append(outcomes, fixOutcome{
		root:      "/discovery",
		discovery: []surface.Issue{{Rule: surface.RuleGoModUnparseable}},
	})

	assert.Equal(
		t,
		exitFindings,
		exitFromOutcomes(outcomes),
		"discovery findings count as findings, matching check",
	)

	outcomes = append(outcomes, fixOutcome{root: "/err", err: assertError{}})
	assert.Equal(
		t,
		exitError,
		exitFromOutcomes(outcomes),
		"hard errors dominate discovery findings",
	)
}

// assertError is a distinct error type for exit-contract tests.
type assertError struct{}

func (assertError) Error() string { return "assert error" }

func TestPrintFloorsRendersEveryRowShape(t *testing.T) {
	t.Parallel()

	rows := []fix.ModuleFloors{
		{
			Path:        "go.mod",
			Kind:        surface.KindGoMod,
			Module:      "example.com/m",
			Directive:   "1.26",
			MaxDepFloor: "1.26.7",
			PoisonerFloors: []fix.PoisonerFloor{
				{Module: "github.com/larsartmann/go-finding", Version: "v1.12.0", Floor: "1.26.7"},
			},
			Poisoned: true,
		},
		{Path: "go.work", Kind: surface.KindGoWork, Directive: "1.26"},
		{
			Path:   "broken/go.mod",
			Kind:   surface.KindGoMod,
			Module: "example.com/broken",
			Error:  "go list failed",
		},
	}

	var out strings.Builder

	printFloors(&out, rows)
	text := out.String()

	assert.Contains(t, text, "POISONED: tidy re-raises the directive to go 1.26.7, forced by:")
	assert.Contains(t, text, "github.com/larsartmann/go-finding@v1.12.0  (floor go 1.26.7)")
	assert.Contains(t, text, "workspace: declares no dependencies; floor analysis skipped")
	assert.Contains(t, text, "analysis failed: go list failed")
}

func TestPrintFixReportRendersDiscoveryOnlyRepo(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	printFixReport(&out, fixOutcome{
		root: "/repo",
		discovery: []surface.Issue{
			{Rule: surface.RuleGoModUnparseable, File: "go.mod", Message: "parse go.mod: broken"},
		},
	})

	text := out.String()
	assert.Contains(t, text, "/repo: 1 discovery finding(s); nothing mechanical to fix")
	assert.Contains(t, text, "DISCOVERY  go-mod-unparseable")
	assert.Contains(t, text, "parse go.mod: broken")
}

// BenchmarkAnalyzeAll measures the parallel discovery sweep over seeded
// repos; run with -benchtime to compare --parallel settings.
func BenchmarkAnalyzeAll(b *testing.B) {
	const repoCount = 8

	roots := make([]string, 0, repoCount)
	for range repoCount {
		root := b.TempDir()

		if err := os.WriteFile(
			filepath.Join(root, "go.mod"),
			[]byte("module example.com/m\n\ngo 1.26.7\n"),
			0o644,
		); err != nil {
			b.Fatal(err)
		}

		roots = append(roots, root)
	}

	b.ResetTimer()

	for b.Loop() {
		analyzeAll(roots, 0)
	}
}

func TestRun_CheckExpectMinorEnforcesFleetPolicy(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.28\n")
	writeFile(t, root, "flake.nix", "{ x = pkgs.go_1_28; }\n")

	var out strings.Builder

	code := run([]string{"check", "--json", "--expect-minor", "1.27", root}, &out)
	assert.Equal(t, exitFindings, code, "surfaces above the fleet policy are findings")

	doc := decodeJSON[checkDoc](t, out.String())

	rules := map[string]int{}

	for _, f := range doc.Repos[0].Findings {
		rules[f.Rule]++
	}

	assert.Equal(t, 2, rules["minor-exceeds-expectation"], "directive and flake pin are each named once")

	var clean strings.Builder

	code = run([]string{"check", "--json", "--expect-minor", "1.28", root}, &clean)
	assert.Equal(t, exitOK, code, "at the policy minor the repo is clean of expectation findings")

	var bad strings.Builder

	code = run([]string{"check", "--expect-minor", "banana", root}, &bad)
	assert.Equal(t, exitError, code, "an unparseable policy value is a usage error")
}

func TestRun_FixExpectMinorSurfacesSuggestions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.28\n")
	writeFile(t, root, "flake.nix", "{ x = pkgs.go_1_28; }\n")

	var out strings.Builder

	code := run([]string{"fix", "--dry-run", "--json", "--expect-minor", "1.27", root}, &out)
	assert.Equal(t, exitOK, code, "suggestions alone do not fail fix")

	doc := decodeJSON[fixDoc](t, out.String())

	found := false

	for _, r := range doc.Repos {
		for _, s := range r.Suggested {
			if s.Rule == "minor-exceeds-expectation" {
				found = true
			}
		}
	}

	assert.True(t, found, "fix --json carries the expectation findings as suggestions")
}
