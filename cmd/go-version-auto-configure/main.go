// Command go-version-auto-configure detects and repairs Go toolchain
// version-surface drift across one or more repositories: patch components
// in go.mod/go.work `go` directives (auto-fixed via go mod edit / go work
// edit), Nix/CI pins that trail the effective floor (reported with
// suggestions), and the dependencies that force a module's floor
// (`who-forces`). Repositories are analyzed in parallel; --json emits a
// machine-readable report for CI and fleet sweeps.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/larsartmann/go-version-auto-configure/pkg/version"
)

// Process exit codes.
const (
	exitOK       = 0
	exitFindings = 1
	exitError    = 2
)

const usage = `go-version-auto-configure — unify the Go toolchain version surface

Usage:
  go-version-auto-configure check [--json] [--quiet] [--parallel N] [root ...]
                                                 detect drift, exit 1 when found
  go-version-auto-configure fix [--dry-run] [--json] [--parallel N] [root ...]
                                                 auto-fix directive form,
                                                 suggest the rest
  go-version-auto-configure who-forces [--json] [--allow-partial] [--parallel N] [root ...]
                                                 name the dependencies forcing
                                                 each go directive
  go-version-auto-configure version              print the tool version
                                                 (--version works too)

Roots default to the working directory; multiple roots are analyzed in
parallel and reported sorted by path.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)

		return exitError
	}

	switch args[0] {
	case "version", "--version", "-version":
		fmt.Fprintf(out, "go-version-auto-configure %s\n", version.Version)

		return exitOK
	case "check":
		return cmdCheck(args[1:], out)
	case "fix":
		return cmdFix(args[1:], out)
	case "who-forces":
		return cmdWhoForces(args[1:], out)
	default:
		fmt.Fprint(os.Stderr, usage)

		return exitError
	}
}

// report is the full analysis of one repository.
type report struct {
	mechanical []surface.Issue
	suggested  []surface.Issue
	discovery  []surface.Issue
}

// analyzeAt discovers and analyzes one repository root.
func analyzeAt(root string) (*report, error) {
	surf, discovery, err := surface.Discover(root)
	if err != nil {
		return nil, fmt.Errorf("analyze %q: %w", root, err)
	}

	r := &report{discovery: discovery}

	for _, issue := range surface.Analyze(surf) {
		if issue.Fix != nil {
			r.mechanical = append(r.mechanical, issue)

			continue
		}

		r.suggested = append(r.suggested, issue)
	}

	return r, nil
}

// all returns every issue in a stable order: discovery, mechanical, then
// suggested.
func (r *report) all() []surface.Issue {
	return slices.Concat(r.discovery, r.mechanical, r.suggested)
}

// repoAnalysis pairs one root with its analysis or failure.
type repoAnalysis struct {
	root   string
	report *report
	err    error
}

// analyzeAll analyzes every root with a bounded worker pool and returns the
// results sorted by root for deterministic output. parallel <= 0 selects the
// automatic worker count.
func analyzeAll(roots []string, parallel int) []repoAnalysis {
	analyses := make([]repoAnalysis, len(roots)) //nolint:makezero // pre-sized for index assignment
	sem := make(chan struct{}, workersFor(len(roots), parallel))

	var wg sync.WaitGroup

	for i, root := range roots {
		wg.Go(func() {
			sem <- struct{}{}

			defer func() { <-sem }()

			analyzed, err := analyzeAt(root)
			analyses[i] = repoAnalysis{root: root, report: analyzed, err: err}
		})
	}

	wg.Wait()

	slices.SortFunc(analyses, func(a, b repoAnalysis) int {
		return strings.Compare(a.root, b.root)
	})

	return analyses
}

// rootsFrom normalizes positional roots to absolute paths, defaulting to
// the working directory, so reports and sweeps name the repo unambiguously.
func rootsFrom(args []string) []string {
	if len(args) == 0 {
		args = []string{"."}
	}

	roots := make([]string, 0, len(args))

	for _, arg := range args {
		abs, err := filepath.Abs(arg)
		if err != nil {
			abs = arg
		}

		roots = append(roots, abs)
	}

	return roots
}

// workersFor bounds the pool by the CPU count, the work size, and the
// operator's --parallel limit when one is given (limit <= 0 means auto).
func workersFor(work, limit int) int {
	n := max(min(runtime.GOMAXPROCS(0), work), 1)

	if limit > 0 {
		n = max(min(limit, n), 1)
	}

	return n
}

func cmdCheck(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)

	var asJSON, quiet bool

	var parallel int

	fs.BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	fs.BoolVar(&quiet, "quiet", false, "exit-code-only: suppress the human report (JSON is still emitted with --json)")
	fs.IntVar(&parallel, "parallel", 0, "max repositories analyzed concurrently (0 = auto: CPU count)")

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	analyses := analyzeAll(rootsFrom(fs.Args()), parallel)

	if asJSON {
		return emitCheckJSON(out, analyses)
	}

	if !quiet {
		printCheckReports(out, analyses)
	}

	return exitFromAnalyses(analyses)
}

// printCheckReports renders every repository's findings for humans.
func printCheckReports(out io.Writer, analyses []repoAnalysis) {
	for _, a := range analyses {
		if a.err != nil {
			fmt.Fprintf(os.Stderr, "check: %s: %v\n", a.root, a.err)

			continue
		}

		if len(analyses) > 1 {
			fmt.Fprintf(out, "== %s ==\n", a.root)
		}

		printCheckReport(out, a)
	}

	if len(analyses) > 1 {
		clean, findings, failed := summarize(analyses)
		fmt.Fprintf(
			out,
			"\n%d repos: %d clean, %d with findings, %d failed analysis\n",
			len(analyses),
			clean,
			findings,
			failed,
		)
	}
}

// printCheckReport renders one repository's findings.
func printCheckReport(out io.Writer, a repoAnalysis) {
	all := a.report.all()

	for _, issue := range all {
		fmt.Fprintf(out, "FOUND  %-26s %s:%d\n       %s\n", issue.Rule, issue.File, issue.Line, issue.Message)

		if issue.Suggestion != "" {
			fmt.Fprintf(out, "       fix: %s\n", issue.Suggestion)
		}
	}

	switch {
	case len(all) == 0:
		fmt.Fprintf(
			out,
			"%s: version surface clean: go directives are major.minor, pins align with the module floor\n",
			a.root,
		)
	case len(a.report.suggested) == 0:
		fmt.Fprintf(out, "%s: %d finding(s), all auto-fixable with 'fix'\n", a.root, len(all))
	default:
		fmt.Fprintf(
			out,
			"%s: %d finding(s): %d auto-fixable with 'fix', %d need a maintainer decision\n",
			a.root,
			len(all),
			len(a.report.mechanical),
			len(a.report.suggested),
		)
	}
}

// summarize counts clean, finding-carrying, and failed repositories:
// clean, findings, failed.
func summarize(analyses []repoAnalysis) (int, int, int) {
	var clean, findings, failed int

	for _, a := range analyses {
		switch {
		case a.err != nil:
			failed++
		case len(a.report.all()) == 0:
			clean++
		default:
			findings++
		}
	}

	return clean, findings, failed
}

// exitFromAnalyses maps results onto the exit contract: hard errors
// dominate, then findings, then clean.
func exitFromAnalyses(analyses []repoAnalysis) int {
	findings, failed := false, false

	for _, a := range analyses {
		if a.err != nil {
			failed = true

			continue
		}

		if len(a.report.all()) > 0 {
			findings = true
		}
	}

	switch {
	case failed:
		return exitError
	case findings:
		return exitFindings
	default:
		return exitOK
	}
}

// fixOutcome pairs one root with its fix application or failure.
type fixOutcome struct {
	root      string
	result    *fix.Result
	suggested []surface.Issue
	discovery []surface.Issue
	err       error
}

func cmdFix(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)

	var dryRun, asJSON bool

	var parallel int

	fs.BoolVar(&dryRun, "dry-run", false, "report what would change without touching files")
	fs.BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	fs.IntVar(&parallel, "parallel", 0, "max repositories analyzed concurrently (0 = auto: CPU count)")

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	analyses := analyzeAll(rootsFrom(fs.Args()), parallel)

	outcomes := applyAll(context.Background(), analyses, fix.Options{DryRun: dryRun}, parallel)

	if asJSON {
		return emitFixJSON(out, outcomes)
	}

	printFixReports(out, outcomes, len(analyses) > 1)

	return exitFromOutcomes(outcomes)
}

// applyAll applies every repository's mechanical fixes in parallel. Clean
// and suggest-only repositories skip the go tool entirely — the fast path
// that keeps fleet sweeps linear in the drifted-repo count, not the repo
// count.
func applyAll(ctx context.Context, analyses []repoAnalysis, opts fix.Options, parallel int) []fixOutcome {
	outcomes := make([]fixOutcome, len(analyses)) //nolint:makezero // pre-sized for index assignment
	sem := make(chan struct{}, workersFor(len(analyses), parallel))

	var wg sync.WaitGroup

	for i, a := range analyses {
		wg.Go(func() {
			sem <- struct{}{}

			defer func() { <-sem }()

			outcomes[i] = fixOne(ctx, a, opts)
		})
	}

	wg.Wait()

	slices.SortFunc(outcomes, func(a, b fixOutcome) int {
		return strings.Compare(a.root, b.root)
	})

	return outcomes
}

// fixOne applies one repository's mechanical fixes.
func fixOne(ctx context.Context, a repoAnalysis, opts fix.Options) fixOutcome {
	out := fixOutcome{root: a.root}

	if a.err != nil {
		out.err = a.err

		return out
	}

	out.suggested = a.report.suggested
	out.discovery = a.report.discovery

	fixes := make([]surface.Fix, 0, len(a.report.mechanical))

	for _, issue := range a.report.mechanical {
		fixes = append(fixes, *issue.Fix)
	}

	if len(fixes) == 0 {
		return out
	}

	res, err := fix.Apply(ctx, a.root, fixes, opts, nil)
	if err != nil {
		out.err = err

		return out
	}

	out.result = res

	return out
}

// printFixReports renders every repository's fix outcome for humans.
func printFixReports(out io.Writer, outcomes []fixOutcome, header bool) {
	for _, outcome := range outcomes {
		if outcome.err != nil {
			fmt.Fprintf(os.Stderr, "fix: %s: %v\n", outcome.root, outcome.err)

			continue
		}

		if header {
			fmt.Fprintf(out, "== %s ==\n", outcome.root)
		}

		printFixReport(out, outcome)
	}
}

// printFixReport renders one repository's fix outcome.
func printFixReport(out io.Writer, outcome fixOutcome) {
	switch {
	case outcome.result != nil:
		fmt.Fprintln(out, outcome.result.Report())
	case len(outcome.suggested) > 0:
		fmt.Fprintf(
			out,
			"%s: %d finding(s) need a maintainer decision; nothing mechanical to fix\n",
			outcome.root,
			len(outcome.suggested),
		)
	case len(outcome.discovery) > 0:
		fmt.Fprintf(
			out,
			"%s: %d discovery finding(s); nothing mechanical to fix\n",
			outcome.root,
			len(outcome.discovery),
		)
	default:
		fmt.Fprintf(out, "%s: version surface clean: nothing to fix\n", outcome.root)
	}

	for _, issue := range outcome.discovery {
		fmt.Fprintf(out, "DISCOVERY  %-22s %s:%d\n           %s\n", issue.Rule, issue.File, issue.Line, issue.Message)
	}

	for _, issue := range outcome.suggested {
		fmt.Fprintf(out, "SUGGEST  %-24s %s:%d\n         %s\n", issue.Rule, issue.File, issue.Line, issue.Suggestion)
	}
}

// exitFromOutcomes maps fix results onto the exit contract: hard errors
// dominate, then failed repairs or discovery findings, then success.
func exitFromOutcomes(outcomes []fixOutcome) int {
	errored, failed := false, false

	for _, o := range outcomes {
		if o.err != nil {
			errored = true

			continue
		}

		if len(o.discovery) > 0 {
			failed = true

			continue
		}

		if o.result != nil && len(o.result.Failures) > 0 {
			failed = true
		}
	}

	switch {
	case errored:
		return exitError
	case failed:
		return exitFindings
	default:
		return exitOK
	}
}

func cmdWhoForces(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("who-forces", flag.ContinueOnError)

	var asJSON, allowPartial bool

	var parallel int

	fs.BoolVar(&asJSON, "json", false, "emit machine-readable JSON")
	fs.BoolVar(&allowPartial, "allow-partial", false,
		"downgrade per-module go list failures from exit 2 to exit 1 (default: fail closed)")
	fs.IntVar(&parallel, "parallel", 0, "max repositories analyzed concurrently (0 = auto: CPU count)")

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	roots := rootsFrom(fs.Args())

	results := make([]floorsResult, len(roots)) //nolint:makezero // pre-sized for index assignment
	sem := make(chan struct{}, workersFor(len(roots), parallel))

	var wg sync.WaitGroup

	for i, root := range roots {
		wg.Go(func() {
			sem <- struct{}{}

			defer func() { <-sem }()

			rows, err := fix.AnalyzeFloors(context.Background(), root, nil)
			results[i] = floorsResult{root: root, rows: rows, err: err}
		})
	}

	wg.Wait()

	slices.SortFunc(results, func(a, b floorsResult) int {
		return strings.Compare(a.root, b.root)
	})

	if asJSON {
		return emitFloorsJSON(out, results, allowPartial)
	}

	printFloorsReports(out, results)

	return exitFromFloors(results, allowPartial)
}

// floorsResult pairs one root with its dependency-floor matrix or failure.
type floorsResult struct {
	root string
	rows []fix.ModuleFloors
	err  error
}

// printFloorsReports renders every repository's floor matrix for humans.
func printFloorsReports(out io.Writer, results []floorsResult) {
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "who-forces: %s: %v\n", r.root, r.err)

			continue
		}

		if len(results) > 1 {
			fmt.Fprintf(out, "== %s ==\n", r.root)
		}

		printFloors(out, r.rows)
	}
}

// printFloors renders one repository's floor matrix rows. go.work rows are
// marked: a workspace declares no dependencies, so there is no graph to list.
func printFloors(out io.Writer, rows []fix.ModuleFloors) {
	for _, row := range rows {
		fmt.Fprintf(out, "%s  %s\n", row.Path, row.Module)

		if row.Kind == surface.KindGoWork {
			fmt.Fprintln(out, "  workspace: declares no dependencies; floor analysis skipped")

			continue
		}

		if row.Error != "" {
			fmt.Fprintf(out, "  analysis failed: %s\n", row.Error)

			continue
		}

		fmt.Fprintf(
			out,
			"  directive: %s   max dep floor: %s\n",
			goVersionOrNone(string(row.Directive)),
			goVersionOrNone(string(row.MaxDepFloor)),
		)

		if !row.Poisoned {
			fmt.Fprintln(out, "  clean: no dependency forces a higher floor")

			continue
		}

		fmt.Fprintf(
			out,
			"  POISONED: tidy re-raises the directive to %s, forced by:\n",
			goVersionOrNone(string(row.MaxDepFloor)),
		)

		for _, poisoner := range row.PoisonerFloors {
			fmt.Fprintf(out, "    %s@%s  (floor go %s)\n", poisoner.Module, poisoner.Version, poisoner.Floor)
		}
	}
}

// goVersionOrNone prefixes a bare version with "go", or reports none.
func goVersionOrNone(v string) string {
	if v == "" {
		return "(none)"
	}

	return "go " + v
}

// exitFromFloors maps floor matrices onto the exit contract: hard errors —
// including a module whose graph could not be listed — dominate, then
// poisoned directives, then clean. allowPartial downgrades per-module listing
// failures to findings (exit 1) for fleets where some modules cannot resolve.
func exitFromFloors(results []floorsResult, allowPartial bool) int {
	errored, poisoned := false, false

	for _, r := range results {
		if r.err != nil {
			errored = true

			continue
		}

		for _, row := range r.rows {
			switch {
			case row.Error != "" && !allowPartial:
				errored = true
			case row.Error != "" || row.Poisoned:
				poisoned = true
			}
		}
	}

	switch {
	case errored:
		return exitError
	case poisoned:
		return exitFindings
	default:
		return exitOK
	}
}
