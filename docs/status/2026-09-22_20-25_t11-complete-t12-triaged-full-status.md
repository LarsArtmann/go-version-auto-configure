# Status Report — 2026-09-22 20:25 CEST — T11 complete, T12 triaged, gate green

Session scope: one working session (~19:30–20:25) executing TODO_LIST.md T11 (tool hardening, 12 items) and triaging T12 (lint & environment debt). Baseline at session start: `e22cf90` docs-formatted, all tests green. This report covers ONLY what this session did and observed; no new research beyond the session's scope.

**Format note:** the status-report skill's canonical output is a styled HTML dashboard; the user explicitly requested `.md` this time, so this file is Markdown (one-off override, not propagated into the skill).

**Verification state at time of writing** (all re-run in the final sweep):

- `GOEXPERIMENT=jsonv2 go build ./...` ✅
- `go test -count=1 ./...` ✅ (5 packages) and `go test -race ./...` ✅
- `buildflow -s golangci-lint` → **✓ BuildFlow passed, 0 findings**
- `erraudit . --type-aware` → **0 findings**; `erraudit nolint-audit` → "5 directives: 5 needed, 0 stale"
- `buildflow --dry-run` findings gate → **passes at `--fail-on=error`** (379 residual findings, all warning/info; see (b)/(d))
- Binary smoke tests: `--version`, `check --quiet` (exit 1 on drift ✅), `who-forces --allow-partial` (exit 1 ✅), `fix --dry-run --json` (schema 1, heldBack `1.26.7` ✅)

---

## a) FULLY DONE

All items verified by tests, lint, or direct CLI evidence. The auto-commit daemon committed the work incrementally (heuristic messages `5c46687`, `335df68`, `71d768c`, …); final state re-verified above.

