package surface

import (
	"fmt"
	"strings"
)

// Analyze encodes the fleet's Go toolchain versioning policy as checks over
// a discovered Surface:
//
//  1. `go` directives (go.mod and go.work) are major.minor only — a patch
//     component raises the minimum toolchain to one exact patch and breaks
//     environments that trail the newest release (Nix, CI runners).
//  2. A go.work directive must cover every workspace module at FULL patch
//     granularity: one below the module floor is a mechanical fix (raise
//     it); one whose patch component is required by a dep-forced module
//     floor is correct and reported by no rule at all.
//  3. Nix and CI pins must not trail the repo's effective floor — the
//     module floor, or a newer `toolchain` minor; otherwise the sandbox
//     builds with (or downloads) a different toolchain than local.
//  4. `toolchain` directives below the same file's `go` directive are dead
//     weight: the go command ignores them.
//  5. CI patch pins are flagged as suggestions: they silently diverge from
//     the nixpkgs minor the flakes track.
//
// Form violations (rule 1, the raisable half of rule 2) carry a mechanical
// Fix; alignment violations carry suggestions only, because which side
// moves (pin vs floor) is a maintainer decision and downgrades are not
// auto-applied.
func Analyze(surf *Surface, opts ...AnalyzeOption) []Issue {
	var issues []Issue

	policy := analyzePolicy{}

	for _, opt := range opts {
		opt(&policy)
	}

	fullFloor, hasFull := surf.FullModuleFloor()

	issues = append(issues, formIssues(surf, fullFloor, hasFull)...)
	issues = append(issues, goWorkBelowFloor(surf, fullFloor, hasFull)...)
	issues = append(issues, nonVersionToolchains(surf)...)
	issues = append(issues, staleToolchains(surf)...)

	if policy.hasExpectMinor {
		issues = append(issues, exceedsExpectation(surf, policy.expectMinor)...)
	}

	floor, toolDriver, hasAlign := pinAlignment(surf)

	if hasAlign {
		issues = append(issues, nixPinIssues(surf, floor, toolDriver)...)
		issues = append(issues, ciPinIssues(surf, floor, toolDriver)...)
	}

	return issues
}

// AnalyzeOption adjusts the policy Analyze enforces over a Surface.
type AnalyzeOption func(*analyzePolicy)

// analyzePolicy carries the policy inputs Analyze applies beyond the
// structural rules: currently only the fleet-expected minor.
type analyzePolicy struct {
	expectMinor    majorMinor
	hasExpectMinor bool
}

// WithExpectedMinor sets the fleet-expected Go minor (e.g. "1.27", ADR-0001):
// any version surface whose minor exceeds it fires RuleMinorExceedsExpectation.
// The value must parse as a major.minor version; the error names the
// offending value so CLI flag validation can surface it.
func WithExpectedMinor(v string) (AnalyzeOption, error) {
	mm, err := parseMajorMinor(v)
	if err != nil {
		return nil, fmt.Errorf("expect-minor %q: %w", v, err)
	}

	return func(p *analyzePolicy) { p.expectMinor, p.hasExpectMinor = mm, true }, nil
}

// exceedsExpectation reports every version surface whose minor is NEWER
// than the expected fleet minor. Patch components are ignored here: minor
// policy is major.minor (patch-form surfaces have their own rules).
func exceedsExpectation(surf *Surface, expect majorMinor) []Issue {
	var issues []Issue

	for _, m := range surf.Modules {
		if !minorExceeds(string(m.Version), expect) {
			continue
		}

		issues = append(issues, Issue{
			Rule: RuleMinorExceedsExpectation,
			Message: fmt.Sprintf(
				"%s declares go %s, above the expected minor go %s (fleet policy)",
				m.Path,
				m.Version,
				expect.String(),
			),
			File: m.Path,
			Line: m.Line,
			Suggestion: fmt.Sprintf(
				"align the directive down to go %s, or revisit the fleet minor (downgrades are never auto-applied)",
				expect.String(),
			),
		})
	}

	for _, tc := range surf.Toolchains {
		if !minorExceeds(string(tc.Version), expect) {
			continue
		}

		issues = append(issues, Issue{
			Rule: RuleMinorExceedsExpectation,
			Message: fmt.Sprintf(
				"%s pins toolchain %s, above the expected minor go %s (fleet policy)",
				tc.Path,
				tc.Version,
				expect.String(),
			),
			File: tc.Path,
			Line: tc.Line,
			Suggestion: fmt.Sprintf(
				"align the toolchain pin down to a go %s release, or revisit the fleet minor (downgrades are never auto-applied)",
				expect.String(),
			),
		})
	}

	issues = append(issues, pinExceedsExpectation(surf.NixPins, expect)...)
	issues = append(issues, pinExceedsExpectation(surf.CIPins, expect)...)

	return issues
}

