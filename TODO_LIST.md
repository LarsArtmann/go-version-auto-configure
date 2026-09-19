# TODO List

Short- and mid-term actionable work. Ordered by impact (Pareto). Harvested from `docs/status/2026-09-18_15-13_fleet-versioning-unification.md` (section f) on 2026-09-18; swept 2026-09-18 (T7 fleet-sweep enablers and T10 repo hygiene shipped — see CHANGELOG Unreleased).

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

- [x] v0.1.0 tag (cut 2026-09-18 for BuildFlow integration; README install says "build from source" until T1 lands the first clean supply-side versions)
- [ ] GitHub Actions CI: lint + test matrix + dogfood `go-version-auto-configure check .` as a gate
- [ ] GoReleaser config with ldflags version stamping for `version`
- [ ] pkg.go.dev verification after the first tag
- [ ] Website launch (sibling-project pattern) if it earns one

## T9 — Parser coverage — PARTIALLY DONE

- [x] go.work / go.mod `toolchain` directive handling — modeled in `Surface.Toolchains`; stale toolchains (below the same file's `go` directive) report `toolchain-below-directive`; a newer toolchain minor raises the effective floor for Nix/CI pin alignment (`pkg/surface/rules.go`)
- [ ] flake.lock effective Go revision parsing — **blocked by design**: the lock records only a nixpkgs rev, not the Go version it packages; resolving it requires an impure `nix eval`, but `Discover` must stay pure (reads files, writes nothing). A separate opt-in command (or BuildFlow step) would be the right home — design needed before building

## T5 — Upstream gomod-checker rule: "tidy revert" detection — WORTH CONSIDERING

- [ ] BuildFlow gomod-checker rule: go directive carrying a patch component after tidy (the poisoning signature) — closes the loop for repos that never run this tool

## T6 — Release-authority drift (Layer-B versioning) — WORTH CONSIDERING

- [ ] Extend `pkg/surface` (or project-dependency-graph) to detect VERSION file vs CHANGELOG top vs newest git tag drift (known case: project-dependency-graph VERSION=0.7.0, tags at v0.2.0)
