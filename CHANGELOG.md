# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.2.8] - 2026-11-28

### Changed

- **Upstream bridge landed — the provider now speaks the full finding vocabulary (sdk v0.8.1 → v0.9.0 adoption, plan M7/M7b):** linter-autoconfigure-sdk v0.9.0 shipped the fleet's upstream asks (`ConfigIssue.Metadata`/`Category`/`Tags`, `ProviderSpec.RepairStructured`, `ProviderSpec.Coverage`), and this repo adopts all of it. Dep-forced findings now carry machine-readable provenance (`dep-forced.floor`/`dep-forced.carriers`/`dep-forced.remediation` metadata keys alongside the message), every finding is classified through a tested rule→Category/Tag mapping (`pkg/provider/vocabulary.go`: form→style, alignment→configuration, drift→best-practice, discovery→correctness, each with its taxonomy-class tag), the repair contract migrated to `RepairStructured` (ModifiedFiles names the rewritten files; Facts stamp applied/dep-forced/held-back/failed/dry-run counts), and `spec.Coverage` feeds DetectV2 with the same filesScanned/skippedModules evidence the CLI `--json` coverage object exposes. The schema-2 CLI wire contract is unchanged (goldens green); the severity policy is unchanged (advisories stay warning).
- **The 2026-11-24 cluster wave killed every live third-party poisoner but go-daemon** (T14 + plan M4/M5/M10): a five-repo pin-in-tag spiral (go-codec v0.3.3 → go-cqrs-lite 23 regressed trains → cqrs-htmx 11 trains → go-taskqueue; go-appkit v0.13.0 forcing setup independently) re-tagged 61 minor-form tags +0.1 patch across the cluster with sibling pins flipped to the wave versions; the 13-module `go 1.27.2` tidy re-lift inside go-taskqueue is gone (all 26 modules tidy-stable at `go 1.27`, cmd/tq post-push). Proxy-verified consumer-visible `.mod` for all five repos (all `go 1.27`). go-taskqueue/cqrs-htmx/go-cqrs-lite + go-codec/go-appkit rows moved to Resolved in [docs/POISONERS.md](docs/POISONERS.md); go-daemon is the last Active poisoner; the Active table also records the wave's std-floor live example (cqrs-htmx setup's master-side `go 1.27.4`). ADR-0001 appendix carries the wave record and four lessons (tag-layout divergence, gate-deadlock tag-first push pattern, master-vs-tag floor honesty, the CI-lockstep floor-raise collision).

### Added

