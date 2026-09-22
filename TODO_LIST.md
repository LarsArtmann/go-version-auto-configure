# TODO List

Short- and mid-term actionable work. Ordered by impact (Pareto). Harvested from `docs/status/archived/2026-09-18_15-13_fleet-versioning-unification.md` (section f) on 2026-09-18; swept 2026-09-18 (T7 fleet-sweep enablers and T10 repo hygiene shipped — see CHANGELOG Unreleased); re-harvested 2026-09-19 from `docs/status/archived/2026-09-19_06-39_t7-t9-t10-shipped-lint-zero-full-gate-green.md` (section f → T11/T12, T3 additions); T11 fully shipped and T12 mostly dispositioned 2026-09-22 (see CHANGELOG Unreleased).

## T0 — go.work-aware fixer: never strip the go.work patch floor below a module's requirement — NEW (2026-09-22, S87 root cause)

- [ ] **The fixer normalizes go.work's `go` directive to minor-only (`go 1.27.1` → `go 1.27`) without checking module floors.** Under `go.work`, the go line must cover every module's directive at FULL patch granularity (`go work` errors with "module X listed in go.work requires go >= 1.27.1, but go.work lists go 1.27"), so a dep-forced patch floor (deps declaring 1.27.1) makes every `go` subprocess in the workspace fail after a "helpful" normalization. Verified live on the BuildFlow workspace 2026-09-20 (gotcha #169; the whole pipeline's go subprocesses failed until the floor was restored). Required behavior: before rewriting any go.work go line, compute `max(module go directives)` over all `use`'d modules (full patch granularity) and never write a line below it; ideally surface a finding when the fixer wants to go lower and refuses. BuildFlow already defends downstream (go-work-sync `RestoreGoWorkFloor` + `DependsOn: [go-version-auto-configure]` ordering) — this entry tracks the SOURCE-side fix so every consumer stops needing the arbiter.

## T1 — Supply-side re-tag campaign (BLOCKING for fleet convergence) — PLANNED

Published library versions carry patch-form `go` floors that re-poison every consumer on `go mod tidy` (verified 2026-09-18: stripping this repo's directive to `go 1.26` and running plain `go mod tidy` re-raises it to `go 1.26.7`):

- [ ] `go-finding` (local root `go 1.27` after form fix; decide 1.26 vs 1.27 — its toolsdk module is `go 1.26`, flake pins `go_1_26`): re-tag with major.minor-only directives
- [ ] `go-atomic-write`: floor is accidental (`go 1.27.1`; deps allow 1.26 — xxhash 1.11, flock 1.25.0). Downgrade candidate, needs owner confirmation (F16), then re-tag
- [ ] `go-error-family`, remaining go-* libraries with published patch-form floors
- [ ] After re-tags: bump the autoconfigure family + fleet consumers to clean versions (go-ecosystem-upgrade protocol: baseline → sweep → test → commit per repo)
- [ ] Re-run `go-version-auto-configure check` fleet-wide; expect ~0 mechanical findings that survive `go mod tidy`
- [ ] Re-tag hygiene: verify no `replace` directives leak into any re-tagged go.mod (go-release Phase 3)
- [ ] Post-release: `go get @vX.Y.Z` clean-module verification per re-tag (proxy check)

## T4 — Decide the fleet minor: 1.26 vs 1.27 — PLANNED (owner decision)

- [ ] 62 modules declare 1.27/1.27.1 (accidental, floor-copied; go-atomic-write is the identified source; installed toolchain + 241 flakes are go_1_26 with `GOTOOLCHAIN=local`)
- [ ] Option A (recommended): downgrade floors to `go 1.26` fleet-wide (supply-side first), keeping nixpkgs 1.26 as the single toolchain
- [ ] Option B: adopt 1.27 — bump nixpkgs + CI pins fleet-wide (42 flakes already do)
- [ ] Record the decision and rationale as an ADR so the fleet policy is citable
- [ ] Whichever wins, encode it: this tool reports minors above the environment as alignment findings; a `--expect-minor` flag could enforce the decision

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
- [ ] Module-path casing check (`github.com/larsartmann/…` vs `LarsArtmann`) with a real `go get` after the next tag
- [ ] Verify the README build-from-source steps in a clean environment (container/nix shell)
- [ ] Cut v0.2.0 from CHANGELOG `[Unreleased]` once CI + GoReleaser + pkg.go.dev land

## T9 — Parser coverage — PARTIALLY DONE (toolchain directives shipped; flake.lock blocked by design)

- [ ] flake.lock effective Go revision parsing — **blocked by design**: the lock records only a nixpkgs rev, not the Go version it packages; resolving it requires an impure `nix eval`, but `Discover` must stay pure (reads files, writes nothing). A separate opt-in command (or BuildFlow step) would be the right home — design needed before building

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

## T12 — Lint & environment debt (from the 2026-09-19 full-mode run) — PARTIALLY DONE 2026-09-22

- [x] cqrs-lint A009/A018 verified as false positives (zero go-cqrs-lite references in go.mod/go.sum, re-verified 2026-09-22) and suppressed via `skip_steps: [cqrs-lint]` with rationale
- [x] go-error-modernization workflow run properly (`erraudit fix ./... --type-aware`: no auto-fixable; 5 `silent_swallow` findings dispositioned as deliberate skip-on-unparseable filters, each suppressed with `//nolint:erraudit` + reason; `nolint-audit` confirms 5 needed, 0 stale)
- [x] BuildFlow findings gate: 3 critical branching-flow PHANTOM_TYPE findings on new `who.go` code fixed by restructuring `parseFloorLine` to return named types (`GoVersion`, `ModulePath`, `ModuleVersion`); gate now passes at `--fail-on=error`
- [x] `buildflow doctor` run: the unavailable binaries (bandit, cargo-*, codespell, dprint, eslint, hadolint, jest, lychee, madge, …) are non-Go-ecosystem tools this Go-only repo never triggers ("not applicable", not failing); environment checks (disk, git identity, GOEXPERIMENT) green
- [x] skip_steps WARN "go-mod-update matches no registered tool" diagnosed: cosmetic, single-step mode only — in full pipeline runs both entries skip correctly ("skipped via skip_steps config"); no tool-name drift
- [ ] Triage the 366 go-auto-upgrade findings (grew from 265): all are testify → stdlib/testing migration suggestions on test assertions — a fleet-wide policy call (migrate off testify or keep it), not repo debt; needs an owner decision before any mechanical migration
- [ ] The 3 remaining branching-flow INDEX_OUT_OF_RANGE warnings on the worker-pool `slices[i] = …` pattern are provably safe (index bounded by the range) but unlabeled — either a linter upstream fix or documented nolint
- [ ] dependabot-auto-configure 2 findings remain (documented false positive, AGENTS.md known-tool-bugs)
- [ ] forbidigo vanishing (9 `fmt.Print*` hits gone after `buildflow format`) not reproducible in the 2026-09-22 run (forbidigo findings absent from both pre- and post-format states); watch for recurrence

## T5 — Upstream gomod-checker rule: "tidy revert" detection — WORTH CONSIDERING

- [ ] BuildFlow gomod-checker rule: go directive carrying a patch component after tidy (the poisoning signature) — closes the loop for repos that never run this tool

## T6 — Release-authority drift (Layer-B versioning) — WORTH CONSIDERING

- [ ] Extend `pkg/surface` (or project-dependency-graph) to detect VERSION file vs CHANGELOG top vs newest git tag drift (known case: project-dependency-graph VERSION=0.7.0, tags at v0.2.0)