1. **T11: `Floor()`/`toolchainFloor()` unification** — both surface-floor helpers now share one `highestMajorMinor([]GoVersion)` helper; duplication removed. Evidence: `pkg/surface/surface.go:151-215`, full suite green.
2. **T11: JSON schema versioning** — all three `--json` documents (`check`, `fix`, `who-forces`) carry a top-level `"schema": 1` field (`jsonSchemaVersion` const). Evidence: `cmd/json.go`, test asserts `schema == 1`.
3. **T11: discovery issues in `fix --json`** — `fixOutcome` carries discovery findings; `fix --json` gained a `discovery` array; human output renders `DISCOVERY` lines; fix now exits 1 when discovery findings exist (consistency with check). Evidence: `TestRun_FixJSONSurfacesDiscoveryIssues` (exit 1 + rule asserted on an unparseable-go.mod fixture).
4. **T11: `check --quiet`** — exit-code-only scripting mode; `--json` output still emitted when explicitly requested. Evidence: `TestRun_CheckQuietSuppressesOutput` (empty stdout at exit 1; JSON present under combined flags).
5. **T11: `--version` / `-version` aliases** alongside the `version` subcommand. Evidence: `TestRun_VersionAndUsage`.
6. **T11: `--parallel N`** on `check`/`fix`/`who-forces` (0 = auto = CPU count) + `BenchmarkAnalyzeAll` (8 seeded repos, ~67µs/op at 20 iterations). Evidence: `TestRun_CheckParallelLimitCoversMoreRootsThanWorkers` (6 repos through `--parallel 1`, sorted output, all analyzed).
7. **T11: non-version toolchain surfacing** — informational `toolchain-non-version` rule fires for `toolchain default` (pins nothing, previously silently skipped by floor analysis). Key research finding: `toolchain local` is **rejected by the real go tool** ("must match format go1.23.0 or default", verified via `go mod edit -json` / `go work edit -json` probes) — the actual silent gap was go.work parse failures vanishing, so `go-work-unparseable` discovery finding was added (go.work parse errors previously returned `nil, nil` silently). Evidence: `TestAnalyze_ToolchainDefaultIsSurfaced`, `TestDiscover_ToolchainLocalMakesGoWorkUnparseable`.
8. **T11: who-forces policy hardening** — `--allow-partial` downgrades per-module `go list` failures from exit 2 to exit 1 (fail-closed default kept); `poisonerFloors` carries EVERY forcing dependency with its own floor (sorted highest floor first), not just max-floor carriers; go.work rows appear as marked `kind: "go.work"` rows instead of being silently skipped. Evidence: `TestExitFromFloors_AllowPartialDowngradesModuleErrors`, `TestAnalyzeFloors_PoisonerFloorsCarryEachForcersFloor`, `TestAnalyzeFloors_MarksWorkspaceRowWithoutAnalysis`.
9. **T11: provider tests for toolchain directives** — stale toolchain (`toolchain-below-directive`) and toolchain-raises-alignment-floor (`nix-pin-below-floor`) fixtures through `Provider.Detect`. Evidence: `pkg/provider/provider_test.go`, both green.
10. **T11: coverage measurement + gap closing** — per-package: **pkg/fix 73.4% → 90.3%**, **cmd 81.5% → 87.9%** (also surface 87.9%, provider 83.8%, version 95.5%). Closed: `moduleScoped`, `DepForcedError.Error`, `Result.Report` (all sections), `SelfCheck`, real-tool `applyOne`/`ensureTidyStable`/`EditRunner` integration tests (dependency-free temp module, no network), `summarize`, `printFloors` all row shapes, `printFixReport` discovery branch, `exitFromOutcomes`/`exitFromAnalyses` contracts, `comparePoisonerFloors`, `Fix.Describe`. All remaining sub-80% entries are error-branch functions in otherwise well-covered packages.
11. **T11: who-forces dogfood on real foreign repos** — go-finding (6 modules incl. go.work row), go-atomic-write, go-error-family, go-output, linter-autoconfigure-sdk. Verified: network resolution works where toolchains allow; the 1.27-floor-under-`GOTOOLCHAIN=local` failure mode is recorded truthfully per module; fail-closed exit 2 / `--allow-partial` exit 1 verified without pipe-masking.
12. **T11: mini-sweep validation** — `check --json` across 6 fleet repos on real drift: go-atomic-write 1 mechanical, go-error-family 8 mechanical, go-output 17 mechanical, linter-autoconfigure-sdk 1, this repo 1 (the known steady-state directive), go-finding 14 alignment suggestions (flake + 13 CI pins trailing its 1.27 floor) — output eyeballed, suggestions actionable.
13. **T12: cqrs-lint suppression** — A009/A018 re-verified as false positives 2026-09-22 (zero `go-cqrs-lite` matches in go.mod AND go.sum, `rg -c` exit 1) and suppressed via `skip_steps: [cqrs-lint]` with rationale in `.buildflow.yml`. Full-run dry-run: skipped via config.
14. **T12: erraudit workflow run properly** (per go-error-modernization skill): `erraudit fix ./... --type-aware` → "No fixes found" (nothing auto-fixable); 5 `silent_swallow` findings dispositioned as **deliberate** skip-on-unparseable filters in `pkg/surface` (the skip IS the policy: unparseable surface entries carry no rule); each suppressed with `//nolint:erraudit` + specific reason; `nolint-audit` confirms 5 needed, 0 stale. erraudit now reports 0 findings.
15. **T12: BuildFlow findings gate restored** — the 3 **critical** branching-flow PHANTOM_TYPE findings (on this session's new `splitFloorEntry(floor, dep, entry string)` — exactly the fleet phantom-type policy) were fixed structurally: `parseFloorLine` now returns named types (`surface.GoVersion`, `surface.ModulePath`, `fix.ModuleVersion`) and the ad-hoc string-splitting helper was deleted. Gate passes at `--fail-on=error` (was failing before this fix).
16. **T12: `buildflow doctor` run** — the unavailable binaries (bandit, cargo-audit, cargo-deny, cargo-machete, codespell, dprint, eslint, hadolint, interrogate, jest, c8, knip, go-licenses, lychee, madge, …) are non-Go-ecosystem tools this Go-only repo never triggers ("not applicable", not failing); environment checks (disk space, git identity, GOEXPERIMENT) green.
17. **T12: skip_steps WARN diagnosed** — "go-mod-update matches no registered tool" appears **only in `-s <tool>` single-step runs** (the skipped tools aren't in that pipeline); in full pipeline runs both entries skip correctly ("skipped via skip_steps config"). No tool-name drift. Documented in AGENTS.md as a known-tool-bug note; not fixable from this repo.
18. **Docs updated**: CHANGELOG Unreleased (9 Added, 3 Changed, 4 Fixed entries); TODO_LIST.md T11 → DONE with per-item evidence, T12 → PARTIALLY DONE with dispositions; README (fleet-sweep paragraph with `--parallel`/`--quiet`, JSON contract table incl. `schema`/`discovery`/`kind`/`poisonerFloors`, who-forces `--allow-partial` prose); FEATURES.md (policy rules 6→8, CLI flags row, JSON schema row, who-forces row); AGENTS.md (policy 4 extended to go.work, policy 6 corrected re: `toolchain local` being tool-rejected, cqrs-lint note updated to suppressed, 2 new known-tool-bug notes: single-step skip_steps WARN cosmetic, INDEX_OUT_OF_RANGE safe pattern).

---

## b) PARTIALLY DONE

1. **T12 — lint & environment debt** (marked PARTIALLY DONE in TODO_LIST).
   - Done: cqrs-lint (suppressed), erraudit (0 findings), critical gate findings (0), doctor (documented), skip_steps WARN (diagnosed).
   - Open: **366 go-auto-upgrade findings** (grew from the 265 in the TODO; 360 warning + 6 info) — characterized this session: they are testify → stdlib/testing migration suggestions on every `assert.*`/`require.*` call (94× Equal, 72× NoError, 53× Contains, 37× Len, …). Disposition requires a fleet policy decision, not mechanical fixing. Blocker: owner decision. Effort if "keep testify": S (suppress/annotate policy + document). If "migrate": L per repo, fleet-wide campaign.
   - Open: 3 branching-flow INDEX_OUT_OF_RANGE warnings (cmd/main.go:139, 374, 534) — the worker-pool `results[i] = …` pattern inside `for i, root := range roots`; index provably bounded by the range. Pre-existing (in the 13-finding baseline before this session), warning-severity, gate passes. Not fixed because branching-flow nolint support is unverified; documented in AGENTS.md instead. Effort: S if a suppression works, otherwise upstream linter fix.
   - Open: dependabot-auto-configure 2 findings — documented false positive (AGENTS.md), upstream fix needed.
   - Open: forbidigo vanishing (9 `fmt.Print*` hits gone after `buildflow format` in the 2026-09-19 session) — **not reproducible this run** (no forbidigo findings in either state); left as watch-item.
2. **T3 — publish this tool** (v0.1.0 tagged 2026-09-18) — untouched this session; CI workflow, GoReleaser, pkg.go.dev, v0.2.0 cut etc. all outstanding (see (c)).
3. **T9 — parser coverage** — flake.lock remains blocked by design (unchanged).
4. **This repo's own version surface** — `check .` still exits 1 on the known steady-state drift: go.mod at `go 1.26.7` because every published dependency (go-finding v1.12.0, toolsdk, linter-autoconfigure-sdk) carries that patch floor. `who-forces` truthfully reports `max dep floor: go 1.26.7` = directive (not poisoned, but at the poisoned equilibrium). Cannot stick until T1 re-tags. Unchanged from session start (by design — documented in AGENTS.md dogfooding caveat).

## c) NOT STARTED

(No code written this session or before; unchanged priorities.)

- **T1 — supply-side re-tag campaign** (BLOCKING for fleet convergence): go-finding, go-atomic-write, go-error-family + remaining libs re-tag major.minor-only; then consumer bump sweep; then fleet-wide check. Blocked on: T4 owner decision + release execution. Still the highest-impact work in the fleet.
- **T4 — fleet minor decision (1.26 vs 1.27) + ADR**: owner decision pending; 62 modules on accidental 1.27/1.27.1 vs installed toolchain go1.26.7 with `GOTOOLCHAIN=local`.
- **T2 — BuildFlow blank-import wiring** (BuildFlow dev task).
- **T5 — gomod-checker "tidy revert" upstream rule**.
- **T6 — release-authority drift detection** (VERSION vs CHANGELOG vs tags).
- **T9 — flake.lock parsing** (blocked by design: needs impure `nix eval`; Discover must stay pure).
- **T3 sub-items**: GitHub Actions CI, GoReleaser + ldflags stamping, pkg.go.dev verification, website launch, branch-protection decision, repo topics, CI badge, module-path casing `go get` check, clean-environment README verification, v0.2.0 cut.
- **New from this session**: `--expect-minor` flag (T4 follow-up), erraudit CI gating (`--type legacy_as` fallback), DOMAIN_LANGUAGE.md vocabulary update (see (d)/(e)) — listed in (f).

## d) TOTALLY FUCKED UP

Nothing in the final state is broken — build/test/race/lint/erraudit/gate all green — but radical honesty requires naming these:

1. **I initially wrote the toolchain-local rule on an unverified assumption — and it was wrong.** First implementation assumed `toolchain local` is a valid go.mod directive that parsing silently accepts. It is not: `golang.org/x/mod/modfile` AND the real go tool reject it ("invalid toolchain version 'local'"). The failing test caught it only after I probed. Root cause: I violated my own READ→RESEARCH→execute loop — a 20-second `go mod edit -json` probe BEFORE writing the rule would have produced the correct design (non-version names + go-work-unparseable) on the first attempt. The final design is right; the process cost was ~3 extra edit/test cycles and one misnamed rule (`RuleToolchainLocal` → `RuleToolchainNonVersion`).
2. **A careless multiedit deleted a comment block.** Restructuring `formIssues`'s nolint line, my `old_string` included the following comment line and my `new_string` dropped it, silently mangling the go.work workspace-floor comment. Caught on the very next view (the surrounding lines made the gap obvious) and repaired; tests never went red because comments don't compile. Lesson: multiedit fragments that straddle adjacent content are dangerous; never include lines in `old_string` that the replacement doesn't re-emit.
3. **Twice I read exit codes through a pipe** (`cmd | head; echo $?`) and got `head`'s exit code instead of the tool's — the first dogfood "exit=0" was wrong (real value 2). Both caught immediately and re-measured without pipes, but a less careful read of that output would have concluded the tool "passes" on failing repos.
4. **The lll lint loop took 3 iterations** (129 → 125 → 123 → under-120 chars) on my own nolint rationale comments: I added lint suppressions without checking the lll line-length budget those same comments now count against. Each iteration was a full buildflow -s golangci-lint round trip.
5. **A wrong test assertion shipped in an intermediate state** (`comparePoisonerFloors` ordering: asserted `z < a` positive-negative inverted) — caught by the suite before any commit of that hunk, but it was a think-it-through failure, not a code failure.
6. **DOMAIN_LANGUAGE.md was not updated.** This session coined real domain vocabulary — `poisonerFloors`, `toolchain-non-version`, `go-work-unparseable`, `ModuleVersion`, the `schema` contract field — and AGENTS.md explicitly names docs/DOMAIN_LANGUAGE.md as the glossary home. It is now stale relative to the rules this session added. (Fix queued as item 30 in (f).)
7. **Standing condition (not new, not fixed, still fucked):** the fleet's supply side remains un-retagged, so (a) this repo's own `check` can never go green, and (b) `who-forces` **cannot analyze go-finding or go-atomic-write at all in this environment** — every module row errors with "go.mod requires go >= 1.27 (running go 1.26.7; GOTOOLCHAIN=local)". The tool reports this truthfully, but T11's "dogfood the poisoner matrix on real foreign repos" is only half-proven: the poisonerFloors path was validated against fakes and the poisoner resolution in `fix` against this repo, never against a real resolvable 1.27-floor graph, because none exists here until T1/T4 land.

## e) WHAT WE SHOULD IMPROVE

1. **Probe the real tool before encoding assumptions.** The `toolchain local` mistake (d1) generalizes: any rule about go tool behavior should start with a throwaway probe against the installed `go` binary, not from memory of docs. Cost when skipped: wrong rule + rename + test rewrite.
2. **Run the linter against each file immediately after writing it**, not in a batch at the end. The lll loop (d4) and the prealloc/varnamelen findings all appeared in one batch; incremental linting would have surfaced each at its cheapest fix point.
3. **Never read exit codes through pipes.** Use `cmd > file 2> err; echo $?` or capture `$?` immediately. Should be a reflex; twice-bitten this session.
4. **Multiedit hygiene:** when `old_string` spans a code line plus adjacent comment lines, the replacement must re-emit everything or the edit must be split. Consider verifying with an immediate `view` after any multiedit that touches comment boundaries.
5. **Same-commit glossary discipline:** every new Rule constant and JSON contract field should get a one-line DOMAIN_LANGUAGE.md entry in the same change set. This session added ~5 terms and updated 4 docs but missed the glossary.
6. **Every new JSON field deserves a decode-level assertion in the same session.** `schema` is asserted for check/fix docs but the who-forces test struct still decodes without `schema`, and `kind: "go.work"` is asserted only on the Go struct, not through the JSON. Small gaps, cheap to close (items 31, 38 in (f)).
7. **Upstream bug reports are piling up with evidence in hand** (BuildFlow single-step skip_steps WARN; cqrs-lint A018 claiming "imports go-cqrs-lite" with zero imports; branching-flow INDEX_OUT_OF_RANGE false positive on the range-bounded worker-pool pattern). Each is documented locally but not filed upstream — verify-before-filing + github-voice would turn these notes into fixes that benefit every fleet repo. The branching-flow worker-pool false positive is a candidate for a fleet-wide lessons.md entry.
8. **The `--parallel` flag exists but the fleet standard for sweeps is undocumented** — a one-paragraph README recommendation ("use `--quiet --parallel N --json` for fleet cron sweeps; exit codes are the contract") would make the scripting path discoverable.
9. **Benchmark hygiene:** `BenchmarkAnalyzeAll` runs the full analyze path but nothing compares parallel settings (1 vs 4 vs auto) or reports allocations (`-benchmem`); a table in the docs would let the sweep claims ("scales with drifted repos, not repo count") be cited with numbers.
10. **coverage gaps remaining are all error-branches** (ParseModulePath errors, syntaxLine nil, currentDirective read-failure paths). Fine to leave, but a single sentence in FEATURES/test docs stating "error-branch functions below 80% are accepted" would prevent future sessions from re-litigating them.

## f) Up to 50 things we should get done next

Ranked by impact; effort S <30min, M 30min–2h, L >2h. Items 1–28 are pre-existing backlog; 29–50 are session-originated. (HARVEST note: 1–28 belong in TODO_LIST.md as-is; 29–50 should be triaged — TODO_LIST vs ROADMAP per docs-health.)

| # | Task | Impact | Effort | Category |
|---|------|--------|--------|----------|
| 1 | T4: Decide fleet minor 1.26 vs 1.27, record ADR | Critical | S | Decision |
| 2 | T1: Re-tag go-finding with major.minor-only go directives | Critical | M | Release |
| 3 | T1: Re-tag go-atomic-write (accidental 1.27.1 → 1.26 downgrade, F16 confirmation) | Critical | M | Release |
| 4 | T1: Sweep remaining go-* libraries with published patch-form floors (go-error-family, …) | Critical | M | Release |
| 5 | T1: After re-tags, bump autoconfigure family + fleet consumers (go-ecosystem-upgrade protocol) | Critical | L | Release |
| 6 | T1: Re-run `check` fleet-wide; expect ~0 mechanical findings surviving tidy | High | M | Verification |
| 7 | T1: Verify no `replace` directives leak into any re-tagged go.mod | High | S | Release hygiene |
| 8 | T1: `go get @vX.Y.Z` clean-module proxy verification per re-tag | High | S | Verification |
| 9 | T3: GitHub Actions CI — lint + test matrix + dogfood `check .` gate (GOEXPERIMENT=jsonv2) | High | M | Feature |
| 10 | T3: GoReleaser config with ldflags version stamping | High | M | Feature |
| 11 | T3: pkg.go.dev verification after next tag | High | S | Verification |
| 12 | T3: Cut v0.2.0 from CHANGELOG Unreleased once CI + GoReleaser + pkg.go.dev land | High | S | Release |
| 13 | T3: Branch protection decision for master (daemon direct pushes vs required checks) | High | S | Decision |
| 14 | T3: Repo topics (go, buildflow, golangci, auto-configure, version-surface) | Medium | S | Cleanup |
| 15 | T3: README CI badge once workflow lands | Low | S | Documentation |
| 16 | T3: Module-path casing check with real `go get` after next tag | Medium | S | Verification |
| 17 | T3: Verify README build-from-source steps in clean environment | Medium | S | Verification |
| 18 | T3: Website launch decision (sibling-project pattern) | Low | L | Feature |
| 19 | T2: Add provider blank-import to BuildFlow SDK import set | High | S | Feature |
| 20 | T2: BuildFlow docs/provider catalog entry + `buildflow --dry-run` discovery confirm | Medium | S | Feature |
| 21 | T2: Decide DAG position (after go-mod-update, before nix-checker) | Medium | S | Decision |
| 22 | T12: Testify keep-or-migrate fleet decision; disposition the 366 go-auto-upgrade findings | High | S (decide) / L (migrate) | Decision |
| 23 | T12: Investigate forbidigo vanishing if it recurs | Low | S | Bug |
| 24 | T5: Upstream gomod-checker "tidy revert" rule (patch-form directive after tidy) | Medium | L | Feature |
| 25 | T6: Release-authority drift detection (VERSION vs CHANGELOG vs newest tag) | Medium | M | Feature |
| 26 | T9: Design opt-in impure command for flake.lock Go revision | Low | M | Design |
| 27 | Build `--expect-minor` flag once T4 lands (enforce the ADR) | High | M | Feature |
| 28 | Wire erraudit into CI gating (`--type-aware` primary, `--type legacy_as` fallback) | Medium | S | Quality |
| 29 | Update docs/DOMAIN_LANGUAGE.md with session vocabulary: poisonerFloors, ModuleVersion, toolchain-non-version, go-work-unparseable, schema field | High | S | Documentation |
| 30 | File upstream: BuildFlow single-step `skip_steps` WARN is misleading (evidence in AGENTS.md) | Medium | S | Bug (upstream) |
| 31 | File upstream: cqrs-lint A018 "imports go-cqrs-lite" fires with zero imports | Medium | S | Bug (upstream) |
| 32 | File upstream/check: branching-flow INDEX_OUT_OF_RANGE false positive on range-bounded worker pool | Medium | S | Bug (upstream) |
| 33 | Add decode-level assertions: `schema` in who-forces JSON test; `kind: "go.work"` through JSON | Medium | S | Quality |
| 34 | Provider test for a go-work-unparseable fixture (unparseable go.work through `Provider.Detect`) | Medium | S | Quality |
| 35 | Dogfood `fix` end-to-end (not dry-run) on a disposable copy of a foreign drifted repo to exercise the full dep-forced supply-side naming path | High | S | Verification |
| 36 | Dogfood who-forces against a real, resolvable poisoned graph (possible only after items 2–3) — validates poisonerFloors against reality | High | S | Verification |
| 37 | Extend benchmark: `-benchmem` + parallel 1/4/auto comparison table, record in docs | Medium | S | Quality |
| 38 | Push remaining sub-80%-per-function error branches (ParseModulePath, syntaxLine, currentDirective) above the bar or document the acceptance | Low | S | Quality |
| 39 | README: scripting recipe `check --quiet --parallel N --json ~/projects/*/ ; echo $?` as the canonical fleet cron line | Medium | S | Documentation |
| 40 | Decide schema-2 plan: fold `Poisoners []string` into `poisonerFloors` (breaking) or keep duplicated forever | Medium | S | Decision |
| 41 | Consider `fix --quiet` and `who-forces --quiet` for scripting symmetry | Low | S | Feature |
| 42 | Verify `resolvePoisoners` output ordering matches who-forces' sorted poisoner presentation (consistency check) | Low | S | Quality |
| 43 | ROADMAP entry: state filepath/POSIX assumptions (no Windows testing exists) | Low | S | Documentation |
| 44 | Decide whether this repo gets its own flake.nix (fleet-standard packaging + version stamping; currently BuildFlow-only) | Medium | M | Decision |
| 45 | Sweep validation at scale: `check --quiet --parallel auto` across all ~/projects (383+ modules), record timing + finding counts as the session-independent baseline | Medium | M | Verification |
| 46 | Add `who-forces` JSON contract table to README's machine-contract section for `poisonerFloors` field semantics (already partially done — verify final wording) | Low | S | Documentation |
| 47 | Add `check --parallel 0` (explicit auto) vs default parity test | Low | S | Quality |
| 48 | Post-re-tag: re-run this repo's own `fix` expecting `applied 1, dep-forced 0` — the actual finish line of the whole campaign | High | S | Verification |
| 49 | Record a cross-project lesson: "probe the real toolchain before encoding go-tool behavior in rules" in crush-config references/lessons.md | Medium | S | Process |
| 50 | Record a cross-project lesson: "exit codes must be captured before pipes" alongside the existing pipeline-masking lessons | Low | S | Process |

## g) Questions I cannot figure out myself

1. **Fleet minor: does the fleet standardize on `go 1.26` (downgrade everything, keep nixpkgs 1.26 as the single toolchain) or adopt `1.27` (bump nixpkgs + 42 flake pins + CI)?** I cannot decide this: it trades 62 accidental 1.27 downgrades against a toolchain fleet upgrade, and T1's re-tag targets (1.26 vs 1.27 for go-finding) are undefined until it lands. Everything in (f) items 1–8 and 48 queues behind it.
2. **Testify: is it fleet policy to keep testify, or migrate tests to stdlib `testing`?** The 366 go-auto-upgrade findings are all testify→stdlib suggestions. I can suppress them (if testify stays, it's a one-line policy annotation per repo) or plan a mechanical migration campaign — but the direction is a fleet-wide code-style decision that no amount of local code reading can determine.
3. **JSON contract philosophy: when `who-forces` schema bumps to 2, should `poisoners []string` be removed (clean, breaking) or kept alongside `poisonerFloors` forever (additive-only)?** This is a stability-vs-cleanliness call on the wire contract that consumers (CI scripts, BuildFlow) depend on; I can implement either, but only you know which consumers actually parse `poisoners` today.

---

*Point-in-time snapshot 2026-09-22 20:25 CEST. Section (f) is HARVEST input for TODO_LIST.md/ROADMAP.md; items 29–50 need routing triage. Report itself: no commit per harness rule — the auto-commit daemon picks it up.*
