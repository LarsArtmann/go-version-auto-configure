# Pareto Execution Window 3 — M11–M27 + closeout shipped (2026-11-25)

**Window:** 2026-11-24 ~23:00 → 2026-11-25 ~03:10 CET · **Continues:** [2026-11-24_21-50 (windows 1–2, M1–M10)](2026-11-24_21-50_pareto-window2-cluster-wave-m1-m10-shipped.md) · **Plan:** [2026-11-23 pareto plan](../planning/2026-11-23_01-32_SUPERB-library-utilization-pareto-execution-plan.md) (M1–M27 + 137 micro-tasks)

## a) What shipped this window

1. **M4 closeout:** go-taskqueue master pushed (`645e870c..af5b6ec5`, 0 refused after the parallel sessions quiesced); the stash already superseded by the foreign session's `648feecc` (verified mcpserver coverage, nothing dropped).
2. **M17 run closeout:** 10 highest-stakes Group B/C cites re-read at source, all verdicts hold, zero errata (one prose nuance recorded); compact grade format **declared canonical** for NOT-ADOPTED reviews (Format rows in all three Group C reports + index legend). Appendix in the [fleet-run report](2026-11-23_01-28_library-utilization-fleet-run-shipped-self-review.md).
3. **M18 sweep v2:** `citation-sweep.py` v2 — module-cache (`@vX.Y.Z`) cites verify against the PINNED version (including submodule-cache paths `go-finding/toolsdk@v1.16.0`), `--summary`/`--json`, `--self-test` (10/10), wired as docs-gate check 4; all 22 basename-fragile cites lib-qualified (baseline **360/0/0-fallbacks**); `scripts/README.md`.
4. **M20 T12 batch:** branching-flow#1, dependabot-ac#3, go-cqrs-lite#42 all CLOSED; fixes verified live (worker-pool site no longer flags; cqrs-lint un-skipped → 0 findings); AGENTS known-tool-bugs updated; `cqrs-lint` removed from skip_steps.
5. **M7+M7b (the fleet unblock):** linter-autoconfigure-sdk **v0.9.0** tagged + proxy-verified (`go 1.27` minor-form): `ConfigIssue.Metadata/Category/Tags`, `ProviderSpec.RepairStructured` (ModifiedFiles/Facts), `ProviderSpec.Coverage` (DetectV2). Adopted here: vocabulary wiring (`vocabularyFor`), `dep-forced.*` metadata on held strips, structured repair facts, DetectV2 coverage. Schema-2 wire contract unchanged (goldens green).
6. **M12 (T6 closed):** `release-tag-drift` second pass shipped (`pkg/fix/release_tag.go` + pure `surface.ReleaseTagIssues`, `x/mod/semver` NOT `go/version` — plan correction recorded in DOMAIN_LANGUAGE). Live-verified on project-dependency-graph (the exact planned case fires).
7. **M24:** `check --sarif` — SARIF 2.1.0 export through the shared bridge, run propertyBag stamping (scanRoots/schemaVersion/failedRoots), absolute locations, golden + reader round-trip + exit-code tests, README section.
8. **M25:** coverage evidence in `check --json` (additive `coverage.filesScanned`/`skippedModules`, schema stays 2) + provider boundary migrated to the typed `FindingError` family; `fix.SelfCheck` keeps its own PATH lookup **deliberately** (pkg/fix stays finding-free).
9. **M26:** rule→Category/Tag vocabulary (`pkg/provider/vocabulary.go`, table-tested); severity policy unchanged (advisories stay warning — owner may flip).
10. **M27:** sdk audit/ is root-resident (tagged through v0.8.1, nothing to wait for); **go-compliance/hub `go 1.27.2` recorded as CANDIDATE poisoner** (repo lease-held by a parallel session at discovery); go-linter-sdk convergence confirmed (6 fleet importers).
11. **M14:** go-nix-helpers pin c42fd77 → e0c4b70; FOD rebuilt, `nix flake check` green incl. `checks.go-fod-floor`.
12. **M15:** `scripts/license-check-isolation.sh` harness; 5/5 clean — the intermittent class did not reproduce; BuildFlow deferral stays.
13. **M16:** resolved by evidence — last 20 go.mod commits stable at `go 1.27`.
14. **M11 (BuildFlow):** gomod-checker `patch-form-directive` rule (T5 fixtures a/b/c, CompareDirective semantics ported, non-mutating revert probe via atomicwrite; BuildFlow's audit gates green).
15. **M13 (BuildFlow):** `nix-go-pin-check` step — the T9 2026-10-03 decision executed (lock rev → cached `nix eval` → `nix-lock-pin-trails` finding, fake-eval fixtures).
16. **M9 (reframed):** fleet re-measure 327 swept / **108 patch-form projects** (94→108, treadmill confirmed); **pilot proven end-to-end on DiscordSync** (bump-to-wave-tags → strip → auto-tidy stays minor → push `53bd8a21`); recipe + remaining ~107 repos documented in ADR-0001.
17. **M19:** SKILLS library-deep-dive runbook items 14–17 + literal inventory-table template row (19.1 dialect sentence stays owner-gated).
18. **M22:** CHANGELOG convention resolved by practice (docs/tooling bullets in `[Unreleased]`; owner can veto).
19. **Infrastructure:** BuildFlow rebuilt ff4aaa4 + reinstalled; this repo's docs-gates battery driven to **ALL GREEN** (444 malformed strikes repaired, 5 F/spacing fixed, 9 stale reports archived with Live-remainder banners, link-depth repairs); cue-policy `permissions:`/`concurrency:` added to ci.yml/release.yml.

## b) Verification state (end of window)

`go test ./...` green · citation sweep **360/0** + self-test 10/10 · `docs-gates run .` **ALL PASS** · `nix run .#docs-gate` exit 0 · `nix flake check` exit 0 · BuildFlow gomod-checker + audit suites green · sdk suite green.

## c) Still open / parked

- **M21** — parked on owner Q1 (firefox yolo-blocked; runbook items 14/17 carry the ask).
- **M9 campaign body** — ~107 repos run the proven recipe, each gated on its lease check.
- **M19.1** — dialect-spec sentence, owner Q2.
- **go-compliance/hub** candidate row — remediation once its repo quiesces.
- BuildFlow pre-commit hook currently blocked by the parallel session's in-flight docs-gates state (their lease; my staged `tools/go.mod` sync rides the daemon sweep).

## d) Questions for the owner (unchanged + one new)

1. Floor-policy collision ruling (76a05058 lockstep raise vs ADR minor-form re-strip) — status report of window 2, question 1.
2. Parallel-session protocol for cross-repo waves — window 2, question 2.
3. M21 rendering permission (firefox safe / chromium in devShell) — window 2, question 3 (now runbook item 14).
4. Severity policy flip for advisories (M26 kept warning; flipping changes BuildFlow gate behavior fleet-wide).
