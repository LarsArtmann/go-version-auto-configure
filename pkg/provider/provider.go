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

	// providerOptionRespectPatchFloor is the declared tool option (set via
	// BuildFlow's tool_options config key) that leaves patch-form go
	// directives alone while they do not exceed the installed toolchain.
	providerOptionRespectPatchFloor = "respect_patch_floor"

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
	spec.Options = []toolsdk.Option{{
		Name:    providerOptionRespectPatchFloor,
		Kind:    toolsdk.OptionKindBool,
		Default: false,
		Description: "leave patch-form go directives alone while they do not exceed the " +
			"installed toolchain (fleet hosts pinning an exact patch); above it they stay drift",
	}}

	return toolsdk.Register(spec)
}

// analyze discovers the version surface under the working directory and
// converts policy issues into SDK config issues.
func analyze(ctx context.Context) ([]autoconfigure.ConfigIssue, error) {
	root := workingDir(ctx)

	analyzeOpts, err := respectPatchFloorOptions(ctx, root)
	if err != nil {
		return nil, err
	}

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

	for _, issue := range surface.Analyze(surf, analyzeOpts...) {
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

	analyzeOpts, err := respectPatchFloorOptions(ctx, root)
	if err != nil {
		return "", err
	}

	s, _, err := surface.Discover(root)
	if err != nil {
		return "", fmt.Errorf("%s repair: %w", toolName, err)
	}

	var fixes []surface.Fix

	for _, issue := range surface.Analyze(s, analyzeOpts...) {
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

// respectPatchFloorOptions reads the respect_patch_floor tool option from
// the context (set per repo via BuildFlow's tool_options key) and resolves
// it into the surface analyze option: installed toolchain probed once per
// call, passed to every rule. A run without the option (or with it unset)
// analyzes on the default policy. A probe failure is an error, not a silent
// fallback: flipping the policy mid-fleet would rewrite exactly the pins
// the option exists to protect.
func respectPatchFloorOptions(ctx context.Context, root string) ([]surface.AnalyzeOption, error) {
	values, ok := toolsdk.OptionsFromContext(ctx)
	if !ok {
		return nil, nil
	}

	respect, ok := values[providerOptionRespectPatchFloor].(bool)
	if !ok || !respect {
		return nil, nil
	}

	installed, err := fix.InstalledToolchain(ctx, root)
	if err != nil {
		return nil, fmt.Errorf(
			"%s: resolve installed toolchain for %s: %w", toolName, providerOptionRespectPatchFloor, err,
		)
	}

	opt, err := surface.WithRespectPatchFloor(string(installed))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", toolName, err)
	}

	return []surface.AnalyzeOption{opt}, nil
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
