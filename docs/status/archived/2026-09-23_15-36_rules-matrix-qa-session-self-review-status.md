# Status Report — Rules-Matrix Q&A Session + Self-Review

**Date:** 2026-09-23 15:36 CEST
**Session scope:** ONE question-answering turn (the auto-fixable vs suggest-only rules matrix), then this report. No code was changed, nothing was committed by me, the working tree was clean at report time.
**Report type:** Combined status report (a–g) + brutal self-review of the session's own work.
**Evidence convention:** claims below are tagged `[code-verified]` (read this session), `[memory]` (from AGENTS.md/project context, NOT re-verified this session), or `[assumption]`.

---

## a) FULLY DONE (this session)

1. **Answered the rules-matrix question** with a two-table matrix (auto-fixable vs suggest-only), citing `pkg/surface/surface.go:30-95` (rule constants + doc comments) and `pkg/surface/rules.go:213-271` (the two `Fix{}` attachment sites). `[code-verified]`
2. **Confirmed exactly two Fix{} attachment sites exist** — `formRule()` (covering both `go-directive-patch-form` and `go-work-patch-form` through one code path) and `goWorkBelowFloor` (`go-work-below-floor`). Grep across `rules.go` + `discover.go` found no others. `[code-verified]`
3. **Correctly excluded all eight other rules** from the auto-fixable table (`nix-pin-below-floor`, `ci-pin-below-floor`, `ci-pin-patch-form`, `toolchain-below-directive`, `toolchain-non-version`, `minor-exceeds-expectation`, `go-mod-unparseable`, `go-work-unparseable`). `[code-verified]`
4. **Preserved the floor-guard nuance in the answer**: the go.work patch-strip is only offered when the stripped minor still covers the full module floor (the `rules.go:209-211` guard), and dep-forced floors keep their patch. `[code-verified]`
5. **Flagged the one policy exception** (`fix.CanonicalizeGoMod` + `StripToolchain: true`, BuildFlow-only) instead of flattening it away. `[memory]`
6. **Followed both loaded skills** (brutal-self-review, status-report) for this report's process; ran `date` first as instructed.

## b) PARTIALLY DONE (verification gaps in the matrix answer)

