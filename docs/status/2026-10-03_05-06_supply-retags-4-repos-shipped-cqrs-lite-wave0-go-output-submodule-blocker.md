# Status — WP-04 repin complete; supply-side re-tags shipped for 4 repos; cqrs-lite Wave-0 blocked on go-output submodules

**Session:** 2026-10-03 ~03:44–05:06 CEST (resumed from the 02:13 mid-flight handoff)
**Plan:** [docs/planning/2026-10-03_00-56_SUPERB-fleet-convergence-execution-plan.md](../planning/2026-10-03_00-56_SUPERB-fleet-convergence-execution-plan.md)
**State at write:** gvac master @ `57f9cfe` (= v0.2.4, clean; daemon sweeps). BuildFlow `eb35fce` (repinned). **WAITING FOR INSTRUCTIONS** — 3 owner questions in §g.

---

## a) FULLY DONE (this session, verified)

### WP-04 — BuildFlow repin + S87 defense retirement (all 6 micro-tasks 27–32)

- `execution/go.mod:149` bumped v0.2.3 → v0.2.4 via `go mod edit` (the member the interrupted session missed); all three gvac-requiring members (root/tools/execution) now at v0.2.4.
- `go work vendor` re-run inside devShell → **single** gvac stanza in vendor/modules.txt (v0.2.4, `## explicit; go 1.27`) — the two-stanza corruption is gone.
- `nix run .#update-vendor-hash` (current, nothing to update) + `nix build .` **green**; profile binary upgraded via `nix profile upgrade buildflow` → `buildflow version` = **eb35fce** (was 5c5cfb8, 68 commits behind).
- **Live gate proof through the installed binary:** seeded `/tmp/dfx/simple` (`go 1.26.7`) → `-s go-version-auto-configure` fires *"the go directive is a floor and must be major.minor only"* (1 finding, warning); `--fix` → `go 1.26`, "1 fixed". Gate + fixer + vendor fallback are live fleet-wide.
- **dependabot-auto-configure#3 fix confirmed live:** `-s dependabot-auto-configure` on gvac is silent on the "no update groups" false positive (issue closed 2026-10-02; old binary was the cause, exactly as diagnosed).
- **S87 disposition recorded:** `GoWorkFloorFinding`/`RestoreGoWorkFloor` stay retired in BuildFlow (moved into gvac — the floor-safe fixer IS the defense); go-work-sync `DependsOn: [go-version-auto-configure]` **KEPT** as belt-and-suspenders. Written into BuildFlow AGENTS.md gotcha #187 (the stale "RELEASE-CRITICAL … flake input still pins gvac v0.1.0" paragraph rewritten to RESOLVED with full evidence).
- gvac TODO_LIST **T14-② deleted** (completed-rows-to-CHANGELOG policy); CHANGELOG `[Unreleased]` records the repin + S87 decision.
- Parallel-session handling: BuildFlow's other session (telemetry/dbstore files) was active; I touched only gvac-repin files, verified tree state before/after, lost nothing.

### Supply-side re-tag waves — 4 repos, 12 tags, all pushed + proxy-verified minor-form

Campaign was **re-sequenced by the real dependency DAG** (plan had WP-01 before WP-02; reality is leaf-first):

```
go-sse (self-poisoned, leaf)  →  go-datastar  →  go-health-dashboard
go-etag (independent leaf)
go-cqrs-lite (89 modules; forced partly by go-sse — fixed)  →  cqrs-htmx (14 tags, last)
```

Every tag below was verified by resolving it in a scratch module and reading the cached proxy `.mod` (the evidence rule):

| Repo | Tags cut | Proxy `.mod` | Notes |
| ---- | -------- | ------------- | ----- |
| go-sse | `v0.6.2`, `sseparse/v0.2.1` | `go 1.27` both | tidy-stable, tests green pre-tag; ssetest converged on master afterwards (pins v0.6.2/v0.2.1, directive 1.27) — its tidy initially re-raised on the stale `sseparse v0.2.0` pin, fixed by pinning v0.2.1 |
| go-datastar | `v0.6.2`, `broadcast/v0.6.2`, `datastartest/v0.6.2` | `go 1.27` all three | root cut+pushed first, then submodules pinned to it; static stays 1.26 (already clean) |
| go-etag | `v0.6.1` ×5 (root, client, entitytag, metrics, server) | `go 1.27` all five | internal DAG entitytag→server→{root,metrics,client} via forward pins; go.work replaces moved to v0.6.1 keys + directive 1.27; post-push standalone tidy+build per module |
| go-health-dashboard | `v0.10.2` | `go 1.27` | pins bumped to sse/datastar v0.6.2; pre-existing golden drift regenerated (see §d/§e); full suite green ×2 after |

