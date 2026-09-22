// Package surface models the Go toolchain version surface of a repository:
// every location where a Go version is declared — go.mod `go` and `toolchain`
// directives across all modules, go.work, flake.nix nixpkgs Go pins, and CI
// `go-version:` pins — plus the policy rules that detect drift between them.
//
// This package is the reusable SDK half of go-version-auto-configure: it
// discovers and models the surface and reports issues, but never writes
// files. Repairs live in pkg/fix.
package surface

import "fmt"

// Rule identifies one policy violation kind reported by Analyze. Each maps
// 1:1 to a go-finding RuleName at the provider boundary.
type Rule string

// GoVersion is a Go version as written on the version surface: "1.26",
// "1.26.7", or the toolchain form "go1.26.7". A named type keeps
// version-shaped strings from flowing into arbitrary text and back.
type GoVersion string

// ModulePath is a Go module path, e.g. "github.com/larsartmann/go-finding".
type ModulePath string

// FilePath is a file path relative to the repository root, as carried by
// every surface location.
type FilePath string

// Policy rules reported by Analyze.
const (
	// RuleGoDirectivePatchForm fires when a go.mod `go` directive carries a
	// patch component (e.g. `go 1.26.7`). Fleet policy: major.minor only.
	RuleGoDirectivePatchForm Rule = "go-directive-patch-form"

	// RuleWorkDirectivePatchForm fires when a go.work `go` directive carries
	// a patch component.
	RuleWorkDirectivePatchForm Rule = "go-work-patch-form"

	// RuleNixPinBelowFloor fires when the flake's nixpkgs Go pin is older
	// than the newest module floor (e.g. flake pins go_1_26, a module
	// declares go 1.27). Not auto-fixed: raising the pin is a flake +
	// lockfile decision.
	RuleNixPinBelowFloor Rule = "nix-pin-below-floor"

	// RuleCIPinBelowFloor fires when a CI `go-version:` pin is older than
	// the newest module floor.
	RuleCIPinBelowFloor Rule = "ci-pin-below-floor"

	// RuleCIPinPatchForm fires when a CI `go-version:` pin hardcodes a patch
	// version (e.g. 1.26.7). CI should track the same major.minor the flake
	// and go.mod floors use, so patch pins silently diverge from local
	// toolchains as nixpkgs moves.
	RuleCIPinPatchForm Rule = "ci-pin-patch-form"

	// RuleGoModUnparseable fires when a discovered go.mod cannot be parsed.
	RuleGoModUnparseable Rule = "go-mod-unparseable"

	// RuleToolchainBelowDirective fires when a `toolchain` directive names a
	// toolchain older than the same file's `go` directive: the go command
	// ignores such a line, so it is dead weight (usually stale after the go
	// directive was raised).
	RuleToolchainBelowDirective Rule = "toolchain-below-directive"

	// RuleToolchainNonVersion fires when a `toolchain` directive names
	// something that is not an explicit toolchain version ("local",
	// "default"): the go command reads these as "never switch toolchains",
	// so the line pins nothing and is invisible to floor analysis.
	// Informational: legal, but it silently opts the module out of
	// toolchain resolution, which fleet policy wants surfaced.
	RuleToolchainNonVersion Rule = "toolchain-non-version"

	// RuleGoWorkUnparseable fires when a discovered go.work cannot be
	// parsed (for example a `toolchain local` line, which the go tool
	// rejects). Without this the file's `go` directive would vanish from
	// the surface silently.
	RuleGoWorkUnparseable Rule = "go-work-unparseable"

	// RuleGoWorkBelowFloor fires when a go.work `go` directive sits BELOW a
	// workspace module's `go` directive at FULL patch granularity. In that
	// state every `go` command in the workspace fails with "module X listed
	// in go.work file requires go >= N, but go.work lists go M". The fix is
	// mechanical and always safe: raise the go.work directive to the module
	// floor (raising a floor never invalidates anything). This is the
	// invalidation class a workspace-mode `go get` produces when it
	// rewrites the go.work directive at minor granularity while modules
	// are dep-forced at a patch.
	RuleGoWorkBelowFloor Rule = "go-work-below-floor"

	// RuleMinorExceedsExpectation fires when any version surface (a `go` or
	// `toolchain` directive, a flake pin, or a CI pin) declares a minor
	// NEWER than the fleet policy minor passed via WithExpectedMinor
	// (ADR-0001 sets the fleet minor to 1.27). Suggestion-only: moving a
	// surface down is a maintainer decision and is never auto-applied.
	RuleMinorExceedsExpectation Rule = "minor-exceeds-expectation"
)

// DirectiveKind distinguishes which file declares a Go version.
type DirectiveKind string

const (
	// KindGoMod is a go.mod `go` directive.
	KindGoMod DirectiveKind = "go.mod"
	// KindGoWork is a go.work `go` directive.
	KindGoWork DirectiveKind = "go.work"
)

// ModuleDirective is a parsed `go` directive from one module file.
type ModuleDirective struct {
	// Path is the file path relative to the repository root.
	Path string
	// Kind is KindGoMod or KindGoWork.
	Kind DirectiveKind
	// Module is the module path declared in the file; empty for go.work,
	// which declares no module.
	Module ModulePath
	// Version is the declared version string, e.g. "1.26.7".
	Version GoVersion
	// Line is the 1-based line of the directive in the file.
	Line int
}

