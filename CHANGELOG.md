# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.2.2] - 2026-09-25

### Fixed

- Lint gate: the v0.2.1 comment-stripping scanner tripped golangci-lint (gocognit, makezero, wsl_v5, mnd) — CI at the v0.2.1 tag is red. The scanner is refactored into a `nixCommentScanner` with per-token methods (behavior identical, all fixtures unchanged and green). No user-facing change; cut so the fleet-pinned tag carries a green CI run.

## [0.2.1] - 2026-09-25

### Fixed

- **False positive in `nix-pin-below-floor`: Nix comments were scanned as pins.** A flake.nix comment explaining why the pin IS `go_1_27` while mentioning `go_1_26` (go-health's actual flake, found 2026-09-25) was reported as a below-floor pin on a correctly configured repo. Pin scanning now strips comments first, string-aware: `#` line comments and `/* ... */` block comments are ignored outside strings, quoted text stays scannable, and line numbers remain faithful to the source file.

### Changed

- `check` no longer promises unconditional auto-fixability for go.mod form fixes. When a patch-form directive finding is present, the summary notes that `fix` still passes the dependency-floor gate — a dependency holding the floor reverts the rewrite (dep-forced) and the actual remediation is supply-side (re-tag the poisoner with a major.minor-only directive). JSON output is unchanged (schema 2 stable).
- Exit contract documented precisely and pinned by a regression test: `fix` exits 0 when every finding was dep-forced — the state is externally forced and the tool did its job; failed repairs still exit 1.

## [0.2.0] - 2026-09-22

### Changed

- **BREAKING: `--json` schema 2.** The versioned wire documents drop the
  `poisoners` string array: forcing dependencies are fully carried by
  `poisonerFloors` (module, version, own floor; sorted highest floor first)
  in `who-forces` rows and by the dep-forced report in `fix`. The old field
  named only the max-floor carriers and was redundant with it. Every
  document carries the top-level `"schema": 2` marker.
- `--help`/`-h` now render the fang-styled help and exit 0 (previously the hand-rolled skeleton printed plain usage and exited 2). `-h` per subcommand behaves the same way.
- Hard errors (unknown flags, unknown commands) print as plain `Error: …` lines instead of the fang-styled block — the custom error handler must stay silent for the exit-1 findings sentinel (the report is the output) and therefore replaces the styled default entirely.
- A `version` subcommand and `--version`/`-version` aliases keep printing the fleet-stamped version line; fang's `--version` shows its own styled output.
- `who-forces` output naming: the human report lists every forcing dependency as `module@version (floor go X)`.
- Domain strings in `pkg/surface` and `pkg/fix` are now named types (`surface.Rule`, `surface.GoVersion`, `surface.ModulePath`, `fix.FailureCause`, `fix.ModuleVersion`), matching go-finding's branded-type convention; the JSON wire format is unchanged.
- Test coverage raised across the tool: pkg/fix 90.3%, cmd 87.9%, surface 87.9%, provider 83.8% (previously unmeasured for pkg/fix and cmd; several sub-80% functions closed).

### Added

- `cmd/` migrated to the fleet CLI framework `github.com/larsartmann/cmdguard/v4`: struct-tag flag structs with construction-time validation, shared `--json`/`--parallel`/`--quiet` flags via embedded structs, fang-styled help, and the exit contract (0 clean / 1 findings / 2 errors) mapped through sentinel errors. The hand-rolled `flag`-package skeleton (`runFlags`/`newRunFlagSet`/`parseRoots`/`usage`) is deleted; guard tests ban the raw `flag`/cobra skeleton class from returning and lock the shared flag contract and the `--json` wire documents with golden tests.
- Generic `runSorted` worker pool: the triplicated bounded-pool-plus-sort plumbing in `analyzeAll`/`applyAll`/`analyzeFloorsAll` is now one helper.
- go.work workspace-floor handling (T0): the new `go-work-below-floor` rule detects a go.work `go` directive sitting below the workspace module floor at FULL patch granularity and restores it mechanically (the go-finding/BuildFlow gotcha-#169 outage class); the `go-work-patch-form` strip is now suppressed when a dep-forced module floor REQUIRES the patch form, so the fixer can no longer invalidate a workspace.
- `pkg/fix.CanonicalizeGoMod`: byte-preserving go.mod directive canonicalization (patch-form go line rewrite via text surgery, optional `toolchain` directive stripping) guarded by a non-mutating `go mod tidy -diff` dependency-floor gate with atomic revert, plus an installed-toolchain guard. This is the fleet-policy normalizer behind BuildFlow's `go-mod-normalize`.
- `pkg/fix.SyncGoWorkDirectives`: scoped entry point that applies only the go.work directive fixes (below-floor restorations and floor-safe patch strips) for workspace-arbiter consumers.
- `pkg/surface.FullModuleFloor`: the highest FULL module directive (patch included), complementing the major.minor-only `Floor`.
- `--expect-minor N` on `check` and `fix`: the fleet-policy expectation (e.g. `--expect-minor 1.27`). Any `go` directive, `toolchain` directive, flake.nix pin, or CI pin naming a HIGHER minor fires `minor-exceeds-expectation` (suggest-only); comparison is at minor granularity, so accepted patch-form floors (`x/text`'s `1.26.0`, json/v2's `1.27.1`) do not fire. Invalid values are a usage error (exit 2).
- `fix --quiet` and `who-forces --quiet`: exit-code-only operation, completing the `--quiet` family with `check --quiet` (`--json` output is still emitted when explicitly requested).
- `who-forces` policy and matrix hardening: `--allow-partial` downgrades per-module `go list` failures from exit 2 to exit 1 (fail-closed remains the default); every dependency forcing a module's directive upward is carried with its own floor; go.work rows appear as marked `kind: "go.work"` rows (no dependency graph) instead of being silently skipped.
- `--version` / `-version` flag aliases alongside the `version` subcommand.
- `--parallel N` flag on `check`, `fix`, and `who-forces` to cap the concurrent repository pool (default: CPU count); parallel-sweep benchmark (`BenchmarkAnalyzeAll`).
- `toolchain-non-version` informational finding: a `toolchain` directive naming no explicit toolchain version (`default`, and `local` where a parser accepts it) pins nothing and was silently excluded from floor analysis.
- `go-work-unparseable` discovery finding: an unparseable go.work (e.g. a `toolchain local` line, which the go tool rejects) surfaced as an issue instead of the whole file vanishing from the surface; previously only go.mod parse failures were reported.
- `fix --json` now reports discovery issues (`discovery` array) and `fix` exits 1 when they are present, matching `check`'s contract.
- Fleet-sweep enablers: `check`/`fix`/`who-forces` accept multiple roots analyzed in parallel, and `fix` skips clean repos entirely (no go invocations) so sweeps scale with drifted repos, not repo count.
- `--json` machine-readable output for `check`, `fix`, and `who-forces` (`encoding/json/v2`, stable camelCase contract).
- `toolchain` directive coverage: `toolchain` lines in go.mod/go.work are discovered; a stale one below the same file's `go` directive surfaces as `toolchain-below-directive` (suggest-only), and a toolchain naming a newer minor raises the effective floor for Nix/CI pin alignment.
- `flake.nix`: package (stamped binary via ldflags) and devShell (go_1_27, `GOEXPERIMENT=jsonv2`, `GOTOOLCHAIN=go1.27.1`, `GOWORK` cleared so `go work edit` works) — `nix build` and unprefixed commands inside `nix develop`.
- `docs/DEDUPLICATION.md`: the accepted-duplication baseline for future art-dupl sweeps.
- `.editorconfig` and `.gitattributes`; dependabot config gained an explicit `open-pull-requests-limit` and an entry-level `gomod` update group.

### Fixed

- Version ranking now matches the go tool exactly: `go 1.26` ranks strictly BELOW `go 1.26.0` (probed live — the go command rejects a go.work at `1.26` over a module floor of `1.26.0`). The new `CompareDirective` primitive (missing patch = −1) backs `GreaterVersion`, the go.work strip guard (a strip is offered only when the minor form covers the full module floor), and `go-work-below-floor`.
- `fix`'s dep-forced classification gained a `go list -e` fallback, so the poisoning dependency is named even on untidy trees where strict `go list -m` fails.
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