- **`check --sarif` exports SARIF 2.1.0** (plan M24): the go-finding report export, reached through the same bridge the BuildFlow provider uses (`provider.FindingsFromIssues`, now the single CLI→finding conversion point). One run per invocation with absolute finding locations (multi-root stays unambiguous), and a run propertyBag stamping the scanned roots, the schema-2 marker tying the export to the locked `--json` contract, the tool version, and any roots whose analysis failed (named, never dropped). Exit codes are unchanged (SARIF is an output format, not a policy), locked by a wire golden, a reader round-trip test, and the clean/drifted exit-code table.
- **Coverage evidence in the check report** (plan M25): `Discover` now measures what it actually examined — `check --json` carries an additive `coverage` object (`filesScanned` counting the six recognized surface classes read; `skippedModules` naming the vendored trees the walk deliberately did not descend into), so a clean answer is provable against what was scanned and a real zero (empty tree) is distinguishable from a missing measurement. Schema stays 2 (additive, `omitempty`). The provider-side `DetectorV2` passthrough of the same evidence needs the sdk bridge's DetectV2 support (plan M7) and is tracked with it.
- **The provider boundary now raises go-finding's typed `FindingError` family** (plan M25) instead of bare `fmt.Errorf` wraps: discovery/repair IO failures are `NewIOError`, option validation is `NewValidationError`, and repair-apply failures are `NewConflictError` — hosts can classify provider errors programmatically, and `Unwrap` keeps every `errors.Is` chain intact. `fix.SelfCheck` deliberately keeps its own PATH lookup rather than importing `finding.CheckBinary`: pkg/fix is finding-free by design (pkg/provider is the sole import site), and a PATH check does not justify coupling the fix module to the findings model.
- **Release-authority second pass: `release-tag-drift`** (plan M12, T6 closed) — `check` now compares the repository's newest `v*` git tag against the pure-file sources and fires when BOTH files agree AND the tag disagrees: the corroborating-files state the first pass (`release-authority-drift`) cannot see (project-dependency-graph's VERSION=0.7.0 = CHANGELOG 0.7.0 against tags topping at v0.2.0, the exact planned live case, verified the day it shipped). pkg/fix shells `git for-each-ref refs/tags/v* --sort=-version:refname` (`GitTagRunner`, parseable-tag selection through the shared `ParseReleaseVersion` entry point) and degrades silently when git is unavailable; the pure comparison lives in `surface.ReleaseTagIssues` with `x/mod/semver` (not stdlib `go/version`, which wants `go1.21.0`-form strings). Suggest-only, wired into the CLI `check` path; provider-side wiring follows with the M7b adoption pass.
- **The BuildFlow provider now declares its true read/write surface** (the library-utilization run's top finding): `ProviderSpec.ConfigFiles` covers every file class `Discover` dispatches on — go.mod, go.work, flake.nix, `.github/workflows/*.yml`+`*.yaml`, VERSION, CHANGELOG.md — so the bridge-derived `Spec.Inputs` gates BuildFlow re-runs and data-flow edges on flake.nix and CI-pin drift (previously only `go.mod`, leaving the majority of the tool's core scenarios under-triggered), and `Spec.Outputs` declares the repair write contract (go.mod, go.work, `.github/workflows/*`; flake.nix/VERSION/CHANGELOG.md are read-only surfaces the tool never writes). Pinned by a surface-declaration test and a flake-only-drift detect test.
- **The `respect_patch_floor` option now fails loudly on wrong-typed values** (library-utilization run, sdk report gap 3): the raw `map[string]any` bool assertion read a config typo (e.g. the string `"true"`) as unset and silently ran the default policy; the swap to `toolsdk.BoolFromContext` surfaces `ErrOptionKindMismatch` naming the option instead. Also adopts the SDK's `WorkingDir` helper in place of the hand-rolled identical fallback (sdk report row 5).
- **go-finding bumped v1.15.1 → v1.16.0** (plan M3): the delta is SARIF-layer only (run-level propertyBag, the future M24 export path); the CLI and the schema-2 wire contract are unchanged — verified by the full suite including the wire goldens plus a live fixture re-run against the schema-2 document shape.
- **Dep-forced findings now explain themselves in the BuildFlow provider view** (the "N finding(s) remain (could not auto-fix)" case): a `go-directive-patch-form` finding whose mechanical strip `go mod tidy` would revert now carries the why and the what-next in the finding itself. The message names the dependencies holding the floor (`not auto-fixed: go mod tidy reverts the strip, go 1.27.1 is forced by go-daemon@v0.4.0, go-sse/sseparse@v0.2.0`), and the suggestion swaps the withheld rewrite for the supply-side remediation (re-tag the carriers with a major.minor-only directive, then bump the repo; who-forces reports the full matrix); a poisoned row (floor above the directive) suggests raising the directive instead. Resolved read-only from the who-forces matrix via `fix.ExplainHeldStrips` (pkg/fix/held.go): the module graph is listed only when a patch-form finding exists (clean repos pay nothing), and a listing failure degrades to the plain finding rather than red detection. The std-floor/replace class (`depForced.cause` in Apply) stays unannotated at detect time by design: `go list -m` cannot see those floors, only the tidy-diff gate can. Found via nextd (2026-11-17), whose bare finding named neither the poisoners (go-daemon v0.4.0, newly registered in POISONERS.md) nor the remediation. CLI output and the `--json` wire contract are unchanged (schema 2).

