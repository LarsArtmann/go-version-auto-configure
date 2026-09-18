# Status Report — Fleet Versioning Unification (go-version-auto-configure)

**Date:** 2026-09-18 15:13 CEST
**Session scope:** Find all Go projects, audit how versioning works, unify it into a fully automated auto-fixing mechanism via `linter-autoconfigure-sdk` + a mini SDK.
**Verdict:** Mechanism SHIPPED and dogfooded; root cause of fleet drift PROVEN; fleet convergence BLOCKED on one supply-side release campaign and one owner decision.

---

## a) FULLY DONE

| # | Work | Evidence |
|---|------|----------|
| 1 | **Fleet versioning audit** — 383 `go.mod` modules across ~260 repos enumerated; drift distributions measured per surface location | Numbers in this report; reproducible via `/tmp/gvac check <repo>` per repo |
| 2 | **Root cause proven: floor poisoning.** `go mod tidy`/get-style passes copy a dependency's `go` floor verbatim. Published `go-finding@v1.10.0`/`v1.12.0` = `go 1.26.7`, `linter-autoconfigure-sdk@v0.2.0` = `go 1.26.7`, `go-atomic-write@v0.5.x` = `go 1.27.1` (accidental: its deps' floors are xxhash 1.11 / flock 1.25.0) | Pilot reproduction: golangci-lint-auto-configure + linter-autoconfigure-sdk directives reverted to 1.26.7 after tidy; `go list -m` poisoner output |
| 3 | **New repo `~/projects/go-version-auto-configure`** — single-module Go CLI + BuildFlow provider; 4 packages, all tests green; `go vet` clean; golangci-lint **0 issues**; BuildFlow fast gate **exit 0** | `GOEXPERIMENT=jsonv2 go test ./...` → 4 ok; `BUILDFLOW_NO_RESULT_CACHE=1 buildflow --build-mode fast` → exit 0 |
| 4 | **`pkg/surface` mini SDK** — `Discover` (walks go.mod/go.work/flake.nix/.github/workflows; skips vendor/node_modules/.git/result), `Analyze` (5 policy rules), `ParseDirective` single parsing entry point shared by detection AND fix verification | Table-driven tests incl. real-fleet-shape regression (go-finding workspace case) |
| 5 | **`pkg/fix`** — directive fixes via `go mod edit`/`go work edit` (never sed); GOWORK=off only for `mod` (its misuse broke `go work edit`); go.work target = max(stripped, workspace floor); every fix verified by re-parse AND by surviving `go mod tidy`; `DepForcedError` names poisoning dependencies via `go list -m` | fix_test.go: verification-catches-no-op, dep-forced naming, workspace-floor, subdirectory modules |
| 6 | **`pkg/provider`** — `linter-autoconfigure-sdk.ProviderFromSpec` → `toolsdk.Register`, package-level `var Provider` blank-import contract, honors BuildFlow dry-run context, HealthCheck verifies go binary | provider_test.go: registration, detect, repair, dry-run-leaves-file |
| 7 | **CLI** `check` (exit 1 on drift) / `fix --dry-run` / `version` + e2e tests | main_test.go; manual runs against linter-autoconfigure-sdk, go-finding, own repo |
| 8 | **Pilot fixes in 7 real repos** — linter-autoconfigure-sdk, go-finding (4 modules + go.work), go-atomic-write, go-error-family (6 fixes, build+test **green**), oxlint-auto-configure, golangci-lint-auto-configure, dependabot-auto-configure | tool output captured in session; go-error-family: build=0 test=0 |
| 9 | **Fleet-wide dry-run report** — 260 repos scanned: **700 findings in 152 repos** (462 go-directive-patch-form, 126 ci-pin-below-floor, 38 ci-pin-patch-form, 37 nix-pin-below-floor, 37 go-work-patch-form); top: go-cqrs-lite 88, project-discovery-sdk 36, cqrs-htmx 35 | /tmp/fleet_report.txt, /tmp/fleet_rules.txt (persisted in /tmp — see e-8) |
| 10 | **Full docs + scaffolding** — README (sales), AGENTS.md (policy + poisoning section), FEATURES, TODO_LIST (T1–T6), CHANGELOG, LICENSE, .gitignore, .github/dependabot.yml, .golangci.yml (via golangci-lint-auto-configure + gosec G304/G204 exclusions with rationale), .buildflow.yml (go-mod-update + go-structure-linter skips with rationale) | files in repo |
| 11 | **Dogfooding loop** — tool detects its own repo's drift and names its own dependencies as poisoners | `/tmp/gvac fix .` → "dep-forced: floor go 1.26.7 forced by: go-finding, go-finding/toolsdk, linter-autoconfigure-sdk" |

