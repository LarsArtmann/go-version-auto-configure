# Status Report — Plan Execution: P0 + Phase PA complete, v0.2.4 shipped, BuildFlow repin mid-flight

**Session:** 2026-10-03 01:43 – 02:13 CEST · **Repo:** go-version-auto-configure (master @ `57f9cfe` = tag `v0.2.4`, clean) · **Plan:** [docs/planning/2026-10-03_00-56_SUPERB-fleet-convergence-execution-plan.md](../planning/2026-10-03_00-56_SUPERB-fleet-convergence-execution-plan.md)
**Owner gates answered at session start:** g1 = YES coordinated batch · g2 = cut v0.2.4 after WP-06/07 · G3 = YES provenance · archive rule = resolved-or-owned.

---

## a) Fully done (verified)

### P0 / WP-25 — audit tail (5/5)

1. **Canonical dependabot verify:** config verified correct (groups present on both entries, `.github/dependabot.yml`); the 2 warnings persist because the INSTALLED buildflow (`5c5cfb8`, 68 commits behind BuildFlow HEAD) predates the vendored dependabot fix — **dependabot-auto-configure#3 is CLOSED (2026-10-02, fixed via `HasUnmodeledGroups` in BuildFlow master, commit-class evidence)**. The false positive dies at WP-04's reinstall. The AGENTS "known-tool-bug" note is now stale (fix closed; nuance: unreleased in the pinned binary).
2. **First-hand `go mod tidy -diff`:** empty, exit 0 — audit item b4 closed; master directive stays `go 1.27`.
3. **Full BuildFlow gate inside devShell:** passed with warnings (44 success / 0 failed on the post-change run); findings all warning-class and previously cataloged (vulnix nixpkgs advisories, flake meta.description, vendorHash inline style, dist ignore info).
4. **Upstream issues:** branching-flow#1 **OPEN** (keep suppression) · dependabot-auto-configure#3 **CLOSED 2026-10-02** (see 1) · go-cqrs-lite#42 **OPEN** (cqrs-lint stays skipped).
5. **Forbidido watch:** zero `fmt.Print*` in non-vendor source, zero findings — no recurrence; any future hit would be new code.

### WP-06 — wire-trust goldens (6/6, micro 6–11)

- **Offline-safe dep-forced fixture** (`seedDepForcedRepo`, cmd/main_test.go): temp consumer with patch-form `go 1.26.7` + replace-to-local poisoner at floor `1.27.1`; runs the REAL `go mod tidy -diff` gate and REAL `go list -m -e` — zero network. Prototyped in /tmp first; wire output recorded before assertions were written.
- **`fix --json` dep-forced golden** (contract_test.go): locks `depForced[].fix{file,kind,from,to,line}` + `floor: "1.27.1"`, `cause` ABSENT in the carrier-named case, all list fields `[]`, schema 2.
- **Clean-state snapshots** for `check` and `fix` (clean:true, counts 0, empty arrays) — the who-forces clean golden already existed.
- **Incident fixture:** synthetic consumer pinning fake `github.com/larsartmann/go-output` at floor 1.27.1 — asserts dep-forced classification, carrier named in human output (`floor go 1.27.1 is forced by: github.com/larsartmann/go-output`), exit 0, go.mod untouched (atomic revert). `fixDoc` gained `depForced` decoding.
- **Honest gap (documented in-test):** the `cause`-bearing wire shape could NOT be reproduced with a real offline fixture (json/v2 at `go 1.27` does not re-raise to 1.27.1 — tested empirically); `cause` stays pkg-level locked (fake gates) + JSON mapping is 4 lines in json.go. A cmd-level `cause` golden would need a test-only gate-injection seam (owner call, see g).
- Fixture tests self-skip below toolchain 1.27.1 (`skipBelowFloor`), so older-Go contributors aren't broken.

### WP-07 — who-forces at-parity carriers (3/3, micro 12–14)

- `ModuleFloors.ParityFloors []DependencyFloor` (`parityFloors`, omitempty, schema stays 2): dependencies whose floor EQUALS the directive when directive == maxDepFloor — the 2026-10-02 incident's unanswered "who holds me at parity", now one command.
- `Poisoned`, exit contract, below-parity and poisoned rows: unchanged (pinned by tests). Human output line: `at parity: module@version holds the directive at go X`.
- **Verified live** on the fixture: JSON carries `parityFloors` + `source` and exits 0; poisoned fixture still reports POISONED with carriers.
- Type renamed `PoisonerFloor` → `DependencyFloor` (serves both lists now); JSON wire names untouched.

