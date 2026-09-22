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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"charm.land/fang/v2"
	v4 "github.com/larsartmann/cmdguard/v4/pkg/cmdguard/v4"
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

// errFindings is the sentinel returned by command handlers when the exit
// contract says exit 1 (findings / failed repairs). The CLI's error handler
// suppresses it — the report itself IS the user-facing output — and run/main
// map it onto exitFindings.
var errFindings = errors.New("drift found")

// errSilent is the sentinel for exit-2 conditions whose diagnostics have
// already been written by the report printers (per-root analysis failures,
// invalid --expect-minor). It must not be printed again.
var errSilent = errors.New("failed")

// codeFrom maps an internal exit code onto the handler error contract:
// nil for clean, errFindings for exit 1, errSilent for exit 2.
func codeFrom(code int) error {
	switch code {
	case exitOK:
		return nil
	case exitFindings:
		return fmt.Errorf("%w", errFindings)
	default:
		return fmt.Errorf("%w", errSilent)
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(
			os.Stderr,
			"go-version-auto-configure — unify the Go toolchain version surface\n\nRun 'go-version-auto-configure --help' for usage.\n",
		)

		return exitError
	}

	// Aliases kept for byte-compat with the pre-cmdguard surface; fang's
	// --version renders differently ("tool version X" vs the stamped line).
	switch args[0] {
	case "version", "--version", "-version":
		fmt.Fprintf(out, "go-version-auto-configure %s\n", version.Version)

		return exitOK
	}

	cli, err := buildCLI(out, version.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "go-version-auto-configure: %v\n", err)

		return exitError
	}

	execErr := cli.ExecuteWithArgs(context.Background(), args)
	switch {
	case execErr == nil:
		return exitOK
	case errors.Is(execErr, errFindings):
		return exitFindings
	default:
		// Flag, argument, and command errors, plus errSilent: the message
		// is already on stderr, the contract calls for exit 2.
		return exitError
	}
}

// CommonFlags holds the flags every subcommand shares: output format and
// worker-pool sizing. Embedded into each command's flag struct; cmdguard
// recurses into embedded structs so the names, defaults, and help text
// cannot drift between commands.
type CommonFlags struct {
	JSON     bool `default:"false" flag:"json"     help:"emit machine-readable JSON"`
	Parallel int  `default:"0"     flag:"parallel" help:"max repositories analyzed concurrently (0 = auto: CPU count)"`
}

// QuietFlags is shared by all three analysis commands.
type QuietFlags struct {
	Quiet bool `default:"false" flag:"quiet" help:"exit-code-only: suppress the human report (JSON is still emitted with --json)"`
}

// AnalysisFlags is shared by check and fix: fleet-policy expectation.
type AnalysisFlags struct {
	ExpectMinor string `default:"" flag:"expect-minor" help:"fleet-expected Go minor (e.g. 1.27): surfaces above it fire alignment findings"`
}

type checkFlags struct {
	CommonFlags
	QuietFlags
	AnalysisFlags
}

type fixFlags struct {
	CommonFlags
	QuietFlags
	AnalysisFlags

	DryRun bool `default:"false" flag:"dry-run" help:"report what would change without touching files"`
}

type whoForcesFlags struct {
	CommonFlags
	QuietFlags

	AllowPartial bool `default:"false" flag:"allow-partial" help:"downgrade per-module go list failures from exit 2 to exit 1 (default: fail closed)"`
}

// appConfig carries no configuration file state; the tool is flag-driven.
type appConfig struct{}

