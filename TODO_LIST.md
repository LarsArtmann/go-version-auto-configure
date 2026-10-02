# TODO List

Short- and mid-term actionable work, **open items only** — completed items live in [CHANGELOG.md](CHANGELOG.md). Rebuilt by the docs-health audit of 2026-10-03 (all DONE sections deleted: T0, T2, T4, T11, T13 fully shipped; T1/T3/T12/T14 reduced to their open rows — see the annotated reports in [docs/status/](docs/status/)). Long-term and cross-repo material lives in [ROADMAP.md](ROADMAP.md).

## T14 — Supply-side re-tags still pending (largest fleet impact)

- [ ] Coordinated re-tag campaign for the remaining PENDING poisoners in [docs/POISONERS.md](docs/POISONERS.md): go-cqrs-lite v4 family (~27 modules, largest carrier set), go-health-dashboard v0.10.x (master already fixed 2026-09-25, needs the tag), go-etag, go-sse; map the cqrs-htmx poisoner surface (who-forces across its importers) before its re-tag
- [ ] BuildFlow: decide retiring the go-work-sync `GoWorkFloorFinding`/`RestoreGoWorkFloor` defenses + `DependsOn` ordering (their TODO S87 safety-net precondition is met — gvac's go-work-aware fixer is active) and repin BuildFlow to gvac **v0.2.3** in the same motion (still at v0.2.1/v0.2.2)

## T1 — Fleet consumer campaign (remainder)

- [ ] Consumer sweep: bump repos still requiring old tags (ADR-0001 appendix baseline 2026-09-22: 36/48 drifted; flagship go-health-dashboard resolved 2026-09-25). Now includes the **go-output → v0.38.3** consumer bump fleet-wide (2026-10-02 incident: repos pinning v0.38.2 keep re-poisoning until they bump). Re-run `check --quiet --expect-minor 1.27 ~/projects/*/` and update the ADR appendix counts
- [ ] Re-verify the remaining POISONERS.md Active rows with fresh `who-forces` runs (go-health-dashboard v0.10.x, cqrs-htmx/oauth2, go-etag, go-sse — 2026-10-02 asked "are those still accurate?")

## T15 — who-forces honesty + wire goldens (post-v0.2.3)

- [ ] `who-forces` at-parity carriers: when directive == max dep floor, name the carriers instead of answering "clean" — the exact question the 2026-10-02 session had to answer with raw `go list -m`
- [ ] Golden-lock the `depForced.cause` wire field: a dep-forced `fix --json` golden scenario in cmd/ (the wire goldens cover held-back/fix only; the additive field is unit-tested in pkg/fix but not golden-locked — schema 2 can drift silently). Include a clean-state `--json` snapshot
- [ ] Regression fixture: a synthetic consumer pinning go-output v0.38.2, asserting `fix` classifies dep-forced and names the carrier (validates the T14 classification work against the real incident)
- [ ] `who-forces` provenance: additive `source: "vendor"|"list"` field so vendor-annotation fallback rows are visible to machines (owner policy call open — ROADMAP Open questions)
- [ ] Vendor fallback covers only `## explicit` stanzas — indirect-only poisoners in a skewed vendor tree stay unnamed; parse the full stanza set or document the limitation in `--help`
- [ ] Better error message when `GOTOOLCHAIN=auto` needs a toolchain download in offline CI (today: raw go output)
- [ ] README: add a real `who-forces` output example block next to the dep-forced example (needs a poisoned fixture; this repo's own matrix reads clean)

## T16 — Tooling, CI & test depth

- [ ] CI depth: pin golangci-lint to the devShell's version (reproducible lint), add an erraudit step, add a concurrency group
- [ ] CI dogfood additions: `go mod tidy -diff` gate (catches floor drift before it ships — the 2026-09-30 sweep class), README floor-line check (assert README ≥ go.mod minor; the "Requires Go 1.26" lie survived 4 days), `version`-stamp assertion
- [ ] e2e test running the real `go mod tidy -diff` gate against a seeded fixture repo (today the gate is always faked in unit tests)
- [ ] Extend the `ParseDirective`-shared-verification invariant tests for the post-v0.38.3 clean state
- [ ] Re-run `buildflow update` deliberately to prove the sweep now settles at `go 1.27` (not a re-poisoning loop)
- [ ] Bump the go-nix-helpers pin (65+ commits behind; auto-newest `goPkgAttr` + eval-time floor check makes the FOD-class nix failure impossible instead of documented)
- [ ] Flake: add a `checks` entry asserting the FOD go version ≥ go.mod floor (belt & braces for the `goPkgAttr` class)
- [ ] Unify the dependency-floor triple model (`vendorModuleFloor` generalized to a shared `depFloor`) across who.go and floor.go — kills the last floor-parser split brain
- [ ] Re-run the benchmark suite vs the T11 baseline (2026-09-22) after the v0.2.3 hot-path refactor (`who.go` `accumulateFloor`/`finalizeFloors` reshaped the path; never re-measured) and re-measure per-package coverage against the 80% bar
- [ ] license-check flake isolation: run it N times in isolation, capture what differs (cache vs network vs package set); root-cause or add a reasoned skip
- [ ] gomod-check ↔ go-mod-normalize disposition alignment (BuildFlow warning: this repo's go line flipped 20× in 20 commits — two steps fighting over the same directive)
- [ ] Review the 9 "tools unavailable (health check failed)" BuildFlow doctor warnings; triage the vulnix gcc-10.4.0 CVE-2023-4039 advisory (warning-severity)
- [ ] art-dupl 56 findings: diff against the [docs/DEDUPLICATION.md](docs/DEDUPLICATION.md) baseline (gate passed; is the delta all intentional?)
- [ ] Test-depth backlog (small, batchable): flag-help property test vs old `--help` snapshot, `exitFrom*` outcome-combination table, `runSorted` zero-items + `--parallel 1`/`100` equivalence, `version` output golden through cmdguard, `BenchmarkApplyAll` with a temp git repo
- [ ] Standing docs gate (script or BuildFlow step): lychee link check + "no `[x]` in TODO_LIST" grep + stale-report harvest probe — the three checks the 2026-09-19 audit wanted mechanical

## T17 — Fleet docs (ADR / POISONERS / glossary)

- [ ] ADR-0001 evidence appendix: record the 2026-09-26 classification-fix session AND the go-output v0.38.2 regression incident (2026-09-30 → 2026-10-02, roles-inverted-for-10-days) next to the go-health incident
- [ ] POISONERS.md evidence convention: every "release X is clean" claim carries a proxy `.mod` URL + verification date; maintenance rule "no re-tag ships without published-`.mod` verification" (the rule that would have prevented the 10-day inversion)
- [ ] docs/DOMAIN_LANGUAGE.md: document the incident mechanism (MVS floor propagation + pin-in-tag) as vocabulary
- [ ] Check the sibling autoconfigurer repos for copies of the inverted "v0.38.2 is minor-form" claim (it lived in fleet docs for 10 days)

## T5 — Upstream gomod-checker rule: "tidy revert" detection — WORTH CONSIDERING

- [ ] BuildFlow gomod-checker rule: go directive carrying a patch component after tidy (the poisoning signature) — closes the loop for repos that never run this tool
  - Rule spec sketch (2026-09-22): the rule fires when `go mod tidy` is a no-op AND some `go`/`toolchain` directive in the module graph carries a patch component that is NOT forced by a dependency floor — that is the accidental-minor signature. Fixtures: (a) x/text-forced `1.26.0` (legit, rule stays silent — the floor carrier is the dependency, named via `who-forces`), (b) json/v2-forced `1.27.1` std floor (legit, silent, message names the std floor), (c) `go 1.26.7` with max dep floor `go 1.26` (FIRE — this is the poisoning signature this tool strips). The rule must reuse `CompareDirective` semantics (bare minor ranks below zero patch) to avoid re-deriving them wrongly

## T6 — Release-authority drift (Layer-B versioning) — WORTH CONSIDERING

- [ ] Extend `pkg/surface` (or project-dependency-graph) to detect VERSION file vs CHANGELOG top vs newest git tag drift (known case: project-dependency-graph VERSION=0.7.0, tags at v0.2.0)
  - Design sketch (2026-09-22): read-only comparison of three sources — `VERSION` file, top `## [x.y.z]` in CHANGELOG.md, newest `v*` git tag (via `go/version` on annotated tags). Drift matrix reported as a new finding kind (`release-authority-drift`, suggest-only — which one is authoritative is per-repo policy, same reasoning as pin alignment). Stays out of `Discover`'s pure file walk only if git access is required; a pure first pass (VERSION vs CHANGELOG) can live in Discover with the tag comparison as an optional second pass

## T9 — Parser coverage — PARTIALLY DONE (flake.lock blocked by design)

- [ ] flake.lock effective Go revision parsing — **blocked by design**: the lock records only a nixpkgs rev, not the Go version it packages; resolving it requires an impure `nix eval`, but `Discover` must stay pure (reads files, writes nothing). A separate opt-in command (or BuildFlow step) would be the right home — design sketch below. Decide the home (CLI command vs BuildFlow step) before implementing
  - Design sketch (2026-09-22): new command `gvac nix-pin [root ...]` (NOT part of `Discover`): for each flake.lock, `nix eval <locked nixpkgs rev>.go.version` (impure, cached by rev), compare against the flake's `goPkgAttr`/module floor, and report the same alignment findings as the pure path. BuildFlow home: a `nix-go-pin-check` step that can afford impurity and network. Exit contract identical to `check`; `--json` reuses the schema envelope with `source: "nix-pin"`

## T12 — Lint & environment debt (open watch items)

- [ ] The remaining branching-flow INDEX_OUT_OF_RANGE warnings on the worker-pool `results[i] = …` pattern are provably safe (index bounded by the range) — root cause filed as branching-flow#1; un-nolint when it closes
- [ ] dependabot-auto-configure 2 findings remain (documented false positive, AGENTS.md known-tool-bugs; upstream dependabot-auto-configure#3) — un-ignore when it closes
- [ ] forbidigo vanishing (9 `fmt.Print*` hits gone after `buildflow format`) not reproducible in the 2026-09-22 run — watch for recurrence
- [ ] When go-cqrs-lite#42 closes (cqrs-lint none-import guard), un-skip `cqrs-lint` in .buildflow.yml

## T3 — Publish (remainder)

- [ ] Website launch (sibling-project pattern) if it earns one