// ToolchainDirective is a parsed `toolchain` directive from go.mod or
// go.work. Unlike the `go` directive it names an exact toolchain (patch
// component included, e.g. "go1.26.7") and is advisory: the go command
// honors it only when it is newer than the `go` floor.
type ToolchainDirective struct {
	// Path is the file path relative to the repository root.
	Path string
	// Kind is KindGoMod or KindGoWork.
	Kind DirectiveKind
	// Version is the toolchain as written, e.g. "go1.26.7".
	Version GoVersion
	// Line is the 1-based line of the directive in the file.
	Line int
}

// Pin is a Go version reference outside the Go module system: a flake.nix
// nixpkgs pin or a CI workflow pin.
type Pin struct {
	// Path is the file path relative to the repository root.
	Path string
	// Version is the parsed major.minor string, e.g. "1.26". Range or
	// expression pins (1.26.x, ${{ … }}, stable) are not recorded at all:
	// they carry no comparable floor.
	Version GoVersion
	// Raw is the pin text as written in the file, preserving patch
	// components and formatting for form rules. Equals Version when the
	// source only ever states major.minor.
	Raw GoVersion
	// Line is the 1-based line of the pin.
	Line int
	// Source records where the pin came from, for messages.
	Source PinSource
}

// PinSource distinguishes pin origins.
type PinSource string

const (
	// PinNixFlake is a flake.nix nixpkgs Go pin (go_1_26, buildGo126Module).
	PinNixFlake PinSource = "nix-flake"
	// PinCI is a CI workflow go-version pin.
	PinCI PinSource = "ci"
)

// Surface is the discovered Go version surface of one repository.
type Surface struct {
	// Root is the absolute repository root that was scanned.
	Root string
	// Modules holds every parsed `go` directive, one per module file.
	Modules []ModuleDirective
	// Toolchains holds every parsed `toolchain` directive, one per file
	// that declares one.
	Toolchains []ToolchainDirective
	// NixPins holds flake.nix Go pins, newest first per file.
	NixPins []Pin
	// CIPins holds CI workflow go-version pins.
	CIPins []Pin
}

// Floor returns the highest declared major.minor across module directives,
// which is the minimum toolchain the whole repo needs. The bool is false
// when no module directive declares a parseable version.
func (s *Surface) Floor() (majorMinor, bool) {
	versions := make([]GoVersion, 0, len(s.Modules))

	for _, m := range s.Modules {
		versions = append(versions, m.Version)
	}

	return highestMajorMinor(versions)
}

// FullModuleFloor returns the highest FULL directive version (patch
// component included) across the go.mod module directives, e.g. "1.27.1".
// Unlike Floor, which is major.minor only, the patch component matters
// here: the go tool lifts a directive to a dependency's exact patch floor,
// and a go.work directive must cover every module at that granularity or
// the workspace cannot resolve its own modules. The bool is false when no
// go.mod module directive parses.
func (s *Surface) FullModuleFloor() (GoVersion, bool) {
	best, found := "", false

	for _, m := range s.Modules {
		if m.Kind != KindGoMod {
			continue
		}

		if _, err := parseMajorMinor(string(m.Version)); err != nil {
			continue
		}

		if !found || GreaterVersion(string(m.Version), best) {
			best, found = string(m.Version), true
		}
	}

	return GoVersion(best), found
}

// toolchainFloor returns the highest major.minor pinned by any `toolchain`
// directive. The bool is false when none parses.
func (s *Surface) toolchainFloor() (majorMinor, bool) {
	versions := make([]GoVersion, 0, len(s.Toolchains))

	for _, tc := range s.Toolchains {
		versions = append(versions, tc.Version)
	}

	return highestMajorMinor(versions)
}

// highestMajorMinor returns the highest parseable major.minor among the
// given versions; unparseable entries are skipped. The bool is false when
// no version parses.
func highestMajorMinor(versions []GoVersion) (majorMinor, bool) {
	var best majorMinor

	found := false

	for _, v := range versions {
		parsed, err := parseMajorMinor(string(v))
		if err != nil {
			continue //nolint:erraudit // deliberate filter: unparseable entries carry no floor, documented on highestMajorMinor
		}

		if !found || parsed.greaterThan(best) {
			best = parsed
			found = true
		}
	}

	if !found {
		return majorMinor{}, false
	}

	return best, true
}

// Issue is one policy violation on the version surface.
type Issue struct {
	// Rule is one of the Rule* constants.
	Rule Rule
	// Message describes the violation for the user.
	Message string
	// File is the path relative to the repository root.
	File string
	// Line is the 1-based line (0 when unknown).
	Line int
	// Suggestion is the recommended fix text for non-mechanical issues.
	Suggestion string
	// Fix is non-nil only for mechanically safe repairs: form violations
	// where the exact replacement is unambiguous.
	Fix *Fix
}

// Fix describes a mechanical, unambiguous replacement of one directive.
type Fix struct {
	// File is the path relative to the repository root.
	File string
	// Kind is the file kind the fix applies to.
	Kind DirectiveKind
	// From is the current directive version, e.g. "1.26.7".
	From GoVersion
	// To is the replacement version, e.g. "1.26".
	To GoVersion
	// Line is the 1-based line of the directive.
	Line int
}

// Describe renders a human-readable summary of the fix.
func (f Fix) Describe() string {
	return fmt.Sprintf("rewrite %s directive in %s: go %s → go %s", f.Kind, f.File, f.From, f.To)
}
