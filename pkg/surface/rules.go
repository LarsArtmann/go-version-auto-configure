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
//  2. Nix and CI pins must not trail the repo's effective floor — the
//     module floor, or a newer `toolchain` minor; otherwise the sandbox
//     builds with (or downloads) a different toolchain than local.
//  3. `toolchain` directives below the same file's `go` directive are dead
//     weight: the go command ignores them.
//  4. CI patch pins are flagged as suggestions: they silently diverge from
//     the nixpkgs minor the flakes track.
//
// Form violations (rule 1) carry a mechanical Fix; alignment violations
// carry suggestions only, because which side moves (pin vs floor) is a
// maintainer decision and downgrades are not auto-applied.
func Analyze(s *Surface) []Issue {
	var issues []Issue

	workspaceFloor, hasFloor := s.Floor()

	issues = append(issues, formIssues(s, workspaceFloor, hasFloor)...)
	issues = append(issues, staleToolchains(s)...)

	floor, toolDriver, hasAlign := pinAlignment(s)

	if hasAlign {
		issues = append(issues, nixPinIssues(s, floor, toolDriver)...)
		issues = append(issues, ciPinIssues(s, floor, toolDriver)...)
	}

	return issues
}

// formIssues reports patch-form `go` directives together with their
// mechanical rewrites.
func formIssues(s *Surface, workspaceFloor majorMinor, hasFloor bool) []Issue {
	var issues []Issue

	for _, m := range s.Modules {
		if !hasPatch(m.Version) {
			continue
		}

		parsed, err := parseMajorMinor(m.Version)
		if err != nil {
			continue
		}

		// go.work must cover every module it lists: when the workspace
		// floor is higher than this directive's minor, the normalized
		// target is the workspace floor, not the stripped directive —
		// otherwise the rewrite would leave the workspace unable to
		// resolve its own modules.
		target := parsed

		if m.Kind == KindGoWork && hasFloor && target.lessThan(workspaceFloor) {
			target = workspaceFloor
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
				To:   target.String(),
				Line: m.Line,
			},
		})
	}

	return issues
}

// formRule names the patch-form rule for a directive kind.
func formRule(kind DirectiveKind) string {
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

// staleToolchains reports `toolchain` directives the go command ignores
// because they sit below the same file's `go` directive.
func staleToolchains(s *Surface) []Issue {
	var issues []Issue

	for _, tc := range s.Toolchains {
		goVersion, ok := goDirectiveIn(s, tc)

		if !ok {
			continue
		}

		toolParsed, err := parseMajorMinor(tc.Version)
		if err != nil {
			continue
		}

		goParsed, err := parseMajorMinor(goVersion)
		if err != nil {
			continue
		}

		if !toolParsed.lessThan(goParsed) {
			continue
		}

		issues = append(issues, Issue{
			Rule:       RuleToolchainBelowDirective,
			Message:    staleToolchainMessage(tc, goVersion),
			File:       tc.Path,
			Line:       tc.Line,
			Suggestion: fmt.Sprintf("remove the stale directive: go %s edit -toolchain=none", editVerb(tc.Kind)),
		})
	}

	return issues
}

// staleToolchainMessage describes a toolchain directive the go command
// ignores.
func staleToolchainMessage(tc ToolchainDirective, goVersion string) string {
	return fmt.Sprintf(
		"%s pins toolchain %s below the same file's go %s: "+
			"the go command ignores a toolchain older than the go directive, so the line is dead weight",
		tc.Path, tc.Version, goVersion,
	)
}

// goDirectiveIn returns the `go` directive version declared in the same
// file as the given toolchain directive.
func goDirectiveIn(s *Surface, tc ToolchainDirective) (string, bool) {
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

		if parsed, err := parseMajorMinor(tc.Version); err == nil && parsed == toolFloor {
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
		parsed, err := parseMajorMinor(pin.Version)
		if err != nil {
			continue
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

	return fmt.Sprintf("%s pins Go %s but %s: %s", pin.Path, pin.Version, floorPhrase(floor, toolDriver), tail)
}

// ciPinIssues report workflow pins that trail the effective floor or pin
// one exact patch.
func ciPinIssues(s *Surface, floor majorMinor, toolDriver *ToolchainDirective) []Issue {
	var issues []Issue

	for _, pin := range s.CIPins {
		parsed, ok := parseCIPin(pin.Version)

		if !ok {
			continue
		}

		if parsed.lessThan(floor) {
			issues = append(issues, Issue{
				Rule:       RuleCIPinBelowFloor,
				Message:    ciPinMessage(pin, floor, toolDriver),
				File:       pin.Path,
				Line:       pin.Line,
				Suggestion: fmt.Sprintf("set go-version to %s to match the effective floor", floor.String()),
			})

			continue
		}

		if hasPatch(pin.Raw) {
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

	return fmt.Sprintf("%s pins go-version %s but %s: %s", pin.Path, pin.Version, floorPhrase(floor, toolDriver), tail)
}

// ciPatchPinMessage describes a CI pin that tracks one exact patch.
func ciPatchPinMessage(pin Pin, floor majorMinor) string {
	return fmt.Sprintf(
		"%s pins go-version %s with a patch component: "+
			"CI tracks one exact patch while the flakes move with nixpkgs %s, so they silently diverge",
		pin.Path, pin.Version, floor,
	)
}
