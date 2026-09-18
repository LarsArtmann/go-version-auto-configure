package main

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"os"

	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// The JSON shapes below are the machine-readable contract of --json: field
// names and presence (omitempty) are stable for CI consumers.

// jsonFix mirrors surface.Fix.
type jsonFix struct {
	File string `json:"file"`
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
	Line int    `json:"line"`
}

// jsonIssue mirrors surface.Issue; Fix is present only for mechanical
// findings, Suggestion only for alignment findings.
type jsonIssue struct {
	Rule       string   `json:"rule"`
	Message    string   `json:"message"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Suggestion string   `json:"suggestion,omitempty"`
	Fix        *jsonFix `json:"fix,omitempty"`
}

// jsonCounts summarizes one repository's findings by category.
type jsonCounts struct {
	Total      int `json:"total"`
	Mechanical int `json:"mechanical"`
	Suggested  int `json:"suggested"`
	Discovery  int `json:"discovery"`
}

// jsonRepo is one repository's check report.
type jsonRepo struct {
	Root     string      `json:"root"`
	Error    string      `json:"error,omitempty"`
	Clean    bool        `json:"clean"`
	Counts   jsonCounts  `json:"counts"`
	Findings []jsonIssue `json:"findings"`
}

// checkDocument is the --json payload of check.
type checkDocument struct {
	Repos []jsonRepo `json:"repos"`
}

// jsonDepForced is one tidy-reverted fix with its named poisoners.
type jsonDepForced struct {
	Fix       jsonFix  `json:"fix"`
	Floor     string   `json:"floor"`
	Poisoners []string `json:"poisoners,omitempty"`
}

// jsonFailure is one fix that could not be applied or verified.
type jsonFailure struct {
	Fix   jsonFix `json:"fix"`
	Cause string  `json:"cause"`
}

// jsonFixRepo is one repository's fix report.
type jsonFixRepo struct {
	Root      string          `json:"root"`
	Error     string          `json:"error,omitempty"`
	Applied   []jsonFix       `json:"applied"`
	HeldBack  []jsonFix       `json:"heldBack"`
	DepForced []jsonDepForced `json:"depForced"`
	Failures  []jsonFailure   `json:"failures"`
	Suggested []jsonIssue     `json:"suggested"`
}

// fixDocument is the --json payload of fix.
type fixDocument struct {
	Repos []jsonFixRepo `json:"repos"`
}

// jsonFloorsRepo is one repository's who-forces report.
type jsonFloorsRepo struct {
	Root    string             `json:"root"`
	Error   string             `json:"error,omitempty"`
	Modules []fix.ModuleFloors `json:"modules"`
}

// floorsDocument is the --json payload of who-forces.
type floorsDocument struct {
	Repos []jsonFloorsRepo `json:"repos"`
}

func toFixJSON(f *surface.Fix) *jsonFix {
	if f == nil {
		return nil
	}

	return &jsonFix{File: f.File, Kind: string(f.Kind), From: f.From, To: f.To, Line: f.Line}
}

func toIssuesJSON(issues []surface.Issue) []jsonIssue {
	out := make([]jsonIssue, 0, len(issues))

	for _, issue := range issues {
		out = append(out, jsonIssue{
			Rule:       issue.Rule,
			Message:    issue.Message,
			File:       issue.File,
			Line:       issue.Line,
			Suggestion: issue.Suggestion,
			Fix:        toFixJSON(issue.Fix),
		})
	}

	return out
}

func toFixesJSON(fixes []surface.Fix) []jsonFix {
	out := make([]jsonFix, 0, len(fixes))

	for _, f := range fixes {
		out = append(out, jsonFix{File: f.File, Kind: string(f.Kind), From: f.From, To: f.To, Line: f.Line})
	}

	return out
}

// toJSONRepo converts one analysis into the check report shape.
func toJSONRepo(a repoAnalysis) jsonRepo {
	if a.err != nil {
		return jsonRepo{Root: a.root, Error: a.err.Error()}
	}

	findings := make([]jsonIssue, 0, len(a.report.all()))
	findings = append(findings, toIssuesJSON(a.report.discovery)...)
	findings = append(findings, toIssuesJSON(a.report.mechanical)...)
	findings = append(findings, toIssuesJSON(a.report.suggested)...)

	return jsonRepo{
		Root:  a.root,
		Clean: len(findings) == 0,
		Counts: jsonCounts{
			Total:      len(findings),
			Mechanical: len(a.report.mechanical),
			Suggested:  len(a.report.suggested),
			Discovery:  len(a.report.discovery),
		},
		Findings: findings,
	}
}

// emitCheckJSON writes the check document, preserving the findings exit
// contract for machines.
func emitCheckJSON(out io.Writer, analyses []repoAnalysis) int {
	doc := checkDocument{Repos: make([]jsonRepo, 0, len(analyses))}

	for _, a := range analyses {
		doc.Repos = append(doc.Repos, toJSONRepo(a))
	}

	if code := emitJSON(out, &doc); code != exitOK {
		return code
	}

	return exitFromAnalyses(analyses)
}

// toJSONFixRepo converts one outcome into the fix report shape. The list
// fields are always present (empty, never null) so consumers can range
// without nil checks.
func toJSONFixRepo(outcome fixOutcome) jsonFixRepo {
	repo := jsonFixRepo{
		Root:      outcome.root,
		Applied:   []jsonFix{},
		HeldBack:  []jsonFix{},
		DepForced: []jsonDepForced{},
		Failures:  []jsonFailure{},
		Suggested: toIssuesJSON(outcome.suggested),
	}

	if outcome.err != nil {
		repo.Error = outcome.err.Error()

		return repo
	}

	if outcome.result == nil {
		return repo
	}

	repo.Applied = toFixesJSON(outcome.result.Applied)
	repo.HeldBack = toFixesJSON(outcome.result.HeldBack)

	for _, d := range outcome.result.DepForced {
		repo.DepForced = append(repo.DepForced, jsonDepForced{
			Fix:       *toFixJSON(&d.Fix),
			Floor:     d.Floor,
			Poisoners: d.Poisoners,
		})
	}

	for _, f := range outcome.result.Failures {
		repo.Failures = append(repo.Failures, jsonFailure{Fix: *toFixJSON(&f.Fix), Cause: f.Cause})
	}

	return repo
}

// emitFixJSON writes the fix document, preserving the failures exit
// contract for machines.
func emitFixJSON(out io.Writer, outcomes []fixOutcome) int {
	doc := fixDocument{Repos: make([]jsonFixRepo, 0, len(outcomes))}

	for _, o := range outcomes {
		doc.Repos = append(doc.Repos, toJSONFixRepo(o))
	}

	if code := emitJSON(out, &doc); code != exitOK {
		return code
	}

	return exitFromOutcomes(outcomes)
}

// emitFloorsJSON writes the who-forces document, preserving the poisoned
// exit contract for machines.
func emitFloorsJSON(out io.Writer, results []floorsResult) int {
	doc := floorsDocument{Repos: make([]jsonFloorsRepo, 0, len(results))}

	for _, r := range results {
		repo := jsonFloorsRepo{Root: r.root, Modules: r.rows}

		if repo.Modules == nil {
			repo.Modules = []fix.ModuleFloors{}
		}

		if r.err != nil {
			repo.Error = r.err.Error()
		}

		doc.Repos = append(doc.Repos, repo)
	}

	if code := emitJSON(out, &doc); code != exitOK {
		return code
	}

	return exitFromFloors(results)
}

// emitJSON writes doc as indented JSON on stdout.
func emitJSON(out io.Writer, doc any) int {
	enc := jsontext.NewEncoder(out, jsontext.WithIndent("  "))

	if err := jsonv2.MarshalEncode(enc, doc); err != nil {
		fmt.Fprintf(os.Stderr, "json: %v\n", err)

		return exitError
	}

	return exitOK
}
