# ADR-0001: The fleet Go minor is 1.27

- **Status:** Accepted (2026-09-22, decided by Lars via structured review; supersedes the open T4 question in TODO_LIST.md)
- **Scope:** every LarsArtmann Go repository (the fleet), the BuildFlow pipeline, and this tool's enforcement surface

## Context

The fleet's version surface drifted through the floor-poisoning mechanism this tool exists to fix:

1. `go-atomic-write@v0.5.x` published `go 1.27.1` accidentally (its real dependency floors were xxhash 1.11 / flock 1.25.0 — nothing needs 1.27). Every consumer's `go mod tidy` copied that floor up.
2. That seeded an accidental 1.27/1.27.1 wave across the fleet; a snapshot in early planning counted 62 modules at 1.27 while the installed toolchain was go1.26.7 with `GOTOOLCHAIN=local` and ~241 flakes pinned `go_1_26`.
3. The supply side has since moved: `go-finding@v1.13.0` (2026-09-22) publishes root floor `go 1.27`, and its workspace now carries `go 1.27` in all four modules (untagged head). A current sweep of `~/projects` finds ~400 `go.mod` files declaring `go 1.27` and 47 flakes pinning `go_1_27` (vs ~205 still on `go_1_26`).
4. Consequence of (3): consumers are dep-forced to `go 1.27` on tidy — this repository's own go.mod was lifted to `go 1.27` and went red under the local go1.26.7 toolchain. nixpkgs `go_1_27` is currently 1.27.1 and builds everything green.

The open question (TODO_LIST T4) was: standardize the fleet minor at **1.26** (downgrade campaign) or **1.27** (adoption campaign)?

## Decision

**Adopt 1.27.** The fleet Go minor is `go 1.27`.

Concretely:

- **Directive form:** all `go` directives settle at major.minor (`go 1.27`), never a patch component; patch-form floors are the poisoning signature and stay a form finding.
- **go-finding v1.13.0 stands** — no second re-tag at 1.26. Its head extends 1.27 to the toolsdk/analysis/cmd modules; that ships in its next minor tag.
- **go-atomic-write** gets a *form fix*, not a downgrade: `go 1.27.1` → `go 1.27` (its deps allow it; nothing is moved down a minor, so the F16 no-auto-downgrade rule is not triggered).
- **Toolchain:** builds and tests run on nixpkgs `go_1_27` (1.27.1 today). `GOTOOLCHAIN=local` remains valid once the PATH toolchain is 1.27; devShells and flakes migrate to `go_1_27` per repo as packaging work touches them (this repo's flake.nix ships with its devShell on `go_1_27`).
- **CI:** GitHub Actions workflows across the fleet pin Go 1.27.x.
- **Enforcement:** this tool gains `--expect-minor 1.27` so the decision is machine-checkable: any surface (directive, flake pin, CI pin) whose minor *exceeds* the expectation is an alignment finding; anything below is caught by existing alignment rules.

## Consequences

**Positive**

- Matches reality that already shipped (go-finding v1.13.0, ~400 lifted go.mods) instead of fighting it with a second supply-side campaign.
- No downgrade of any published version — zero re-tags of already-published minors, no version retraction.
- Unblocks the T1 finish line: consumer bumps converge at `go 1.27`, this repo's `check` can go green.

**Negative / follow-ups**

- ~205 flakes still pin `go_1_26` → a fleet flake-bump campaign is owed (mechanical, per-repo, tracked outside this ADR).
- Environments whose PATH toolchain is older than 1.27 fail to build 1.27-floored modules by design; the fix is the devShell, not directive downgrades.
- nixpkgs `go` (default) may trail `go_1_27`; flake/devShell authors must reference `go_1_27` explicitly until nixpkgs defaults move.

**Neutral**

- The poisoning *mechanism* (tidy lifting to the highest dependency floor) is unchanged and still fleet-critical: patch-form published floors remain the thing this tool and T1 exist to eliminate.

## Evidence appendix (2026-09-22 sweeps)

| Signal                                                     | Value |
| ---------------------------------------------------------- | ----- |
| `go.mod` files under `~/projects` declaring `go 1.27*`     | ~407  |
| `go.mod` files under `~/projects` declaring `go 1.26*`     | ~406  |
| flakes pinning `go_1_26`                                   | ~205  |
| flakes pinning `go_1_27`                                   | 47    |
| go-finding modules at `go 1.27` (head, untagged)           | 4/4   |
| go-finding v1.13.0 root floor                              | `go 1.27` |
| nixpkgs `go_1_27`                                          | 1.27.1 |
| This repo under go.mod `go 1.27` + nixpkgs go_1_27         | build + full test suite green |
