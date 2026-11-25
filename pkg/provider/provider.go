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
		"Nix/CI pins below the module floor) and auto-fixes directive form and below-floor CI " +
		"go-version pins"
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
		// ConfigFiles is the full read surface Discover dispatches on, not
		// just the write target: the bridge derives Spec.Inputs from it, so
		// BuildFlow gates re-runs and derives data-flow edges for every
		// file class the tool actually reads (flake.nix and CI-pin drift
		// included). Keep in sync with Surface.absorbFile's dispatch.
		ConfigFiles: []finding.FilePath{
			"go.mod",
			"go.work",
			"flake.nix",
			".github/workflows/*.yml",
			".github/workflows/*.yaml",
			"VERSION",
			"CHANGELOG.md",
		},
		Analyze: analyze,
		Repair:  repair,
	})
	if err != nil {
		panic("provider: invalid spec: " + err.Error())
	}

	spec.Trigger = toolsdk.OnFiles("go", "go.mod", "go.work")
	// Outputs is the repair write contract: the only files fix.Apply
	// rewrites (go.mod directive surgery, go work edit, CI go-version pin
	// raises). flake.nix, VERSION and CHANGELOG.md are read-only surfaces —
	// moving those pins is a maintainer decision the tool never performs.
	spec.Outputs = []string{
		"go.mod",
		"go.work",
		".github/workflows/*.yml",
		".github/workflows/*.yaml",
	}
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

// ToolName is the provider's tool identity, shared by the CLI's SARIF
// export so CLI and BuildFlow findings carry the same tool descriptor.
const ToolName = toolName

// analyze discovers the version surface under the working directory and
// converts policy issues into SDK config issues, annotating tidy-held
// patch-form findings with the dependencies holding the floor.
func analyze(ctx context.Context) ([]autoconfigure.ConfigIssue, error) {
	return analyzeWith(ctx, fix.EditRunner())
}

// analyzeWith is analyze with the dependency-floor listing runner injected:
// production shells out through EditRunner, tests fake `go list -m` output.
func analyzeWith(ctx context.Context, run fix.GoCommandRunner) ([]autoconfigure.ConfigIssue, error) {
	root := autoconfigure.WorkingDir(ctx)

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

	policyIssues := surface.Analyze(surf, analyzeOpts...)
	held := heldStrips(ctx, root, run, policyIssues)

	for _, issue := range policyIssues {
		issues = append(issues, toConfigIssue(issue, held))
	}

	return issues, nil
}

// heldStrips resolves the dep-forced annotations for the policy issues:
// which patch-form go.mod strips `go mod tidy` would revert, and which
// dependencies hold the floor. The module graph is listed only when a
// patch-form finding exists (clean repos pay nothing); a listing failure
// degrades to no annotation, because the finding is already true and a
// failed enrichment must not red detection. Only go.mod patch-form
// findings annotate: a go.work strip is offered solely when its minor form
// still covers the full module floor, so a dep-forced module floor
// suppresses that fix instead of holding it.
func heldStrips(
	ctx context.Context,
	root string,
	run fix.GoCommandRunner,
	issues []surface.Issue,
) map[string]fix.HeldStrip {
	var files []string

	for _, issue := range issues {
		if issue.Rule == surface.RuleGoDirectivePatchForm && issue.Fix != nil {
			files = append(files, issue.File)
		}
	}

	return fix.ExplainHeldStrips(ctx, root, run, files)
}

// toConfigIssue maps one surface issue to the SDK shape. Mechanical fixes
// carry their exact rewrite as the suggestion; alignment issues carry
// their suggested action. A tidy-held strip swaps both for the dep-forced
// explanation: the rewrite would be silently reverted, so naming the
// carriers and the supply-side remediation is the honest suggestion.
func toConfigIssue(issue surface.Issue, held map[string]fix.HeldStrip) autoconfigure.ConfigIssue {
	suggestion := issue.Suggestion
	if issue.Fix != nil {
		suggestion = issue.Fix.Describe()
	}

	message := issue.Message

	if issue.Rule == surface.RuleGoDirectivePatchForm {
		if strip, ok := held[issue.File]; ok {
			message = message + "; " + strip.Reason()
			suggestion = strip.Remediation()
		}
	}

	return autoconfigure.ConfigIssue{
		Rule:       finding.RuleName(issue.Rule),
		Message:    message,
		Severity:   finding.SeverityWarning,
		File:       finding.FilePath(issue.File),
		Line:       issue.Line,
		Suggestion: suggestion,
	}
}

// FindingsFromIssues converts CLI-side surface issues into go-finding
// findings — the SARIF export path's single conversion point, so CLI and
// BuildFlow findings share the bridge. It is the plain conversion: the
// dep-forced held-strip annotation is detect-time-only (toConfigIssue's
// held map) and intentionally absent here, matching what the CLI's human
// and schema-2 JSON reports show.
func FindingsFromIssues(tool string, issues []surface.Issue) []finding.Finding {
	findings := make([]finding.Finding, 0, len(issues))
	for _, issue := range issues {
		f, err := autoconfigure.FindingFromIssue(finding.ToolName(tool), toConfigIssue(issue, nil))
		if err != nil {
			continue
		}

		findings = append(findings, f)
	}

	return findings
}

// repair applies every mechanical fix, honoring BuildFlow's dry-run
// context. Suggest-only issues are intentionally untouched: which side of
// an alignment moves (pin or floor) is a maintainer decision.
func repair(ctx context.Context) (string, error) {
	root := autoconfigure.WorkingDir(ctx)

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
	respect, err := toolsdk.BoolFromContext(ctx, providerOptionRespectPatchFloor, false)
	if err != nil {
		return nil, fmt.Errorf(
			"%s: read %s option: %w", toolName, providerOptionRespectPatchFloor, err,
		)
	}

	if !respect {
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