### WP-08 — provenance `source` field (4/4, micro 15–18; gate G3 = YES)

- `FloorSource` (`"list"` | `"vendor"`) additive `source` field on who-forces rows; empty when the row errored.
- Human directive line appends `(floors via vendor/modules.txt)` on vendor rows.
- Tests: list rows `source:"list"`; vendor-skew rows `"vendor"`; unresolved rows omit; vendor at-parity rows carry both `parityFloors` and `"vendor"`.
- README JSON-contract table + who-forces prose updated (`parityFloors?`, `source?`).

### WP-18 — vendor/UX edges (3/3, micro 19–21)

- **Decision (fixture reality):** `--help` limitation note, NOT parser extension — modules.txt annotates only explicit stanzas (measured on BuildFlow: 251 annotated / 301 stanzas; the indirect 50 carry NO floor data — extending is impossible, not merely hard). Wired via `v4.WithLong` on who-forces; pinned by `TestWhoForcesHelpDocumentsVendorLimitation` (asserts on the cobra command since fang renders help outside the test writer).
- **Offline toolchain error:** `hintToolchainDownload` recognizes failed `golang.org/toolchain@` fetches in BOTH runners (EditRunner, ExecSplitRunner) and appends the actionable hint (pre-install toolchain / pin GOTOOLCHAIN); `errors.Is` identity preserved. Test covers hit/pass-through/nil.

### WP-03 — v0.2.4 cut (5/5, micro 22–26; gate g2 = after-goldens)

- CHANGELOG: `[Unreleased]` folded into `[0.2.4] - 2026-10-03` + Added section (at-parity carriers, provenance, goldens, offline hint, help note) + internal rename note.
- Release gates in devShell: `go build` ✓, `go test -race ./...` ✓ (5/5 pkgs), dogfood `check --quiet --expect-minor 1.27 .` exit 0 ✓, full BuildFlow gate 44 success / 0 failed ✓.
- Annotated tag `v0.2.4` on `57f9cfe`, pushed master + tag.
- **Proxy evidence (the 2026-10-02 lesson, executed):** `go get ...@v0.2.4` resolves; `proxy.golang.org/.../@v/v0.2.4.mod` fetched 2026-10-03: declares `go 1.27` (minor-form) + `go-output v0.38.3` — **the tag is clean, verified against the published artifact.**

### Quality gates held throughout

`go build ./...` + `go test ./...` green after every WP; gofmt clean; no `[Unreleased]`/CHANGELOG drift; v0.2.4 tag == master content (daemon-swept commit, verified post-hoc via proxy .mod).

---

## b) Partially done — WP-04 BuildFlow repin (interrupted mid-flight)

