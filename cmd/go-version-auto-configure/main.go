// Command go-version-auto-configure detects and repairs Go toolchain
// version-surface drift in a repository: patch components in go.mod/go.work
// `go` directives (auto-fixed via go mod edit / go work edit) and Nix/CI
// pins that trail the module floor (reported with suggestions).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	"github.com/larsartmann/go-version-auto-configure/pkg/version"
)

const usage = `go-version-auto-configure — unify the Go toolchain version surface

Usage:
  go-version-auto-configure check [root]         detect drift, exit 1 when found
  go-version-auto-configure fix [--dry-run] [root]
                                                 auto-fix directive form,
                                                 suggest the rest
  go-version-auto-configure version              print the tool version
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Printf("go-version-auto-configure %s\n", version.Version)
		return 0
	case "check":
		return cmdCheck(args[1:])
	case "fix":
		return cmdFix(args[1:])
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
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
	s, discovery, err := surface.Discover(root)
	if err != nil {
		return nil, err
	}
	r := &report{discovery: discovery}
	for _, issue := range surface.Analyze(s) {
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
	out := make([]surface.Issue, 0, len(r.discovery)+len(r.mechanical)+len(r.suggested))
	out = append(out, r.discovery...)
	out = append(out, r.mechanical...)
	out = append(out, r.suggested...)
	return out
}

func rootFrom(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "."
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	r, err := analyzeAt(rootFrom(fs.Args()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "check: %v\n", err)
		return 2
	}

	all := r.all()
	for _, issue := range all {
		fmt.Printf("FOUND  %-26s %s:%d\n       %s\n", issue.Rule, issue.File, issue.Line, issue.Message)
		if issue.Suggestion != "" {
			fmt.Printf("       fix: %s\n", issue.Suggestion)
		}
	}
	switch {
	case len(all) == 0:
		fmt.Println("version surface clean: go directives are major.minor, pins align with the module floor")
	case len(r.suggested) == 0:
		fmt.Printf("\n%d finding(s), all auto-fixable with 'fix'\n", len(all))
	default:
		fmt.Printf("\n%d finding(s): %d auto-fixable with 'fix', %d need a maintainer decision\n", len(all), len(r.mechanical), len(r.suggested))
	}
	if len(all) > 0 {
		return 1
	}
	return 0
}

func cmdFix(args []string) int {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "report what would change without touching files")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	r, err := analyzeAt(rootFrom(fs.Args()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "fix: %v\n", err)
		return 2
	}

	if len(r.all()) == 0 {
		fmt.Println("version surface clean: nothing to fix")
		return 0
	}

	fixes := make([]surface.Fix, 0, len(r.mechanical))
	for _, issue := range r.mechanical {
		fixes = append(fixes, *issue.Fix)
	}

	res, err := fix.Apply(context.Background(), rootFrom(fs.Args()), fixes, fix.Options{DryRun: *dryRun}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fix: %v\n", err)
		return 2
	}

	fmt.Println(res.Report())
	for _, issue := range r.suggested {
		fmt.Printf("SUGGEST  %-24s %s:%d\n         %s\n", issue.Rule, issue.File, issue.Line, issue.Suggestion)
	}

	if len(res.Failures) > 0 {
		return 1
	}
	return 0
}