## b) PARTIALLY DONE

1. **Pilot convergence (7 repos)** — form-fixed, but daemon/get-style passes re-raise floors. Works: fixes apply and verify. Open: permanent stickiness. Blocker: T1 supply-side re-tags. Effort to finish: L (campaign).
2. **go-finding state** — 5 directives normalized; still unbuildable in this shell (pre-existing `go 1.27` floor vs `GOTOOLCHAIN=local` go1.26.7 — broken BEFORE my edits, not a regression). 14 Nix/CI alignment suggestions open (flake go_1_26 + 13 CI pins vs 1.27 floor). Blocker: T4 decision. Effort: S after decision.
3. **BuildFlow integration (T2)** — provider self-registers; the one-line blank import in BuildFlow's SDK import set is NOT added; DAG position undecided. Effort: S.
4. **Verification depth** — fast-mode gate green; full mode (race, coverage) not run on the new repo. Effort: S.
5. **My repo's own go.mod** — accepted oscillation 1.26↔1.26.7 (documented in AGENTS.md); not permanently clean until T1 lands.

## c) NOT STARTED

- **T1 supply-side re-tag campaign** (the BLOCKING item: go-atomic-write, go-finding, SDK, remaining go-* libs) — waiting on owner go/no-go (publishing = irreversible proxy writes)
- **Fleet-wide fix sweep** (152 repos with findings) — deliberately deferred until after T1 (one pass instead of two)
- **T2** BuildFlow blank import + provider catalog/docs entry
- **T3** tag v0.1.0, GitHub CI workflows, GoReleaser, pkg.go.dev verification for the new repo
- **T4** fleet minor decision (1.26 vs 1.27) + its execution
- **T5** upstream gomod-checker rule for the tidy-revert signature
- **T6** release-authority drift detection (VERSION vs CHANGELOG vs tags; known case: project-dependency-graph)
- JSON output; `--expect-minor` enforcement; Dockerfile/.tool-versions/mise pin coverage; flake.lock parsing; go.work `toolchain` directive handling
- .editorconfig/.gitattributes; website launch; HARVEST of this report's section (f) into TODO_LIST/ROADMAP
- Skill/doc updates: floor-poisoning section for go-ecosystem-upgrade's version-surface.md

## d) TOTALLY FUCKED UP

