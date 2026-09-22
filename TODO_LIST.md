# TODO List

Short- and mid-term actionable work. Ordered by impact (Pareto). Harvested from `docs/status/archived/2026-09-18_15-13_fleet-versioning-unification.md` (section f) on 2026-09-18; swept 2026-09-18 (T7 fleet-sweep enablers and T10 repo hygiene shipped — see CHANGELOG Unreleased); re-harvested 2026-09-19 from `docs/status/archived/2026-09-19_06-39_t7-t9-t10-shipped-lint-zero-full-gate-green.md` (section f → T11/T12, T3 additions); T11 fully shipped and T12 mostly dispositioned 2026-09-22 (see CHANGELOG Unreleased).

## T0 — go.work-aware fixer: never strip the go.work patch floor below a module's requirement — DONE (2026-09-22, source-side fix shipped)

- [x] **SHIPPED:** the fixer no longer normalizes a go.work `go` directive below any module's requirement. Under `go.work`, the go line must cover every module's directive at FULL patch granularity (`go work` errors with "module X listed in go.work requires go >= 1.27.1, but go.work lists go 1.27"); verified live on the BuildFlow workspace 2026-09-20 (BuildFlow gotcha #169). Implementation: `surface.Analyze` offers the patch-form strip only when the stripped minor form still covers `Surface.FullModuleFloor()` (max module go directive, patch included); when a module floor requires the patch, no strip is offered (that state is correct, not a violation). A go.work directive already BELOW the floor is the new `go-work-below-floor` rule with the always-safe mechanical fix (raise to the full floor); `fix.SyncGoWorkDirectives` is the scoped entry BuildFlow's go-work-sync arbiter calls. Regression tests: `TestAnalyze_GoWorkBelowFloorRestoresFullFloor` (S82 shape), `TestAnalyze_GoWorkPatchFormRequiredByFloorIsSilent`, `TestSyncGoWorkDirectives_*`. BuildFlow's downstream arbiter (`RestoreGoWorkFloor`) remains as defense-in-depth; its `DependsOn` ordering is now a safety net rather than a required repair.

## T1 — Supply-side re-tag campaign (BLOCKING for fleet convergence) — DONE 2026-09-22

Published library versions carried patch-form `go` floors that re-poisoned every consumer on `go mod tidy`. The campaign shipped 2026-09-22:

- [x] `go-finding` **v1.13.0** (root floor `go 1.27`; all 4 modules at minor form on head)
- [x] `go-atomic-write` **v0.6.0** (floor raise to `go 1.27` shipped as a 0.x minor, not v0.5.3 — the directive had already moved past v0.5.2's `go 1.26.7`)
- [x] `go-error-family` **7-tag coordinated release** (root v0.10.2, agent/bridge/diagnose/git/postgres/examples at true floors: minor form except x/text-forced `1.26.0`)
- [x] `go-output` **v0.38.2** (17 modules; supersedes broken v0.38.1 — retraction question open with the owner)
- [x] `go-branded-id` **v0.6.0**, `linter-autoconfigure-sdk` **v0.3.0**
- [x] Sibling autoconfigurers re-tagged: v0.8.2 / v0.6.4 / v0.2.1 (go-finding v1.13.0, directive `go 1.27`)
- [x] Consumer bumps for this repo's graph: `fix` applied 0 / dep-forced 0; `tidy` stable
- [x] Re-tag hygiene: no `replace` directives leaked (go-release Phase 3 checks)
- [x] Post-release `go get @vX.Y.Z` proxy verification per tag
- [ ] Remaining: consumer repos still requiring the old tags keep re-poisoning until they bump — ADR appendix holds the 2026-09-22 fleet baseline (36/48 repos with drift); sweep them with `check`/`fix` fleet-wide

## T4 — Decide the fleet minor: 1.26 vs 1.27 — DONE 2026-09-22 (Option B: adopt 1.27)

- [x] Decision recorded as [ADR-0001](docs/adr/0001-fleet-go-minor.md) (Option B, owner-confirmed; evidence appendix with fleet counts and the post-campaign consumer baseline)
- [x] Encoded in the tool: `--expect-minor N` on `check`/`fix` (dogfooded: `check --expect-minor 1.27 .` exit 0)

## T2 — BuildFlow blank-import wiring — PLANNED (BuildFlow dev task)

- [ ] Add `_ "github.com/larsartmann/go-version-auto-configure/pkg/provider"` to BuildFlow's SDK import set (mirrors sdk_imports_test.go for oxlint)
- [ ] Add to BuildFlow docs/provider catalog; run `buildflow --dry-run` to confirm discovery
- [ ] Decide DAG position: after `go-mod-update`, before `nix-checker` (alignment suggestions inform hash repairs)

## T3 — Publish this tool — PARTIALLY DONE (v0.1.0 tagged 2026-09-18)

- [ ] GitHub Actions CI: lint + test matrix + dogfood `go-version-auto-configure check .` as a gate (remember `GOEXPERIMENT=jsonv2` in the workflow env)
- [ ] GoReleaser config with ldflags version stamping for `version`
- [ ] pkg.go.dev verification after the first tag
- [ ] Website launch (sibling-project pattern) if it earns one
- [ ] Branch protection decision for `master`: required status checks would break the auto-commit daemon's direct pushes (owner call)
- [ ] Repo topics: `go`, `buildflow`, `golangci`, `auto-configure`, `version-surface`
- [ ] CI badge in README once the workflow lands
- [x] Module-path casing check (`github.com/larsartmann/…` vs `LarsArtmann`) with a real `go get` — verified 2026-09-22: lowercase path resolves from the proxy (`go get github.com/larsartmann/go-version-auto-configure@v0.1.0` + the three sibling apps at their new tags)
- [ ] Verify the README build-from-source steps in a clean environment (container/nix shell)

## T9 — Parser coverage — PARTIALLY DONE (toolchain directives shipped; flake.lock blocked by design)

- [ ] flake.lock effective Go revision parsing — **blocked by design**: the lock records only a nixpkgs rev, not the Go version it packages; resolving it requires an impure `nix eval`, but `Discover` must stay pure (reads files, writes nothing). A separate opt-in command (or BuildFlow step) would be the right home — design sketch below
  - Design sketch (2026-09-22): new command `gvac nix-pin [root ...]` (NOT part of `Discover`): for each flake.lock, `nix eval <locked nixpkgs rev>.go.version` (impure, cached by rev), compare against the flake's `goPkgAttr`/module floor, and report the same alignment findings as the pure path. BuildFlow home: a `nix-go-pin-check` step that can afford impurity and network. Exit contract identical to `check`; `--json` reuses the schema envelope with `source: "nix-pin"`

## T11 — Tool hardening (from the 2026-09-19 full-gate session) — DONE 2026-09-22

All twelve items shipped (see CHANGELOG Unreleased for the full list):

- [x] Dogfood `who-forces` on real foreign repos (go-finding, go-atomic-write, go-error-family, go-output, linter-autoconfigure-sdk): real dependency graphs, network resolution, and the 1.27-floor-under-`GOTOOLCHAIN=local` failure mode verified (per-module errors recorded, fail-closed exit 2, `--allow-partial` exit 1)
- [x] Benchmark the parallel sweep (`BenchmarkAnalyzeAll`, 8 seeded repos); worker count exposed as `--parallel N` on check/fix/who-forces (default: CPU count)
- [x] Schema-version field ("schema": 1) added to all three `--json` documents
- [x] Discovery issues surfaced in `fix --json` (`discovery` array) and human output; fix now exits 1 on discovery findings, matching check
- [x] Provider tests against `toolchain`-directive fixtures (stale toolchain; toolchain raising the alignment floor)
- [x] `Floor()`/`toolchainFloor()` unified onto a shared `highestMajorMinor` helper
- [x] Per-package coverage measured: pkg/fix 90.3%, cmd 87.9%, provider 83.8%, surface 87.9% (all above the 80% bar)
- [x] `check --quiet` (exit-code-only) for scripting; `--version` / `-version` flag aliases
- [x] Non-version toolchain directives surfaced: `toolchain default` fires an informational `toolchain-non-version` finding; `toolchain local` (which the go tool rejects) now makes an unparseable go.work surface as `go-work-unparseable` instead of vanishing silently
- [x] `who-forces` policy: `--allow-partial` downgrades module listing errors to exit 1 (fail-closed default kept); every forcing dependency carried with its own floor (`poisonerFloors`, sorted highest first); go.work rows marked `kind: go.work` and skipped with a note
- [x] Multi-root pool-contention test (6 repos through `--parallel 1`)
- [x] Mini-sweep validation: `check --json` across 6 fleet repos on real drift (go-finding 14 alignment suggestions, go-atomic-write/go-error-family/go-output patch-form findings)
- [x] Benchmark baseline (2026-09-22, go1.27.1, `-benchmem`, Ryzen AI MAX+ 395): `BenchmarkDiscover` 51.3µs / 29.3KB / 435 allocs; `BenchmarkAnalyze` 4.0µs / 3.6KB / 79 allocs; `BenchmarkParseDirective` 1.65µs / 1.5KB / 24 allocs; `BenchmarkAnalyzeAll` (8 seeded repos) 112.6µs / 62.6KB / 855 allocs — re-run before/after hot-path changes

## T12 — Lint & environment debt (from the 2026-09-19 full-mode run) — PARTIALLY DONE 2026-09-22

- [x] cqrs-lint A009/A018 verified as false positives (zero go-cqrs-lite references in go.mod/go.sum, re-verified 2026-09-22) and suppressed via `skip_steps: [cqrs-lint]` with rationale
- [x] go-error-modernization workflow run properly (`erraudit fix ./... --type-aware`: no auto-fixable; 5 `silent_swallow` findings dispositioned as deliberate skip-on-unparseable filters, each suppressed with `//nolint:erraudit` + reason; `nolint-audit` confirms 5 needed, 0 stale)
- [x] BuildFlow findings gate: 3 critical branching-flow PHANTOM_TYPE findings on new `who.go` code fixed by restructuring `parseFloorLine` to return named types (`GoVersion`, `ModulePath`, `ModuleVersion`); gate now passes at `--fail-on=error`
- [x] `buildflow doctor` run: the unavailable binaries (bandit, cargo-*, codespell, dprint, eslint, hadolint, jest, lychee, madge, …) are non-Go-ecosystem tools this Go-only repo never triggers ("not applicable", not failing); environment checks (disk, git identity, GOEXPERIMENT) green
- [x] skip_steps WARN "go-mod-update matches no registered tool" diagnosed: cosmetic, single-step mode only — in full pipeline runs both entries skip correctly ("skipped via skip_steps config"); no tool-name drift
- [ ] Triage the 366 go-auto-upgrade findings (grew from 265): all are testify → stdlib/testing migration suggestions on test assertions — a fleet-wide policy call (migrate off testify or keep it), not repo debt; RESOLVED BY POLICY 2026-09-22 (owner decision: keep testify; `.go-auto-upgrade.json` excludes `testifyassert` here — apply fleet-wide or leave per-repo)
- [ ] The remaining branching-flow INDEX_OUT_OF_RANGE warnings on the worker-pool `results[i] = …` pattern are provably safe (index bounded by the range) — root cause filed as branching-flow#1; un-nolint when it closes
- [ ] dependabot-auto-configure 2 findings remain (documented false positive, AGENTS.md known-tool-bugs)
- [ ] forbidigo vanishing (9 `fmt.Print*` hits gone after `buildflow format`) not reproducible in the 2026-09-22 run (forbidigo findings absent from both pre- and post-format states); watch for recurrence
- [ ] WATCH (2026-09-22): `check --json ~/projects/go-*` glob matches non-repo FILES (e.g. stray `.md` files) and reports them as empty clean repos instead of a root-not-found error — cosmetic noise in fleet sweeps; consider an `error?` row for non-directory roots

## T13 — cmdguard CLI surface migration (from the 2026-09-22 dedup session + Pareto plan) — MOSTLY DONE 2026-09-22

Plan: `docs/planning/2026-09-22_22-07_cmdguard-cli-surface-and-verification-plan.md` (spike verdict addendum §6).

- [x] WP-A exit-contract spike: silent exit-1-findings verified through `v4.ExitError` + selective `WithFangErrorHandler` suppression; verdict GO (plan §6)
- [x] WP-B: all four command surfaces migrated to `github.com/larsartmann/cmdguard/v4`; `runFlags`/`newRunFlagSet`/`parseRoots`/`usage` deleted; shared flags via embedded exported flag structs (`CommonFlags`/`QuietFlags`/`AnalysisFlags` — cmdguard skips unexported embedded types)
- [x] WP-C: exit-code matrix + `-h` contract tests; JSON wire contract locked by full-document goldens (schema 2)
- [x] WP-D: guard-rails — shared-flag-contract golden test + `TestNoRawFlagSkeleton` (bans `flag`/cobra imports in `cmd/`)
- [x] WP-F: `GOTOOLCHAIN=go1.27.1` pinned in flake.nix devShell; AGENTS commands de-prefixed; BuildFlow's global `GOWORK=off` overridden to empty (it broke `go work edit` tests inside `nix develop`)
- [x] WP-J: `readLines`/`rootsFrom`/`workersFor` unit tests
- [x] WP-K: [docs/DEDUPLICATION.md](docs/DEDUPLICATION.md) baseline; post-migration art-dupl run shows zero harmful `cmd/` clones
- [x] WP-L: triplicated worker pools consolidated into the generic `runSorted[I, R]` (also shrinks the branching-flow warning surface)
- [ ] WP-I: who-forces child-process env — pass/normalize the toolchain so `go list` works on 1.27-floor repos from older shells (reproduce first: known failure from the 2026-09-19 dogfood)
- [x] WP-H: reconcile the AGENTS.md floor-policy text with the sibling plan's fleet-minor ADR (T4) outcome — done 2026-09-22 (AGENTS floor-poisoning section rewritten for the post-campaign state)
- [x] WP-Q: go-atomic-write direct-dep tidy warning — resolved by the v0.6.0 bump (listed direct in go.mod, imported in pkg/fix)

## T5 — Upstream gomod-checker rule: "tidy revert" detection — WORTH CONSIDERING

- [ ] BuildFlow gomod-checker rule: go directive carrying a patch component after tidy (the poisoning signature) — closes the loop for repos that never run this tool
  - Rule spec sketch (2026-09-22): the rule fires when `go mod tidy` is a no-op AND some `go`/`toolchain` directive in the module graph carries a patch component that is NOT forced by a dependency floor — that is the accidental-minor signature. Fixtures: (a) x/text-forced `1.26.0` (legit, rule stays silent — the floor carrier is the dependency, named via `who-forces`), (b) json/v2-forced `1.27.1` std floor (legit, silent, message names the std floor), (c) `go 1.26.7` with max dep floor `go 1.26` (FIRE — this is the poisoning signature this tool strips). The rule must reuse `CompareDirective` semantics (bare minor ranks below zero patch) to avoid re-deriving them wrongly
- [ ] Test style: ginkgo/gomega BDD suites for NEW behavior specs (owner decision 2026-09-22 keeps testify for the existing table-driven suites; see AGENTS.md Testing policy)

## T6 — Release-authority drift (Layer-B versioning) — WORTH CONSIDERING

- [ ] Extend `pkg/surface` (or project-dependency-graph) to detect VERSION file vs CHANGELOG top vs newest git tag drift (known case: project-dependency-graph VERSION=0.7.0, tags at v0.2.0)
  - Design sketch (2026-09-22): read-only comparison of three sources — `VERSION` file, top `## [x.y.z]` in CHANGELOG.md, newest `v*` git tag (via `go/version` on annotated tags). Drift matrix reported as a new finding kind (`release-authority-drift`, suggest-only — which one is authoritative is per-repo policy, same reasoning as pin alignment). Stays out of `Discover`'s pure file walk only if git access is required; a pure first pass (VERSION vs CHANGELOG) can live in Discover with the tag comparison as an optional second pass
