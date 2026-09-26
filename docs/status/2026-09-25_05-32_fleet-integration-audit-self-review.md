# Status Report — Fleet Integration Audit (go-health / go-health-dashboard) + Session Self-Review

**Date:** 2026-09-25 05:32 CEST
**Session scope:** Two Q&A turns, read-only investigation + one throwaway-copy verification run. **No production code changed in this repo** (only AGENTS.md poisoner-note update + this report).
**Tools exercised:** repo greps, `git show` (tags), `/tmp/gvac check|fix` (this repo's v0.2.0+ build from current master), `cp -r` throwaway gate test.

---

## a) FULLY DONE

1. **Q1 answered: no `/version` HTTP-endpoint awareness exists.** Repo-wide grep for `/version|http|endpoint` — zero hits in any rule, provider, or doc. `pkg/version` stamps a CLI `version` command (ldflags > VCS > "dev"), not an HTTP route. Scope is the toolchain version surface only (go/toolchain directives, go.work, flake.nix, CI pins). Evidence: grep output; AGENTS.md architecture section.
2. **Direct-integration layer mapped: none exists, both directions.** No reference to `go-version-auto-configure` in either repo's go.mod, docs, or configs; no reference to either repo in ours. Evidence: cross-repo greps (0 hits each).
3. **Indirect-integration layer fully mapped (the real answer):**
   - **BuildFlow** is the integration point: toolsdk self-registered provider (blank import, 115 providers), `go.mod:171` requires v0.2.0, `go.mod:404` local replace to this checkout, `flake.nix:165` input pinned `refs/tags/v0.2.0`; `go-mod-normalize` step calls our `CanonicalizeGoMod`.
   - **Supply chain:** go-health v0.4.0 (tagged 2026-09-22 21:21) ships `go 1.27.1` — verified via `git show v0.4.0:go.mod`. Its master already carries `go 1.27` (unreleased). go-health-dashboard requires go-health v0.4.0 → its floor is dep-forced at 1.27.1.
4. **Incident chain root-caused:** our fixer was the documented mutator behind the dashboard's 3× `go 1.27.1 → 1.27` downgrade incidents on 2026-09-22 ("twice before the guard, once after" — their status docs) and BuildFlow's 2026-09-20 go.work corruption (BuildFlow gotcha #169 / S87). The dashboard built `scripts/check-go-directive.sh` (CI guard asserting `1.27.1`) and still holds an open fleet-filing decision.
5. **Live `check` on both repos (built from current master):**
   - go-health: 1 finding — `nix-pin-below-floor` (flake.nix:41 pins `go_1_26`, module floor 1.27), suggest-only.
   - go-health-dashboard: 1 finding — `go-directive-patch-form` (go.mod:3, `go 1.27.1`), reported "all auto-fixable".
6. **Dep-floor gate verified working, empirically:** real (non-dry) `fix` on a throwaway copy of the dashboard → `applied 0, dep-forced 1`, message `floor go 1.27.1 is forced by: github.com/larsartmann/go-health` + supply-side remediation hint, **go.mod untouched**. The v0.2.0 tidy gate closes the dashboard incident class. Copy cleaned up via `trash`.
7. **AGENTS.md memory updated:** go-health v0.4.0 added as a PENDING (not accepted) patch-form poisoner with the gate-verification evidence.

## b) PARTIALLY DONE

1. **BuildFlow's installed binary version NOT verified.** The gate fix only protects repos if the _running_ BuildFlow binary is built at the v0.2.0 pin — go-health's own status (#39) documented a stale binary. I flagged the assumption but never ran `buildflow doctor` / version check. Remaining: one command. Effort: S. Blocker: none (skipped for scope).
2. **Finding severity/gate impact on the dashboard inferred, not verified.** I claimed the patch-form finding lands advisory for the dashboard's BuildFlow gate — inferred from BuildFlow gotcha #171 ("first run found 21 REAL warnings, exit 0"), not checked against the dashboard's actual `fail_on` config. Effort: S.
3. **Fix exit-code discrepancy observed but not diagnosed:** live `fix` on the dep-forced copy exited **0**, while AGENTS.md documents "exit 1 = findings/failed fixes/poisoned". Is a dep-forced-only run "poisoned"? Behavior and doc disagree or the doc is ambiguous. Effort: S (read cmd exit logic + decide).
4. **check vs fix contract mismatch noticed late:** `check` promises "all auto-fixable with 'fix'" for the dashboard's finding, but `fix` then classifies it dep-forced (not fixable until supply-side re-tags). I mentioned "residual friction" in conversation but did not raise it as a product gap. Effort: M to fix properly.

## c) NOT STARTED

1. **`/version` HTTP-endpoint candidate task** — offered in turn 1, never entered into TODO_LIST.md/ROADMAP.md. Still wanted? (See question g3.)
2. **ADR 0001 consumer-baseline cross-check** — never verified whether go-health / go-health-dashboard are tracked in `docs/adr/0001-fleet-go-minor.md`'s consumer campaign appendix. If absent, our own campaign tracking has a hole. Priority: High (it is the documented single source for consumer-campaign progress).
3. **BuildFlow TODO row reconciliation** — BuildFlow's S87 ("upstream fix open") and P1 (release chain pending) rows predate the v0.2.0 pin their flake already carries; both are stale or partially done. Not touched.
4. **`who-forces` evidence for the dashboard's fleet-filing decision** — their open question ("file go-mod-normalize downgrade upstream?") is now moot-ish (fixed by our gate) but nobody has told them; running `gvac who-forces` on their repo and recording it would close the row. Not done.

## d) TOTALLY FUCKED UP

1. **Hallucinated rule name in the turn-1 answer.** I cited `go-patch-in-directive` as a rule of this tool. The real rule is **`go-directive-patch-form`** — learned only when running the tool in turn 2 (`go-work-below-floor` and `toolchain-below-directive` were real; that one was invented from an AGENTS.md paraphrase). Severity: low (conversational), class: high (confidently stated an unverified identifier — exactly what the verify-external-claims skill exists to prevent). Root cause: answered from memory-of-docs instead of from code/tool output. Mitigation: rule for myself — never name an identifier without seeing it in source or tool output first.
2. **Nothing else damaged.** No repo mutations (fix ran only on a /tmp copy), no commits forced, no reverts, no pushes. Honest ledger: 1 factual error, 0 damage.

## e) WHAT WE SHOULD IMPROVE

1. **Verify identifiers against source/output before stating them** (rule names, step names, flags). Cost when skipped: confident wrongness in fleet advice. Fix: mandatory grep/view before first use in a session.
2. **Memory writes at discovery time, not report time.** The go-health-v0.4.0-poisoner fact belonged in AGENTS.md the moment the gate test proved it (it got there only during this report). The global protocol already mandates this; I deferred.
3. **Contract-reconciliation reflex.** When live behavior deviates from a documented contract (fix exit 0 vs "exit 1 poisoned"), chase it in the moment — it took a self-review prompt to itemize it.
4. **Product gap — check's "auto-fixable" promise is Analyze-time optimism.** For patch-form go lines, fixability depends on the dep floor, knowable only via the tidy gate / `go list -m`. Options: relabel ("fixable pending gate"), make `check` dep-aware behind a flag (reuse `AnalyzeFloors`), or always append the supply-side hint to check output. Impact: every poisoned consumer sees a false "all auto-fixable" claim today — the exact repos that most need the truth.
5. **Policy split brain across repos (named, not created by us):** the dashboard's CI guard asserts `go 1.27.1` is CORRECT while our tool's finding tells the same repo to strip the patch. Two automations asserting opposite truths about one line. Resolution is the go-health re-tag (supply side), not code here — but the conflict should be named in the finding text so dashboard maintainers aren't left guessing (ties into e4).
6. **Test-copy hygiene:** `cp -r` of the whole dashboard repo (incl. `.git`) for a gate test — wasteful. A filtered copy (`--exclude .git` / shallow) is faster and safer. Trivial, but repeat it right.
7. **Ghost systems: none found in session scope.** `pkg/version` is wired (CLI + ldflags + flake stamping); `CanonicalizeGoMod`/`SyncGoWorkDirectives` have verified BuildFlow consumers. No integration debt discovered — the audit's negative results are real negatives.

## f) Next tasks (ranked by impact; session-derived — feeds docs-health HARVEST)

| #  | Task                                                                                                                                                                                 | Repo      | Impact   | Effort | Category   |
| -- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------- | -------- | ------ | ---------- |
| 1  | Cut go-health minor-form floor release (master already `go 1.27`; pick v0.4.1 vs v0.5.0 — see g1)                                                                                    | go-health | Critical | S      | Release    |
| 2  | After #1: bump go-health-dashboard to the tag, let tidy settle `go 1.27`, delete `check-go-directive.sh` + CI wiring, close their fleet-filing row with gate-fixed-upstream evidence | dashboard | Critical | S      | Cleanup    |
| 3  | Rebuild/reinstall the BuildFlow binary at the v0.2.0 pin (stale-binary warning documented in go-health status #39) — the gate protects no one until the running binary has it        | BuildFlow | High     | S      | Bug        |
| 4  | Diagnose fix exit code for dep-forced-only runs (observed 0; doc says 1 for "poisoned"); align code or AGENTS.md wording                                                             | this repo | High     | S      | Bug/Docs   |
| 5  | Fix check's "all auto-fixable" claim for gate-dependent fixes (relabel, or dep-aware check via AnalyzeFloors flag)                                                                   | this repo | High     | M      | Feature/UX |
| 6  | Cross-check ADR 0001 consumer baseline includes go-health-dashboard (add if missing)                                                                                                 | this repo | High     | S      | Docs       |
| 7  | Verify BuildFlow S87 defenses (GoWorkFloorFinding + DependsOn ordering) can be retired now that v0.2.0 carries the go-work-aware fixer; update their TODO S87/P1 rows                | BuildFlow | High     | M      | Cleanup    |
| 8  | Raise go-health flake pin `go_1_26` → `go_1_27` (our real finding: nix-pin-below-floor)                                                                                              | go-health | Medium   | S      | Bug        |
| 9  | Run `gvac who-forces` on dashboard; record output in their fleet-filing row as closing evidence                                                                                      | dashboard | Medium   | S      | Docs       |
| 10 | Decide the `/version` HTTP-endpoint idea: TODO_LIST candidate or reject (boundary: arguably belongs in go-health, which owns health endpoints — see g3)                              | this repo | Medium   | S      | Decision   |
| 11 | Add dep-forced fixture to fix tests if CanonicalizeGoMod's gate lacks a dashboard-shaped case (dependency forcing a patch floor)                                                     | this repo | Medium   | S      | Quality    |
| 12 | Surface the supply-side remediation hint ("re-tag those modules…") in check output, not only fix output                                                                              | this repo | Medium   | S      | Feature/UX |
| 13 | Name the cross-repo policy split brain in the patch-form finding text (dep-forced floors are correct state, not violation — mirrors policy #2's go.work wording)                     | this repo | Medium   | S      | Feature/UX |
| 14 | After go-health re-tag: sweep all fleet consumers of go-health for re-poisoning; confirm each settles minor-form                                                                     | fleet     | Medium   | M      | Quality    |
| 15 | Consider a single fleet "poisoner registry" doc (x/text, encoding/json/v2, go-health v0.4.0-until-retag) — currently spread across AGENTS.md + ADR appendix                          | this repo | Low      | S      | Docs       |
| 16 | Document the throwaway-copy gate-verification recipe (safe way to test a poisoner consumer without mutating the repo)                                                                | this repo | Low      | S      | Docs       |
| 17 | Re-run this repo's full gate suite (`go test ./...` in devShell) at next session start — none run this session (no changes, but cheap confirmation)                                  | this repo | Low      | S      | Quality    |
| 18 | Severity review: is `go-directive-patch-form` warning-severity right for BuildFlow gates fleet-wide, given poisoned consumers will carry it indefinitely?                            | this repo | Low      | M      | Decision   |

_(18 real items; not padded to 50 — the rest of the 50-slot budget stays empty rather than inventing filler. HARVEST: items 4-7, 10-13, 15-18 route to TODO_LIST; 14 is fleet-coordination.)_

## g) Questions I cannot answer myself

1. **go-health re-tag version:** v0.4.1 (patch — floor-_lowering_ is consumer-compatible) or v0.5.0? And should the flake `go_1_27` pin fix (task f8) ride in the same release or land separately first? Your release-policy call; I did not check their CHANGELOG state for pending entries.
2. **fix exit semantics:** should a dep-forced-only run exit 0 ("tool did its job; state is externally forced") or 1 ("repo is poisoned; CI should signal")? Current behavior: 0. AGENTS.md wording ("exit 1 … poisoned") reads like 1. Which is the contract — behavior or doc?
3. **`/version` endpoint check: in-scope here or belongs in go-health?** Detecting an HTTP server and requiring a `/version` route serving `pkg/version.Version` is a runtime-contract concern; this tool's domain is static toolchain-surface files. If wanted at all, is it a new rule family here, or a go-health feature (it already owns health endpoints)?

---

_Point-in-time snapshot. Auto-commit daemon will pick up this report + the AGENTS.md poisoner note._

---

## Correction appendix (added 2026-09-25, post-planning research)

**Section a/6 and task f8 of this report were wrong:** go-health's flake does NOT pin `go_1_26` below its floor. The `nix-pin-below-floor flake.nix:41` finding was OUR false positive — line 41 is a Nix comment mentioning `go_1_26`; the real pin (line 43) is `go_1_27`. Fixed in v0.2.1 (`stripNixComments`), regression fixtures added, live re-check on go-health exits 0. The "bundle flake fix into the release" planning question was moot — nothing to fix there. Lesson folded into the plan's guardrails: verify a finding's line content before treating it as the consumer's defect.