// pinExceedsExpectation reports flake or CI pins above the expected minor.
func pinExceedsExpectation(pins []Pin, expect majorMinor) []Issue {
	var issues []Issue

	for _, pin := range pins {
		if !minorExceeds(string(pin.Version), expect) {
			continue
		}

		issues = append(issues, Issue{
			Rule: RuleMinorExceedsExpectation,
			Message: fmt.Sprintf(
				"%s pins %s (%s), above the expected minor go %s (fleet policy)",
				pin.Path,
				pin.Raw,
				pin.Source,
				expect.String(),
			),
			File: pin.Path,
			Line: pin.Line,
			Suggestion: fmt.Sprintf(
				"align the pin down to go %s, or revisit the fleet minor (downgrades are never auto-applied)",
				expect.String(),
			),
		})
	}

	return issues
}

// minorExceeds reports whether v parses and its major.minor is strictly
// newer than expect. Unparseable versions never exceed: they surface
// through their own discovery rules.
func minorExceeds(v string, expect majorMinor) bool {
	mm, err := parseMajorMinor(v)
	if err != nil {
		return false
	}

	return mm.Major > expect.Major || (mm.Major == expect.Major && mm.Minor > expect.Minor)
}

// formIssues reports patch-form `go` directives together with their
// mechanical rewrites. A go.work patch form is only a violation when the
// stripped minor form still covers the workspace's full module floor.
//
// Severity policy: a dep-forced patch-form go directive (a dependency holds
// the floor; fix's tidy gate reverts the rewrite) is a CORRECT state held
// for supply-side reasons — the finding stays reportable for visibility but
// advisory by design, never an error: the remediation is re-tagging the
// poisoner, which only the poisoner's repo can do (owner decision
// 2026-09-25, see docs/adr/0001 appendix go-health v0.4.0).
func formIssues(s *Surface, fullFloor GoVersion, hasFull bool) []Issue {
	var issues []Issue

	for _, m := range s.Modules {
		if !hasPatch(string(m.Version)) {
			continue
		}

		parsed, err := parseMajorMinor(string(m.Version))
		if err != nil {
			continue //nolint:erraudit // deliberate filter: unparseable directives surface as unparseable discovery findings
		}

		// A go.work directive must cover every workspace module under the go
		// tool's FULL-patch comparison: the tool rejects a workspace whose
		// directive is below any module floor AT PATCH GRANULARITY, and
		// patch-less minor forms rank below every patch form of the same
		// minor (go 1.26 < go 1.26.0 — verified against the go tool:
		// "module m listed in go.work file requires go >= 1.26.0, but
		// go.work lists go 1.26"). Offer the patch-form strip only when the
		// stripped minor form still covers the full module floor; a floor
		// carrying any patch component (including .0) keeps the go.work
		// patch REQUIRED. A go.work directive already BELOW the full floor
		// is goWorkBelowFloor's concern instead; its fix restores the floor
		// rather than stripping the patch.
		if m.Kind == KindGoWork && hasFull && !floorCoveredByDirective(GoVersion(parsed.String()), fullFloor) {
			continue
		}

		issues = append(issues, Issue{
			Rule:    formRule(m.Kind),
			Message: formMessage(m),
			File:    m.Path,
			Line:    m.Line,
			Fix: &Fix{
				File: m.Path,
				Kind: m.Kind,
				From: m.Version,
				To:   GoVersion(parsed.String()),
				Line: m.Line,
			},
		})
	}

	return issues
}

// goWorkBelowFloor reports go.work directives sitting below the workspace's
// full module floor and carries the always-safe mechanical fix: raise the
// directive to the floor. See RuleGoWorkBelowFloor for the breakage class.
func goWorkBelowFloor(s *Surface, fullFloor GoVersion, hasFull bool) []Issue {
	var issues []Issue

	if !hasFull {
		return issues
	}

	for _, m := range s.Modules {
		if m.Kind != KindGoWork {
			continue
		}

		if _, err := parseMajorMinor(string(m.Version)); err != nil {
			continue //nolint:erraudit // deliberate filter: unparseable directives surface as unparseable discovery findings
		}

		if floorCoveredByDirective(m.Version, fullFloor) {
			continue
		}

		issues = append(issues, Issue{
			Rule: RuleGoWorkBelowFloor,
			Message: fmt.Sprintf(
				"%s declares go %s: it does not cover the workspace module floor go %s; "+
					"every `go` command in the workspace fails with \"module X listed in go.work file "+
					"requires go >= %s, but go.work lists go %s\" until the floor is restored",
				m.Path, m.Version, fullFloor, fullFloor, m.Version,
			),
			File: m.Path,
			Line: m.Line,
			Fix: &Fix{
				File: m.Path,
				Kind: KindGoWork,
				From: m.Version,
				To:   fullFloor,
				Line: m.Line,
			},
		})
	}

	return issues
}

