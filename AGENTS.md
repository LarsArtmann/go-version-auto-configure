# go-version-auto-configure — AGENTS.md

## Project Identity

- **Type:** Single-module Go CLI + BuildFlow provider (the "auto-configure" family: golangci-lint, oxlint, dependabot, and now the Go version surface)
- **Purpose:** Detect Go toolchain version-surface drift (patch components in `go` directives, `go.work` below the workspace floor, Nix/CI pins trailing the module floor) and auto-fix the mechanically safe part
- **Repo:** `github.com/larsartmann/go-version-auto-configure`
- **Version:** 0.1.0 (untagged)
- **Dogfooding caveat (this repo IS poisoned):** plain `go mod tidy` re-raises this repo's `go` directive to `go 1.26.7` — every published dependency (linter-autoconfigure-sdk, go-finding, go-finding/toolsdk) carries that patch floor, and tidy lifts the main module to the highest dependency floor (verified 2026-09-18: strip → tidy → back to `go 1.26.7`, build green; the daemon's dependency passes only make it happen sooner). Expected steady state: `go.mod` sits at 1.26.7; `/tmp/gvac fix .` reports it truthfully as `applied 0, dep-forced 1`, naming those modules as the poisoners — the strip cannot stick until the supply side re-tags (TODO_LIST.md T1). Do NOT "fix" go.mod to match the structure-linter's "1.27.1 available" suggestion — that direction is the accidental 1.27 wave (see Floor-poisoning section).

## Build & Run

```bash
GOEXPERIMENT=jsonv2 go build ./...
GOEXPERIMENT=jsonv2 go test ./...
go build -o /tmp/gvac ./cmd/go-version-auto-configure
/tmp/gvac check ~/projects/<repo>    # exit 1 = drift found
/tmp/gvac fix  ~/projects/<repo>
```

`GOEXPERIMENT=jsonv2` is required by the go-finding dependency (as in linter-autoconfigure-sdk and the other auto-configurers).

## Architecture (3 pkg packages + cmd/)

- `pkg/surface` — the **mini SDK**: `Discover(root) → Surface` (walks go.mod/go.work/flake.nix/.github/workflows, skips vendor/node_modules/.git/result) + `Analyze(Surface) → []Issue`. Pure: reads files, writes nothing. `ParseDirective` is the single go.mod/go.work parsing entry point shared by discovery AND fix verification, so a fix is checked with exactly the code that detected it.
- `pkg/fix` — applies `Issue.Fix` entries via `go mod edit` / `go work edit` (never sed). Every fix is verified by re-parsing the file; a fix counts as applied only when the directive actually changed AND — for go.mod — survives `go mod tidy`. Tidy-reverted fixes are classified dep-forced and name the poisoning dependencies via `go list -m`. `Options.DryRun` holds fixes back.
- `pkg/provider` — BuildFlow wiring: `linter-autoconfigure-sdk.ProviderFromSpec` → `toolsdk.Register`, package-level `var Provider` (blank-import contract, same as oxlint's).

## Policy decisions encoded here (do not regress)

1. **Form fixes are mechanical, alignment fixes are suggest-only.** `go 1.26.7 → go 1.26` is unambiguous; moving a flake pin or downgrading a minor is a maintainer decision (go-ecosystem-upgrade F16: never auto-downgrade).
2. **go.work target = max(stripped directive, workspace floor).** Rewriting go.work below the highest module floor leaves the workspace unable to resolve its own modules (found the hard way in go-finding: go.work 1.26.7 + root 1.27).
3. **`GOWORK=off` only for `go mod edit`.** `go work edit` REQUIRES workspace discovery — with GOWORK=off it fails "no go.work file found". Runner dispatches on `args[0]`.
4. **Unparseable go.mod files become findings, not errors** — one broken fixture must not hide drift in a hundred real modules.
5. **Vendor/node_modules/.git/result are skipped**; vendored trees don't declare this repo's floor.

## The floor-poisoning root cause (fleet-critical)

`go mod tidy` lifts a consumer's `go` directive to the highest dependency floor. Published `go-finding@v1.10.0` declares `go 1.26.7`, `go-atomic-write@v0.5.x` declares `go 1.27.1` (accidental: its only floors are xxhash 1.11 / flock 1.25.0 — nothing needs 1.27). Consequence: every consumer tidy re-poisons its directive; per-repo fixes revert. Fleet convergence REQUIRES the supply-side campaign in TODO_LIST.md before (or together with) consumer sweeps. This tool fixes form; tidy WILL revert it until the supply side is re-tagged — reported as dep-forced, expected, not a bug.

## Environment reality

- Installed toolchain: go1.26.7 with `GOTOOLCHAIN=local` — modules with a 1.27 floor do NOT build in this shell (go-finding, go-atomic-write, oxlint-auto-configure at head). That predates this tool; the tool *surfaces* it as nix-pin/ci-pin alignment findings.
- The auto-commit daemon commits changes in fleet repos quickly (heuristic messages). Verify with `git log`, don't assume.

## Known limitations (v0.1)

- `check` counts discovery issues (unparseable go.mod) as plain findings; they are never auto-fixable.
- CI YAML parsing is line-oriented (regex on `go-version:` keys); matrix expressions (`${{ }}`) and ranges (`1.26.x`) are intentionally skipped as non-comparable.
- `flake.lock` pins are not parsed (flake.nix only).

## References

- [TODO_LIST.md](TODO_LIST.md) — actionable work (T-numbers are cited across the fleet)
- [ROADMAP.md](ROADMAP.md) — long-term themes and non-goals
- [docs/DOMAIN_LANGUAGE.md](docs/DOMAIN_LANGUAGE.md) — vocabulary (surface, floor, poisoner, dep-forced)
- [docs/status/](docs/status/) — historical session snapshots (harvest source)
