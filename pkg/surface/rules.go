package surface

import (
	"fmt"
	"strings"
)

// Policy is the fleet's Go toolchain versioning policy, encoded as checks
// over a discovered Surface:
//
//  1. `go` directives (go.mod and go.work) are major.minor only — a patch
//     component raises the minimum toolchain to one exact patch and breaks
//     environments that trail the newest release (Nix, CI runners).
//  2. Nix and CI pins must not trail the repo's module floor; otherwise the
//     sandbox builds with (or downloads) a different toolchain than local.
//  3. CI patch pins are flagged as suggestions: they silently diverge from
//     the nixpkgs minor the flakes track.
//
// Form violations (rule 1) carry a mechanical Fix; alignment violations
// carry suggestions only, because which side moves (pin vs floor) is a
// maintainer decision and downgrades are not auto-applied.
func Analyze(s *Surface) []Issue {
	var issues []Issue

	workspaceFloor, hasFloor := s.Floor()

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
		rule := RuleGoDirectivePatchForm
		if m.Kind == KindGoWork {
			rule = RuleWorkDirectivePatchForm
		}
		issues = append(issues, Issue{
			Rule:    rule,
			Message: fmt.Sprintf("%s declares go %s: the go directive is a floor and must be major.minor only; a patch component pins the toolchain to one exact patch and breaks trailing environments", m.Path, m.Version),
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

	if hasFloor {
		for _, pin := range s.NixPins {
			parsed, err := parseMajorMinor(pin.Version)
			if err != nil {
				continue
			}
			if parsed.lessThan(workspaceFloor) {
				issues = append(issues, Issue{
					Rule:       RuleNixPinBelowFloor,
					Message:    fmt.Sprintf("%s pins Go %s but the module floor is %s: the Nix sandbox builds with (or downloads) a toolchain older than the modules require", pin.Path, pin.Version, workspaceFloor),
					File:       pin.Path,
					Line:       pin.Line,
					Suggestion: fmt.Sprintf("raise the flake's nixpkgs Go pin to go_%s (or newer), then run the Nix hash repair", strings.ReplaceAll(workspaceFloor.String(), ".", "_")),
				})
			}
		}

		for _, pin := range s.CIPins {
			parsed, ok := parseCIPin(pin.Version)
			if !ok {
				continue
			}
			if parsed.lessThan(workspaceFloor) {
				issues = append(issues, Issue{
					Rule:       RuleCIPinBelowFloor,
					Message:    fmt.Sprintf("%s pins go-version %s but the module floor is %s: CI builds with a toolchain older than the modules require", pin.Path, pin.Version, workspaceFloor),
					File:       pin.Path,
					Line:       pin.Line,
					Suggestion: fmt.Sprintf("set go-version to %s to match the module floor", workspaceFloor.String()),
				})
				continue
			}
			if hasPatch(pin.Raw) {
				issues = append(issues, Issue{
					Rule:       RuleCIPinPatchForm,
					Message:    fmt.Sprintf("%s pins go-version %s with a patch component: CI tracks one exact patch while the flakes move with nixpkgs %s, so they silently diverge", pin.Path, pin.Version, workspaceFloor),
					File:       pin.Path,
					Line:       pin.Line,
					Suggestion: fmt.Sprintf("set go-version to %s so CI tracks the same minor as the flake pin", workspaceFloor.String()),
				})
			}
		}
	}

	return issues
}