CHANGELOGs cut in all four repos (Keep-a-Changelog sections with the de-poisoning rationale; Unreleased folded where present). Daemon races on commits were absorbed/retried with `commit -am --no-verify` (their documented mechanical-sweep pattern).

### WP-01 groundwork (inventory + DAG + Wave 0)

- **True poisoner inventory** (published latest-tag floors, from `git show <tag>:<dir>/go.mod`): go-cqrs-lite has **89 tagged modules publishing `go 1.27.1`** — not the ~27 the plan estimated. cqrs-htmx adds 14. Legacy prefixes (codec/, core/, memory/, saga/, … at 1.26.x) are irrelevant (below fleet floor).
- External-forcer scan across all 98 tree modules: only go-sse (1.27.1) and go-output pins (v0.38.2) forced cqrs-lite above 1.27 — both addressed in Wave 0.
- **Wave 0 executed:** `go-sse v0.6.1 → v0.6.2` in **47 modules**; `go-output v0.38.2 → v0.38.3` in the **6 cmd modules** (api-stability, cqrs-bench, cqrs-gen, cqrs-lint, cqrs-upgrade, doc-check). Per-module `go mod tidy -e` clean, workspace `go build ./...` green.
- Their release runbook was read and honored: `scripts/batch-release.sh` (path-vs-tag guard, tagged-modules-only replace strip, GOWORK=off standalone-build gate, restore), `scripts/pin-sweep.sh`, the tag-wave four hard mechanics + 92-tag-train lessons (dependency-ordered invocations, GOPRIVATE direct-VCS resolution, daemon race handling, forward pins).

---

## b) PARTIALLY DONE

- **WP-01 (cqrs-lite batch re-tag):** inventory ✅, floor plan (external forcers eliminated) ✅, Wave-0 pin bumps ✅ — **strip to `go 1.27`, dependency-level computation, ordered batch-release invocations, tags, push, proxy verification: NOT started.** Blocked by the go-output submodule discovery (§g Q1). Full test suite not yet run on the Wave-0 tree (build-only so far — their `verify-module`/`test` gates pending).
- **WP-02:** 4 of 5 repos done (§a). Remaining: **cqrs-htmx** — who-forces surface map across importers (micro-task 49) + strip/build/test/tag its 14 modules (micro-task 50), which must run AFTER the cqrs-lite wave (it pins cqrs-lite modules).

## c) NOT STARTED (unchanged from the plan)

WP-05 (fleet sweep + ADR counts), WP-09 (CI depth), WP-10 (test truth), WP-11 (nix hardening), WP-12 (depFloor unify), WP-13 (benchmarks/coverage), WP-14 (stability triage), WP-15 (gomod-check alignment), WP-16 (fleet-truth docs), WP-17 (sibling claim check), WP-19 (docs polish), WP-20 (docs gate), WP-21 (test-depth backlog), WP-22 (T5 rule), WP-23 (T6 rule), WP-24 (T9 decision), WP-26 (website row), final v0.2.5 + living-docs refresh + plan ANNOTATE + harvest.

## d) TOTALLY FUCKED UP (honest ledger — all recovered, none shipped broken)

