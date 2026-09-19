# Roadmap

Long-term direction and raw ideas. Bounded, actionable work lives in [TODO_LIST.md](TODO_LIST.md); shipped features in [FEATURES.md](FEATURES.md).

## Themes

### 1. Fleet convergence execution (after T1 + T4)

The supply-side re-tags (TODO_LIST T1) unblock the consumer half of convergence. The raw, not-yet-bounded shape:

- Fleet-wide fix sweep across the ~152 repos with findings: fix + tidy + build + test, one pass against clean supply-side versions instead of two
- CI pin normalization campaign (126 ci-pin-below-floor findings at audit time)
- CI patch-pin cleanup (38 ci-pin-patch-form findings at audit time)
- Nix pin alignment (37 nix-pin-below-floor) paired with `buildflow -s nix-hash-fix --fix`
- Fix flake typos the scan surfaced (`go_256`, `go_1_`)
- Fleet poisoner matrix: aggregate the per-repo `who-forces` reports (shipped 2026-09-19) across module caches into one fleet-wide table of every published library carrying a patch floor

### 2. Version-surface coverage expansion

- Tier-2 pin sources: `.tool-versions`, `mise.toml`, Dockerfiles (the same version declared in yet more places)
- Per-repo config file (`.goversionrc`) for floor expectations, so policy exceptions are declarative instead of tribal
- Performance characterization: benchmark `Discover` on the largest fleet monorepo before scaling the sweep

### 3. Ecosystem integration

- project-dependency-graph consuming `pkg/surface` for release-overview alignment
- Cross-check these policy rules against BuildFlow's gomod-checker for overlap and dedupe — one drift class should have one owner
- Finish the go-ecosystem-upgrade `version-surface.md` cross-reference: its floor-poisoning section exists; add `go-version-auto-configure check` as the detection command
- Resolve the structure-linter split brain upstream: its "1.27.1 available" rule demands the exact accidental-minor bump this tool polices (worked around locally via `.buildflow.yml` skip)
- Promote `pkg/surface` to its own submodule once a second repo imports it (mirrors go-finding/toolsdk)
- Retire ad-hoc scan artifacts (`/tmp/gvac`, `/tmp/fleet_report.txt`) into committed `bin/` + `docs/` so audits are reproducible after a reboot

## Explicit non-goals

- Never auto-apply downgrades — moving a floor down is always a maintainer decision (F16)
- Never rewrite directives with text editing — only `go mod edit` / `go work edit`
- Never auto-move alignment sides (pin vs floor) — suggestions only
- Never auto-fix unparseable go.mod files — they are findings, not fix targets
- Not a replacement for BuildFlow's gomod-check / go-mod-update: this tool owns the version-surface policy, not general go.mod hygiene
