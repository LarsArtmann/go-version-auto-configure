# Status Report — 2026-11-02 00:13 CET — v0.2.7 release cut; master CI redness root-caused (erraudit supply-side)

**Session scope:** the v0.2.7 release run only (go-release skill end-to-end), from release assessment through post-push verification and doc sync. Prior session's work (CI-pin graduation, BuildFlow #82, desire-secrets edits) is context, not re-reported.

**Headline:** v0.2.7 is LIVE and fully verified (tag `v0.2.7` at `3fd34b4`, GoReleaser green, 7 assets, proxy + sumdb + pkg.go.dev + binary stamp all confirmed). Along the way, the weeks-old master CI redness was root-caused: the erraudit CI step installs from a **private, never-published module** — it has been unpassable since 2026-10-03 and is outside the release path.

---

## a) FULLY DONE

1. **go-release skill loaded and followed** (SKILL.md + quick-reference checklist); release shape identified: application/binary, GoReleaser-driven via `release.yml` on `v*` tags.
2. **Release assessment (Phase 0/1)**: content since v0.2.6 = the `ci-pin-below-floor` graduation to mechanical auto-fix, `depForced.cause` golden, cmd/ test-depth batch, `BenchmarkAnalyzeFloors` + `BenchmarkApplyAll_CleanFastPath`, `checks.go-fod-floor` flake guard, 2026-10-17 docs refresh. Bump: **v0.2.7** (patch; fleet precedent — v0.2.5 shipped features as a patch bump in this 0.x CLI).
3. **Release conventions verified before acting**: `.goreleaser.yaml` (`changelog.disable: true`, ldflags injection `pkg/version.injected={{ .Version }}`, 6-platform matrix), `release.yml` triggers only on tags, prior release bodies empty → chose to curate notes post-creation (skill Phase 2.4).
4. **CHANGELOG cut** to `## [0.2.7] - 2026-11-01`; `[Unreleased]` left as bare header per this repo's own convention (`git show v0.2.6:CHANGELOG.md`).
5. **go.mod hygiene (Phase 3)**: no `replace`, no `00010101` pseudo-versions, module path correct, `go mod tidy -diff` clean.
6. **All local gates green**: `go build`, `go vet`, `go test -race -count=1` (5/5 packages), `buildflow -s golangci-lint` (✓ passed, 0 findings — the editor-panel exhaustive warnings are the known stale-LSP phantoms, ignored per AGENTS.md), docs-gate (TODO hygiene OK; only the 29d stale-report advisories).
7. **CI redness ROOT-CAUSED** (see d/e context): the `Error-handling audit (erraudit)` step (added 2026-10-03, commit `c811e7c`) runs `go install github.com/larsartmann/erraudit/cmd/erraudit@latest`; that repo is **private** (created 2026-01-23, still private, actively pushed 2026-11-01 16:20 by a parallel session) and **absent from the module proxy** → anonymous CI can never resolve it. Every visible master CI run since 2026-10-15 failed at exactly this step, four weeks before this session's work. `ci.yml` triggers on master/PR only, **not tags**; `release.yml` has no erraudit step and has been green on all six releases. Proceeding was safe and precedented (AGENTS.md: master CI is informational).
8. **Tag + push**: annotated `v0.2.7` (single-line fleet convention) on `3fd34b4` — the CHANGELOG-cut commit; verified via `git tag --points-at HEAD` and `git show v0.2.7:CHANGELOG.md` carrying the `[0.2.7]` section. Pushed master (was 1 ahead) + tag.
9. **GoReleaser run 41343366459 SUCCESS** (~2m38s-class run): all 7 assets (checksums.txt + linux/darwin/windows × amd64/arm64 archives).
10. **Release notes curated and set** (`gh release edit v0.2.7 --notes-file --latest`): Highlights + Compatibility sections; verified rendered on the release page. First release in the repo with a non-empty body.
11. **Post-push verification (Phase 6), all definitive checks passed**:
    - proxy `@v/v0.2.7.info` → exact tag commit `3fd34b4d9b…` ✓
    - `go get github.com/larsartmann/go-version-auto-configure@v0.2.7` in a fresh module → GO GET OK (sumdb-verified, no SECURITY ERROR) ✓
    - pkg.go.dev page LIVE: "Version: v0.2.7, Latest, Published Nov 1 2026, MIT" ✓
    - release binary downloaded from assets and run: prints `go-version-auto-configure 0.2.7`, exit 0 — ldflags injection landed ✓ (archive also carries LICENSE/README/CHANGELOG)
