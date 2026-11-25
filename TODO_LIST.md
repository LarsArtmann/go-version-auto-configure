# TODO List

Short- and mid-term actionable work, **open items only** — completed items live in [CHANGELOG.md](CHANGELOG.md). Rebuilt by the docs-health audit of 2026-10-17: the v0.2.4-v0.2.6 releases plus the 2026-10-17 backlog pass closed most of the former T15 (who-forces honesty + wire goldens), T16 (CI depth, dogfood gates, e2e tidy gate, floor unification, test depth, flake FOD guard) and T17 (fleet docs) rows. Long-term and cross-repo material lives in [ROADMAP.md](ROADMAP.md).

## T14 — Supply-side re-tags still pending

Completed 2026-11-24 (the cluster wave, CHANGELOG [Unreleased] + ADR-0001 appendix): go-taskqueue 26-tag wave, cqrs-htmx 11-tag wave, go-cqrs-lite 23-tag second wave, go-codec v0.3.4, go-appkit v0.13.1; poisoner surface mapped via who-forces (go-codec deepest carrier). Still open below.

- [ ] **`go-daemon` re-tag (the last Active poisoner)** — strip master (drifted to `go 1.27.2`), re-tag v0.4.1 minor-form with bumped sibling pins (sseparse v0.2.1, go-codec v0.3.4 already minor-form), then bump nextd. Published-`.mod` evidence in [docs/POISONERS.md](docs/POISONERS.md).
- [ ] go-taskqueue master push — blocked by the repo's push-timing guard while a parallel session folds commits; tags are live and consumer-visible (the poison fix), master carries the policy re-strip.

## T1 — Fleet consumer campaign (remainder)

- [ ] Consumer sweep: bump the repos still pinning `go-output v0.38.2` (verified 2026-10-17: `index`, `projects-management-automation/pkg/domain`, `projects-management-automation/pma-daemon`), then re-run `check --quiet --expect-minor 1.27 ~/projects/*/` and refresh the ADR-0001 appendix counts.
- [ ] Patch-form directive campaign — REFRAMED + PILOT PROVEN 2026-11-25 (plan M9): the 2026-11-24 re-measure grew the count 91→94→**108** (2026-11-25 sweep: 327 repos, 134 dirty) — masters re-lift mechanically, so per [ADR-0001](docs/adr/0001-fleet-go-minor.md) the campaign shape is consumer-side: bump larsartmann deps to the post-wave minor-form tags, then `fix` strips, and the strip HOLDS (tidy under `GOTOOLCHAIN=auto` no longer re-lifts — pilot proven end-to-end on DiscordSync, pushed 53bd8a21; the recipe + the one pre-existing red test are recorded in the ADR). Remaining ~107 repos run the same recipe, each gated on its parallel-session lease check.
- [ ] Re-verify the live POISONERS.md Active rows — **DONE 2026-11-24** (plan M10): all five wave rows re-verified with fresh proxy evidence; Active trimmed to x/text + json/v2 + go-daemon (+ the 2026-11-25 go-compliance/hub candidate from M27); cqrs-htmx/go-taskqueue/go-cqrs-lite/go-codec/go-appkit moved to Resolved.

## T16 — Tooling, CI & test depth (open remainder)

