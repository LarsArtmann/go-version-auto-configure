# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- `--json` documents are now versioned: every document carries a top-level `"schema": 1` field, bumped only on breaking shape changes (additive fields keep the version).
- `who-forces` policy and matrix hardening: `--allow-partial` downgrades per-module `go list` failures from exit 2 to exit 1 (fail-closed remains the default); every dependency forcing a module's directive upward is now carried with its own floor (`poisonerFloors`, sorted highest floor first) instead of only the max-floor carriers; go.work rows appear as marked `kind: "go.work"` rows (no dependency graph) instead of being silently skipped.
- `check --quiet` flag: exit-code-only operation for scripting (`--json` output is still emitted when explicitly requested).
- `--version` / `-version` flag aliases alongside the `version` subcommand.
- `--parallel N` flag on `check`, `fix`, and `who-forces` to cap the concurrent repository pool (default: CPU count); parallel-sweep benchmark (`BenchmarkAnalyzeAll`).
- `toolchain-non-version` informational finding: a `toolchain` directive naming no explicit toolchain version (`default`, and `local` where a parser accepts it) pins nothing and was silently excluded from floor analysis.
- `go-work-unparseable` discovery finding: an unparseable go.work (e.g. a `toolchain local` line, which the go tool rejects) surfaced as an issue instead of the whole file vanishing from the surface; previously only go.mod parse failures were reported.
- `fix --json` now reports discovery issues (`discovery` array) and `fix` exits 1 when they are present, matching `check`'s contract.
- Fleet-sweep enablers: `check`/`fix`/`who-forces` accept multiple roots analyzed in parallel, and `fix` skips clean repos entirely (no go invocations) so sweeps scale with drifted repos, not repo count.
- `--json` machine-readable output for `check`, `fix`, and `who-forces` (`encoding/json/v2`, stable camelCase contract).
- `toolchain` directive coverage: `toolchain` lines in go.mod/go.work are discovered; a stale one below the same file's `go` directive surfaces as `toolchain-below-directive` (suggest-only), and a toolchain naming a newer minor raises the effective floor for Nix/CI pin alignment.
- `.editorconfig` and `.gitattributes`; dependabot config gained an explicit `open-pull-requests-limit` and an entry-level `gomod` update group.

### Changed

- Domain strings in `pkg/surface` and `pkg/fix` are now named types (`surface.Rule`, `surface.GoVersion`, `surface.ModulePath`, `fix.FailureCause`, `fix.ModuleVersion`), matching go-finding's branded-type convention; the JSON wire format is unchanged.
- `who-forces` output naming: the human report lists every forcing dependency as `module@version (floor go X)`; `poisoners` continues to name only the max-floor carriers for compatibility.
- Test coverage raised across the tool: pkg/fix 90.3%, cmd 87.9%, surface 87.9%, provider 83.8% (previously unmeasured for pkg/fix and cmd; several sub-80% functions closed).

### Fixed

- `go list -m` invocations run with `GOWORK=off` so a module's dependency graph cannot be resolved through an enclosing workspace.
- Cleared every warning-severity golangci-lint finding (159 → 0); tests run in parallel; full-mode BuildFlow (race + coverage) green.
- cqrs-lint A009/A018 false positives (repo imports no go-cqrs-lite; re-verified 2026-09-22) suppressed via `skip_steps` with rationale.
- erraudit `silent_swallow` findings dispositioned: the five skip-on-unparseable filters in `pkg/surface` are deliberate policy, each now carries a reasoned `//nolint:erraudit` (staleness-audited, 0 stale).
- BuildFlow findings gate passes at `--fail-on=error`: three critical branching-flow phantom-type findings on the new who-forces code were fixed by returning named types from the `go list` line parser.

## [0.1.0] - 2026-09-18

### Added

- Initial release candidate: Go toolchain version-surface detection and mechanical auto-fix.
  - `pkg/surface` mini SDK: discovery + policy analysis over go.mod/go.work directives, flake.nix pins, and CI go-version pins.
  - `pkg/fix`: `go mod edit` / `go work edit` appliers with re-parse verification and dry-run.
  - `pkg/provider`: BuildFlow provider self-registered through linter-autoconfigure-sdk's `ProviderFromSpec`.
  - CLI: `check`, `fix [--dry-run]`, `version`.
- Fleet audit findings that motivated the tool (2026-09-18): 284/383 modules with patch-form `go` directives; floor poisoning propagates through `go mod tidy` from published go-finding/go-atomic-write versions; go.work-below-workspace-floor breakage class found in go-finding.

### Fixed

- README install instructions: replaced the `go get` command (unrunnable while the module is untagged) with build-from-source steps; `go get` is documented as available once the first version is tagged.