1. **My first fixer version LIED.** It reported "applied 1" for fixes that `go mod tidy` silently reverted (directive back to 1.26.7). A false-green auto-fixer is the worst failure class for this product. Caught during self-verification on my own repo. Fixed: tidy-stability check + `go list -m` poisoner naming + regression test. **Residual risk:** repos where the daemon re-raises AFTER my tool runs still end up poisoned — the tool reports truth at run time, cannot prevent later writes.
2. **My tool briefly broke go-finding's workspace.** The go.work form fix originally targeted the stripped directive (1.26) while a module needed 1.27 → "module . listed in go.work requires go >= 1.27, but go.work lists go 1.26.7"-class breakage shipped BY the fixer mid-pilot. Fixed: target = max(stripped, workspace floor) + regression test. Lesson: verified-by-reparse was not enough; semantics needed modeling.
3. **`GOWORK=off` broke `go work edit`** ("no go.work file found") — my own workspace-protection guard disabled the workspace command. Fixed via arg dispatch (`mod` only).
4. **Pipeline masking in MY verification, twice** — `go build ./... | tail -2 && echo BUILD_OK` printed BUILD_OK on a failed build; `check_exit=$?` after `| tail` measured tail's exit. This is EXACTLY the anti-pattern documented in my own global AGENTS.md (set -o pipefail lesson) and I still did it. Both instances caught and corrected by re-running with direct exit capture.
5. **Skipped Phase-1 baseline for pilot repos** — the go-ecosystem-upgrade protocol demands recording build/test state BEFORE edits. I edited go-finding first and only then discovered it was already unbuildable in this shell. Consequence: for 3 of 7 pilot repos (go-finding, go-atomic-write, oxlint) form fixes are correct but **behaviorally unverified here** (1.27 floors can't compile under local go1.26.7). Mitigation: form fixes are semantics-preserving text-level directive rewrites; full verification deferred to the campaign.
6. **Edit-tool stale-read churn** — 3 rejected edits because gofmt/the daemon mutated files between my read and edit; one bulk `sed` rename corrupted human-facing strings ("match the module workspaceFloor"), caught only because I read the pilot output. Process fix adopted: re-view after any external mutation; never bulk-rename across string literals.
7. **Environment facts I can't change:** 62 modules declaring 1.27/1.27.1 cannot build in this shell at all (GOTOOLCHAIN=local, go1.26.7, nixpkgs go_1_26) — the fleet contains repos that are red-by-environment today (go-finding, go-atomic-write, oxlint-auto-configure at HEAD).

## e) WHAT WE SHOULD IMPROVE

1. **Exit-code discipline** — never read `$?` after a pipe; never `&& echo OK` after piped commands. Re-learned the hard way; belongs in every session's reflexes (already in AGENTS.md — needs teeth: I should have applied it, not just remembered it).
2. **Baseline-first, always** — record build+test state before ANY cross-repo mutation, even for "obviously safe" form fixes.
3. **Daemon awareness before mutation** — check `git log -1` + `git status` before and after each fleet-repo edit; the daemon races and re-poisons go.mod files via get-style passes.
4. **Scan → decide → fix ordering** — I piloted fixes before running the fleet scan; inverting gives a decision baseline and prevents redundant passes.
5. **Performance before fleet sweep** — `go mod tidy` per module makes a 152-repo fix sweep a multi-hour job; add parallelism + a skip-if-clean fast path first.
6. **Scaffold with lint config from commit zero** — gosec exclusions and .golangci.yml should be born with the repo, not retrofitted.
7. **Persist scan artifacts in-repo** — fleet_report.txt lives in /tmp (reboot = gone); reports should land in docs/ or a workdir the daemon commits.
8. **Fixture realism** — the bugs that mattered (workspace floor, nix pin forms) were caught by tests modeled on REAL fleet shapes; derive fixtures from fleet data by default.

## f) 50 THINGS TO GET DONE NEXT

| # | Task | Impact | Effort | Category |
|---|------|--------|--------|----------|
| 1 | T1: re-tag go-atomic-write with major.minor-only floor (downgrade 1.27.1→1.26, owner-confirmed) | Critical | M | Release |
| 2 | T1: re-tag go-finding root + modules after minor decision | Critical | M | Release |
| 3 | T1: re-tag linter-autoconfigure-sdk v0.3.0 with clean floor | Critical | S | Release |
| 4 | Build poisoner matrix: `go list -m -f '{{.Path}} {{.GoVersion}}' all` across fleet caches; every published lib with patch floor listed | Critical | M | Feature |
| 5 | T4 decision doc (ADR): fleet canonical minor 1.26 vs 1.27 | Critical | S | Documentation |
| 6 | Execute T4 outcome (downgrade 62 modules to 1.26 OR bump 241 flakes + CI to 1.27) | Critical | L | Feature |
| 7 | Owner go/no-go for proxy publishing (T1 gate) | Critical | S | Decision |
| 8 | T2: blank-import provider in BuildFlow + confirm discovery via `buildflow --dry-run` | High | S | Feature |
| 9 | T5: gomod-checker upstream rule "directive re-poisoned after tidy" | High | M | Feature |
| 10 | Fleet sweep: gvac fix + tidy + build+test across 152 finding repos | High | L | Feature |
| 11 | gvac: parallel repo execution + skip-if-clean fast path | High | M | Feature |
| 12 | gvac: JSON output for CI/machines | High | S | Feature |
| 13 | gvac: `who-forces` command — per-repo dep-floor matrix | Medium | M | Feature |
| 14 | gvac: `--expect-minor` flag encoding the T4 decision | Medium | S | Feature |
| 15 | CI pin normalization campaign (126 ci-pin-below-floor findings) | High | L | Cleanup |
| 16 | CI patch-pin cleanup (38 ci-pin-patch-form findings) | Medium | M | Cleanup |
| 17 | Nix pin alignment (37 nix-pin-below-floor) paired with `buildflow -s nix-hash-fix --fix` | High | M | Cleanup |
| 18 | Fix flake typos found in scan (`go_256`, `go_1_`) | Medium | S | Bug |
| 19 | Post-T1 consumer bumps: autoconfigure family + templ-components, go-cqrs-lite (88 findings), cqrs-htmx (35) | High | L | Feature |
| 20 | T3: v0.1.0 tag + GitHub release + pkg.go.dev verification | High | S | Release |
| 21 | CI workflow for the new repo (lint + test matrix + dogfood `gvac check .` gate) | High | S | Quality |
| 22 | GoReleaser setup for the new repo | Medium | M | Feature |
| 23 | T6: VERSION/CHANGELOG/tag authority drift detection (start: project-dependency-graph 0.7.0-vs-v0.2.0) | Medium | M | Feature |
| 24 | Update go-ecosystem-upgrade skill version-surface.md: floor-poisoning section + gvac check command | High | S | Documentation |
| 25 | Update global AGENTS.md: daemon floor re-raise + exit-code-after-pipe recurrence | Medium | S | Documentation |
| 26 | Document directive state in the 7 pilot repos' AGENTS.md files | Medium | M | Documentation |
| 27 | Root-cause the re-raise force: which daemon pass runs `go get`-style floor bumps | High | M | Bug |
| 28 | Full-mode BuildFlow run on new repo (race + coverage) | Medium | S | Quality |
| 29 | .editorconfig + .gitattributes for new repo | Low | S | Cleanup |
| 30 | dependabot.yml refinement (groups + open-pull-requests-limit per auto-fixer suggestion) | Low | S | Cleanup |
| 31 | project-dependency-graph: consume pkg/surface for release-overview alignment | Medium | M | Feature |
| 32 | gvac: go.work `toolchain` directive coverage | Medium | S | Feature |
| 33 | gvac: flake.lock effective-go-rev parsing | Medium | M | Feature |
| 34 | gvac: Dockerfile/.tool-versions/mise pin coverage | Low | M | Feature |
| 35 | gvac: per-repo config file (.goversionrc) for floor expectations | Low | M | Feature |
| 36 | Promote pkg/surface to its own submodule once a second consumer exists | Low | S | Cleanup |
| 37 | Cross-check gvac rules vs BuildFlow gomod-checker for overlap/dedupe | Medium | S | Quality |
| 38 | Benchmark Discover on go-cqrs-lite (largest monorepo) | Low | S | Quality |
| 39 | website-launch for go-version-auto-configure (sibling pattern) | Low | L | Feature |
| 40 | docs-health HARVEST: pull section (f) into TODO_LIST.md / ROADMAP.md | Medium | S | Documentation |
| 41 | Annotate project-dependency-graph status Q3 (version authority) with link to T6 | Low | S | Documentation |
| 42 | Refresh go-cqrs-lite-ecosystem-update-plan.md using gvac fleet data | Low | S | Documentation |
| 43 | oxlint-auto-configure: drop vendored/replace SDK pin after SDK v0.3.0 | Low | S | Cleanup |
| 44 | Verify no `replace` directives leak into any re-tagged go.mod (go-release Phase 3) | High | S | Quality |
| 45 | Post-release: `go get @vX.Y.Z` clean-module verification per re-tag (proxy check) | High | M | Quality |
| 46 | Add `gvac version` ldflags stamping in release pipeline | Low | S | Feature |
| 47 | Decide go-finding root minor as part of T1 (its flake/CI must follow) | Critical | S | Decision |
| 48 | Split-brain check: structure-linter's "1.27.1 available" rule vs fleet policy — tune upstream or document exception per repo | Medium | M | Quality |
| 49 | Add go.work presence detection to provider Trigger (currently triggers on go.mod files only) | Low | S | Bug |
| 50 | Retire /tmp/gvac + /tmp/fleet_report.txt into committed artifacts (bin/ + docs/) | Low | S | Cleanup |

## g) QUESTIONS ONLY YOU CAN ANSWER

1. **Fleet minor (T4):** commit to **Go 1.26** as the fleet canonical (downgrade the 62 accidental 1.27/1.27.1 module floors — downgrades are the risky direction, so I will not do this without your confirmation) — or adopt **1.27** fleet-wide (bump 241 go_1_26 flakes + CI pins + the local toolchain)? Everything downstream re-tags differently depending on your answer.
2. **T1 go/no-go:** do I run the supply-side re-tag campaign (go-atomic-write v0.5.3, go-finding v1.13.0, linter-autoconfigure-sdk v0.3.0 with major.minor-only floors, then consumer bumps)? This writes immutable tags to proxy.golang.org — irreversible by design, so it needs your explicit authorization.
3. **Sweep timing:** run the fleet-wide auto-fix now (152 repos, fix + tidy + build + test, likely hours, many daemon commits, repos on 1.27 floors only verifiable after T4) — or wait until after T1/T4 and sweep once against clean supply-side versions?

---

*Point-in-time snapshot. Section (f) is HARVEST input for TODO_LIST.md/ROADMAP.md (T1–T6 already seeded there). Note: report written as Markdown per explicit user instruction, overriding the skill's HTML default.*
