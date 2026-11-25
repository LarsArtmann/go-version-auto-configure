# TODO List

Short- and mid-term actionable work, **open items only** — completed items live in [CHANGELOG.md](CHANGELOG.md). Rebuilt by the docs-health audit of 2026-10-17: the v0.2.4-v0.2.6 releases plus the 2026-10-17 backlog pass closed most of the former T15 (who-forces honesty + wire goldens), T16 (CI depth, dogfood gates, e2e tidy gate, floor unification, test depth, flake FOD guard) and T17 (fleet docs) rows. Long-term and cross-repo material lives in [ROADMAP.md](ROADMAP.md).

## T14 — Supply-side re-tags still pending

Completed 2026-11-24 (the cluster wave, CHANGELOG [Unreleased] + ADR-0001 appendix): go-taskqueue 26-tag wave, cqrs-htmx 11-tag wave, go-cqrs-lite 23-tag second wave, go-codec v0.3.4, go-appkit v0.13.1; poisoner surface mapped via who-forces (go-codec deepest carrier). Still open below.

- [ ] **`go-daemon` re-tag (the last Active poisoner)** — strip master (drifted to `go 1.27.2`), re-tag v0.4.1 minor-form with bumped sibling pins (sseparse v0.2.1, go-codec v0.3.4 already minor-form), then bump nextd. Published-`.mod` evidence in [docs/POISONERS.md](docs/POISONERS.md).
- [ ] go-taskqueue master push — blocked by the repo's push-timing guard while a parallel session folds commits; tags are live and consumer-visible (the poison fix), master carries the policy re-strip.

## T1 — Fleet consumer campaign (remainder)

- [ ] Consumer sweep: bump the repos still pinning `go-output v0.38.2` (verified 2026-10-17: `index`, `projects-management-automation/pkg/domain`, `projects-management-automation/pma-daemon`), then re-run `check --quiet --expect-minor 1.27 ~/projects/*/` and refresh the ADR-0001 appendix counts.
- [ ] Patch-form directive campaign: 91 distinct top-level projects still carry a patch-form `go` directive (2026-10-17 re-measure, [ADR-0001 appendix](docs/adr/0001-fleet-go-minor.md)). A patch is an upper bound (some are dep-forced) — triage dep-forced vs. drifted with `who-forces`, then drive the drifted ones to minor form.
- [ ] Re-verify the live POISONERS.md Active rows with fresh `who-forces` runs (`x/text` and `encoding/json/v2` are accepted/permanent; `cqrs-htmx` and `go-taskqueue` are the live ones as of 2026-10-17).

## T16 — Tooling, CI & test depth (open remainder)

- [ ] Bump the go-nix-helpers pin (c42fd77, 65+ commits behind; auto-newest `goPkgAttr` plus the eval-time floor check makes the FOD-class nix failure impossible instead of merely guarded). The local `checks.go-fod-floor` guard (added 2026-10-17) covers the same class until then.
- [ ] license-check flake isolation: run it N times in isolation, capture what differs (cache vs network vs package set), then root-cause or add a reasoned skip. Lower urgency since BuildFlow auto-defers the failure class (2026-10-07).
- [ ] gomod-check ↔ go-mod-normalize disposition alignment (BuildFlow warning: this repo's go line flipped 20x in 20 commits — two steps fighting over the same directive).
- [ ] BuildFlow binary is stale (doctor, 2026-10-17: built at `7baef1c` but HEAD is `7108ef9` in `~/projects/BuildFlow`) — run `buildflow upgrade` before trusting doctor/build results. The former "9 tools unavailable" warnings are environment-scoped: a 2026-10-17 doctor outside the devShell failed 32 checks, all for absent `cargo-*`/`eslint`/`pytest`/`ruff`/`shellcheck`/`vulnix`/... binaries (none Go). Re-check `vulnix`'s gcc-10.4.0 CVE-2023-4039 advisory inside `nix develop -c buildflow doctor`.

## T5 — Upstream gomod-checker rule: "tidy revert" detection — WORTH CONSIDERING

- [ ] BuildFlow gomod-checker rule: go directive carrying a patch component after tidy (the poisoning signature) — closes the loop for repos that never run this tool
  - Rule spec sketch (2026-09-22): the rule fires when `go mod tidy` is a no-op AND some `go`/`toolchain` directive in the module graph carries a patch component that is NOT forced by a dependency floor — that is the accidental-minor signature. Fixtures: (a) x/text-forced `1.26.0` (legit, rule stays silent — the floor carrier is the dependency, named via `who-forces`), (b) json/v2-forced `1.27.1` std floor (legit, silent, message names the std floor), (c) `go 1.26.7` with max dep floor `go 1.26` (FIRE — this is the poisoning signature this tool strips). The rule must reuse `CompareDirective` semantics (bare minor ranks below zero patch) to avoid re-deriving them wrongly

## T6 — Release-authority drift (Layer-B versioning) — closed (both passes shipped)

- [ ] Optional second pass — git-tag comparison — **DONE 2026-11-24** (plan M12): shipped as `release-tag-drift` — `pkg/fix/release_tag.go` (`GitTagRunner`, `NewestReleaseTag`, `ReleaseAuthorityTagIssues`, silent on git failure) + pure `surface.ReleaseTagIssues`; comparison is `x/mod/semver`, NOT `go/version` (the stdlib package wants `go1.21.0`-form strings and rejects semver — plan correction, recorded in DOMAIN_LANGUAGE). Live-verified on project-dependency-graph (the exact planned case fires). Wired into the CLI `check` path; provider-side wiring deferred to the M7b adoption pass. T6 header: second pass closed.

## T9 — Parser coverage — flake.lock blocked by design

- [ ] flake.lock effective Go revision parsing — **blocked by design**: the lock records only a nixpkgs rev, not the Go version it packages; resolving it requires an impure `nix eval`, but `Discover` must stay pure (reads files, writes nothing)
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

- [ ] Runbook: literal inventory-table template row in the audit brief template + orchestrator semantic spot-check quota (5 random cited regions per report); the dialect-spec sentence waits on the owner's Q2 answer

## T18 — Fleet-run closeout (routed from the 2026-11-23 Pareto plan) — OPEN

Plan: docs/planning/2026-11-23_01-32_SUPERB-library-utilization-pareto-execution-plan.md (137 micro-tasks; owner-gated M21/M22 and worth-considering M24-M27 stay in the plan until picked).

- [ ] Semantic QC pass: re-read the ~10 highest-stakes Group B/C report citations (ConfigIssue field absence, ParseVersion strictness, README claims), record verdicts; normalize the three Group C reports' inventory format to the four-column contract or declare the compact form canonical (plan M17) — **done 2026-11-24** (verdicts in the fleet-run report's M17 appendix; compact format declared canonical)
- [ ] Sweep tooling v2: module-cache cite resolution (`~/go/pkg/mod/...@vX.Y.Z` paths, verifying against the PINNED version), basename-ambiguity report, `--summary` flag, and wiring into docs-gate over docs/review/libs (plan M18) — **done 2026-11-24** (all 22 fragile cites lib-qualified; baseline 360/0/0-fallbacks; `scripts/README.md`)
- [ ] T12 watch batch: check the four upstream issue states (branching-flow#1, dependabot-auto-configure#3, go-cqrs-lite#42, forbidigo recurrence); un-ignore/un-skip whatever closed; update the AGENTS.md known-tool-bugs section (plan M20) — **done 2026-11-24** (3 issues closed+resolved, cqrs-lint un-skipped with 0 findings, AGENTS updated)