### Fixed

- **A gated go.mod rewrite no longer touches the live file before the verdict (BuildFlow#124).** `CanonicalizeGoMod` (BuildFlow's `go-mod-normalize`) and `Apply`'s patch-form strip used to write the rewrite to the live `go.mod`, run `go mod tidy -diff`, and revert on rejection. Inside that window the module was broken for everything else reading it: BuildFlow's concurrent `go generate` and `govalid` steps failed with `updates to go.mod needed` (three runs out of three on a clean go-service-template), and the auto-commit daemon could sweep the candidate. Both paths now stage the candidate `go.mod` and `go.sum` in a private temp directory and let the go command read them through `-modfile` (`pkg/fix/stage.go`), so the live `go.mod` is written exactly once and only after every gate approves; relative `replace` directives still resolve against the module directory, and the staged copy is removed on every path. The commit is verified against a concurrent writer (`atomicwrite.WriteVerified`): a `go.mod` rewritten while the gates ran is left to its other writer (`CanonicalizeResult.Skipped`, an `Apply` failure naming the concurrent modification) instead of being clobbered or reverted over. `GateDetail` for a dependency-floor rejection now carries the rejecting gate's own diff (the floor tidy would re-raise to) instead of a re-run against the restored file, which printed nothing. New API: `CanonicalizeOptions.ExtraGate func(ctx, dir, modFile string) error`, a caller gate that judges the same staged copy before the commit (BuildFlow's `go vet` gate moves inside the probe) and reports a rejection as `CanonicalizeResult.HeldBackByExtraGate`; the dependency-floor gate runs first and its rejection skips the caller gate. Dry runs stage nothing and run no gate, unchanged. Pinned by gate-observation tests (the live file must still hold the original directive when a gate runs), real-toolchain e2e tests for both verdicts, and mutation verification.
- **Lint baseline cleared and CI reports again (16 golangci-lint findings, all in files outside the staged-gate change; CI red since at least 2026-11-17).** contextcheck (the caller's context now reaches `fix.ReleaseAuthorityTagIssues`), varnamelen, testifylint, err113, unconvert, nestif (`handleDirEntry` in `pkg/surface/discover.go`), and wrapcheck (go-finding's typed errors join the sanctioned error-family ignore list, matching the provider-boundary design). The CI workflow runs the error-handling audit as its own job: erraudit lives in a private repository, so its `go install` needs `secrets.PRIVATE_REPO_TOKEN`, and the single job had hidden build, test, lint, and dogfood behind that failed step. The lint action is pinned to the devShell's golangci-lint 2.15.0 (it was 2.14.0).
- **`provider.FindingsFromIssues` returns an error** (erraudit `silent_swallow`, found by the local audit the same day): an issue the go-finding bridge refuses used to vanish from the `check --sarif` export without a trace. The conversion now fails and names the issue (`convert <file>:<line> (<rule>): ...`), and the SARIF export reports it on stderr with exit 1, the same way a failed root is named, never dropped.

## [0.2.7] - 2026-11-01

### Added

- **`ci-pin-below-floor` graduated from suggestion to mechanical auto-fix** — the tool's own class of drift (fleet repos whose workflows pin `go-version` below the module floor, e.g. desire-secrets at five `1.22` pins against a `go 1.27` floor) is now repaired by `fix` and by the BuildFlow provider's Repair phase. The rule only fires below the floor, so every rewrite is a raise, never a downgrade (F16-safe), and a below-floor pin already lies: under `GOTOOLCHAIN=auto` (the default) the go command switches above the pin anyway. Rewrites are byte-preserving single-line surgery with the original quoting style kept (double/single/backtick, bare, YAML flow-sequence brackets), guarded by an exact-text drift check and verified by re-parsing the rewritten content with `ParseCIPins` — the same parser discovery used to detect the pin. `ParseCIPins` (pkg/surface) is now the single CI-pin parsing entry point shared by discovery AND fix verification, mirroring `ParseDirective`'s contract. Flake pin moves and CI patch-form pins stay suggest-only (maintainer decisions). Wire shape unchanged (schema 2; additive `fix` objects on `ci-pin-below-floor` findings, new fix `kind: "ci"`), locked by a new wire golden and a detect→fix→verify e2e suite.
- **`depForced.cause` is now golden-locked** (`fix --json`): the cause-PRESENT shape (a replace target, a vendored module, or the standard library's floor) is frozen by a golden test beside the existing cause-absent real-gate golden, so the additive field cannot drift under schema 2 unnoticed.
- **Test-depth batch** (cmd/): the full `exitFromOutcomes` outcome-combination table, `runSorted` zero-items + pool-width equivalence (`--parallel 1` == `--parallel 100`), a byte-exact `version` output golden for the subcommand and both aliases, a flag-help property test (every declared flag carries help text, no duplicate names), and `BenchmarkApplyAll_CleanFastPath` for the fleet skip-clean path.
- **`BenchmarkAnalyzeFloors`** (pkg/fix): a hermetic benchmark for the who-forces per-module hot path (`accumulateFloor`/`finalizeFloors`), which the v0.2.3 reshape had left unmeasured. Baselines (benchmarks + per-package coverage) recorded in AGENTS.md.
- **`checks.go-fod-floor` flake guard**: an eval-time assertion that the go-modules FOD's Go package (`goPkgAttr`) covers go.mod's floor. The FOD runs `GOTOOLCHAIN=local`, so a stale `goPkgAttr` used to die inside the sandbox with a raw toolchain error; the guard turns it into an actionable eval-time message.

### Changed

- **Docs refresh (2026-10-17)**: TODO_LIST rebuilt against the code (the v0.2.4-v0.2.6 releases had already shipped most of the former T15/T16/T17 rows). POISONERS.md records a NEW Active poisoner (`go-taskqueue`, all modules patch-form at source and in published tags) and the cqrs-htmx regression (minor-form at v4.13.3, back to `go 1.27.2` at v4.14.0/v4.14.1); ADR-0001 appendix carries the 2026-10-17 fleet re-measure; AGENTS.md version/dependency lines and baselines refreshed; docs/DEDUPLICATION.md baseline re-scanned (228 groups, 7 above the noise floor, all intentional).

## [0.2.6] - 2026-10-14

### Added

- **`Discover` never walks `testdata` trees (BF1)** — testdata is the go toolchain's own fixture convention (the go command never builds or tidies inside it), so fixture go.mod files are not part of a repository's version floor. Walking them made gvac tidy go.mod-only fixture modules, delete every require line, and vacuously pass fixture guard tests (cqrs-htmx 27057e5c). Red-first test pins root and nested testdata exclusion.

## [0.2.5] - 2026-10-07

### Added

- **Patch-floor policy: `--respect-patch-floor` (check/fix) and the `respect_patch_floor` BuildFlow tool option** — fleet hosts that pin an exact Go patch (Nix profiles, locked runners) now have a first-class way to say so: a patch-form `go` directive the INSTALLED toolchain already satisfies stops being drift. Detection (`surface.WithRespectPatchFloor(installed)`) leaves form findings silent for pins at or below the installed version under the go tool's full-granularity directive ranking (`go 1.26 < go 1.26.0 < go 1.26.7`); the canonicalizer (`CanonicalizeOptions.RespectPatchFloor`) skips the go-line rewrite for the same class with skip reason "patch floor respected", independently of the toolchain-strip path; pins ABOVE the installed toolchain stay flagged and rewritten — they break builds on exactly that host. The CLI resolves the installed toolchain once per run (`go env GOVERSION`) and rides it on the option; the BuildFlow provider declares `respect_patch_floor` (bool) via toolsdk Options, set per repo through BuildFlow's `tool_options` config key, honored by BOTH detect and repair. This is the knob that retires the 14-repo skip-step fleet (wise-go class deliberate pins) instead of masking them.

- **New rule: `release-authority-drift` (suggest-only)** — `Discover` now reads the root release-authority files: the VERSION stamp and the top released CHANGELOG.md section (first versioned `## [...]` heading, `[Unreleased]` skipped). When both carry a version and they disagree, `check` reports the drift — which source is authoritative is per-repo policy, so neither side is a mechanical rewrite. Release stamps get their own named type (`ReleaseVersion`) and never enter Go-version floor arithmetic. Validated live the day it shipped: project-discovery-sdk fires (VERSION 0.14.0 vs CHANGELOG 0.22.0); project-dependency-graph stays silent (its two file sources agree — its drift is against git tags, the designed second pass). The git-tag comparison stays out of this release: it needs git access, which Discover's pure file walk deliberately lacks (TODO_LIST T6 records the design).
- **E2E test truth: `pkg/fix` now runs the REAL `go mod tidy -diff` gate** (seeded fixture, `GOTOOLCHAIN=auto` where needed) instead of faking the tidy outcome — the dep-forced classification, carrier naming, and revert-on-rejected-downgrade paths are exercised against the actual go command this tool gates on (WP-10). The deliberate `buildflow update` re-run (2026-10-03) proved the sweep now SETTLES at `go 1.27`: full pipeline green, no dependency lifts, `tidy -diff` clean, directive untouched at minor form — the re-poisoning cycle this repo lived in before the supply-side campaign is gone.
- **`ParseDirective`-shared-verification invariants extended for the post-v0.38.3 state** (`pkg/surface/parse_test.go`): discovery and fix-verification still share the single parsing entry point, with new invariant cases pinning the minor-form floor shapes the 2026-10-02/03 campaigns established (WP-10).

### Changed

- **Internal: dependency-floor resolution unified** — `pkg/fix`'s split floor parsers are one model now: the vendor-annotation parser (`vendorModuleFloors`) and the `go list -m` path both produce the shared `DependencyFloor` type (with `FloorSource` list/vendor provenance) through `resolveDependencyFloors`, consumed identically by the fix gate (`floor.go`) and `who-forces` (`who.go`). Behavior-identical; the wire-trust goldens and the full suite prove it (WP-12, kills the last floor-parser split brain).
- BuildFlow repinned to gvac **v0.2.4** (flake input rev `57f9cfe`; all three workspace members; single vendor stanza; profile binary upgraded to `eb35fce`) — the v0.2.4 gate, go-work-aware fixer, at-parity `who-forces`, and vendor fallback are live in the binary that runs fleet-wide. S87 disposition: `GoWorkFloorFinding`/`RestoreGoWorkFloor` stay retired in BuildFlow (the floor-safe gvac fixer is the defense); go-work-sync `DependsOn` KEPT as belt-and-suspenders. Closes TODO T14-② (recorded in BuildFlow AGENTS.md gotcha #187).

## [0.2.4] - 2026-10-03

### Added

- **`who-forces` answers "who holds me at parity"** (the 2026-10-02 go-output incident's core unanswered question): when a directive equals the max dependency floor, the carriers are named — human output lines `at parity: module@version holds the directive at go X`, JSON gains an additive `parityFloors` array (schema stays 2). `poisoned`, the exit contract, and below-parity rows are unchanged.
- **`who-forces` floor provenance**: an additive `source` field (`"list"` | `"vendor"`) marks whether a row's floors resolved from `go list -m` or from vendor/modules.txt annotations; the human directive line appends `(floors via vendor/modules.txt)` on vendor rows.
- Wire-trust goldens in cmd/: the `fix --json` contract is now golden-locked end-to-end against the REAL `go mod tidy -diff` gate via an offline-safe dep-forced fixture (replace-to-local poisoner), including the dep-forced shape (floor present, `cause` absent while a listed dependency carries it), clean-state snapshots for `check` and `fix`, and an incident-shaped consumer pinning a fake `github.com/larsartmann/go-output` at floor 1.27.1 asserting dep-forced classification, carrier naming, exit 0, and untouched files.
- Offline-CI errors are actionable: a failed toolchain download (GOTOOLCHAIN=auto fetching golang.org/toolchain without network) gains a hint naming the two fixes (pre-install the toolchain or pin GOTOOLCHAIN); other failures pass through with `errors.Is` identity intact.
- `who-forces --help` documents the vendor-annotation limitation: modules.txt annotates only explicit requirements, so indirect-only carriers in a skewed vendor tree stay unnamed.

### Changed

- Dependencies: go-output family bumped to v0.38.3 (2026-10-02). The 2026-09-30 dependency sweep had pulled in v0.38.2, whose published go.mod files are patch-form (`go 1.27.1`) — re-poisoning this repo's `go` directive via `go mod tidy`. v0.38.3 ships minor-form floors; the directive is back at `go 1.27`, tidy stable, `check` clean. Incident record: [docs/POISONERS.md](docs/POISONERS.md).
- Internal: the who-forces row type `PoisonerFloor` is renamed `DependencyFloor` (it now serves both the poisoner and parity lists); JSON wire names are unchanged.

## [0.2.3] - 2026-09-27

### Fixed

- **Dep-forced classification no longer fails when `go list -m` cannot name the forcer** (the two T14 bugs from the 2026-09-25 go-health sweep, both reproduced live and re-verified after the fix):
  - When `go mod tidy -diff` re-raises the directive but no listed module carries the floor, the fix is classified dep-forced instead of FAILED. The tidy diff names the enforced floor; the go tool's diagnostic ("X requires go >= Y") names the forcer when it can (a replaced local module, a vendored dependency), and the standard library (e.g. encoding/json/v2 requiring the toolchain patch) is the documented residual explanation. Fixed case: projects-management-automation/pkg/domain was FAILED with "dependency floor 1.27 does not exceed the target"; now dep-forced at floor 1.27.1 naming the `../../../go-output` replace target, exit 0.
  - Vendor-mode repos: when a vendor/modules.txt skewed against go.mod refuses BOTH `go list -m` and `go list -m -e`, the `## explicit; go X` annotations resolve the floor and its carriers (named as path@version). Fixed case: dnsblockd was FAILED ("dependency graph could not be listed"); now dep-forced at floor 1.27.1 naming the seven vendored carriers (cqrs-htmx/oauth2, go-cqrs-lite claiming/dedup/metaengine/sqliteengine/metaengine/record, go-sse), exit 0.
  - `who-forces` gains the same vendor fallback: a module whose listing fails on inconsistent vendoring reports `maxDepFloor`/`poisonerFloors` from the annotations instead of a per-module error (dnsblockd: error row with fail-closed exit 2 became a resolved row, exit 0).
- `fix --json` dep-forced entries carry an optional additive `cause` field (schema stays 2 per the additive-fields policy) explaining floors no listed dependency carries; the human report prints it in place of the generic "poisoner resolution unavailable" line.
- Warning-severity lint findings cleared from `pkg/fix` while touching it (`envWithout` via `slices.Contains`, wsl whitespace).
- **`nix build` failed: the go-modules FOD ran go 1.26.7 against the `go 1.27` floor.** The pinned go-nix-helpers (c42fd77) defaults `goPkgAttr` to `"go_1_26"` (the null auto-newest default landed after the pin), and `GOTOOLCHAIN=local` forbids the toolchain switch in the sandbox — "go.mod requires go >= 1.27 (running go 1.26.7)". `flake.nix` now pins `goPkgAttr = "go_1_27"` (locked nixpkgs ships 1.27.1); `nix build .#go-version-auto-configure` green with the stamped version line. This was the chronic nix-hash-fix/nix-build red in BuildFlow history (12/12 consecutive failures).

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