**Done (all committed by BuildFlow's daemon):**
- Flake input ref `v0.2.3` → `v0.2.4` + version comment; `nix flake update go-version-auto-configure` (lock rev `57f9cfe` = tag target).
- `tools/go.mod` require → v0.2.4; root `go.mod` indirect → v0.2.4.
- S87 research complete: `GoWorkFloorFinding`/`RestoreGoWorkFloor` were REMOVED from BuildFlow 2026-09-22 (gotcha #187 — logic moved upstream into gvac); CHANGELOG records S87 RESOLVED at v0.2.1 repin; the only remaining defense is go-work-sync `DependsOn: [go-version-auto-configure]`, documented as belt-and-suspenders. My retirement verdict: KEEP DependsOn (cheap determinism), record as decided.
- Parallel-session check performed: BuildFlow has an ACTIVE session (pass-4 plan, commits 01:41–01:53); their plan does NOT own the gvac repin/S87 rows; tree was clean at each touch.

**Blocked-then-diagnosed:** after `go work vendor`, `vendor/modules.txt` carries TWO gvac stanzas (v0.2.3 + v0.2.4). **Root cause found at 02:13: `execution/go.mod:149` still requires v0.2.3** — a workspace member I missed (checked root + tools only before vendoring).

**Remaining WP-04 steps (exact):** bump `execution/go.mod` → v0.2.4 → `go work vendor` (single stanza) → `nix run .#update-vendor-hash` → `nix build .` green → reinstall to profile → `buildflow version` shows the new rev → live `buildflow -s go-version-auto-configure` run on the /tmp/dfx drifted fixture → verify dependabot step silent (P0 micro-1's death certificate) → record S87/DependsOn decision + T14-②/P1 row updates.

---

## c) Not started (from the plan)

WP-01 (cqrs-lite ~27-module batch re-tag — g1 authorized), WP-02 (dashboard/etag/sse re-tags + cqrs-htmx map), WP-05 (fleet sweep + go-output v0.38.2 pinner bumps + ADR counts), WP-09 (CI depth incl. tidy-diff gate), WP-10 (test truth), WP-11 (nix hardening), WP-12 (depFloor unify), WP-13 (benchmarks/coverage), WP-14 (stability triage), WP-15 (disposition alignment), WP-16 (fleet-truth docs), WP-17 (sibling claim check), WP-19 (docs polish), WP-20 (standing docs gate), WP-21 (test-depth backlog), WP-22 (T5 rule), WP-23 (T6 drift rule), WP-24 (T9 nix-pin), WP-26 (website row), final living-docs refresh + plan annotation.

---

## d) What I fucked up (honest ledger)

1. **sed substring rename:** `s/PoisonerFloor/DependencyFloor/g` also renamed the FIELD `PoisonerFloors` (substring!) — build silently passed with a type-aligned-but-wire-mismatched field name; caught by grep review, reverted with word-boundary sed. Should have used `\b` from the start or per-identifier edits.
2. **multiedit stale-file failures ×3:** after every bash-side file mutation (sed/gofmt), my cached view went stale and edits bounced. Discipline: re-View immediately before Edit when bash touched the file.
3. **fix.go edit mangled the moduleScoped doc block; my "fix" then DELETED `moduleScopedExtraEnv` entirely** — caught by reading the result and restored. Root cause: oversized edit + trusting whitespace-equivalent matching. Smaller, precise edits from here on.
4. **Chained-command carelessness:** left `go mod edit -go=` (no value) in a `&&` chain — flake edit succeeded, tools edit never ran; recovered on next call.
5. **Record-then-assert violation (small):** first golden assertion `"dep-forced: 1"` written from memory; actual is `"dep-forced 1"`. Exactly the failure mode the plan's guardrail 3 warns about. Caught by the test run.
6. **Incomplete pre-vendor census:** bumped 2 of 3 workspace members requiring gvac → duplicate modules.txt stanza → a wasted vendor cycle. Root cause found (execution/go.mod). Lesson recorded: grep ALL workspace members before `go work vendor`.
7. **First full-gate run exit code not captured** (piped to `tail`); relied on rendered verdict text. Second run grepped pass/fail lines properly.

---

## e) Improvements (beyond the TODO rows)

- **Golden record-then-assert:** paste the recorded fixture output into the test comment next to the golden (I did this implicitly via the prototype; make it explicit).
- **Foreign-repo pre-flight checklist** (used ad-hoc here, formalize): tree clean? recent commit timestamps? untracked files? their plan owns the rows? — then work small and fast, expect the daemon to commit under you.
- **cmd-level `cause` golden:** needs a test-only gate seam (e.g. `GVAC_GATE_BIN`) — small prod-code cost; owner call (see g).
- **Multi-carrier parity human output:** current per-carrier `at parity: …` lines read fine for 1 carrier; for N>1 a header + bulleted list would be nicer. Only matters once a real multi-carrier parity repo shows up.
- **LSP was down all session** (`jsonrpc2: connection is closed`) — renames fell back to sed; `lsp_restart` early next session.
- **BuildFlow timing-regression warnings** on every run (erraudit +12500% etc.) are cold-cache noise vs its baseline DB — ignore or reset that baseline, don't chase.

---

## f) Next up to 50 things (grouped, ordered)

**WP-04 finish (7):** 1 bump execution/go.mod → v0.2.4 · 2 re-vendor until single stanza · 3 `nix run .#update-vendor-hash` · 4 `nix build .` green · 5 reinstall + `buildflow version` verify · 6 live `-s go-version-auto-configure` on drifted fixture · 7 dependabot step silence check + S87/DependsOn + T14-②/P1 row updates.

**WP-01 cqrs batch re-tag (11):** 8 inventory ~27 modules (tags/directives/x-text exceptions) · 9 per-module floor plan · 10 strip via `go mod edit` · 11–13 build+test groups A/B/C · 14 replace-leak check · 15 CHANGELOGs · 16 annotated tags batch with count assert · 17 push master+tags · 18 per-tag proxy `.mod` verification (URLs+dates) · 19 POISONERS rows → Resolved · 20 ADR-0001 counts.

**WP-02 remaining re-tags (5):** 21 dashboard v0.10.x tag+verify · 22 go-etag · 23 go-sse · 24 cqrs-htmx who-forces map then re-tag · 25 POISONERS rows.

**WP-05 fleet sweep (5):** 26 baseline sweep snapshot · 27 enumerate go-output v0.38.2 pinners · 28 bump each to v0.38.3 (+gvac fix) · 29 post-re-tag re-sweep deltas · 30 spot-check `git status` per outcome class.

**WP-09 CI depth (6):** 31 pin golangci-lint version · 32 erraudit step · 33 concurrency group · 34 **`go mod tidy -diff` gate job** · 35 README floor-line dogfood + version-stamp assert · 36 verify workflow green (note: CI go-version must be ≥1.27.1 for the real-gate goldens).

**WP-10..15 (19):** 37 e2e seeded fixture through real tidy gate · 38 ParseDirective invariants post-v0.38.3 · 39 deliberate `buildflow update` settle assert · 40 go-nix-helpers bump + hash · 41 FOD floor `checks` entry · 42–45 depFloor unification (extract/who/floor/golden-proof) · 46 benchmarks vs T11 into living doc · 47 coverage refresh · 48 license-check isolated runs + disposition · 49 doctor 9-warnings review · 50 vulnix triage · 51 art-dupl vs DEDUPLICATION · 52 gomod-check↔normalize alignment.

**WP-16/17/19/20/21 (15):** 53 POISONERS evidence convention + maintenance rule · 54–55 ADR-0001 incident records (09-26 session, v0.38.2 regression) · 56 DOMAIN_LANGUAGE (MVS floor, pin-in-tag) · 57 sibling inverted-claim grep+fix · 58 README who-forces real example · 59 CanonicalizeGoMod nuance · 60 FEATURES evidence re-walk · 61 ADR cited-row verify · 62 docs-gate script + wire · 63–67 test-depth batch (flag-help, exitFrom table, runSorted, version golden, BenchmarkApplyAll).

**WP-22/23/24/26 + closeout (9):** 68 T5 spec vs gomod-checker API · 69 T5 fixtures a/b/c · 70 T5 rule in BuildFlow · 71 T6 Discover pass + tests · 72 T6 git-tag design note · 73 T9 home decision (recommend BuildFlow step) · 74 website decision row · 75 TODO_LIST/FEATURES/AGENTS refresh (incl. dependabot#3-closed note) · 76 plan ANNOTATE + this report harvest under resolved-or-owned rule.

---

## g) Up to 3 questions I can't figure out myself

1. **BuildFlow parallel session policy:** their pass-4 session is active (commits every few minutes, 110 rows planned). Do I (a) finish WP-04 in BuildFlow right now anyway (small, mechanical, tree-touches are disjoint from their plan rows), (b) wait for their session to idle, or (c) limit myself to non-BuildFlow WPs (01/02/05/09…) and circle back? My recommendation: (a) — the remaining steps are mechanical and I verify with `nix build` + live run — but it's your repo under active parallel work.
2. **g1 scope vs BuildFlow GV1:** their TODO GV1 says "10 sibling repos floor at `go 1.27.1` … bottom-up normalization (tags + flake repins) needs the owner's go-ahead — USER DECISION". Does my g1 batch authorization cover ONLY the POISONERS-named set (cqrs-lite v4 family, dashboard, etag, sse, cqrs-htmx), or the FULL GV1 sibling set of patch-form repos too?
3. **v0.2.5 release batching:** after WP-04 + the gate-free WPs (09–16, 19–21) — one v0.2.5 at the end, or cut per phase (e.g. after WP-09 CI depth)? Related micro-decision I'll take myself unless you care: the cmd-level `cause` golden test seam (env-var gate override) — I lean NO seam (pkg coverage suffices; no prod code for tests).

---

_Waiting for instructions._