// floorCoveredByDirective reports whether a go.work directive satisfies a
// module floor under the go tool's comparison rules. The tool compares
// directives at FULL patch granularity AND ranks the bare minor below
// every patch form of that minor: go 1.26 does not cover go 1.26.0
// (verified against the go tool 2026-09-22: "module m listed in go.work
// file requires go >= 1.26.0, but go.work lists go 1.26"). GreaterVersion
// alone misses that edge: it reads go 1.26.0 and go 1.26 as equal.
func floorCoveredByDirective(directive, floor GoVersion) bool {
	if directive == floor {
		return true
	}

	return GreaterVersion(string(directive), string(floor))
}

// formRule names the patch-form rule for a directive kind.
func formRule(kind DirectiveKind) Rule {
	if kind == KindGoWork {
		return RuleWorkDirectivePatchForm
	}

	return RuleGoDirectivePatchForm
}

// formMessage describes a patch-form directive.
func formMessage(m ModuleDirective) string {
	return fmt.Sprintf(
		"%s declares go %s: the go directive is a floor and must be major.minor only; "+
			"a patch component pins the toolchain to one exact patch and breaks trailing environments",
		m.Path, m.Version,
	)
}

// nonVersionToolchains reports `toolchain` directives that name no explicit
// toolchain version ("local", "default"): legal, but they pin nothing and
// opt the module out of toolchain resolution, so floor analysis would
// otherwise silently ignore the line.
func nonVersionToolchains(s *Surface) []Issue {
	var issues []Issue

	for _, tc := range s.Toolchains {
		if _, err := parseMajorMinor(string(tc.Version)); err == nil {
			continue
		}

		issues = append(issues, Issue{
			Rule: RuleToolchainNonVersion,
			Message: fmt.Sprintf(
				"%s declares toolchain %s: this names no explicit toolchain version, "+
					"so the line pins nothing and is excluded from floor analysis",
				tc.Path, tc.Version,
			),
			File: tc.Path,
			Line: tc.Line,
		})
	}

	return issues
}

// staleToolchains reports `toolchain` directives the go command ignores
// because they sit below the same file's `go` directive.
func staleToolchains(s *Surface) []Issue {
	var issues []Issue

	for _, tc := range s.Toolchains {
		goVersion, ok := goDirectiveIn(s, tc)

		if !ok {
			continue
		}

		toolParsed, err := parseMajorMinor(string(tc.Version))
		if err != nil {
			continue //nolint:erraudit // deliberate filter: non-version toolchains surface as toolchain-non-version instead
		}

		goParsed, err := parseMajorMinor(string(goVersion))
		if err != nil {
			continue //nolint:erraudit // deliberate filter: the go directive of this file was already validated during discovery
		}

		if !toolParsed.lessThan(goParsed) {
			continue
		}

		issues = append(issues, Issue{
			Rule:    RuleToolchainBelowDirective,
			Message: staleToolchainMessage(tc, goVersion),
			File:    tc.Path,
			Line:    tc.Line,
			Suggestion: fmt.Sprintf(
				"remove the stale directive: go %s edit -toolchain=none",
				editVerb(tc.Kind),
			),
		})
	}

	return issues
}

// staleToolchainMessage describes a toolchain directive the go command
// ignores.
func staleToolchainMessage(tc ToolchainDirective, goVersion GoVersion) string {
	return fmt.Sprintf(
		"%s pins toolchain %s below the same file's go %s: "+
			"the go command ignores a toolchain older than the go directive, so the line is dead weight",
		tc.Path, tc.Version, goVersion,
	)
}

// goDirectiveIn returns the `go` directive version declared in the same
// file as the given toolchain directive.
func goDirectiveIn(s *Surface, tc ToolchainDirective) (GoVersion, bool) {
	for _, m := range s.Modules {
		if m.Path == tc.Path {
			return m.Version, true
		}
	}

	return "", false
}

// editVerb names the go subcommand that edits a directive kind's file.
func editVerb(kind DirectiveKind) string {
	if kind == KindGoWork {
		return "work"
	}

	return "mod"
}