// buildCLI assembles the cmdguard CLI: four commands, the exit-contract
// sentinels, and a fang error handler that stays silent for findings and
// already-reported failures (their output is the report itself) while
// printing genuine usage errors plainly.
func buildCLI(out io.Writer, ver string) (*v4.CLI[appConfig], error) {
	checkCmd, err := v4.NewCommand(
		"check",
		&checkFlags{},
		func(ctx context.Context, _ *appConfig, f *checkFlags) error {
			roots := rootsFrom(v4.ArgsFromContext(ctx))

			opts, code := expectMinorOptions(out, f.ExpectMinor)
			if code != exitOK {
				return codeFrom(code)
			}

			analyses := analyzeAll(roots, f.Parallel, opts...)

			if f.JSON {
				return codeFrom(emitCheckJSON(out, analyses))
			}

			if !f.Quiet {
				printCheckReports(out, analyses)
			}

			return codeFrom(exitFromAnalyses(analyses))
		},
		v4.WithShort("detect version-surface drift, exit 1 when found"),
		v4.WithMinimumArgs(0),
	)
	if err != nil {
		return nil, err
	}

	fixCmd, err := v4.NewCommand("fix", &fixFlags{}, func(ctx context.Context, _ *appConfig, f *fixFlags) error {
		roots := rootsFrom(v4.ArgsFromContext(ctx))

		opts, code := expectMinorOptions(out, f.ExpectMinor)
		if code != exitOK {
			return codeFrom(code)
		}

		analyses := analyzeAll(roots, f.Parallel, opts...)
		outcomes := applyAll(context.Background(), analyses, fix.Options{DryRun: f.DryRun}, f.Parallel)

		if f.JSON {
			return codeFrom(emitFixJSON(out, outcomes))
		}

		if !f.Quiet {
			printFixReports(out, outcomes, len(analyses) > 1)
		}

		return codeFrom(exitFromOutcomes(outcomes))
	}, v4.WithShort("auto-fix directive form, suggest the rest"), v4.WithMinimumArgs(0))
	if err != nil {
		return nil, err
	}

	whoCmd, err := v4.NewCommand(
		"who-forces",
		&whoForcesFlags{},
		func(ctx context.Context, _ *appConfig, f *whoForcesFlags) error {
			roots := rootsFrom(v4.ArgsFromContext(ctx))
			results := analyzeFloorsAll(roots, f.Parallel)

			if f.JSON {
				return codeFrom(emitFloorsJSON(out, results, f.AllowPartial))
			}

			if !f.Quiet {
				printFloorsReports(out, results)
			}

			return codeFrom(exitFromFloors(results, f.AllowPartial))
		},
		v4.WithShort("name the dependencies forcing each go directive"),
		v4.WithMinimumArgs(0),
	)
	if err != nil {
		return nil, err
	}

	cli, err := v4.NewCLI[appConfig](
		"go-version-auto-configure",
		"unify the Go toolchain version surface",
		appConfig{},
		v4.WithCLIVersion(ver),
		v4.WithCLILong(`Detects and repairs Go toolchain version-surface drift across one or more
repositories: patch components in go.mod/go.work go directives (auto-fixed
via go mod edit / go work edit), Nix/CI pins that trail the effective floor
(reported with suggestions), and the dependencies that force a module's floor
(who-forces).

Roots default to the working directory; multiple roots are analyzed in
parallel and reported sorted by path.`),
		v4.WithFangErrorHandler(func(w io.Writer, _ fang.Styles, e error) {
			if errors.Is(e, errFindings) || errors.Is(e, errSilent) {
				return
			}

			fmt.Fprintf(w, "Error: %v\n", e)
		}),
		v4.WithSilenceUsage(),
	)
	if err != nil {
		return nil, err
	}

	if err := v4.AddCommand(cli, checkCmd); err != nil {
		return nil, err
	}

	if err := v4.AddCommand(cli, fixCmd); err != nil {
		return nil, err
	}

	if err := v4.AddCommand(cli, whoCmd); err != nil {
		return nil, err
	}

	return cli, nil
}

// report is the full analysis of one repository.
type report struct {
	mechanical []surface.Issue
	suggested  []surface.Issue
	discovery  []surface.Issue
}

