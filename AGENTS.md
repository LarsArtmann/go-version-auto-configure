# go-version-auto-configure — AGENTS.md

## Project Identity

- **Type:** Single-module Go CLI + BuildFlow provider (the "auto-configure" family: golangci-lint, oxlint, dependabot, and now the Go version surface)
- **Purpose:** Detect Go toolchain version-surface drift (patch components in `go` directives, `go.work` below the workspace floor, Nix/CI pins trailing the module floor) and auto-fix the mechanically safe part
- **Repo:** `github.com/larsartmann/go-version-auto-configure`
- **Version:** 0.1.0 (tagged 2026-09-18 for BuildFlow integration; README install stays build-from-source until T1's clean re-tags land)
- **Toolchain floor (updated 2026-09-22):** the module `go` directive is `go 1.27` (minor-only): the jsonv2 APIs in cmd/ are gated at language go1.27, and the fleet toolchain is go1.27 since the nixpkgs bump. `go mod tidy` may re-raise it to a dep-forced patch floor while published deps still carry one; that state is reported honestly by this tool's own `check` and is the T1 supply-side campaign's target, not a bug.

## Build & Run

Inside `nix develop` the toolchain is pinned (`GOTOOLCHAIN=go1.27.1`, `GOEXPERIMENT=jsonv2`, `GOWORK` cleared so `go work edit` finds the workspace) — plain commands work:

```bash
go build ./...
go test ./...
go build -o /tmp/gvac ./cmd/go-version-auto-configure
/tmp/gvac check ~/projects/<repo>    # exit 1 = drift found
/tmp/gvac fix  ~/projects/<repo>
/tmp/gvac who-forces ~/projects/<repo>   # which deps force each go directive
```

Outside nix, prefix every command with `GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2` (go1.27.1 is cached in the module cache).

All commands accept multiple roots (parallel, sorted output) and `--json` (stable machine contract). `fix` skips clean repos entirely, so fleet sweeps scale with drifted repos, not repo count.

`GOEXPERIMENT=jsonv2` is required by the go-finding dependency (as in linter-autoconfigure-sdk and the other auto-configurers).

## Architecture (4 pkg packages + cmd/)

- `pkg/surface` — the **mini SDK**: `Discover(root) → Surface` (walks go.mod/go.work/toolchain directives/flake.nix/.github/workflows, skips vendor/node_modules/.git/result) + `Analyze(Surface) → []Issue`. Pure: reads files, writes nothing. `ParseDirective` is the single go.mod/go.work parsing entry point shared by discovery AND fix verification, so a fix is checked with exactly the code that detected it. Domain strings are named types (`Rule`, `GoVersion`, `ModulePath`, `FilePath`) — the branching-flow gate enforces this on pkg/ structs and params, matching go-finding's branded-type convention.
- `pkg/fix` — applies `Issue.Fix` entries (go.mod rewrites are byte-preserving text surgery guarded by a non-mutating `go mod tidy -diff` dependency-floor gate with atomic revert; go.work rewrites go through `go work edit`, which needs workspace discovery). Rejected go.mod downgrades are classified dep-forced; the floor and its carriers come from `go list -m` (the highest dependency go line is what tidy enforces). `CanonicalizeGoMod` (canonicalize.go) is the fleet-policy normalizer BuildFlow's `go-mod-normalize` calls: it derives the changes itself (patch-form go line, optional `toolchain` strip behind `StripToolchain`) and carries an installed-toolchain guard. `SyncGoWorkDirectives` (workspace.go) applies only the go.work directive fixes for workspace-arbiter consumers. `Options.DryRun` holds fixes back. `AnalyzeFloors` (who.go) is the read-only poisoner matrix behind `who-forces`.
- `pkg/provider` — BuildFlow wiring: `linter-autoconfigure-sdk.ProviderFromSpec` → `toolsdk.Register`, package-level `var Provider` (blank-import contract, same as oxlint's).
- `pkg/version` — fleet version-stamp kit (copied from file-and-image-renamer): `Version` resolves `ldflags injection > toolchain VCS stamp > "dev"`. The `version` command prints it; plain `go build` binaries self-identify as `<7-char-shortrev>[-dirty]`. Fleet standard + adoption guide: `../file-and-image-renamer/docs/FLEET-STANDARD-VERSION-STAMPS.md`.
- `cmd/go-version-auto-configure` — thin CLI shell: multi-root worker pools, human/JSON rendering (json.go owns the wire DTOs; pkg types carry the camelCase JSON tags for who-forces). Exit contract: 0 clean, 1 findings/failed fixes/poisoned, 2 hard errors.

## Policy decisions encoded here (do not regress)

1. **Form fixes are mechanical, alignment fixes are suggest-only.** `go 1.26.7 → go 1.26` is unambiguous; moving a flake pin or downgrading a minor is a maintainer decision (go-ecosystem-upgrade F16: never auto-downgrade).
2. **go.work target respects the FULL module floor (patch included).** The patch-form strip of a go.work directive is only offered when the stripped minor form still covers `Surface.FullModuleFloor()`; when a dep-forced module floor requires the patch, no strip is offered (that state is correct, not a violation). A go.work directive below the full floor is the `go-work-below-floor` rule with the always-safe raise-to-floor fix (T0, 2026-09-22; the go-finding/BuildFlow gotcha-#169 outage class). Rewriting go.work below the highest module floor leaves the workspace unable to resolve its own modules (found the hard way in go-finding: go.work 1.26.7 + root 1.27).
3. **`GOWORK=off` for `go mod edit` AND `go list -m` (module-scoped commands); never for `go work edit`.** `go work edit` REQUIRES workspace discovery — with GOWORK=off it fails "no go.work file found". Runner dispatches on `args[0]` (`moduleScoped` in fix.go). Without GOWORK=off, `go list` resolves a module's graph through an enclosing workspace — wrong per-module floors in `who-forces`.
4. **Unparseable go.mod files become findings, not errors** — one broken fixture must not hide drift in a hundred real modules. Same for go.work since 2026-09-22 (`go-work-unparseable`; before that a bad go.work vanished silently).
5. **Vendor/node_modules/.git/result are skipped**; vendored trees don't declare this repo's floor.
6. **`toolchain` directives are modeled, not stripped (default).** A toolchain below the same file's `go` directive is provably ignored by the go command → `toolchain-below-directive`, suggest-only (`go mod edit -toolchain=none`). A toolchain naming a newer minor than every go floor RAISES the effective floor for Nix/CI pin alignment (builds would switch toolchains). Toolchain pinning a patch within the go floor's minor is the normal post-`go get` shape and stays silent. EXCEPTION: `fix.CanonicalizeGoMod` accepts `StripToolchain: true` for consumers whose fleet policy bans toolchain lines outright (BuildFlow's go-mod-normalize passes it, per its GOTOOLCHAIN=local rationale); the provider and CLI keep the conservative default.

## The floor-poisoning root cause (fleet-critical)

`go mod tidy` lifts a consumer's `go` directive to the highest dependency floor. Published `go-finding@v1.10.0` declares `go 1.26.7`, `go-atomic-write@v0.5.x` declares `go 1.27.1` (accidental: its only floors are xxhash 1.11 / flock 1.25.0 — nothing needs 1.27). Consequence: every consumer tidy re-poisons its directive; per-repo fixes revert. Fleet convergence REQUIRES the supply-side campaign in TODO_LIST.md before (or together with) consumer sweeps. This tool fixes form; tidy WILL revert it until the supply side is re-tagged — reported as dep-forced, expected, not a bug.

## Environment reality

- Installed toolchain: go1.26.7 with `GOTOOLCHAIN=local` — modules with a 1.27 floor do NOT build in this shell (go-finding, go-atomic-write, oxlint-auto-configure at head). That predates this tool; the tool _surfaces_ it as nix-pin/ci-pin alignment findings. This repo's own head go.mod now also declares `go 1.27` (since e0932b0), so plain `go build`/`go test` fail here with "go.mod requires go >= 1.27"; prefix commands with `GOTOOLCHAIN=go1.27.1` (verified 2026-09-22: that toolchain is already cached under `$(go env GOPATH)/pkg/mod/golang.org/toolchain@v0.0.1-go1.27.1-*`, no download needed).
- The auto-commit daemon commits changes in fleet repos quickly (heuristic messages). Verify with `git log`, don't assume.
- `reports/` (coverage output) and `.crush/` (session DB) are gitignored local artifacts — expected to be dirty, nothing to commit.

## Known limitations (v0.2)

- [docs/DEDUPLICATION.md](docs/DEDUPLICATION.md) is the accepted-duplication baseline; diff future art-dupl reports against it instead of re-litigating the intentional groups.

- `check` counts discovery issues (unparseable go.mod) as plain findings; they are never auto-fixable.
- CI YAML parsing is line-oriented (regex on `go-version:` keys); matrix expressions (`${{ }}`) and ranges (`1.26.x`) are intentionally skipped as non-comparable.
- `flake.lock` pins are not parsed (flake.nix only) — blocked by design: the lock records a nixpkgs rev, not the Go version it packages; resolving it needs an impure `nix eval`, but `Discover` must stay pure. Right home would be a separate opt-in command or BuildFlow step (TODO_LIST T9).

## Known-tool-bug notes (do not "fix" what these report)

- **dependabot-auto-configure reports "no update groups" even for valid configs.** The config's `groups:` sits at the entry level (GitHub's schema; go-finding uses the same shape in production) and the tool still warns. Verified false positive 2026-09-18 by comparing against go-finding. The `open-pull-requests-limit` and groups entries are real and present; ignore the warning.
- **cqrs-lint reports 2 info findings here, but this repo imports no go-cqrs-lite.** Verified against go.mod 2026-09-19 and re-verified 2026-09-22 (zero matches in go.mod and go.sum). Suppressed via `skip_steps: [cqrs-lint]` in .buildflow.yml; un-skip if the linter learns to skip non-consumers.
- **BuildFlow `skip_steps` WARN "matches no registered tool" appears in `-s <tool>` single-step runs only.** Cosmetic: in full pipeline runs both entries skip correctly ("skipped via skip_steps config", confirmed 2026-09-22). The names `go-mod-update` / `go-structure-linter` are registered and correct; do not "fix" them.
- **branching-flow INDEX_OUT_OF_RANGE warnings (3) on the worker-pool pattern in cmd/main.go** (`results[i] = ...` inside `for i, root := range roots`): the index is bounded by the range, provably safe; the linter cannot see it. Pre-existing, warning-severity, gate passes at `--fail-on=error`.
- **branching-flow flags string fields/params without domain types in pkg/ code** ("should use phantom type"). This is fleet policy, not noise — go-finding models everything as named types (`RuleName`, `FilePath`, ...). This repo complies via `surface.Rule`/`GoVersion`/`ModulePath`/`FilePath` and `fix.FailureCause`. Wire DTOs in cmd/ (main package) are exempt.

## References

- [TODO_LIST.md](TODO_LIST.md) — actionable work (T-numbers are cited across the fleet)
- [ROADMAP.md](ROADMAP.md) — long-term themes and non-goals
- [docs/DOMAIN_LANGUAGE.md](docs/DOMAIN_LANGUAGE.md) — vocabulary (surface, floor, poisoner, dep-forced)
- [docs/status/](docs/status/) — historical session snapshots (harvest source)