// pinAlignment returns the floor that Nix and CI pins must not trail: the
// highest `go` floor, unless a `toolchain` directive names a newer minor
// (the go command would switch toolchains). toolDriver names that directive
// when it drives the floor; ok is false when the repo declares no floor at
// all.
func pinAlignment(s *Surface) (majorMinor, *ToolchainDirective, bool) {
	floor, ok := s.Floor()
	toolFloor, hasTool := s.toolchainFloor()

	if !hasTool || (ok && !toolFloor.greaterThan(floor)) {
		return floor, nil, ok
	}

	floor = toolFloor

	for i := range s.Toolchains {
		tc := &s.Toolchains[i]

		if parsed, err := parseMajorMinor(string(tc.Version)); err == nil && parsed == toolFloor {
			return floor, tc, true
		}
	}

	return floor, nil, true
}

// floorPhrase describes the floor a pin trails, for messages: "the module
// floor is 1.26", or the toolchain directive that raises it higher.
func floorPhrase(floor majorMinor, toolchain *ToolchainDirective) string {
	if toolchain != nil {
		return fmt.Sprintf(
			"toolchain %s in %s raises the effective floor to %s",
			toolchain.Version,
			toolchain.Path,
			floor,
		)
	}

	return fmt.Sprintf("the module floor is %s", floor)
}

// nixPinIssues report flake pins that trail the effective floor.
func nixPinIssues(s *Surface, floor majorMinor, toolDriver *ToolchainDirective) []Issue {
	var issues []Issue

	for _, pin := range s.NixPins {
		parsed, err := parseMajorMinor(string(pin.Version))
		if err != nil {
			continue //nolint:erraudit // deliberate filter: the scanner records only comparable pins
		}

		if !parsed.lessThan(floor) {
			continue
		}

		issues = append(issues, Issue{
			Rule:    RuleNixPinBelowFloor,
			Message: nixPinMessage(pin, floor, toolDriver),
			File:    pin.Path,
			Line:    pin.Line,
			Suggestion: fmt.Sprintf(
				"raise the flake's nixpkgs Go pin to go_%s (or newer), then run the Nix hash repair",
				strings.ReplaceAll(floor.String(), ".", "_"),
			),
		})
	}

	return issues
}

// nixPinMessage describes a flake pin trailing the effective floor.
func nixPinMessage(pin Pin, floor majorMinor, toolDriver *ToolchainDirective) string {
	const tail = "the Nix sandbox builds with (or downloads) a toolchain older than the repo requires"

	return fmt.Sprintf(
		"%s pins Go %s but %s: %s",
		pin.Path,
		pin.Version,
		floorPhrase(floor, toolDriver),
		tail,
	)
}

// ciPinIssues report workflow pins that trail the effective floor or pin
// one exact patch.
func ciPinIssues(s *Surface, floor majorMinor, toolDriver *ToolchainDirective) []Issue {
	var issues []Issue

	for _, pin := range s.CIPins {
		parsed, ok := parseCIPin(string(pin.Version))

		if !ok {
			continue
		}

		if parsed.lessThan(floor) {
			issues = append(issues, Issue{
				Rule:    RuleCIPinBelowFloor,
				Message: ciPinMessage(pin, floor, toolDriver),
				File:    pin.Path,
				Line:    pin.Line,
				Suggestion: fmt.Sprintf(
					"set go-version to %s to match the effective floor",
					floor.String(),
				),
			})

			continue
		}

		if hasPatch(string(pin.Raw)) {
			issues = append(issues, Issue{
				Rule:    RuleCIPinPatchForm,
				Message: ciPatchPinMessage(pin, floor),
				File:    pin.Path,
				Line:    pin.Line,
				Suggestion: fmt.Sprintf(
					"set go-version to %s so CI tracks the same minor as the flake pin",
					floor.String(),
				),
			})
		}
	}

	return issues
}

// ciPinMessage describes a CI pin trailing the effective floor.
func ciPinMessage(pin Pin, floor majorMinor, toolDriver *ToolchainDirective) string {
	const tail = "CI builds with a toolchain older than the repo requires"

	return fmt.Sprintf(
		"%s pins go-version %s but %s: %s",
		pin.Path,
		pin.Version,
		floorPhrase(floor, toolDriver),
		tail,
	)
}

// ciPatchPinMessage describes a CI pin that tracks one exact patch.
func ciPatchPinMessage(pin Pin, floor majorMinor) string {
	return fmt.Sprintf(
		"%s pins go-version %s with a patch component: "+
			"CI tracks one exact patch while the flakes move with nixpkgs %s, so they silently diverge",
		pin.Path, pin.Version, floor,
	)
}