~~1. **The matrix was verified by reading, never by execution.** I did not build the binary and run `check`/`fix` against a fixture repo this session. The rule set and Fix attachments are code-verified; the _behavior_ (exit codes, dep-forced classification, JSON `fixable` marking) was quoted from AGENTS.md. `[memory]`~~ done — superseded: goldens + e2e tests verify the matrix mechanically since v0.2.0; claims re-verified 2026-10-03
~~2. **Two claims in the auto-fixable table rest on memory, not code:** (a) the go.mod fix being gated by a non-mutating `go mod tidy -diff` with atomic revert, and (b) rejected go.mod downgrades being classified dep-forced. Both live in `pkg/fix` — which I did not open this session. High confidence they're accurate (AGENTS.md is fresh from 2026-09-22), but "high confidence" is not "verified". `[memory]`~~ done — both verified 2026-10-03 (tidy gate + dep-forced classification read in pkg/fix)
~~3. **`minor-exceeds-expectation` was compressed to one row** without stating it spans five surface kinds (`go` directive, `toolchain` directive, flake pin, CI pin) — the breadth is only visible in the constant's doc comment, which I read but under-reported. `[code-verified, under-reported]`~~ done — breadth documented in the FEATURES policy-analysis row
~~4. **The toolchain-raises-floor interaction was omitted from the matrix.** A `toolchain` directive naming a newer minor than every go floor RAISES the effective floor used for Nix/CI pin alignment (AGENTS.md policy #6). That materially changes when `nix-pin-below-floor`/`ci-pin-below-floor` fire, and my table didn't say so. `[memory]`~~ done — toolchain-raises-floor documented in FEATURES policy-analysis row
~~5. **No persistence of the answer.** The matrix exists only in chat scrollback and code comments. I noticed mid-answer that neither README.md nor docs/ carries this matrix and did nothing about it. (Deliberate: the user asked a question, not for docs work. But it belongs in section e/f now.)~~ done — standalone doc declined; README policy + FEATURES own the matrix (2026-10-03)

## c) NOT STARTED (this session — nothing beyond the Q&A was in scope)

~~1. Persisting the rules matrix into README.md or docs (split-brain risk: code comments vs no doc).~~ declined — README policy section + FEATURES rules row carry the canonical matrix; no standalone RULES.md
~~2. Executable confirmation (build `/tmp/gvac`, run against a drifted fixture, compare output to the table).~~ done — goldens + e2e tests verify the matrix mechanically since v0.2.0
~~3. TODO_LIST harvest of anything (no new work items were created this session that belong there yet).~~ done — routed this pass (2026-10-03); nothing new belonged in TODO_LIST
~~4. Anything fleet/consumer-campaign related.~~ routed — TODO_LIST T1

## d) TOTALLY FUCKED UP!

**Nothing destructive.** No files touched, tree clean, no commits, no builds. The honest failure of this session is **epistemic, not operational**:

1. **I mixed evidence grades silently.** The first answer presented code-verified facts and AGENTS.md-memory facts with identical confidence. A reader cannot tell which cells of the auto-fixable table would survive a refactor of `pkg/fix`. That's exactly the failure mode this repo exists to prevent (drift between claimed state and real state).
2. **I answered "what is auto-fixable" without opening the package that does the fixing.** `pkg/fix` is where auto-fixability is actually enacted (tidy gate, dep-forced classification, `Options.DryRun`). My grep proved where `Fix{}` is _attached_, and I inferred where it is _applied_. The inference is almost certainly right — and that's not the standard.

## e) WHAT WE SHOULD IMPROVE (session-derived, ranked)

1. **Tag evidence in multi-fact answers** — code-verified vs project-memory. Cheap, immediate, kills the mixed-confidence failure.
2. **Persist the rules matrix** (README or `docs/RULES.md`) so this question has a canonical answer that isn't chat scrollback. The matrix is stable policy (AGENTS.md "do not regress" list) — documentation-grade knowledge.
3. **Make the matrix executable truth**: a `rules` subcommand (or a JSON field on `--json` output, or a doc-test) that enumerates rule → fixable → fix kind from the code, so docs/UI can never drift from `Analyze`. The provider already maps rules to go-finding findings; the catalog exists implicitly.
4. **When answering behavior questions, open the behaving package** — rule attachment (`pkg/surface`) and rule application (`pkg/fix`) are different truths.
5. **First-mention doc gaps in the same turn**, even when not acting: "this matrix isn't documented anywhere; want it in README?" instead of noticing and moving on.

## f) Things to get done next (up to 50, honest count: 38)

Grouped, roughly impact-ordered. Sources: `[session]` noticed this session, `[TODO_LIST]` already tracked, `[AGENTS.md]` known context. Per status-report policy these are brainstorm-grade beyond the top few — ROADMAP/TODO fuel, not commitments.

**Session-derived (new):**

~~1. Persist the auto-fixable rules matrix into README.md or docs/RULES.md. `[session]`~~ declined — README policy section + FEATURES rules row carry the canonical matrix; no standalone RULES.md
~~2. Verify this session's two memory-based claims in `pkg/fix` (tidy gate, dep-forced classification) — 10 minutes, closes the evidence gap. `[session]`~~ done — verified 2026-10-03 (tidy gate + dep-forced classification read in pkg/fix)
~~3. Consider a `rules` catalog subcommand / `--json` rule catalog for machine consumers (BuildFlow, agents). `[session]`~~ declined — no consumer signal (owner question g2 never demanded it)
~~4. Document the toolchain-raises-floor interaction wherever the matrix lands (it changes nix/ci-below-floor semantics). `[session]`~~ done — FEATURES policy-analysis row documents the raised-floor semantics
~~5. Add an answer-quality habit: evidence tags in multi-fact technical answers. `[session]`~~ declined — session-communication habit, not repo work

**Already tracked in TODO_LIST (state as of the file read this session):**
~~6. T2 — BuildFlow blank-import wiring (PLANNED; the provider contract is already in `pkg/provider`). `[TODO_LIST]`~~ done — T2 shipped 2026-09-18 (verified live 2026-09-22)
~~7. T9 — finish parser coverage remainder (flake.lock opt-in path; blocked-by-design for `Discover`, right home is opt-in command or BuildFlow step). `[TODO_LIST]`~~ routed — TODO_LIST T9
~~8. T12 — lint & environment debt remainder (PARTIALLY DONE 2026-09-22). `[TODO_LIST]`~~ routed — TODO_LIST T12 (open rows)
~~9. T5 — upstream "tidy revert" detection rule for gomod-checker (WORTH CONSIDERING). `[TODO_LIST]`~~ routed — TODO_LIST T5
~~10. T6 — release-authority drift / Layer-B versioning (WORTH CONSIDERING). `[TODO_LIST]`~~ routed — TODO_LIST T6
~~11. Website for the tool (T3 says "website pending"; website-launch pattern exists fleet-wide). `[TODO_LIST]`~~ routed — TODO_LIST T3

**Fleet consumer campaign (the known remaining half of the floor war):**
~~12. Enumerate consumer repos still on OLD tags (pre-2026-09-22) from the ADR-0001 appendix baseline. `[AGENTS.md]`~~ routed — TODO_LIST T1
~~13. Bump each stale consumer to the new floors (go-atomic-write v0.6.0, go-finding v1.13.0, go-error-family v0.10.2, go-output v0.38.2, go-branded-id v0.6.0, linter-autoconfigure-sdk v0.3.0, autoconfigurers v0.8.2/v0.6.4/v0.2.1). `[AGENTS.md]`~~ routed — TODO_LIST T1
~~14. Re-run fleet sweep with this tool's `fix` after bumps; confirm zero re-poisoning. `[AGENTS.md]`~~ routed — TODO_LIST T1
~~15. Track `golang.org/x/text` (`go 1.26.0`) and `encoding/json/v2` patch floors as the only accepted poisoners — verify nothing new joined them. `[AGENTS.md]`~~ done — both documented as ACCEPTED in POISONERS.md
~~16. Update ADR-0001 appendix progress after each consumer lands. `[AGENTS.md]`~~ routed — TODO_LIST T1/T17

**Upstream issues this repo filed (close-the-loop checks):**
~~17. Check dependabot-auto-configure#3 (custom group names → nil/nil decode); re-verify, then remove the "ignore the warning" note from AGENTS.md when closed. `[AGENTS.md]`~~ routed — TODO_LIST T12
~~18. Check go-cqrs-lite#42 (cqrs-lint none-import guard); un-skip `cqrs-lint` in `.buildflow.yml` when closed. `[AGENTS.md]`~~ routed — TODO_LIST T12
~~19. Check branching-flow#1 (panic analyzer `make(T,len)` ↔ `range` tracking); drop the suppressions when fixed. `[AGENTS.md]`~~ routed — TODO_LIST T12

**Tool behavior / hardening:**
~~20. Consider surfacing dep-forced rejections more prominently in `--json` output (consumers scripting fleet sweeps). `[session, speculative]`~~ done — depForced array + additive cause field carry it (v0.2.3)
~~21. CI YAML matrix expressions (`${{ }}`) and ranges (`1.26.x`) are intentionally skipped — document that exclusion in README if not already. `[AGENTS.md]`~~ done — documented in FEATURES + AGENTS known limitations
~~22. `check` counts discovery issues as plain findings — consider a distinct JSON flag/severity so `go-mod-unparseable` isn't conflated with drift. `[AGENTS.md]`~~ declined — counts.discovery already separates them in --json; further split awaits a consumer ask
~~23. BDD (Ginkgo) consideration for new behavior-spec suites — parked per testing policy; revisit only for genuinely new suites. `[AGENTS.md]`~~ done by policy — parked as future consideration (AGENTS testing policy)

**Docs / meta:**
~~24. Harvest sections of this report (if the owner wants): items 1–5 are new TODO candidates. `[session]`~~ done (this pass, 2026-10-03)
~~25. Confirm pkg.go.dev listing caught up to v0.2.0. `[AGENTS.md]`~~ done — pkg.go.dev verified 2026-09-26
~~26. Keep DEDUPLICATION.md baseline diff-based on next art-dupl run — no re-litigation. `[AGENTS.md]`~~ routed — TODO_LIST T16 (art-dupl baseline diff)
~~27. Re-verify the cosmetic `-s <tool>` skip_steps WARN note still holds after next BuildFlow update. `[AGENTS.md]`~~ done — re-confirmed in full pipeline runs 2026-09-26 (0 failed steps)

**Stretch / ROADMAP-flavored (brainstorm grade):**
~~28. Fleet-wide scheduled sweep (cron/BuildFlow) running this tool across all repos with drift alerting.~~ routed — ROADMAP theme 1 (early-warning monitor)
~~29. `who-forces --json` dashboard across the fleet to watch poisoner counts trend to the accepted two.~~ routed — ROADMAP theme 1 (fleet poisoner matrix)
~~30. Support for GOTOOLCHAIN-agnostic invocation docs (the outside-nix prefix is a footgun for one-off users — maybe a wrapper or a clearer README error path).~~ declined — README Development section documents the env prefix; a wrapper adds nothing
~~31. Consider `--rules` filter flag on `check` for consumers that only care about a subset.~~ routed — ROADMAP theme 3 (sweep ergonomics)
~~32. Telemetry-free usage doc: exit-code contract table (0/1/2) in README next to the matrix.~~ done — README usage documents the 0/1/2 exit contract
~~33. Investigate whether `Discover` should also read `.netrc`/`GOFLAGS`-relevant env for reproducibility notes — likely NO (purity), write down why.~~ declined — purity is the documented behavior; nothing to write down
~~34. Property tests for `floorCoveredByDirective` edge cases (bare minor vs `.0` patch ranking).~~ routed — TODO_LIST T16 (test-depth backlog)
~~35. Golden-file tests for the `--json` wire DTOs to freeze the machine contract.~~ done — full-document wire goldens (v0.2.0)
~~36. Consider a `doctor` command printing the evidence this session lacked: which rules carry fixes, which gates apply, which toolchain runs.~~ declined — no consumer signal; buildflow doctor + gates cover the need
~~37. README quickstart explicitly covering multi-root + parallel behavior (fleet users assume single-root).~~ done — README fleet-sweeps paragraph covers multi-root + parallel
~~38. Add this repo to its own BuildFlow fleet sweep dogfooding list (if not already) — the tool should check the tool.~~ done — BuildFlow runs the step fleet-wide (repinned v0.2.x, live-verified)

## g) Questions I can NOT figure out myself

1. **Where should the rules matrix live — README.md, a new `docs/RULES.md`, or stay chat-only?** I can write it well; placement is an owner/audience call (README is the sales page; a rules matrix is arguably reference material).
2. **Is the `rules` catalog subcommand / `--json` rule catalog (f-item 3, 31) a wanted feature or YAGNI?** It duplicates nothing existing, but you may prefer zero CLI surface growth until BuildFlow asks for it.
3. **Should session-scoped reports like this auto-harvest their (f) sections into TODO_LIST/ROADMAP immediately (docs-health HARVEST), or strictly wait for your instruction?** The status-report skill defaults say harvest; your "THEN WAIT FOR INSTRUCTIONS" said wait — I followed your instruction, and I'd like the standing rule nailed down.

---

_Report written per explicit user instruction as Markdown at docs/status/ (skill default is HTML; override honored and flagged). No commit made — auto-commit daemon owns working-tree commits here._
