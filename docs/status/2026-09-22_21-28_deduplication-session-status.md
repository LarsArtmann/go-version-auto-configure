# Status Report — Deduplication Session (art-dupl cleanup)

- **Date:** 2026-09-22 21:28 CEST
- **Repo:** `github.com/larsartmann/go-version-auto-configure` (branch `master`)
- **Session scope:** `deduplicate` after `art-dupl --sort total-tokens -t 1 --type-aware` reported 2 clone groups. Nothing else was in scope; everything below is from this session or directly noticed during it.
- **Head moved during the session:** my changes were committed by the auto-commit daemon as `8a7466d` and `751ccfa` (heuristic messages, mixed with a _second, concurrent session's_ files).

---

## a) FULLY DONE

| # | What                                                                                                                                                                                                                                                                                                                                                           | Evidence                                                                                       | Files                                   |
| - | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- | --------------------------------------- |
| 1 | Clone group 2 eliminated: extracted `readLines(path)` so `scanNixPins` and `scanCIPins` no longer duplicate the read-error + split preamble                                                                                                                                                                                                                    | art-dupl re-run: the discover.go group is **gone** (1 group left, was 2)                       | `pkg/surface/discover.go`               |
| 2 | Flag-scaffolding dedup across **three** commands (art-dupl only saw check/fix; `who-forces` had the same copy-paste — found by reading the whole dispatch file). New `runFlags` type + `newRunFlagSet(name)` + `parseRoots(fs, args)`; each command now registers only its own flag. The `--json`/`--parallel` help texts can no longer drift between commands | `git grep` in HEAD: 11 hits for `newRunFlagSet`/`parseRoots`; committed in `8a7466d`/`751ccfa` | `cmd/go-version-auto-configure/main.go` |
| 3 | Acceptance rationale for the one remaining clone group documented in the `runFlags` doc comment (why the per-command skeleton stays flat)                                                                                                                                                                                                                      | comment present in HEAD                                                                        | `cmd/go-version-auto-configure/main.go` |
| 4 | Verification pass: gofmt clean, `go vet` clean on touched packages, `go build ./...` green, `go test` green for `cmd/...` + `pkg/surface` (+ provider/version), functional smoke: `check` exit 1 on drifted repo (go-finding), `check --json` valid schema-1 output, `fix --dry-run` fast-path exit 0, `--help` shows all flags with intact text               | command outputs in session log                                                                 | 2 code files                            |
| 5 | AGENTS.md updated: Build & Run commands now carry the `GOTOOLCHAIN=go1.27.1` prefix; Environment reality documents that head `go.mod` declares `go 1.27` (since `e0932b0`) and that the go1.27.1 toolchain is already cached locally                                                                                                                           | AGENTS.md diff (still uncommitted at report time; daemon will sweep it)                        | `AGENTS.md`                             |

## b) PARTIALLY DONE

1. **"Zero harmful duplication"** — achieved in substance: the _harmful_ part (3× copy-pasted flag registration + help text) is gone. The report still shows **1 group at `-t 1`** (main.go ~221-232 vs ~363-374): per-command flag + parse-exit + `analyzeAll` head. Accepted with rationale; eliminating it fully requires a callback-runner framework, which I judged a net quality loss at 3 commands. Remaining effort to force it to zero: M, and it would make the code worse.
2. **Full `go test ./...` green** — NOT achieved. `pkg/fix` fails to build its tests (was already failing at my baseline, before any change of mine). Blocker: a **concurrent session's in-flight WIP** — `pkg/fix/canonicalize.go` references `surface.HasPatch` / `surface.MinorForm` which `pkg/surface/parse.go` (also mid-edit) doesn't define yet, plus a type mismatch (`surface.MinorForm(...)` returns `surface.GoVersion`, passed as `string` to `surface.GreaterVersion`). I verified only my packages + provider + version. Effort after their WIP lands: S.
3. **Lint gate (buildflow/golangci-lint)** — not run. A full run would fail on the other session's broken file regardless of my changes; I ran `go vet` on my two packages only. Rerun once the tree compiles: S.

## c) NOT STARTED

1. **The other session's pkg/fix WIP itself** (`surface.HasPatch`/`MinorForm` helpers, canonicalize signature churn, `SplitRunner` param) — deliberately untouched; actively being edited by someone else while I worked.
2. **GOTOOLCHAIN propagation to child processes** — surfaced in my smoke test: `who-forces` execs `go list` which inherits ambient `GOTOOLCHAIN=local` and fails ("go.mod requires go >= 1.27") even when the parent binary was built with go1.27.1. Policy-sensitive (fix pkg runner dispatch, AGENTS policy #3 territory). Not started.
3. **Repo-wide lint/buildflow run** once the tree settles (see b3).
4. **HARVEST of this report's section (f)** into `TODO_LIST.md`/`ROADMAP.md` — waiting for your instructions.
5. **Reconciling the AGENTS.md dogfooding caveat** ("expected steady state: go.mod sits at 1.26.7") with actual head (`go 1.27`) — I recorded the fact, but did NOT rewrite the contradicting policy paragraph; that needs the floor decision (question 2 below).

## d) TOTALLY FUCKED UP

Radical honesty, my own failures this session:

1. **Misdiagnosed a tool failure that was my own pipeline's fault.** My first art-dupl re-run piped output through `grep -v` filters that swallowed _everything_ and returned exit 1 from grep; I almost reported "art-dupl broken / transient failure". The raw re-run (next step) showed it working perfectly and reporting a _smaller_ clone set. Severity: none to code, one wasted round, near-miss on a false claim. Root cause: filtering output before ever seeing it raw.
2. **Ran the baseline build blind.** First `go build` failed with "go.mod requires go >= 1.27" — in the one fleet whose entire product is _go-directive floors_, and whose AGENTS.md documents exactly this failure mode. I didn't check the `go` directive before building. One wasted cycle; recovered by finding the cached go1.27.1 toolchain.
3. **Hardcoded a point-in-time workaround into a durable command list.** AGENTS.md Build & Run now prefixes every command with `GOTOOLCHAIN=go1.27.1`. If the floor returns to 1.26.x (the documented steady state), those lines are stale on arrival — harmless (newer toolchains build older modules) but lying-by-prefix. The pin belongs in `flake.nix` devShell, not doc commands. Severity: documentation debt, M to fix.
4. **History interleaving.** My changes reached history only via the auto-commit daemon, swept into heuristic commits _together with the other session's unrelated files_ ("3 changed file(s)"). Correct per harness rules (no manual commit), but this repo's commit history now interleaves two sessions' work in single commits — bisectability is degrading. Not solvable by me alone; flagged for process decision.
5. **Initially trusted the clone report as a complete inventory.** art-dupl flagged 2 groups; the _third_ copy of the flag scaffolding (`who-forces`) only surfaced because I read the whole file. Its flag-order difference had dropped it out of the reported group. Lesson: **clone reports are lower bounds, not inventories** — for dispatch/CLI code, read the whole file. Caught in-session, but the initial read was insufficient.

## e) WHAT WE SHOULD IMPROVE

1. **Run detection tools raw first, filter later.** My grep-mangled art-dupl run caused a false "tool broken" diagnosis. Rule: never pipe a tool's output through filters before having seen it unfiltered once.
2. **Read the `go` directive before the first build** in any Go session — should be a fleet reflex; this repo sells that exact check. Candidate for a how-to-golang skill note.
3. **Toolchain overrides belong in `flake.nix` devShell**, not prefixed onto AGENTS.md command lists — single source of truth, survives floor changes in both directions.
4. **art-dupl under-reports near-miss clones** (flag-order difference hid the who-forces copy). A cheap buildflow gate — e.g. grep for repeated `flag.NewFlagSet` literals — would catch what type-aware diffing drops.
5. **Accepted-clone baseline:** the rationale for remaining clones lives in a doc comment. A small DEDUPLICATION.md (or art-dupl config) listing accepted groups would let future runs diff against a known baseline instead of re-triaging the same finding.
6. **fix pkg exec runner env policy:** decide whether child `go` invocations should inherit or normalize `GOTOOLCHAIN` (feeds c2; interacts with policy #3's GOWORK handling).
7. **Multi-session hygiene:** when a tree is mid-WIP by another session, reports should state it up front with file:line evidence and "not mine, untouched" — I did this in chat, but it must be a report convention.

## f) NEXT TASKS (ranked, session-grounded — HARVEST input for `TODO_LIST.md`/`ROADMAP.md`)

| #  | Task                                                                                                                                                                 | Impact           | Effort | Category       |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------- | ------ | -------------- |
| 1  | Land/finish the concurrent session's pkg/fix canonicalize WIP (`surface.HasPatch`/`MinorForm` undefined; `GoVersion` vs `string` in `GreaterVersion` calls)          | Critical         | M      | Bug            |
| 2  | Restore full `go test ./...` green (pkg/fix test-build currently fails)                                                                                              | Critical         | S      | Quality        |
| 3  | Decide go.mod floor policy: stay `go 1.27` at head vs return to documented 1.26.7 steady state                                                                       | Critical         | S      | Decision       |
| 4  | Reconcile AGENTS.md dogfooding caveat with head `go 1.27` after #3                                                                                                   | High             | S      | Documentation  |
| 5  | Move `GOTOOLCHAIN=go1.27.1` pin from AGENTS.md commands into `flake.nix` devShell                                                                                    | High             | S      | Cleanup        |
| 6  | Pass/normalize toolchain env in fix pkg's exec'd `go list` so `who-forces` works on 1.27-floor repos from 1.26.7 shells                                              | High             | M      | Feature        |
| 7  | Run buildflow lint gate repo-wide once tree compiles; confirm dedup introduced zero findings                                                                         | High             | S      | Quality        |
| 8  | HARVEST this report into TODO_LIST.md / ROADMAP.md (docs-health)                                                                                                     | High             | S      | Documentation  |
| 9  | Add regression test asserting identical `--help` text across check/fix/who-forces (golden-file `PrintDefaults`) — guards the sync hazard this session removed        | High             | S      | Quality        |
| 10 | Add grep/buildflow gate for repeated `flag.NewFlagSet` literals (art-dupl under-report guard)                                                                        | Medium           | S      | Quality        |
| 11 | Baseline file for accepted clone groups (DEDUPLICATION.md or art-dupl config)                                                                                        | Medium           | S      | Quality        |
| 12 | Unit test `parseRoots` error path (flag typo → exit 2, usage to stderr)                                                                                              | Medium           | S      | Quality        |
| 13 | Unit test `readLines` unreadable-path path (nil, no panic)                                                                                                           | Medium           | S      | Quality        |
| 14 | who-forces end-to-end smoke on a 1.27-floor repo with GOTOOLCHAIN exported (validates #6 workaround)                                                                 | Medium           | S      | Quality        |
| 15 | Evaluate deduplicating the worker-pool shape shared by `analyzeAll`/`applyAll`/`cmdWhoForces` (sem+wg+sort; generics candidate — art-dupl can't see it across types) | Medium           | M      | Refactor       |
| 16 | Golden tests for JSON contract (`emitCheckJSON`/`emitFixJSON`/`emitFloorsJSON`) proving the refactor changed nothing on the wire                                     | Medium           | M      | Quality        |
| 17 | Teach auto-commit daemon session attribution so parallel sessions don't interleave in one heuristic commit                                                           | Medium           | M      | Tooling        |
| 18 | Record fleet lesson "clone reports are lower bounds — read whole dispatch files" in crush-config `references/lessons.md` (by commit)                                 | Medium           | S      | Documentation  |
| 19 | If GOTOOLCHAIN override becomes fleet standard, update `FLEET-STANDARD-VERSION-STAMPS.md` sibling docs                                                               | Medium           | S      | Documentation  |
| 20 | CHANGELOG entry for the dedup refactor (if CHANGELOG.md is maintained here)                                                                                          | Low              | S      | Documentation  |
| 21 | Test `version`/`--version` stamp behavior in CI (manually verified `ce1d149-dirty` this session)                                                                     | Low              | S      | Quality        |
| 22 | Test `-h` exit-code contract (ContinueOnError → exit 2)                                                                                                              | Low              | S      | Quality        |
| 23 | Re-run art-dupl at default `-t 5` with `--html` to confirm the accepted group vanishes at reporting thresholds                                                       | Low              | S      | Quality        |
| 24 | gopls "go-atomic-write should be direct" go.mod warning — resolve after floor decision #3 (tidy direction depends on it)                                             | Low              | S      | Cleanup        |
| 25 | Standing: un-skip `cqrs-lint` when it learns to skip non-consumers (AGENTS note; not re-verified this session)                                                       | Low              | S      | Cleanup        |
| 26 | Standing: dependabot-auto-configure "no update groups" false positive (AGENTS note)                                                                                  | Low              | S      | Bug (upstream) |
| 27 | Standing: branching-flow INDEX_OUT_OF_RANGE warnings on worker-pool indexing (provably safe, gate passes)                                                            | Low              | S      | Cleanup        |
| 28 | T1 supply-side re-tags (poisoning deps) — after which `/tmp/gvac fix .` should report `applied > 0` here instead of `dep-forced 1`                                   | Critical (fleet) | L      | Release        |
| 29 | After T1: verify the strip-canonicalize path sticks in this repo (fix → re-check clean)                                                                              | High             | S      | Quality        |
| 30 | Add "shared run flags / subcommand scaffolding" vocabulary to docs/DOMAIN_LANGUAGE.md if command-surface terms are tracked there                                     | Low              | S      | Documentation  |

Items 25-27 are standing AGENTS.md notes I did not re-verify this session — listed so the inventory is complete, not as new findings. Items beyond 30 would be padding; I stopped at what this session actually surfaced.

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **Is the pkg/fix canonicalize WIP an active parallel session I should keep avoiding, or abandoned work I should finish (or revert)?** I tried: `git log` (heuristic daemon messages carry no session attribution) and watching gopls diagnostics (they change minute-to-minute — the tree was broken, then 0 errors, then broken again). The answer decides whether I can restore `go test ./...` green (task #2) or must stay out of `pkg/fix`.
2. **Is head `go.mod` at `go 1.27` intentional (T1 supply side landed / new floor policy) or the accidental wave AGENTS.md warns against?** I tried: commit archaeology (`e0932b0` is a heuristic auto-commit; no message explains the bump) and AGENTS.md — which contradicts itself (dogfooding caveat says steady state is 1.26.7; head is 1.27). This decides the AGENTS.md rewrite (#4), whether the doc-level GOTOOLCHAIN pin stays (#5), and the tidy-direction warning (#24).
3. **For the one remaining clone group: keep the flat skeleton with the documented rationale (my recommendation), or do you want it forced to zero** via the callback-runner abstraction, or silenced via an art-dupl baseline config? I tried all three designs; the runner costs more than the 12 repeated lines at 3 commands, but that's a judgment call you may want to overrule — especially if more subcommands are planned.

---

_Point-in-time snapshot. Section (f) is HARVEST input — do not let it die in this timestamped file._

_Format note: user explicitly requested `.md`; the status-report skill's canonical format is styled HTML — override honored per skill rules, not propagated as a default._
