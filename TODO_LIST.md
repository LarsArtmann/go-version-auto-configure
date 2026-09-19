# TODO List

Short- and mid-term actionable work. Ordered by impact (Pareto). Harvested from `docs/status/archived/2026-09-18_15-13_fleet-versioning-unification.md` (section f) on 2026-09-18; swept 2026-09-18 (T7 fleet-sweep enablers and T10 repo hygiene shipped — see CHANGELOG Unreleased); re-harvested 2026-09-19 from `docs/status/archived/2026-09-19_06-39_t7-t9-t10-shipped-lint-zero-full-gate-green.md` (section f → T11/T12, T3 additions).

## T1 — Supply-side re-tag campaign (BLOCKING for fleet convergence) — PLANNED

Published library versions carry patch-form `go` floors that re-poison every consumer on `go mod tidy` (verified 2026-09-18: stripping this repo's directive to `go 1.26` and running plain `go mod tidy` re-raises it to `go 1.26.7`):

- [ ] `go-finding` (local root `go 1.27` after form fix; decide 1.26 vs 1.27 — its toolsdk module is `go 1.26`, flake pins `go_1_26`): re-tag with major.minor-only directives
- [ ] `go-atomic-write`: floor is accidental (`go 1.27.1`; deps allow 1.26 — xxhash 1.11, flock 1.25.0). Downgrade candidate, needs owner confirmation (F16), then re-tag
- [ ] `go-error-family`, remaining go-* libraries with published patch-form floors
- [ ] After re-tags: bump the autoconfigure family + fleet consumers to clean versions (go-ecosystem-upgrade protocol: baseline → sweep → test → commit per repo)
- [ ] Re-run `go-version-auto-configure check` fleet-wide; expect ~0 mechanical findings that survive `go mod tidy`
- [ ] Re-tag hygiene: verify no `replace` directives leak into any re-tagged go.mod (go-release Phase 3)
- [ ] Post-release: `go get @vX.Y.Z` clean-module verification per re-tag (proxy check)

## T4 — Decide the fleet minor: 1.26 vs 1.27 — PLANNED (owner decision)

- [ ] 62 modules declare 1.27/1.27.1 (accidental, floor-copied; go-atomic-write is the identified source; installed toolchain + 241 flakes are go_1_26 with `GOTOOLCHAIN=local`)
- [ ] Option A (recommended): downgrade floors to `go 1.26` fleet-wide (supply-side first), keeping nixpkgs 1.26 as the single toolchain
- [ ] Option B: adopt 1.27 — bump nixpkgs + CI pins fleet-wide (42 flakes already do)
- [ ] Record the decision and rationale as an ADR so the fleet policy is citable
- [ ] Whichever wins, encode it: this tool reports minors above the environment as alignment findings; a `--expect-minor` flag could enforce the decision

## T2 — BuildFlow blank-import wiring — PLANNED (BuildFlow dev task)

- [ ] Add `_ "github.com/larsartmann/go-version-auto-configure/pkg/provider"` to BuildFlow's SDK import set (mirrors sdk_imports_test.go for oxlint)
- [ ] Add to BuildFlow docs/provider catalog; run `buildflow --dry-run` to confirm discovery
- [ ] Decide DAG position: after `go-mod-update`, before `nix-checker` (alignment suggestions inform hash repairs)

## T3 — Publish this tool — PARTIALLY DONE (v0.1.0 tagged 2026-09-18)

- [ ] GitHub Actions CI: lint + test matrix + dogfood `go-version-auto-configure check .` as a gate (remember `GOEXPERIMENT=jsonv2` in the workflow env)
- [ ] GoReleaser config with ldflags version stamping for `version`
- [ ] pkg.go.dev verification after the first tag
- [ ] Website launch (sibling-project pattern) if it earns one
- [ ] Branch protection decision for `master`: required status checks would break the auto-commit daemon's direct pushes (owner call)
- [ ] Repo topics: `go`, `buildflow`, `golangci`, `auto-configure`, `version-surface`
- [ ] CI badge in README once the workflow lands
- [ ] Module-path casing check (`github.com/larsartmann/…` vs `LarsArtmann`) with a real `go get` after the next tag
- [ ] Verify the README build-from-source steps in a clean environment (container/nix shell)
- [ ] Cut v0.2.0 from CHANGELOG `[Unreleased]` once CI + GoReleaser + pkg.go.dev land

## T9 — Parser coverage — PARTIALLY DONE (toolchain directives shipped; flake.lock blocked by design)

- [ ] flake.lock effective Go revision parsing — **blocked by design**: the lock records only a nixpkgs rev, not the Go version it packages; resolving it requires an impure `nix eval`, but `Discover` must stay pure (reads files, writes nothing). A separate opt-in command (or BuildFlow step) would be the right home — design needed before building

## T11 — Tool hardening (from the 2026-09-19 full-gate session)

- [ ] Dogfood `who-forces` on real foreign repos (go-finding, go-atomic-write): real dependency graphs, network resolution, and the 1.27-floor-under-`GOTOOLCHAIN=local` failure mode
- [ ] Benchmark the parallel sweep (N seeded repos, timed); expose the worker count as `--parallel N`
- [ ] Add a schema-version field (e.g. `"schema": 1`) to the three `--json` documents
- [ ] Surface discovery issues in `fix --json` (today only `check` reports them — consistency gap)
- [ ] Provider test against a fixture carrying `toolchain` directives (provider tests predate toolchain modeling)
- [ ] Unify the `Floor()`/`toolchainFloor()` near-duplication in `pkg/surface`
- [ ] Capture per-package coverage for `pkg/fix` + `cmd/` (unmeasured in the full run); close gaps below 80%
- [ ] `check --quiet` (exit-code-only) for scripting; `--version` flag alias alongside the `version` subcommand
- [ ] `toolchain local` is silently ignored by parsing; consider an explicit info finding
- [ ] `who-forces` policy calls: `--allow-partial` vs fail-closed when some modules' `go list` fails; carry each poisoner's own floor in the matrix; document or mark go.work rows (currently skipped)
- [ ] Multi-root test with more roots than workers (pool contention; current test uses 2)
- [ ] Mini-sweep validation: `check --json` across ~5 fleet repos, eyeball the machine output on real drift

## T12 — Lint & environment debt (from the 2026-09-19 full-mode run)

- [ ] Triage the 265 warning findings from go-auto-upgrade: fix or suppress-with-rationale
- [ ] Investigate the 2 cqrs-lint info findings (verified false positives — this repo imports no go-cqrs-lite; confirm, then suppress)
- [ ] Run `buildflow doctor`; identify the 9 tools unavailable in full mode; install or document
- [ ] Investigate the forbidigo vanishing (9 `fmt.Print*` hits gone after `buildflow format` with no code or config change)
- [ ] Run the go-error-modernization workflow properly (`erraudit fix --dry-run`, `--type-aware`) and disposition the 6 "error checked, no error return" warnings
- [ ] Investigate the BuildFlow `skip_steps` WARN "go-mod-update matches no registered tool" (tool-name drift? the skip may be a no-op)

## T5 — Upstream gomod-checker rule: "tidy revert" detection — WORTH CONSIDERING

- [ ] BuildFlow gomod-checker rule: go directive carrying a patch component after tidy (the poisoning signature) — closes the loop for repos that never run this tool

## T6 — Release-authority drift (Layer-B versioning) — WORTH CONSIDERING

- [ ] Extend `pkg/surface` (or project-dependency-graph) to detect VERSION file vs CHANGELOG top vs newest git tag drift (known case: project-dependency-graph VERSION=0.7.0, tags at v0.2.0)
