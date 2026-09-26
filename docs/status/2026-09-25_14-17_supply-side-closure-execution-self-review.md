# Status Report — Supply-Side Closure: Full Execution Session + Self-Review

**Date:** 2026-09-25 14:17 CEST
**Session scope:** The full-execution phase after `docs/planning/2026-09-25_09-57_poisoner-chain-dissolution-supply-side-closure.md` — all 15 planned P-tasks across 4 repos (go-version-auto-configure, go-health, go-health-dashboard, BuildFlow) plus a 12-repo fleet sweep. Skills loaded: pareto-planning, go-release, buildflow.

---

## a) FULLY DONE

**Releases (all proxy-verified, pushed, GitHub Releases created):**

1. **go-health v0.4.1** — minor-form floor fix (`go 1.27.1` → `go 1.27`), CHANGELOG + annotated tag `ffbcd86`, CI green at tag, proxy-verified in scratch module; `/version` HTTP-endpoint idea routed to their TODO_LIST (Open section) per decision.
2. **This tool v0.2.1** — nix-pin comment false-positive fix (`nixCommentScanner`), honest fixability note (`gatedFormFixNote`), dep-forced exit-0 contract (test `TestExitFromOutcomes_DepForcedOnlyExitsZero`), fixtures incl. go-health's exact flake shape; live re-check on go-health: exit 0 (FP gone).
3. **This tool v0.2.2** — behavior-identical lint-clean refactor (v0.2.1's tag CI was red — see d1); CI green at tag; fleet-pinned tag carries green CI.
4. **BuildFlow repinned → v0.2.2** — flake input + tools/go.mod bumped, `go work vendor`, vendorHash verified current, `nix build .` green, user-profile binary upgraded (`buildflow version e881e96`), live step-verified (`-s go-version-auto-configure` green on dashboard). The 7e1fbfe/v0.1.0-era stale binary is now shadowed.

**Consumer closure:**

5. **go-health-dashboard: entire conflict chain dissolved.** Bumped to v0.4.1; tidy no longer lowers a directive, so our `fix` applied `1.27.1 → 1.27` and survived the gate — the tool's full intended lifecycle proven on its flagship consumer. Full suite green incl. browser tests. Guard script + CI wiring deleted; filing row closed with `who-forces` evidence ("no dependency forces a higher floor"); AGENTS gotcha + CHANGELOG updated. **Master CI green** — including fixing their pre-existing 4-run red streak (templ/fmt drift, gci, makezero).

**Sweep + docs:**

6. **Fleet sweep of all 12 go-health importers:** 2 clean (dashboard, go-taskqueue), 7 dep-forced-correct (exit 0, legitimate patch floors held by OTHER poisoners), 4 gate-protected (reverted, zero unintended mutations — verified all directives unchanged).
7. **Sweep discoveries recorded:** new pending poisoners (go-cqrs-lite v4 family ~27 modules, go-health-dashboard v0.10.x tags, go-etag, go-sse, older go-output tags) in `docs/POISONERS.md`; two classification bugs filed as TODO_LIST T14 (std json/v2-only floor → FAILED instead of DEP-FORCED, reproduced on projects-management-automation/pkg/domain; vendor-mode `go list` failure → same, reproduced on dnsblockd).
8. **Documentation shipped:** POISONERS.md registry (+AGENTS link), GATE-VERIFICATION.md recipe, ADR-0001 incident appendix, TODO_LIST T14 + T2 resolved-stale + T1 consumer-row update, status-report correction appendix for the earlier flake-FP claim.
9. **Plan committed and pushed** (`3534420`) before execution, per the requested flow.

## b) PARTIALLY DONE

1. **BuildFlow S87 row text says "at v0.2.1"** — the second repin (v0.2.2) landed via the daemon's heuristic commit (`e881e968`) and I did not circle back to update the row's version mention. Content accurate except that one string. Effort: S.
2. **Unexplained observation:** dashboard's `nix fmt` reports "formatted 4 files (2 changed)" even when the tree is clean afterwards. Verified empirically harmless (drift check no-op, CI green); counter semantics never understood. Low value, noted for honesty.
3. **`go mod tidy` before the v0.2.x tags:** tidy was run as part of release prep for go-health but for this repo I relied on the earlier stable state (T1: tidy stable) rather than re-running it in the release gates. It passed CI, but the gate sequence differed from the go-release skill's checklist. Not a defect; a process wobble.
4. **pkg.go.dev listing for v0.2.1/v0.2.2** — not checked (proxy verification was the release gate; doc indexing lags anyway).

## c) NOT STARTED

1. **Two classification bugs** (std json/v2 floor, vendor-mode `go list`) — diagnosed root cause class, recorded in T14 with repro repos, NOT fixed. Deliberate: they need test fixtures and are v0.2.3 material; hasty classification changes after a fresh release would be verschlimmbessern.
2. **Supply-side re-tags** for the newly discovered poisoners (go-cqrs-lite v4 family, dashboard v0.10.x, go-etag, go-sse) — recorded only. This is the new frontier: 7 of 12 importers stay dep-forced until these re-tag.
3. **BuildFlow defense retirement decision** (`GoWorkFloorFinding`/`RestoreGoWorkFloor` + `DependsOn`) — marked optional in their S87; decision deferred.
4. **`/version` endpoint implementation in go-health** — correctly routed to their TODO; not our work, listed for completeness.

## d) TOTALLY FUCKED UP

1. **Tagged v0.2.1 with red CI.** I gated with build/vet/test/gofmt but NOT golangci-lint; the repo's CI runs it, and my new scanner tripped four rules (gocognit 26>25, makezero, wsl_v5, mnd). Consequence: v0.2.1's CI run is permanently red (tags are immutable), forced a second release (v0.2.2) + a second BuildFlow repin cycle (~30min rework). Root cause: my local gate list was assumed, not read from `.github/workflows/ci.yml`. The go-release skill's Phase 4.1 lists golangci explicitly — I skipped it. Worst mistake of the session precisely because it was foreseeable and cheap to prevent.
2. **Dashboard one-liner took three pushes.** (i) Committed `templ generate` output without `nix fmt` (their documented canonical order is generate → fmt — I ran fmt on the first bump commit but not before the guard-removal commit's regenerated files); hygiene gate red. (ii) My `sed` fix never matched (pattern anchored to a line-end the commented line didn't have) and I committed blind — the wrapped-long-nolint shape — without reading the diff; CI red again. (iii) Third push finally format-stable and green. Root causes: no diff-read before commit, blind sed, comment exceeding the formatter's wrap threshold unchecked.
3. **Fix-run hygiene:** I ran real `fix` across 11 repos and verified outcomes from the tool's own summary (applied 0 everywhere → no tree changes), but never spot-checked a repo's `git status` at sweep time. The inference was sound (and later evidence confirms no churn); the discipline (verify, don't infer) was skipped. No damage — luck of applied-0, not verified safety.
4. **BuildFlow dev-env build failure by runbook inversion:** updated the flake input before tools/go.mod, breaking the dev-env derivation their own P1 row's ordered runbook prevents. Recovered outside the devShell; ~5min lost. Read-the-runbook-after-the-failure, not before.

## e) WHAT WE SHOULD IMPROVE

1. **Gate parity rule:** before ANY release tag, read the repo's CI workflow and run every gate it runs, locally, in order (in BuildFlow-covered repos: `buildflow -s <step>` inside `nix develop`). My assumed gate list ≠ the repo's gate list. This single habit would have prevented d1 entirely.
2. **Diff-before-commit rule:** never `git add` + commit without reading the actual diff; blind `sed` edits on shifted content are how one-line fixes become three-push sagas.
3. **Full-log diagnosis rule:** the dashboard's first red run listed ALL failing findings at once (gci ×3, makezero, hygiene drift); I fixed them in waves as they re-appeared instead of treating the log as the complete worklist. One read, one fix commit, one push.
4. **Runbook-first rule for cross-repo work:** when a repo's TODO/docs contain an ordered runbook for exactly the operation I'm performing (BuildFlow P1's release chain), execute it in the documented order rather than rediscovering it by failure.
5. **Formatter-aware commenting:** inline `//nolint` comments have a line-budget; check the wrap threshold before writing long ones (or put the rationale above the line).
6. **Record anomalies:** the "(2 changed)" fmt counter — either investigate or write it down at the moment; carrying an unexplained observation silently is how later sessions inherit ghosts.
7. **Sweep verification protocol:** fleet `fix` runs should pair the tool summary with one `git status` spot-check per outcome class (applied / dep-forced / failed), not rely on the summary alone.

## f) Next tasks (ranked; feeds docs-health HARVEST)

| #  | Task                                                                                                                                                                                      | Repo            | Impact   | Effort | Category |
| -- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------- | -------- | ------ | -------- |
| 1  | Coordinated re-tag campaign for go-cqrs-lite v4 family (~27 modules, `go 1.27.1` floors) — the largest pending poisoner, blocks 7+ consumers                                              | go-cqrs-lite    | Critical | L      | Release  |
| 2  | Re-tag go-health-dashboard with minor-form floor (master already settled) so its consumers (DiscordSync et al.) can settle                                                                | dashboard       | High     | S      | Release  |
| 3  | Re-tag go-etag + go-sse (minor-form floors)                                                                                                                                               | go-etag, go-sse | High     | S      | Release  |
| 4  | v0.2.3: classification fix — std json/v2-only floor (tidy wants raise, `go list` shows no module forcer) → dep-forced, not FAILED; fixture from projects-management-automation/pkg/domain | this repo       | High     | M      | Bug      |
| 5  | v0.2.3: classification fix — vendor-mode `go list` failure → parse `vendor/modules.txt` `## explicit; go X` as fallback floor; fixture from dnsblockd                                     | this repo       | High     | M      | Bug      |
| 6  | Fix BuildFlow S87 row text v0.2.1 → v0.2.2 (one string)                                                                                                                                   | BuildFlow       | Low      | S      | Docs     |
| 7  | Decide BuildFlow defense retirement (GoWorkFloorFinding + DependsOn) now the fixer is active                                                                                              | BuildFlow       | Medium   | S      | Decision |
| 8  | After poisoner re-tags: re-run the 12-importer sweep; expect 7 dep-forced → clean transitions                                                                                             | fleet           | High     | M      | Quality  |
| 9  | Add a "release gates = CI jobs" checklist to this repo's release flow (read .github/workflows first; document in AGENTS or a release checklist doc)                                       | this repo       | Medium   | S      | Process  |
| 10 | Investigate the `nix fmt` "(2 changed)" counter anomaly on the dashboard (or document it as known-benign)                                                                                 | dashboard       | Low      | S      | Bug      |
| 11 | Re-verify T1 consumer campaign baseline (36/48 drifted repos from 2026-09-22) against today's state; update ADR appendix counts                                                           | this repo       | Medium   | M      | Docs     |
| 12 | pkg.go.dev listing check for v0.2.1/v0.2.2 (T3 open row)                                                                                                                                  | this repo       | Low      | S      | Docs     |
| 13 | go-health: `/version` endpoint helper (their TODO Open row — their session, listed so it isn't lost)                                                                                      | go-health       | Medium   | M      | Feature  |
| 14 | Dashboard dependabot PR run red (pre-existing, noticed this session) — their triage, not touched                                                                                          | dashboard       | Low      | S      | Bug      |
| 15 | Consider a `check --deps` flag (reuse AnalyzeFloors) so check can pre-classify dep-forced patch forms without the gate — closes the honesty gap fully instead of via the caveat note      | this repo       | Medium   | L      | Feature  |
| 16 | CI YAML comment false-positive audit: `scanCIPins` is line-anchored (`^go-version:`) so YAML `#` comments cannot match — write the fixture proving it, cheap insurance after the Nix FP   | this repo       | Low      | S      | Quality  |
| 17 | Fleet CI lint-drift scan: two repos (this one at v0.2.1, dashboard pre-session) went red from golangci-lint@latest drift; consider pinning the lint version in workflows fleet-wide       | fleet           | Medium   | M      | Process  |
| 18 | Status-report correction appendix pattern worked well — make it a standing rule in AGENTS (corrections append, never rewrite)                                                             | this repo       | Low      | S      | Docs     |

_(18 honest items; not padded to 50.)_

## g) Questions I cannot answer myself

1. **Poisoner re-tag campaign scheduling:** go-cqrs-lite v4 (~27 modules) + dashboard v0.10.x + go-etag + go-sse all need minor-form re-tags before 7 of 12 importers can settle. One coordinated campaign session (like 2026-09-22's), or piecemeal per-repo sessions as touched? Your call on sequencing and whether cqrs-lite v4.12.0 (or whatever the next natural version) bundles anything else.
2. **v0.2.3 timing:** fix the two classification bugs now (small focused release; they affect sweep-report quality but not correctness — reverts are always safe) or batch with the `check --deps` feature later? Release-cadence preference.
3. **BuildFlow defenses:** retire `GoWorkFloorFinding`/`RestoreGoWorkFloor` + the `DependsOn` ordering now that the v0.2.2 fixer is active in the running binary, or keep them permanently as belt-and-suspenders? Their repo, your risk appetite — S87 currently marks retirement "optional, not urgent".

---

_Point-in-time snapshot. Daemon picks up the commit. WAITING FOR INSTRUCTIONS._
