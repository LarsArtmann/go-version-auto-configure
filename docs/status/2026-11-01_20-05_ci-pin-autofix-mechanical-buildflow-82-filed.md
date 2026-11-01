# Status Report — CI-pin raise shipped as mechanical fix; BuildFlow #82 filed

**Date:** 2026-11-01 20:05 CET
**Scope:** this session only (go-version-auto-configure + two adjacent fleet actions)
**Trigger:** BuildFlow reported 5 `go-version-auto-configure` findings "could not auto-fix" (CI pins `1.22` vs module floor `1.27`); owner asked why not auto-fixed, then ordered implementation of the CI-pin raise (#1) and the upstream attribution issue (#2).

---

## a) FULLY DONE

1. **Root-caused the pasted findings as wrong-repo attribution.** The five findings (lines 38/75/116/160/180) match `~/projects/desire-secrets/.github/workflows/ci.yml` exactly, not this repo — this repo never contained `go-version: 1.22` in its entire git history (all-revs `git grep`: 0 hits) and its CI pins `"1.27.x"` (range form, deliberately non-comparable). Evidence: line-number match; `/tmp/gvac-final check --quiet .` → exit 0.

2. **desire-secrets CI pins fixed.** All five `go-version: "1.22"` → `"1.27"` in `.github/workflows/ci.yml`; verified with a freshly built binary: `check --quiet` → exit 0. Evidence: working-tree diff (5 lines), clean `check`.

3. **`ci-pin-below-floor` promoted to a mechanical auto-fix** (policy split: below-floor raise = mechanical; flake moves + CI patch-form stay suggest-only). Shipped:
   - `surface.KindCIPin` + `Fix.Describe()` CI branch (`pkg/surface/surface.go`)
   - `ParseCIPins` — new single CI-pin parsing entry point shared by discovery AND fix verification, mirroring the `ParseDirective` contract (`pkg/surface/discover.go`, `scanCIPins` now delegates)
   - `ciPinIssues` emits `Fix` on the below-floor branch, `Suggestion` dropped there; patch-form branch untouched (`pkg/surface/rules.go:537`)
   - `applyCIPinFix` (`pkg/fix/ci.go`): exact-text drift guard, quoting-preserving rewrite (double/single/backtick, bare, YAML flow-sequence brackets), atomic write, revert on verification failure, no dependency gate (workflow pins are inert YAML for module resolution)
   - `applyOne` dispatch case (`pkg/fix/fix.go:342`); provider `repair()` carries CI fixes with zero provider changes (verified by reading `pkg/provider/provider.go:137-143`); provider description updated

4. **Parser hygiene for the new kind.** Explicit `KindCIPin` cases in `ParseDirective`/`ParseToolchain`/`ParseModulePath` via a shared `errCIPinNotDirective` helper — keeps exhaustive and dupl lints green (`pkg/surface/directive.go`).

5. **Test depth:** updated below-floor finding contract (`discover_test.go` — Fix shape, From-as-written, To-bare-floor); new `pkg/fix/ci_test.go` suite (quote styles ×4, five-pin isolation byte-equality, full detect→fix→verify e2e, drift-guard refusal leaves file untouched, missing-file failure, dry-run hold-back); wire golden `TestJSONWireContractGoldens_CIPinFix` in `cmd/go-version-auto-configure/contract_test.go` (check findings + `kind: "ci"` fix object + applied shape + file-content assertion; schema stays 2). Evidence: `go test ./...` all green.

6. **All gates green after lint-driven hardening:** BuildFlow `golangci-lint` step reported 5 findings (exhaustive ×3, golines ×1, wsl ×1) + 2 post-fix dupl findings; all resolved; final `buildflow -s golangci-lint` → `✓ BuildFlow passed`, 0 findings. `gofmt` clean; `go vet` clean.

7. **Live end-to-end proof on a synthesized desire-secrets-class fixture:** `check` → finding + "all auto-fixable with 'fix'"; `fix` → `applied 1`, human line `set go-version in .github/workflows/ci.yml:7: "1.22" → 1.27`; re-`check` → exit 0; file shows `go-version: "1.27"` (quoting preserved). Dogfood on this repo: `check --quiet --expect-minor 1.27 .` → clean.

8. **Docs updated everywhere the feature is described:** AGENTS.md (policy decision #1 rewritten with the carve-out; `pkg/surface` bullet names `ParseCIPins` as the second single-entry-point; `pkg/fix` bullet describes CI surgery + no-gate rationale), `docs/DOMAIN_LANGUAGE.md` (Alignment-violation row gains the CI-raise exception), CHANGELOG `[Unreleased]` Added entry (full rationale + wire-compat statement), README (rules matrix row `ci-pin-below-floor` → YES auto-fix with the GOTOOLCHAIN=auto "pin already lies" rationale; usage comment), FEATURES (Mechanical auto-fix + Suggestion engine rows). Standing docs gate: lychee 13/13 OK, TODO_LIST hygiene OK (advisory 29d stale reports are pre-existing).

9. **BuildFlow issue #82 filed and verified landed** (OPEN): "Summary never names the project: `✗ <tool> N finding(s) remain` with repo-relative paths is unattributable once it leaves the terminal". Verify-before-filing gates passed at source level: `formatFindingLine` renders `f.Position.File` verbatim with no root (`internal/render/workflow_summary_output_6.go:90-97` at 4ffcdee8), the repairs block names tools only (`workflow_summary_output_4.go:71-97`), the status line names BuildFlow only (`workflow_summary_output_7.go:76-80`), `WorkflowResult` carries no dir field, CLI resolves cwd at `internal/cli/workflow_config.go:170`. Duplicate search: #67 (cache leaks findings across repos) is the detection-side twin, referenced as related; no duplicate of the render-side ask. github-voice checker: 0 FAIL, 1 WARN weighed (the ✗ glyphs are quoted CLI evidence, not decoration). Filed with `--body-file` (not stdin), confirmed via `gh issue view` (2861-byte body, OPEN). Draft preserved at `BuildFlow/docs/drafts/2026-11-01_summary-project-attribution.md`.

10. **Session daemon commits absorbed in gvac:** 8ffb84e → fd60d1c → 3896542 → 6382177 → 65d53ea → ca06520 (heuristic messages, `Daemon: true`).

## b) PARTIALLY DONE

1. **desire-secrets fix is uncommitted there.** The ci.yml edit sits in the working tree (`git status`: `M .github/workflows/ci.yml`); the daemon has not absorbed it (unlike gvac, where it commits within seconds). Verified-by-tool (clean `check`), not verified-by-commit. **Open:** commit it there (the buildflow skill explicitly warns the daemon has a known blind spot). Effort: S.

2. **AGENTS.md performance/coverage baseline is now stale for the new code.** New paths (`applyCIPinFix`, `ParseCIPins`) carry tests but no re-measured coverage/benchmark row; the T16 table still shows 2026-10-17 numbers. **Open:** re-measure per-package coverage, decide whether `applyCIPinFix` earns a benchmark row. Effort: S/M.

3. **Issue #82 is filed, not fixed.** The BuildFlow render change (thread cwd → status line + findings block) is proposed with a grounded sketch but not implemented upstream — awaiting owner review of the issue (its MANUALLY REVIEWED ledger is the gate). Effort: M upstream.

4. **TODO_LIST.md not harvested from this report yet.** Section (f) below is the HARVEST input; per the docs-health loop it should flow into TODO_LIST/ROADMAP rather than die in this timestamped file. Effort: S (next step on session resume).

## c) NOT STARTED

1. **Release cut (v0.2.7).** Master carries the feature, CHANGELOG entry written, version stamp still unreleased; go-release protocol (tag, proxy propagation, pkg.go.dev verify, `go get` validation) not started — waiting on release-timing decision.

2. **Fleet auto-heal sweep.** The whole point of the feature — run the upgraded BuildFlow/gvac across `~/projects/*` and let below-floor CI pins heal (desire-secrets is fixed by hand; `template-readme` still carries one `go-version: "1.22"` at line 261, seen during the hunt, deliberately untouched). Not started; blocked on the propagation + authorization questions below.

3. **Provider version propagation.** BuildFlow consumes gvac via its own dependency pin; the new mechanical fix reaches fleet repos only after BuildFlow's gvac bump (go-ecosystem-upgrade supply-side pass: re-tag gvac, bump BuildFlow, verify). Not started.

4. **Stale-LSP root cause.** `golangci_lint_ls` served diagnostics pinned to the first version of `pkg/fix/ci.go` for the entire session (three phantom exhaustive warnings persisted even after restart and contradict current bytes). Worked around by trusting BuildFlow lint runs; the why (Crush LSP overlay invalidation?) never investigated.

## d) TOTALLY FUCKED UP

Nothing shipped broken — every gate is green and the live loop was proven end-to-end. The honest list:

1. **The session's starting confusion was a real product failure, not user error.** BuildFlow's unnamed-project summary made a fleet misread inevitable (I initially investigated it as this repo's problem too, before the line-number arithmetic — a 76-line file cannot have a line-180 finding — exposed it). Root cause verified at source and filed as BuildFlow#82. Workaround until fixed: read findings blocks only together with the invocation cwd, or run `buildflow -s <tool> --format finding` inside the repo.

2. **golangci_lint_ls (Crush LSP integration) served stale diagnostics all session.** Severity: low (annoyance, misdirection risk for future sessions that trust the panel over real runs). Root cause: unknown — diagnostics did not invalidate on file writes and survived `lsp_restart`. Mitigation used: authoritative check via `buildflow -s golangci-lint`. Needs investigation (see c-4).

3. **Two self-caught near-misses during golden writing** (no damage, listed for honesty): the CI golden initially double-applied the fix (a manual `run` before `jsonGolden` re-ran `fix` on the same root → applied-shape golden saw a clean repo), and one edit over-deleted the applied-golden block (caught by re-viewing). Both were test-side, caught by the suite within minutes, and the final goldens are exact-match locked.

## e) WHAT WE SHOULD IMPROVE

1. **Attribution gap is now a known fleet-wide footgun** (BuildFlow#82): until it ships, any BuildFlow output copied out of context is unattributable. Highest-leverage small fix upstream (one field, two render sites).

2. **Trust hierarchy for lint diagnostics needs to be explicit:** this session the LSP panel lied (stale) while BuildFlow lint told the truth. Working rule going forward: in this fleet, `buildflow -s golangci-lint` is the only authoritative lint verdict; editor diagnostics are hints. Worth encoding in AGENTS.md known-tool-bug notes if the stale-LSP behavior recurs.

3. **The `check` human output now says "all auto-fixable with 'fix'" for CI findings** — for go.mod patch-form findings there is the `gatedFormFixNote` honesty mechanism (fixability is gate-dependent). CI fixes have no gate, so the promise is unconditional there — correct today, but the two promise paths should stay explicitly distinct in tests so a future gate on CI fixes cannot silently inherit the unconditional wording.

4. **github-voice check-draft emoji counter flags quoted CLI glyphs** (✗ in evidence blocks) as emoji overuse — a WARN false positive for bodies that quote terminal output. Candidate one-line improvement upstream in the checker (skip code-fence content), keeping the 0-FAIL calibration intact.

5. **YAML comment handling in the CI scanner:** `go-version: 1.22 # pinned` is captured as `1.22 # pinned` and skipped as non-comparable — a real below-floor pin is silently unreported (pre-existing detection limitation, unchanged by this session). If fleet repos carry commented pins, detection under-reports; a strip-trailing-comment pass in `ParseCIPins` would close it. (Candidate fixture check during the fleet sweep.)

6. **`--respect-patch-floor` interplay with CI pins is untested territory:** the option scopes to go directives; a patch-form CI pin at or below the installed toolchain still fires `ci-pin-patch-form`. Probably correct (workflow pins are not host floors), but the interaction deserves one explicit test + one README sentence if it stays as-is.

7. **Policy records:** the alignment→mechanical carve-out lives in AGENTS.md policy #1, CHANGELOG, and DOMAIN_LANGUAGE. If it becomes the template for future promotions (e.g. nix-pin raises), consider a short ADR so the criterion ("always a raise AND the pin already lies") is citable as a decision, not a comment.

## f) NEXT TASKS (up to 50, ranked by impact; HARVEST input — route to TODO_LIST, rest to ROADMAP)

| #  | Task                                                                                                                                                                                                                        | Impact   | Effort | Category      |
| -- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- | ------ | ------------- |
| 1  | Commit desire-secrets ci.yml fix (daemon blind spot; verify `git log` there)                                                                                                                                                | Critical | S      | Bug           |
| 2  | Harvest this report's (f) into TODO_LIST.md (docs-health HARVEST)                                                                                                                                                           | High     | S      | Documentation |
| 3  | Fleet sweep: run upgraded fix across `~/projects/*` to auto-heal below-floor CI pins; record before/after counts in ADR-0001 appendix                                                                                       | High     | M      | Feature       |
| 4  | Release v0.2.7 (tag, proxy propagation, pkg.go.dev, `go get` validation)                                                                                                                                                    | High     | M      | Feature       |
| 5  | Propagate provider version into BuildFlow (go-ecosystem-upgrade: bump their gvac pin) so the mechanical fix reaches every repo's BuildFlow run                                                                              | High     | M      | Feature       |
| 6  | Implement BuildFlow#82 (thread cwd → status line + findings block) after owner review                                                                                                                                       | High     | M      | Feature       |
| 7  | Re-measure T16 coverage baseline incl. `pkg/fix/ci.go` + `ParseCIPins`; refresh AGENTS.md table                                                                                                                             | Medium   | S      | Quality       |
| 8  | Provider-level test: `repair()` applies a CI fix end-to-end through the toolsdk Spec (currently covered only at Apply level)                                                                                                | Medium   | S      | Quality       |
| 9  | Human-output golden for the new `Describe()` line in `fix` (and `check` mechanical rendering of CI findings)                                                                                                                | Medium   | S      | Quality       |
| 10 | Test `gatedFormFixNote` interaction: mechanical CI findings must not inherit the go.mod gate-dependent wording                                                                                                              | Medium   | S      | Quality       |
| 11 | CRLF line-ending test for `applyCIPinFix` (split-on-`\n` keeps `\r` on the line; assert drift guard + rewrite both survive)                                                                                                 | Medium   | S      | Quality       |
| 12 | Trailing-YAML-comment pins (`go-version: 1.22 # x`): extend `ParseCIPins` to strip comments, add fixtures; closes a silent under-report                                                                                     | Medium   | S      | Bug           |
| 13 | Fix `template-readme`'s 1.22 pin (line 261) — or confirm sweep #3 heals it; then verify                                                                                                                                     | Medium   | S      | Bug           |
| 14 | Explicit `--respect-patch-floor` × CI-pin interaction test + README sentence documenting the chosen semantics                                                                                                               | Medium   | S      | Quality       |
| 15 | Investigate stale `golangci_lint_ls` diagnostics (Crush LSP overlay invalidation); file where it belongs once root-caused                                                                                                   | Medium   | M      | Bug           |
| 16 | ADR (or ADR-0001 appendix note) for the alignment→mechanical promotion criterion ("always a raise AND the pin already lies")                                                                                                | Low      | S      | Documentation |
| 17 | After #82 ships upstream: re-run the misread scenario, note the fix in this repo's AGENTS known-tool-bug list                                                                                                               | Low      | S      | Documentation |
| 18 | check-draft checker: skip code-fence content in the emoji counter (fixes today's WARN false positive)                                                                                                                       | Low      | S      | Cleanup       |
| 19 | Wire docs: README contract table — document the new `kind: "ci"` value beside `go.mod`/`go.work`                                                                                                                            | Low      | S      | Documentation |
| 20 | Decide benchmark row for `applyCIPinFix` (file I/O dominated; likely document as "not benchmarked, no hot path" instead of adding one)                                                                                      | Low      | S      | Quality       |
| 21 | Fold `[Unreleased]` into v0.2.7 section at release (task #4 companion)                                                                                                                                                      | Low      | S      | Documentation |
| 22 | Update AGENTS.md version line at release ("next cut unreleased" → v0.2.7)                                                                                                                                                   | Low      | S      | Documentation |
| 23 | Post-release fleet `check` re-baseline sweep (T1 cadence): CI-pin findings collapse to ~0 mechanical                                                                                                                        | Low      | M      | Quality       |
| 24 | Error-branch coverage: `applyCIPinFix` revert-failure path is untested (hard to trigger; decide accept-with-note vs fault-injection test)                                                                                   | Low      | S      | Quality       |
| 25 | Bracket-flow-sequence pins in real fleet YAML: confirm `[1.22]` form actually occurs; if yes, add a real-world fixture to the quote-style table                                                                             | Low      | S      | Quality       |
| 26 | `.yaml` extension parity check in fixtures (scanner accepts `.yml`/`.yaml`; tests only exercise `.yml`)                                                                                                                     | Low      | S      | Quality       |
| 27 | Consider `check --json` carrying the analyzed root per repo entry (schema 3 candidate) — complementary to #82 for machine consumers                                                                                         | Low      | M      | Feature       |
| 28 | `release.yml` of this repo pins `"1.27.x"` too — confirm range-form pins stay silent by design in release workflows (they do; pin the test)                                                                                 | Low      | S      | Quality       |
| 29 | Sketch the nix-pin analogue: would `nix-pin-below-floor` ever qualify for mechanical raise under the same criterion? (expected: no — lockfile coupling; write the two-sentence rationale down)                              | Low      | S      | Documentation |
| 30 | Consolidate duplicated test helpers (`writeFile`/`writeRepoFile` across cmd and pkg test files) into one shared testutil if the fleet linter ever flags them                                                                | Low      | S      | Cleanup       |
| 31 | Add `docs/drafts/` retention note in BuildFlow (draft files are working artifacts; decide archive policy)                                                                                                                   | Low      | S      | Cleanup       |
| 32 | Sweep evidence: capture a `buildflow --fix` run healing CI pins in one fleet repo as the feature's proof artifact for CHANGELOG/README                                                                                      | Low      | S      | Documentation |
| 33 | Verify pickaxe anomaly from this session: `git log -S 'go-version: 1.22'` returned the file's full commit list while all-revs `git grep` found nothing — isolate (tool-layer quoting?) so future forensics don't get misled | Low      | S      | Bug           |
| 34 | Pin the trust rule "BuildFlow lint is authoritative; editor diagnostics are hints" into AGENTS.md if stale-LSP recurs after investigation                                                                                   | Low      | S      | Documentation |
| 35 | Ask upstream (BuildFlow) whether findings blocks could carry the tool's root when fan-out runs per-module (`golangci-lint [tools]` class) — same attribution gap one level deeper                                           | Low      | S      | Feature       |

(35 items; the remaining headroom to 50 is deliberately left to the HARVEST pass rather than padded — items above are all grounded in this session's observations.)

## g) QUESTIONS (3, not answerable by reading code/docs)

1. **Release timing:** cut v0.2.7 now with the CI-pin raise, or batch it with more unreleased work? (I tried: CHANGELOG/AGENTS give no cadence rule; the last three releases shipped 1-2 features each, so either pattern fits.)

2. **Fleet sweep authorization:** heal below-floor CI pins across `~/projects/*` in one authorized sweep now (touches many repos, daemon commits each), or let each repo's own BuildFlow run pick it up passively after the provider propagation lands? (Trade-off is commit-noise/risk vs. time-to-clean; only you can weigh it.)

3. **Propagation path for BuildFlow:** do you upgrade BuildFlow's gvac dependency pin deliberately per release (I run the go-ecosystem-upgrade pass against their repo), or does BuildFlow auto-bump the autoconfigure family on its own cadence? (Determines whether task #5 is mine this week or a waiting move.)

---

_Point-in-time snapshot; section (f) feeds docs-health HARVEST. Report per status-report skill; daemon will commit this file._
