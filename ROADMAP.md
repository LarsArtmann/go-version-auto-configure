# Roadmap

Long-term direction and raw ideas. Bounded, actionable work lives in [TODO_LIST.md](TODO_LIST.md); shipped features in [FEATURES.md](FEATURES.md).

## Themes

### 1. Fleet convergence execution (after T1 + T14)

The supply-side re-tags unblock the consumer half of convergence. The raw, not-yet-bounded shape:

- Fleet-wide fix sweep across the ~152 repos with findings: fix + tidy + build + test, one pass against clean supply-side versions instead of two
- CI pin normalization campaign (126 ci-pin-below-floor findings at audit time)
- CI patch-pin cleanup (38 ci-pin-patch-form findings at audit time)
- Nix pin alignment (37 nix-pin-below-floor) paired with `buildflow -s nix-hash-fix --fix`
- Fix flake typos the scan surfaced (`go_256`, `go_1_`)
- Fleet poisoner matrix: aggregate the per-repo `who-forces` reports across module caches into one fleet-wide table of every published library carrying a patch floor
- Early-warning monitor: a fleet step that diffs every repo's go directive against published dependency floors (catches the next v0.38.2-class regression the day it ships, not 10 days later)
- Add "verify published `.mod`" (proxy URL + date) to every fleet release checklist — the rule that would have prevented the 10-day go-output inversion

### 2. cmdguard adoption across the auto-configurer family

`go-version-auto-configure` migrated its CLI to `github.com/larsartmann/cmdguard/v4` (2026-09-22, TODO_LIST T13). The raw idea, not yet bounded:

- Migrate the sibling auto-configurers (golangci-lint-autoconfigure, oxlint-auto-configure, dependabot-auto-configure, …) onto cmdguard with the same embedded-shared-flags shape, so `--json`/`--parallel`/`--quiet` behave identically fleet-wide; check their help/exit contracts for the same `-h` breaking change first
- Extract a small shared kit (flag structs, exit-contract sentinels, the silent-findings fang error handler) so each migration is mostly deletion; write the migration recipe (spike → flag structs → exit sentinels → goldens → guard test) from the 2026-09-22 session notes
- Revisit the styling tradeoff recorded in CHANGELOG (plain `Error:` lines) if cmdguard/fang grows a delegating default error handler
- Candidate cmdguard upstream issues (from the 2026-09-22 spike): `silenceErrors` spec field with no exported `CommandOption`; unexported embedded flag structs silently skipped instead of erroring at construction; `NewExitError`'s `(value, error)` double return inviting discard bugs

### 3. Version-surface coverage expansion

- Tier-2 pin sources: `.tool-versions`, `mise.toml`, Dockerfiles (the same version declared in yet more places)
- Per-repo config file (`.goversionrc`) for floor expectations, so policy exceptions are declarative instead of tribal
- Performance characterization: benchmark `Discover` on the largest fleet monorepo before scaling the sweep
- `check --deps` flag (reuse `AnalyzeFloors`) so check pre-classifies dep-forced patch forms without the tidy gate
- Schema-version bump policy (when does schema 2 → 3): ADR-0002 candidate
- Release hardening: generated release notes (GoReleaser `changelog.disable: true` today), artifact signing (cosign) or documented checksum verification
- Sweep ergonomics: `--fail-fast` or aggregated root-error rows for multi-root runs; `--rules` filter flag for consumers that care about a subset

### 4. Ecosystem integration

- project-dependency-graph consuming `pkg/surface` for release-overview alignment
- Cross-check these policy rules against BuildFlow's gomod-checker for overlap and dedupe — one drift class should have one owner
- Finish the go-ecosystem-upgrade `version-surface.md` cross-reference: its floor-poisoning section exists; add `go-version-auto-configure check` as the detection command
- Resolve the structure-linter split brain upstream: its "1.27.1 available" rule demands the exact accidental-minor bump this tool polices (worked around locally via `.buildflow.yml` skip)
- Promote `pkg/surface` to its own submodule once a second repo imports it (mirrors go-finding/toolsdk)
- Provider detect-time check: flag PUBLISHED tags whose directive sits below their graph's tidy floor (the v0.38.1/v0.38.2 class — supply-side regression net)
- Wire `--expect-minor` into the BuildFlow provider as a policy input (not just a CLI flag)
- Retire ad-hoc scan artifacts (`/tmp/gvac`, `/tmp/fleet_report.txt`) into committed `bin/` + `docs/` so audits are reproducible after a reboot