- [ ] Bump the go-nix-helpers pin — **DONE 2026-11-25** (plan M14): flake input bumped c42fd77/1a553ce → e0c4b70 (2026-11-19; auto-newest `goPkgAttr` default + templ oracle fix); FOD rebuilt clean, `nix flake check` green including `checks.go-fod-floor` (the explicit `go_1_27` pin stays for determinism — flake comment updated).
- [ ] license-check flake isolation — **DONE 2026-11-25** (plan M15): harness shipped as `scripts/license-check-isolation.sh` (N runs, per-run exit/timing, failure-package capture); 5/5 clean — the intermittent std-package class did not reproduce and no environmental factor surfaced; the BuildFlow auto-defer (2026-10-07) stays the durable answer (AGENTS known-tool-bugs updated with the re-run protocol).
- [ ] gomod-check ↔ go-mod-normalize disposition alignment — **RESOLVED BY EVIDENCE 2026-11-25** (plan M16): the last 20 go.mod commits are stable at `go 1.27` (19/20; the one `1.27.1` excursion was a parallel session's mid-wave raise, superseded by the documented ADR-0001 re-strip). The 20x-flip era ended with the `go-mod-update` skip (2026-09-18, rationale in .buildflow.yml) plus the fleet settling at minor-form (2026-11-24 wave): with no patch-form floors left in the graph, the two dispositions no longer disagree. No BuildFlow-side change needed; if go-mod-update is ever re-enabled for this repo, re-run the flip probe first (last-20-commits go-line walk).
- [ ] vulnix gcc CVE-2023-4039 advisory triage (WP-14, routed 2026-11-25 from the 2026-10-03/10-17 reports): re-check inside `nix develop -c buildflow doctor` after the next vulnix/gcc update; the advisory was doctor-warning-severity, not a build breaker.
- [ ] coverage-floor CI gate: fail the pipeline if any Go package drops below the 80% coverage bar (routed 2026-11-25 from the 2026-10-17 report; T11's accepted sub-80% exceptions live in AGENTS.md's coverage baseline).
- [ ] BuildFlow binary — **DONE 2026-11-25** (plan M6 + M20): rebuilt from checkout (ff4aaa4, vendorHash fixed via `nix-hash-fix --fix`), reinstalled into the nix profile; the doctor stale-binary warning cleared (residual advisories are the non-Go tools, environment-scoped as recorded here).

## T5 — Upstream gomod-checker rule: "tidy revert" detection — closed (shipped in BuildFlow)

- [ ] BuildFlow gomod-checker rule — **DONE 2026-11-25** (plan M11): shipped as BuildFlow `modules/gomod-checker` `patch-form-directive` (commit on BuildFlow master; BuildFlow#audit-gates green). Fires exactly per the sketch: directive carries a patch AND no listed dependency floor ranks at/above it (CompareDirective semantics ported: bare minor ranks BELOW patch-zero) AND a non-mutating rewrite probe (`go mod edit` to minor + `go mod tidy -diff` + byte-exact restore via atomicwrite) shows tidy does NOT re-raise. Fixture classes a (x/text listed carrier: silent), b (unlisted std-floor carrier, tidy re-raises: silent), c (accidental patch, tidy clean: FIRE) all pinned; probe seams are package vars, hermetic. Closes the loop for repos that never run gvac: BuildFlow now names the same poisoning signature.

## T6 — Release-authority drift (Layer-B versioning) — closed (both passes shipped)

- [ ] Optional second pass — git-tag comparison — **DONE 2026-11-24** (plan M12): shipped as `release-tag-drift` — `pkg/fix/release_tag.go` (`GitTagRunner`, `NewestReleaseTag`, `ReleaseAuthorityTagIssues`, silent on git failure) + pure `surface.ReleaseTagIssues`; comparison is `x/mod/semver`, NOT `go/version` (the stdlib package wants `go1.21.0`-form strings and rejects semver — plan correction, recorded in DOMAIN_LANGUAGE). Live-verified on project-dependency-graph (the exact planned case fires). Wired into the CLI `check` path; provider-side wiring deferred to the M7b adoption pass. T6 header: second pass closed.

## T9 — Parser coverage — flake.lock blocked by design

- [ ] flake.lock effective Go revision parsing — **SHIPPED 2026-11-25** (plan M13, the 2026-10-03 decision executed): BuildFlow step `nix-go-pin-check` (tools/providers/nix_go_pin_check.go, registered) resolves the lock's nixpkgs rev's packaged Go via `nix eval --raw ...#go.version` (impure, cached per rev — revs are content-addressed so the cache never invalidates), compares against the flake's goPkgAttr pin, and fires `nix-lock-pin-trails` (warning-severity, message names `source: nix-pin`) when the pin TRAILS the locked nixpkgs. Missing input / eval failure / no pin degrade to no findings. Fake-eval fixtures: trail/match/ahead/eval-failure/no-input. gvac stays pure; the T9 decision's contract is honored end-to-end.
  - **Decision 2026-10-03: the home is a BuildFlow step (`nix-go-pin-check`), not a gvac command.** Rationale: the check is impure by nature (network + `nix eval` against a remote rev); BuildFlow already owns impure steps and the rev→Go-version cache naturally belongs beside its other caches; a gvac CLI command would bolt a second impurity boundary onto a tool whose contract is pure discovery. gvac stays pure; the step consumes the pure surface (goPkgAttr + module floor from `check --json`) and adds the impure half. Findings isomorphic to `RuleNixPinBelowFloor`, `--json` envelope with `source: "nix-pin"`; tests against a fake `nix eval`. Track in BuildFlow's TODO, linked from here.

## T12 — Lint & environment debt (open watch items)

- [ ] forbidigo vanishing (9 `fmt.Print*` hits gone after `buildflow format`) not reproducible in the 2026-09-22 run — watch for recurrence (no recurrence observed through 2026-11-24 full BuildFlow runs)

_(branching-flow#1, dependabot-auto-configure#3, go-cqrs-lite#42 all closed and resolved 2026-11-24 — see AGENTS.md known-tool-bugs and CHANGELOG; M20, plan T12 watch batch.)_

## T3 — Publish (remainder)

- [ ] Website launch — **decision 2026-10-03: demand-gated non-commitment.** Revisit only on a real demand signal (external user question, adoption spike, owner request). If triggered: sibling-project Astro+Starlight pattern per the website-launch skill, demo video as landing centerpiece. Do not spend a session on this otherwise.

## T17 — Library-utilization review outcomes (fleet run 2026-11-23) — OPEN

Reports: docs/review/libs/*.md + index.html (write-once snapshot; re-run the library-deep-dive fleet runbook to re-review). Citation gate: `python3 scripts/citation-sweep.py`.

- [ ] Push upstream (linter-autoconfigure-sdk, unblocks every auto-configurer) — **DONE 2026-11-25** (plan M7): shipped in sdk v0.9.0 — `ConfigIssue.Metadata`/`Category`/`Tags` (all zero-value-safe), `ProviderSpec.RepairStructured` (full `toolsdk.RepairResult`: `ModifiedFiles`/`Facts`), and `ProviderSpec.Coverage` (DetectV2 evidence); tagged + proxy-verified `go 1.27` minor-form
- [ ] Adopt `Finding.Metadata` for dep-forced carriers/floors once the bridge (T17 upstream row) passes it through — **DONE 2026-11-25** (plan M7b): repinned to sdk v0.9.0; `toConfigIssue` wires the M26 vocabulary (`vocabularyFor`, Category+Tags) and stamps `dep-forced.floor`/`dep-forced.carriers`/`dep-forced.remediation` metadata on held strips; repair migrated to `RepairStructured` (ModifiedFiles + Facts: applied/dep-forced/held-back/failed/dry-run counts); `spec.Coverage` feeds DetectV2 with the same measurement the CLI `coverage` JSON exposes. Schema-2 CLI wire contract unchanged (goldens green)

## T19 — Fleet-run runbook hardening (SKILLS repo, owner-gated on the scoring-dialect answer) — OPEN

Plan M19 (docs/planning/2026-11-23_01-32_SUPERB-library-utilization-pareto-execution-plan.md); routed from the 2026-11-23 fleet-run report's §f items 17+19 (annotated routed 2026-11-25).

- [ ] Runbook: literal inventory-table template row in the audit brief template + orchestrator semantic spot-check quota (5 random cited regions per report) — **DONE 2026-11-25** (plan M19): shipped as SKILLS library-deep-dive runbook items 14–17 (ask-on-first-failure, spot-check quota, format canonicity, render preflight) + the literal inventory-table row in the audit brief template. The dialect-spec sentence (19.1) stays owner-gated on Q2.

## T18 — Fleet-run closeout (routed from the 2026-11-23 Pareto plan) — OPEN

Plan: docs/planning/2026-11-23_01-32_SUPERB-library-utilization-pareto-execution-plan.md (137 micro-tasks; owner-gated M21/M22 and worth-considering M24-M27 stay in the plan until picked).

- [ ] Semantic QC pass: re-read the ~10 highest-stakes Group B/C report citations (ConfigIssue field absence, ParseVersion strictness, README claims), record verdicts; normalize the three Group C reports' inventory format to the four-column contract or declare the compact form canonical (plan M17) — **done 2026-11-24** (verdicts in the fleet-run report's M17 appendix; compact format declared canonical)
- [ ] Sweep tooling v2: module-cache cite resolution (`~/go/pkg/mod/...@vX.Y.Z` paths, verifying against the PINNED version), basename-ambiguity report, `--summary` flag, and wiring into docs-gate over docs/review/libs (plan M18) — **done 2026-11-24** (all 22 fragile cites lib-qualified; baseline 360/0/0-fallbacks; `scripts/README.md`)
- [ ] T12 watch batch: check the four upstream issue states (branching-flow#1, dependabot-auto-configure#3, go-cqrs-lite#42, forbidigo recurrence); un-ignore/un-skip whatever closed; update the AGENTS.md known-tool-bugs section (plan M20) — **done 2026-11-24** (3 issues closed+resolved, cqrs-lint un-skipped with 0 findings, AGENTS updated)
- [ ] Worth-considering pack (plan M24–M27) — **done 2026-11-25**: M24 `check --sarif` (wire golden + round-trip + exit-code table); M25 coverage evidence + FindingError boundary (+ deliberate CheckBinary non-adoption, pkg/fix stays finding-free); M26 vocabulary mapping (`vocabularyFor`, severity policy unchanged, owner may still flip); M27 watch rows (sdk audit root-resident through v0.8.1; go-compliance/hub `go 1.27.2` recorded as CANDIDATE poisoner — repo lease-held by a parallel session at discovery; go-linter-sdk six-importer convergence confirmed, no split brain)
- [ ] BuildFlow-side worth-considering (plan M11/M13) — **done 2026-11-25**: M11 gomod-checker `patch-form-directive` rule (T5 sketch fixtures, CompareDirective semantics ported, non-mutating revert probe via atomicwrite) + M13 `nix-go-pin-check` step (T9's 2026-10-03 decision executed) — see T5/T9 rows
- [ ] M21 index.html screenshot + vision verification — **PARKED on owner Q1** (2026-11-25): firefox is yolo-mode-blocked, no headless chromium in PATH; the permission ask is now runbook doctrine (SKILLS library-deep-dive item 14/17). Un-park on the owner's firefox-safe or chromium-in-devShell answer
- [ ] M22 CHANGELOG convention — **RESOLVED BY PRACTICE 2026-11-25** (owner Q3): docs/review runs + shipped tooling DO get `[Unreleased]` entries in this repo (the M17/M18/M20/M24/M25/M7 bullets set the precedent); strictly-binary claims stay in the Changed section. Owner can veto by reverting the bullets