1. **Tag-floor scan bug (worst one):** my first poisoner scan built `git show $tag:$p/go.mod` paths with a trailing-slash doubling for sub-prefixed tags → submodule poisoners were silently missed on the first pass (go-etag's client/entitytag/metrics/server, go-datastar's broadcast/datastartest, go-sse's sseparse all "passed"). Caught by cross-checking `go list -m` graph floors against the scan; the fixed scan is what produced the true 89/14 counts. Lesson violated: validate extraction tooling on one known case before trusting a batch (go-ecosystem-upgrade Phase 4 #5).
2. **Daemon commit race:** go-etag's first `git add` → `git commit` chain died ("no changes added" — the daemon absorbed the staged set in between, 3 daemon commits landed mid-chain). Recovered with `git commit -am`. Should have used `-am` from the start for mechanical sweeps.
3. **`GOWORK=off go work edit`** failed ("no go.work file found") — the exact trap documented in this repo's own AGENTS policy #3. Fixed by dropping GOWORK=off for workspace-scoped commands.
4. **Misleading `rc=$?`** captured `head`'s exit code after piping `go test` output — printed "rc=0" next to real FAIL lines in one dashboard run. The FAILs were investigated properly (pre-existing/flake), but the echo was wrong.
5. **go-etag go.work versioned-replace keys:** `go work edit -replace` ADDS versioned keys rather than replacing — go.work briefly carried both v0.6.0 and v0.6.1 keys; stale keys dropped afterwards.
6. **go-health-dashboard pre-existing red master:** 5 golden-render tests failed identically before and after my pin bumps (dep-drift from an earlier swept bump, un-regenerated goldens). I regenerated + reviewed the diff (templ class-order + one `tabindex`, no semantic change) and folded it into the release — a side-fix riding a supply wave (F10-adjacent), justified as unblocking and documented in the CHANGELOG entry. Flag for the dashboard owner to sanity-check.
7. **TestMetrics_LatencyHistogram flake:** failed once under full-suite load with my bumps, passes 3/3 isolated + full suite green ×2. Timing-dependent (`waitForTrendSamples`), not caused by the bump. Recorded, not "fixed".

## e) WHAT WE SHOULD IMPROVE

1. **Plan estimates vs reality:** "~27 cqrs-lite modules" was off by 3.3× (89), and the campaign's dependency order was inverted (leaves first, not cqrs-lite first). The POISONERS/ADR data was stale relative to the actual published-tag surface. A `who-forces`-driven inventory should precede any wave planning.
2. **Proxy-`.mod` floor scan needs a canonical tool:** the per-repo `git show tag:go.mod` dance is error-prone (see d1). gvac `who-forces` across importers, or a small "latest-tag floor table" command, would kill this class. (Candidate TODO — feeds WP-22/T5 thinking.)
3. **go-output's v0.38.3 "clean" claim was half-true:** the ROOT module re-tagged minor-form, but its 9 submodules (d2, delimited, graph, markdown, markup, plantuml, serialization, table, tree) still publish `go 1.27.1`. The v0.38.3 verification only fetched the root `.mod`. Evidence rule should cover **every tagged module path a repo publishes**, not the root.
4. **Daemon-aware commit discipline:** every cross-repo mechanical change should be `git commit -am --no-verify` in ONE command; multi-step add/commit chains invite the race.
5. **Pre-flight baseline tests per foreign repo** (before bumps) caught two pre-existing reds here — make it mandatory in the wave runbook so regressions can't hide behind "it was already red".
6. **The 6 cmd modules of cqrs-lite may stay dep-forced** if go-output submodules aren't re-tagged — that's a legitimate terminal state per fleet policy, but it means the "cqrs-lite family fully minor-form" outcome has a carve-out that must be documented in POISONERS/ADR either way.

## f) NEXT — up to 50, in execution order

1. Answer §g Q1 (go-output submodule re-tags: yes/no).
2. If yes: re-tag go-output submodules (strip + test + v0.38.4 or submodule-patch tags, their convention decides) + proxy-verify every submodule `.mod`.
3. If no: mark the 6 cqrs-lite cmd modules dep-forced-by-go-output-submodules (documented carve-out).
4. Re-run the external-floor scan on cqrs-lite — expect zero hits ≥1.27.1.
5. Strip all 98 cqrs-lite tree go.mods to `go 1.27` (`go mod edit -go=1.27`, cd per module — gotcha 4).
6. Decide + set go.work directive (1.27 if nothing stays dep-forced; else keep 1.27.1) via `go work edit` (no GOWORK=off).
7. Compute the cqrs-lite sibling-require DAG → dependency levels (leaves: id/metadata/schema/…).
8. Check for cycles in that DAG (must be acyclic for ordered batches).
9. Build the per-level new-version table (latest tag patch+1 per module, ~89 rows).
10. Run their full test gate on the stripped tree (`nix run .#test` / workspace `go test ./...`) — baseline before any tag.
11. Update cqrs-lite AGENTS.md note 96 ("Modules are on go 1.27.1") + any doc that hardcodes the patch floor.
12. Root CHANGELOG release-train section for the wave (their root-CHANGELOG-only policy).
13. Level-0 batch: `batch-release.sh --dry-run` manifest first (existence/collision/path-vs-tag guards).
14. Level-0 batch: real cut → push tags → smoke (`tag-release.sh --smoke` per tag; NOT `--smoke-all` under held lock).
15. Level-1: bump sibling pins to published level-0 tags (`pin-sweep.sh` or manual `go mod edit -require`) → tidy → build+test-compile (`go test -run ZZNONE -count=1`) → batch cut → push → smoke.
16. Repeat 15 per level through the whole DAG (expect ~5–8 invocations).
17. Refresh cqrs-lint taskmanager golden (V006 version-set pin) in-wave.
18. GOWORK=off build matrix over ALL swept modules post-wave (their mechanic 3).
19. Per-module `go mod tidy -e` post-push (missing `/go.mod` hash class, gotcha from 2026-09-11).
20. Scratch-module proxy probe per published symbol surface (their 2026-09-28 lesson 3) — at least root + spot set.
21. Proxy-`.mod` verification for all 89 tags (loop, record URLs+dates) → POISONERS.md cqrs-lite row Active → Resolved with tag-pair + evidence.
22. cqrs-htmx: `who-forces` map across its importers (surface map BEFORE its re-tag).
23. cqrs-htmx: strip 36 go.mods → tidy (needs clean cqrs-lite pins) → build+test → 14 tags → push → proxy-verify → POISONERS row update.
24. WP-05: fleet sweep baseline `check --quiet --expect-minor 1.27 ~/projects/*/` (expect dep-forced → clean transitions across dnsblockd/DiscordSync/…).
25. WP-05: bump remaining go-output v0.38.2 pinners fleet-wide (`go get …@v0.38.3` + tidy + gvac fix).
26. WP-05: ADR-0001 appendix count refresh (baseline 36/48 drifted → new number).
27. WP-16: POISONERS.md evidence convention (proxy `.mod` URL + date per row) + maintenance rule.
28. WP-16: ADR-0001 records — 2026-09-26 classification session + the v0.38.2 regression incident + THIS wave's numbers.
29. WP-09: CI `go mod tidy -diff` gate job (the 2026-09-30 class dies pre-push).
30. WP-09: pin golangci-lint in ci.yml to devShell version; erraudit step; concurrency group.
31. WP-09: README floor-line dogfood + `version`-stamp assertion; verify workflow green end-to-end.
32. WP-10: e2e fixture through the REAL `go mod tidy -diff` gate; ParseDirective invariants post-v0.38.3; deliberate `buildflow update` re-run asserting settle at 1.27.
33. WP-11: go-nix-helpers bump + FOD floor `checks` entry.
34. WP-12: depFloor unification (who.go + floor.go) proven behavior-identical via the WP-06 goldens.
35. WP-13: benchmarks + coverage re-measure into a LIVING doc.
36. WP-14: license-check ×5 isolated runs; doctor warnings; vulnix CVE triage; art-dupl vs DEDUPLICATION baseline.
37. WP-15: gomod-check ↔ go-mod-normalize flipflop reproduction + disposition alignment.
38. WP-17: sibling inverted-claim grep ("v0.38.2 is minor-form") + fixes + date.
39. WP-19: README who-forces example from the WP-06 fixture; CanonicalizeGoMod nuance; FEATURES evidence re-walk; ADR-0001 verify.
40. WP-20: standing docs gate script + wiring.
41. WP-21: test-depth batch (flag-help, exitFrom table, runSorted, version golden, BenchmarkApplyAll).
42. WP-22: T5 gomod-checker tidy-revert rule (spec + fixtures + BuildFlow implementation).
43. WP-23: T6 release-authority-drift Discover pass + tests.
44. WP-24: T9 nix-pin home decision + record.
45. WP-26: website decision row.
46. Final: fold CHANGELOG, cut v0.2.5 (needs §g Q3), full gates, tag+push+proxy-verify.
47. Final: TODO_LIST harvest (delete done rows), FEATURES refresh, plan ANNOTATE (never rewrite), this report's harvest under resolved-or-owned.
48. Post-campaign: re-run `gvac who-forces` on the former poisoner consumers to confirm at-parity carriers now name the clean generation.
49. Record the "scan-tooling validated on known case" lesson in the project AGENTS.md testing notes.
50. Consider a gvac TODO row: "latest-tag floor table" command (from §e2) — the campaign just paid for its absence twice.

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **go-output submodule re-tags (blocking WP-01's clean finish):** the final floor scan shows go-output's 9 submodules (d2, delimited, graph, markdown, markup, plantuml, serialization, table, tree) still publish `go 1.27.1` on the proxy — the v0.38.3 fix only covered the root module. They force the 6 cqrs-lite cmd modules (api-stability, cqrs-bench, cqrs-gen, cqrs-lint, cqrs-upgrade, doc-check) to stay dep-forced at 1.27.1, so cqrs-lite cannot become 100% minor-form until they re-tag. **May I cut supply-side re-tags in go-output (a repo outside g1's named list), or should those 6 modules land as documented dep-forced?** (My recommendation: re-tag — otherwise the campaign ships a carve-out that re-triggers this exact session in a month.)
2. **Confirm the g1 scope as executed:** the real poisoner set turned out to be 89 cqrs-lite modules (not ~27) + 14 cqrs-htmx tags, and the leaf-first re-sequencing meant sse/datastar/etag/dashboard (12 tags) were pushed under WP-02 before any cqrs-lite tag. All are already pushed and proxy-verified. **Green light to continue with cqrs-lite's 89 + cqrs-htmx's 14 on the same authorization?**
3. **v0.2.5 push authorization (standing question from the 02:13 report):** one release at the end of the this-repo WPs vs per-phase cuts — and does the finish-everything directive include tag+push for v0.2.5 the way g2 did for v0.2.4? I will not push it without a yes.

---

*Point-in-time snapshot 2026-10-03 05:06 CEST. Session ends in WAITING FOR INSTRUCTIONS; no further work started until the owner answers.*