// analyzeAt discovers and analyzes one repository root.
func analyzeAt(root string, opts ...surface.AnalyzeOption) (*report, error) {
	surf, discovery, err := surface.Discover(root)
	if err != nil {
		return nil, fmt.Errorf("analyze %q: %w", root, err)
	}

	r := &report{discovery: discovery}

	for _, issue := range surface.Analyze(surf, opts...) {
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

// runSorted applies fn to every item with a bounded worker pool and returns
// the results sorted by key(item) for deterministic output. parallel <= 0
// selects the automatic worker count. This is the one worker pool in the
// codebase; analyzeAll, applyAll, and analyzeFloorsAll are thin wrappers.
func runSorted[I any, R any](items []I, parallel int, key func(R) string, fn func(I) R) []R {
	results := make([]R, len(items)) //nolint:makezero // pre-sized for index assignment
	sem := make(chan struct{}, workersFor(len(items), parallel))

	var wg sync.WaitGroup

	for i, item := range items {
		wg.Go(func() {
			sem <- struct{}{}

			defer func() { <-sem }()

			results[i] = fn(item)
		})
	}

	wg.Wait()

	slices.SortFunc(results, func(a, b R) int {
		return strings.Compare(key(a), key(b))
	})

	return results
}

// analyzeAll analyzes every root with a bounded worker pool and returns the
// results sorted by root for deterministic output. parallel <= 0 selects the
// automatic worker count.
func analyzeAll(roots []string, parallel int, opts ...surface.AnalyzeOption) []repoAnalysis {
	return runSorted(roots, parallel, func(a repoAnalysis) string { return a.root },
		func(root string) repoAnalysis {
			analyzed, err := analyzeAt(root, opts...)

			return repoAnalysis{root: root, report: analyzed, err: err}
		})
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

// expectMinorOptions validates the --expect-minor flag value into Analyze
// options. An invalid value is a usage error: the message names the
// offending value and the accepted shape, exit 2.
func expectMinorOptions(out io.Writer, value string) ([]surface.AnalyzeOption, int) {
	if value == "" {
		return nil, exitOK
	}

	opt, err := surface.WithExpectedMinor(value)
	if err != nil {
		fmt.Fprintf(out, "--expect-minor: %v (want major.minor, e.g. 1.27)\n", err)

		return nil, exitError
	}

	return []surface.AnalyzeOption{opt}, exitOK
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
		fmt.Fprintf(
			out,
			"FOUND  %-26s %s:%d\n       %s\n",
			issue.Rule,
			issue.File,
			issue.Line,
			issue.Message,
		)

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

// applyAll applies every repository's mechanical fixes in parallel. Clean
// and suggest-only repositories skip the go tool entirely — the fast path
// that keeps fleet sweeps linear in the drifted-repo count, not the repo
// count.
func applyAll(
	ctx context.Context,
	analyses []repoAnalysis,
	opts fix.Options,
	parallel int,
) []fixOutcome {
	return runSorted[repoAnalysis, fixOutcome](analyses, parallel,
		func(o fixOutcome) string { return o.root },
		func(a repoAnalysis) fixOutcome { return fixOne(ctx, a, opts) })
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
		fmt.Fprintf(
			out,
			"DISCOVERY  %-22s %s:%d\n           %s\n",
			issue.Rule,
			issue.File,
			issue.Line,
			issue.Message,
		)
	}

	for _, issue := range outcome.suggested {
		fmt.Fprintf(
			out,
			"SUGGEST  %-24s %s:%d\n         %s\n",
			issue.Rule,
			issue.File,
			issue.Line,
			issue.Suggestion,
		)
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

// analyzeFloorsAll runs the who-forces floor analysis over every root with a
// bounded worker pool, returning results sorted by root. parallel <= 0
// selects the automatic worker count.
func analyzeFloorsAll(roots []string, parallel int) []floorsResult {
	return runSorted(roots, parallel, func(r floorsResult) string { return r.root },
		func(root string) floorsResult {
			rows, err := fix.AnalyzeFloors(context.Background(), root, nil)

			return floorsResult{root: root, rows: rows, err: err}
		})
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
			fmt.Fprintf(
				out,
				"    %s@%s  (floor go %s)\n",
				poisoner.Module,
				poisoner.Version,
				poisoner.Floor,
			)
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