12. **Docs synced**: AGENTS.md version line → v0.2.7 (all prior-release history preserved), plus a new known-tool-bug bullet documenting the erraudit CI redness with do-not-fix guidance. README checked — no version references needed bumping.
13. /tmp verification artifacts cleaned with `trash`.

## b) PARTIALLY DONE

1. **pkg.go.dev trigger path**: `/fetch/...@v0.2.7` returned 404 three times while the page itself was already live — the /fetch trigger appears redundant (or laggy) for versions the proxy already indexed. Indexing confirmed via the page; the endpoint's behavior left uncharacterized.
2. **Release-notes automation**: curated manually this time. With `changelog.disable: true`, every future body is a manual step unless GoReleaser `release.notes` (or a notes-template + `gh release edit` script) is added. Improvement chosen, automation not built.
3. **docs-gate exit-code capture**: green inferred from advisory-only output (and the earlier same-session 13/13 lychee run); explicit `$?` not captured.
4. **Version-stamp format**: release binary prints `0.2.7` (GoReleaser `{{ .Version }}` has no `v` prefix). No VERSION file exists here so `release-authority-drift` can't fire, but alignment with the fleet stamp standard (`FLEET-STANDARD-VERSION-STAMPS.md`) was not re-verified.
5. **Dogfood before tag**: the prior session dogfooded `check --quiet --expect-minor 1.27 .` clean, but THIS release pass did not re-run it locally — and CI's dogfood step likely never executes because erraudit fails earlier in the job (step order not verified — see f#6).

## c) NOT STARTED