## Cross-repo follow-ups (this tool's incidents, other repos' backlogs)

Tracked here so they survive archiving; each belongs to its own repo:

- **go-output** (from the 2026-10-02 v0.38.3 incident): verify CI goes green post-pin-fix; diagnose the `release.yml` tag trigger (dark since v0.38.0); add `go-licenses` to the devShell + the license-check known-tool-bug note to its AGENTS.md; fix `RELEASE_CHECKLIST.md` 2b ordering + document bump-before-tag for poisoner fixes; refresh its AGENTS.md dev-shell line (Go 1.27); backfill GitHub Releases v0.38.1/2; verify all 17 v0.38.3 tags + pkg.go.dev; fix the stale `vendorHash.nix` preflight warn; digit-safe sibling-pin bump script; drop one of `cyclop`/`gocyclo`; CHANGELOG-drift check in `pre-tag-check.sh`; review the 5 daemon commits under the v0.38.3 tag; website redeploy state; keep bdd/examples/integration directives minor-form; adopt the go-release pre-release-check wrapper; minor polish (tui ansi constants)
- **BuildFlow**: make `--format finding` emit JSON for every tool step (golangci-only today); reconcile the gomod-check vs go-mod-normalize dispositions (this repo's go line flipped 20× in 20 commits)
- **crush-config**: record the tag-then-bump vs bump-then-tag fleet lesson (v0.38.3 case study); the sed-digit lesson if it generalizes
- **Fleet tooling**: teach the auto-commit daemon session attribution so parallel sessions don't interleave in one heuristic commit; fleet-standard devShell/GOTOOLCHAIN-pin pattern doc (owning repo TBD); pin golangci-lint versions in CI workflows fleet-wide (two repos went red from lint@latest drift); sweep fleet AGENTS.md files for stale "Go 1.26 dev shell" claims

## Explicit non-goals

- Never auto-apply downgrades — moving a floor down is always a maintainer decision (F16)
- Never rewrite directives with text editing — CLI fixes go through `go mod edit` / `go work edit` (the BuildFlow normalizer's byte-preserving text surgery is the documented library exception, gated by the same tidy check)
- Never auto-move alignment sides (pin vs floor) — suggestions only
- Never auto-fix unparseable go.mod files — they are findings, not fix targets
- Not a replacement for BuildFlow's gomod-check / go-mod-update: this tool owns the version-surface policy, not general go.mod hygiene
- POSIX-shell world only: the multi-root sweep recipes (`~/projects/*/` glob expansion, `$?` exit codes, cron) assume a POSIX shell; first-class Windows support (PowerShell glob semantics) is out of scope unless someone needs it

## Open questions (owner calls, parked)

- **Retract go-output v0.38.2?** Fleet precedent (2026-09-22 v0.38.1 call) is "documented-only, never retract", but v0.38.2 actively poisons every new consumer; Go's `retract` is advisory-only. Retract directive in the next go-output release, or documented-only? (2026-10-02)
- **Fleet sweep timing:** run the go-output → v0.38.3 consumer bump across `~/projects/*` now, or let each repo's `buildflow update` cadence pick it up? (2026-10-02)
- **Pin convention:** make bump-before-tag the permanent release standard, or keep tag-then-bump for normal releases and codify the exception only for poisoner fixes? (amend ADR-009 / Pattern B + RELEASE_CHECKLIST.md — 2026-10-02)
- **who-forces provenance:** add the additive `source: "vendor"|"list"` field now (schema stays 2), or freeze the wire until schema 3? (2026-09-26; implementation row in TODO_LIST T15)
- **Testify policy fleet-wide:** apply the `.go-auto-upgrade.json` `testifyassert` exclusion to every LarsArtmann Go repo, or only where go-auto-upgrade fires? (parked since 2026-09-22)
- **`--allow-partial` default** for who-forces in vendor-mode fleets: fail-closed default or flip? (2026-09-26)
- **Re-tag campaign sequencing:** green light for the go-cqrs-lite ~27-module family re-tag as the next primary session, and is it one coordinated campaign (all modules + consumers in a day) or repo-by-repo as touched? Gates dnsblockd, DiscordSync, and friends. (2026-09-26, unanswered)
- **x/text upstream engagement:** file an issue/PR proposing a minor-form floor for the permanent `go 1.26.0`, or accept-and-document? (2026-09-23)
