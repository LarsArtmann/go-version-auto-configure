# TODO List

Short- and mid-term actionable work, **open items only** — completed items live in [CHANGELOG.md](CHANGELOG.md). Rebuilt by the docs-health audit of 2026-10-17: the v0.2.4-v0.2.6 releases plus the 2026-10-17 backlog pass closed most of the former T15 (who-forces honesty + wire goldens), T16 (CI depth, dogfood gates, e2e tidy gate, floor unification, test depth, flake FOD guard) and T17 (fleet docs) rows. Long-term and cross-repo material lives in [ROADMAP.md](ROADMAP.md).

## T14 — Supply-side re-tags still pending

- [ ] **`go-taskqueue` (NEW, found 2026-10-17)** — strip master and re-tag every module minor-form, dependency-ordered. Every module (root, `internal/journal`, `internal/executor`, `cmd/tq`, ...) is `go 1.27.1` at source AND in published tags, so it self-poisons: `who-forces` names only its own published tags as carriers. Published-`.mod` evidence in [docs/POISONERS.md](docs/POISONERS.md); never ran the 2026-09-22 campaign.
- [ ] **`cqrs-htmx` re-tag wave (no longer "in flight")** — the 2026-10-03 wave landed minor-form (`cqrs-htmx/v4@v4.13.3` = `go 1.27`), then **REGRESSED** at v4.14.0/v4.14.1 (2026-10-12) to `go 1.27.2`; the local tree is mixed (7x `1.26.7`, 8x `1.27`, 21x `1.27.2`). Strip master, re-cut minor-form tags dependency-ordered, and find why v4.14.0 re-lifted (a `1.27.2` dependency or a local toolchain patch floor) so the regression does not repeat.
- [ ] Map the cqrs-htmx poisoner surface (`who-forces` across its importers) before its re-tag — carried over from the original T14 row.

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

## T6 — Release-authority drift (Layer-B versioning) — second pass open

- [ ] Optional second pass — git-tag comparison: newest `v*` tag as the third source (the first pass shipped in v0.2.5: VERSION vs. top CHANGELOG section). Needs git access, which `Discover`'s pure file walk deliberately lacks, so it belongs in a caller that already shells out (pkg/fix runs go commands; or a BuildFlow step beside `nix-go-pin-check`). Semver comparison via `go/version`; finding shape mirrors the first pass (suggest-only, names all three sources). Known live case: project-dependency-graph VERSION=0.7.0 = CHANGELOG 0.7.0, tags top out at v0.2.0 — the first pass cannot see it, only the second pass can

## T9 — Parser coverage — flake.lock blocked by design

- [ ] flake.lock effective Go revision parsing — **blocked by design**: the lock records only a nixpkgs rev, not the Go version it packages; resolving it requires an impure `nix eval`, but `Discover` must stay pure (reads files, writes nothing)
  - **Decision 2026-10-03: the home is a BuildFlow step (`nix-go-pin-check`), not a gvac command.** Rationale: the check is impure by nature (network + `nix eval` against a remote rev); BuildFlow already owns impure steps and the rev→Go-version cache naturally belongs beside its other caches; a gvac CLI command would bolt a second impurity boundary onto a tool whose contract is pure discovery. gvac stays pure; the step consumes the pure surface (goPkgAttr + module floor from `check --json`) and adds the impure half. Findings isomorphic to `RuleNixPinBelowFloor`, `--json` envelope with `source: "nix-pin"`; tests against a fake `nix eval`. Track in BuildFlow's TODO, linked from here.

## T12 — Lint & environment debt (open watch items)

- [ ] The remaining branching-flow INDEX_OUT_OF_RANGE warnings on the worker-pool `results[i] = ...` pattern are provably safe (index bounded by the range) — root cause filed as branching-flow#1; un-nolint when it closes
- [ ] dependabot-auto-configure 2 findings remain (documented false positive, AGENTS.md known-tool-bugs; upstream dependabot-auto-configure#3) — un-ignore when it closes
- [ ] forbidigo vanishing (9 `fmt.Print*` hits gone after `buildflow format`) not reproducible in the 2026-09-22 run — watch for recurrence
- [ ] When go-cqrs-lite#42 closes (cqrs-lint none-import guard), un-skip `cqrs-lint` in .buildflow.yml

## T3 — Publish (remainder)

- [ ] Website launch — **decision 2026-10-03: demand-gated non-commitment.** Revisit only on a real demand signal (external user question, adoption spike, owner request). If triggered: sibling-project Astro+Starlight pattern per the website-launch skill, demo video as landing centerpiece. Do not spend a session on this otherwise.
