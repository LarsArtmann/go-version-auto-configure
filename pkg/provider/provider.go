// Package provider wires go-version-auto-configure into BuildFlow's DAG via
// the go-finding/toolsdk Spec contract. BuildFlow discovers this Provider
// automatically through toolsdk.All() when a consumer blank-imports this
// package:
//
//	import _ "github.com/larsartmann/go-version-auto-configure/pkg/provider"
//
// The Spec is built with linter-autoconfigure-sdk's ProviderFromSpec so the
// finding emission is owned by the shared SDK instead of re-implemented
// here.
package provider

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-finding"
	toolsdk "github.com/larsartmann/go-finding/toolsdk"
	"github.com/larsartmann/go-version-auto-configure/pkg/fix"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
	autoconfigure "github.com/larsartmann/linter-autoconfigure-sdk"
)

const (
	// toolName is the BuildFlow DAG tool name.
	toolName = "go-version-auto-configure"

	// description appears in BuildFlow --list output.
	description = "Detects Go toolchain version-surface drift (patch versions in go directives, " +
		"Nix/CI pins below the module floor) and auto-fixes directive form via go mod edit / go work edit"
)

//nolint:gochecknoglobals // BuildFlow plugin SDK requires package-level Provider registration
var Provider = mustProvider()

// mustProvider builds the Spec through the shared SDK bridge and layers the
// Go-specific DAG wiring (Trigger) on top. Registration panics on an
// invalid Spec, the same contract as toolsdk.Register.
func mustProvider() toolsdk.Spec {
	spec, err := autoconfigure.ProviderFromSpec(autoconfigure.ProviderSpec{
		Name:        toolName,
		Description: description,
		ConfigFile:  finding.FilePath("go.mod"),
		Analyze:     analyze,
		Repair:      repair,
	})
	if err != nil {
		panic("provider: invalid spec: " + err.Error())
	}

	spec.Trigger = toolsdk.OnFiles("go", "go.mod", "go.work")
	spec.HealthCheck = fix.SelfCheck

	return toolsdk.Register(spec)
}

// analyze discovers the version surface under the working directory and
// converts policy issues into SDK config issues.
func analyze(ctx context.Context) ([]autoconfigure.ConfigIssue, error) {
	root := workingDir(ctx)

	surf, discoverIssues, err := surface.Discover(root)
	if err != nil {
		return nil, fmt.Errorf("%s analyze: %w", toolName, err)
	}

	issues := make([]autoconfigure.ConfigIssue, 0, len(discoverIssues))
	for _, di := range discoverIssues {
		issues = append(issues, autoconfigure.ConfigIssue{
			Rule:     finding.RuleName(di.Rule),
			Message:  di.Message,
			Severity: finding.SeverityWarning,
			File:     finding.FilePath(di.File),
			Line:     di.Line,
		})
	}

	for _, issue := range surface.Analyze(surf) {
		issues = append(issues, toConfigIssue(issue))
	}

	return issues, nil
}

// toConfigIssue maps one surface issue to the SDK shape. Mechanical fixes
// carry their exact rewrite as the suggestion; alignment issues carry
// their suggested action.
func toConfigIssue(issue surface.Issue) autoconfigure.ConfigIssue {
	suggestion := issue.Suggestion
	if issue.Fix != nil {
		suggestion = issue.Fix.Describe()
	}

	return autoconfigure.ConfigIssue{
		Rule:       finding.RuleName(issue.Rule),
		Message:    issue.Message,
		Severity:   finding.SeverityWarning,
		File:       finding.FilePath(issue.File),
		Line:       issue.Line,
		Suggestion: suggestion,
	}
}

// repair applies every mechanical fix, honoring BuildFlow's dry-run
// context. Suggest-only issues are intentionally untouched: which side of
// an alignment moves (pin or floor) is a maintainer decision.
func repair(ctx context.Context) (string, error) {
	root := workingDir(ctx)

	s, _, err := surface.Discover(root)
	if err != nil {
		return "", fmt.Errorf("%s repair: %w", toolName, err)
	}

	var fixes []surface.Fix

	for _, issue := range surface.Analyze(s) {
		if issue.Fix != nil {
			fixes = append(fixes, *issue.Fix)
		}
	}

	if len(fixes) == 0 {
		return "no mechanical version-surface fixes needed", nil
	}

	res, err := fix.Apply(ctx, root, fixes, fix.Options{DryRun: toolsdk.DryRunFromContext(ctx)}, nil)
	if err != nil {
		return "", fmt.Errorf("%s repair: %w", toolName, err)
	}

	return res.Report(), nil
}

// workingDir resolves the project directory from the context, falling back
// to the process working directory, mirroring the other BuildFlow providers
// so the WithWorkingDir fan-out works identically.
func workingDir(ctx context.Context) string {
	if dir := finding.WorkingDirFromContext(ctx); dir != "" {
		return dir
	}

	return "."
}