1. **BuildFlow repin to gvac v0.2.7** (go-ecosystem-upgrade): the fleet binary still carries the pre-graduation gvac, so `ci-pin-below-floor` fixes are not yet healing fleet repos.
2. **desire-secrets commit**: the five `"1.22"` → `"1.27"` ci.yml edits from the prior session are still uncommitted there (that repo's daemon has a blind spot; needs an explicit order).
3. **Fleet sweep** (check/fix across `~/projects/*`) — timing question carried over, unanswered.
4. **docs-health HARVEST** of the prior report's 35 (f)-items and this report's (f) into TODO_LIST/ROADMAP.
5. **Archive the 8 stale (29d) status reports** the docs-gate advisories list, after harvesting.
6. **erraudit unblocking decision** (make public / publish to proxy / `continue-on-error` in ci.yml meanwhile).
7. TODO_LIST not yet touched with this report's follow-ups (deliberate: standing order is WAIT).

## d) TOTALLY FUCKED UP

1. **Nothing in the release itself** — the release path was clean end-to-end: every gate green before push, every post-push check green after. Honest empty set.
2. **External /tmp interference (flagged, not caused by this session)**: mid-verification, another process on this machine rewrote session tmp dirs (`go.mod`/`go.sum` appeared inside the binary-extraction dir; the earlier `release-verify` dir was emptied). Worked around with a fresh uniquely-named dir + explicit `--repo`. On a machine running parallel agent sessions this is a real cross-contamination risk — needs a protocol (f#14), not a fix in this repo.

## e) WHAT WE SHOULD IMPROVE (self-critique: forgotten / suboptimal)

1. **Forgot `--repo` on the first `gh release download`** after `cd /tmp` — gh fell back to git-based repo resolution, failed with "failed to run git", and I initially misattributed that error to the release binary. Two calls wasted; the binary was never at fault. Rule going forward: always pass `--repo` explicitly when cwd is not the repo.
2. **CI status checked one batch late**: skill Phase 4.4 wants CI green verified during pre-release checks; I ran `gh run list` after build/vet instead of in the first parallel batch.
3. **pkg.go.dev probing order**: hit the `/fetch` endpoint 3× (404) before trying the page itself (live). Check the page first; use `/fetch` only for un-indexed versions.
4. **Inferred instead of captured** the docs-gate exit code; a `; echo $?` would have made it evidence rather than inference.
5. **Shared-/tmp hygiene**: initial extraction used a guessable dir name on a multi-agent machine; session-unique `mktemp -d` dirs should be the default.
6. **No codified pre-release gate script** in this repo (`scripts/pre-release-check.sh` per the skill doesn't exist) — the gate sequence ran ad hoc from the skill; codifying would make releases repeatable and reviewer-checkable.
7. **No green check AT the tag**: `release.yml` only builds/publishes; a verify/dogfood job at the tag would give the frozen commit a green status without touching the informational-CI policy on master.
8. **Release notes are a manual ritual** — script the curate-and-edit step or wire GoReleaser notes templates.

## f) NEXT — ranked (1-15 actionable now; 16+ ROADMAP fuel for HARVEST)

| #  | Task                                                                                                                                         | Why now                                                             |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| 1  | Repin BuildFlow to gvac **v0.2.7** (flake input, all workspace members, vendor stanza, profile binary)                                       | The CI-pin fixer only heals the fleet once the fleet binary has it  |
| 2  | Commit desire-secrets `.github/workflows/ci.yml` (five pins)                                                                                 | Uncommitted fix, daemon blind spot, one command away                |
| 3  | Fleet sweep with a v0.2.7 binary: `check/fix --expect-minor 1.27 ~/projects/*/`                                                              | Mechanically heals remaining ci-pin drift incl. template-readme:261 |
| 4  | Decide sweep timing: now vs passive healing                                                                                                  | Q2 carried over; blocks #3                                          |
| 5  | Decide erraudit unblock path (public / proxy / continue-on-error)                                                                            | Un-reds CI; Q1 below                                                |
| 6  | Verify ci.yml step order: does dogfood run before erraudit? If after, dogfood never executes in CI — reorder or `continue-on-error` erraudit | CI's strongest local-equivalent check may be silently skipped       |
| 7  | docs-health HARVEST: prior report's 35 items + this (f) list → TODO_LIST/ROADMAP                                                             | Reports die in timestamped files otherwise                          |
| 8  | Archive the 8 stale 29d status reports (after harvest)                                                                                       | Clears the docs-gate advisories                                     |
| 9  | Add `scripts/pre-release-check.sh` to this repo                                                                                              | Codify the gate sequence (skill helper absent)                      |
| 10 | Add a verify/dogfood job to `release.yml`                                                                                                    | Green status on the frozen tag commit                               |
| 11 | Automate release notes (GoReleaser `release.notes` or notes-template + edit script)                                                          | Kill the manual body-curation step                                  |
| 12 | Re-measure the T16 coverage baseline for the new CI-pin code (`pkg/fix/ci.go`, `ParseCIPins` paths)                                          | AGENTS.md baseline predates the graduation                          |
| 13 | Pin the stamp-format convention (`v0.2.7` vs `0.2.7`) against FLEET-STANDARD-VERSION-STAMPS.md; align GoReleaser ldflags if needed           | Binary prints `0.2.7`; convention unverified                        |
| 14 | Root-cause the /tmp external rewriter; set a multi-agent tmp protocol (`mktemp -d`, session-unique names)                                    | Cross-agent contamination risk observed live                        |
| 15 | Track erraudit publication; revisit the AGENTS.md bullet when it goes public/proxied                                                         | The known-issue note is conditional                                 |
| 16 | Codify the `[Unreleased]` convention (bare header vs Keep-a-Changelog placeholders) in docs                                                  | Repo convention and skill default disagree; pick one                |
| 17 | After repin: verify the provider emits `fix.kind:"ci"` on a live drifted repo                                                                | Detection-fix parity through the delivering layer                   |
| 18 | Watch consumer bumps of gvac v0.2.7 (pkg/surface importers)                                                                                  | go-ecosystem-upgrade demand side                                    |
| 19 | Dogfood `fix` on this repo pre-tag in the release routine (check clean first)                                                                | Cheap truth check before every cut                                  |
| 20 | Script the post-release verification helper (proxy .info, go get, page fetch, stamp check)                                                   | Same 6 checks every release                                         |
| 21 | Root-cause the stale `golangci_lint_ls` LSP (phantom exhaustive on directive.go)                                                             | Still open from the prior session                                   |
| 22 | Reconcile CI's golangci-lint-action vs the buildflow gate once CI passes lint again                                                          | Two lint authorities should agree                                   |
| 23 | Script a CHANGELOG-vs-tag drift check pre-push                                                                                               | Skill Phase 2.1 by hand today                                       |
| 24 | T9: flake.lock pin parsing home (opt-in command or BuildFlow step)                                                                           | AGENTS.md open item                                                 |
| 25 | T6: git-tag release-authority second pass                                                                                                    | AGENTS.md open item                                                 |
| 26 | go-taskqueue poisoner: supply-side retag campaign (POISONERS.md Active)                                                                      | Fleet floor hygiene                                                 |
| 27 | Verify cqrs-htmx floor state (v4.13.3 minor-form → v4.14.x patch-form regression noted in changelog)                                         | Poisoner registry accuracy                                          |
| 28 | Confirm next CI run goes green end-to-end after erraudit resolution                                                                          | Closes the redness permanently                                      |
| 29 | Post-sweep: re-run dogfood + record evidence in the next report                                                                              | Proof over assertion                                                |
| 30 | Update FLEET-STANDARD-VERSION-STAMPS.md if the empty-body/curated-notes convention diverges                                                  | Keep fleet doc truthful                                             |
| 31 | Re-check who-forces parity matrix after the sweep for new parity rows                                                                        | Sweep byproduct worth capturing                                     |
| 32 | Keep POISONERS.md current with any new carriers the sweep discovers                                                                          | Registry is the single source of truth                              |
| 33 | Re-check README rules-matrix rendering on the tagged tree (committed pre-release)                                                            | Docs gate doesn't render HTML tables                                |
| 34 | Consider `--latest`/prerelease flag conventions documented (GoReleaser vs manual edit overlap)                                               | I set --latest manually; GoReleaser may already                     |
| 35 | Revisit dependabot-auto-configure#3 / go-cqrs-lite#42 / branching-flow#1 when upstream closes                                                | Known-issue notes to retire                                         |

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **erraudit plan?** It's private (since 2026-01-23), never on the proxy, and actively developed in a parallel session. What is the intended publication path — make public, publish a version to the proxy, or keep private and mark the ci.yml step `continue-on-error` until then? This decides whether CI ever goes green and whether the AGENTS.md note should say "wait" or "suppressed".
2. **Fleet sweep: NOW or passive?** With v0.2.7 out, a sweep writes `ci-pin-below-floor` fixes across `~/projects/*` (including repos owned by parallel sessions). Run it now, or let the BuildFlow repin heal passively?
3. **Authorize the two cross-repo writes?** (a) BuildFlow repin to v0.2.7 now, or batch it into a later fleet pass; (b) commit desire-secrets' ci.yml fix (still sitting uncommitted). Both are other repos' working trees — I won't touch them without an explicit order.

---

_Point-in-time snapshot — harvested follow-ups belong in TODO_LIST.md/ROADMAP.md, not here. Report is a candidate for ANNOTATE-mode updates after the erraudit/fleet-sweep decisions land._
