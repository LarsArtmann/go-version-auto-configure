# go-compliance — utilization review

| | |
|---|---|
| Reviewed | 2026-11-23 |
| Consumer | go-version-auto-configure @ 07d4d51 |
| Library | github.com/larsartmann/go-compliance (root) + github.com/larsartmann/go-compliance/hub (submodule) — not a dependency, latest local tag v0.2.0 @ HEAD 01028e6 |
| Provenance | Group C |

## Verdict

**NOT-APPLICABLE.** go-compliance is not a rules engine and not a linter SDK — it is the fleet's *compliance-evidence aggregation* layer: a state model (`compliant`/`failing`/`stale`/`never-covered`, go-compliance/state.go:13), a frozen v1 wire contract (go-compliance/v1.go:32-91), an evidence fold (go-compliance/fold.go:113), a `Source` adapter interface (go-compliance/source.go:12), and a hub service with a realtime WebUI. Its own README states the integration goal: "BuildFlow-covered projects are compliant by default (zero changes)" (go-compliance/README.md:17-19).

This consumer is a BuildFlow-covered project — its findings already reach BuildFlow through the toolsdk provider (pkg/provider/provider.go:67, `toolsdk.Register`). In go-compliance's architecture that makes this repo evidence-*producing* via the path that requires zero changes; the hub's BuildFlow adapter (`NewBuildFlowFileSource`, go-compliance/adapter.go:46) reads the v1 JSON BuildFlow already emits. Adopting anything from go-compliance here would ADD a second evidence channel next to the one that is already the default. The orchestrator's hypothesized overlap with "this tool's rules engine" does not exist: go-compliance derives state from run-evidence windows (`DeriveState(evidence, now, staleAfter)`, go-compliance/state.go:112), it encodes no version-policy rules.

## Adoption score

Rows below: 5, N/A: 4 → A = 1. The one applicable row (BuildFlow evidence production) is already satisfied by the existing stack with zero code — grading the consumer's *alternative* as Fully. Score = **100 / 100** — band **very high**, but read carefully: the 100 means "the capability go-compliance cares about is fully covered WITHOUT it," which is precisely why the verdict is NOT-APPLICABLE.

| # | Capability | Grade |
|---|---|---|
| 1 | State model + window derivation | N/A — this tool reports drift per-repo at run time; no fleet-state table of its own |
| 2 | Frozen v1 wire contract (report marshal/decode) | N/A — its wire contract is the `--json` schema-2 CLI output + the toolsdk Spec, a different wire |
| 3 | Reporter SDK (`Record`/`RecordTo`) for non-BuildFlow tools | N/A — see below; the consumer is BuildFlow-covered, the default path |
| 4 | `Source` adapters (BuildFlow file source, discovery daemon) | N/A — consumer is on the producing side of that adapter |
| 5 | BuildFlow evidence production | Fully — satisfied by the adopted stack: findings → linter-autoconfigure-sdk Spec (pkg/provider/provider.go:46) → `toolsdk.Register` (pkg/provider/provider.go:67); BuildFlow turns runs into the v1 evidence go-compliance reads |

## Version currency

- Latest tag **v0.2.0** (2026-11-20), 2 tags total (v0.1.0 cut 2026-11-19 per go-compliance/CHANGELOG.md "CHANGELOG cut" entry). HEAD 01028e6 is 10 commits past the tag — all `auto-commit (heuristic)` docs/chore commits, no feature commits visible in the window.
- Release intent (the lore check): the skill's 2026-11-19 note about "[Unreleased] waits for the owner's go" has been overtaken by events — v0.2.0 WAS tagged on 2026-11-20 and the CHANGELOG `[Unreleased]` now holds only post-tag work (the hub-replace-drop, "Hub consumes core like a consumer"). However **production readiness is still gated on owner actions**: go-compliance/TODO_LIST.md "Blocked (owner decisions)" carries "Clean-dir `go get github.com/larsartmann/go-compliance@v0.2.0` … (GONOSUMDB='*' then without)" (TODO row, High impact, status TODO) — i.e. proxy-publishability of v0.2.0 is not yet verified — plus "Forgejo repo visibility (public?)" and FLAKE_PAT CI secrets, all BLOCKED on the owner.
- README status line is explicit: "**Status: built, pre-release.**" (go-compliance/README.md:5). The hub is `go 1.27.2` and drags a wide WebUI dependency set (charm.land/lipgloss, a-h/templ, go-daemon — go-compliance/hub/go.mod), while the core module is lean (go-branded-id, go-error-family, ginkgo/gomega for tests — go-compliance/go.mod).
- Maturity read: extremely active (the v0.2.0 CHANGELOG entry is enormous — one day of Pareto waves), but young, pre-release, with owner-gated publish verification outstanding. Not adoption-grade dependency material for a third repo yet.

## Capability inventory

| Capability | Status | Consumer evidence | Library evidence |
|---|---|---|---|
| Compliance state model (`compliant`/`failing`/`stale`/`never-covered`, ranked) | N/A — no fleet-state view; per-repo drift findings only | pkg/surface/surface.go (`Surface`/`Issue` model) is a different domain object | go-compliance/state.go:13 (`State`), :50 (`Rank`), :112 (`DeriveState`) |
| Evidence fold (runs → repo windows, worst-first ordering) | N/A — no run-history aggregation | — | go-compliance/fold.go:113 (`FoldEvidence`), :196 (`CompleteWithDiscovery` for never-covered) |
| Frozen v1 contract (marshal/decode, wire time) | N/A — different wire (`schema: 2` CLI JSON, locked by its own goldens) | cmd/go-version-auto-configure/contract_test.go:152 (schema-2 golden lock) | go-compliance/v1.go:78 (`MarshalV1`), :91 (`DecodeV1Report`), :115 (`BuildV1Report`) |
| Reporter SDK (one-call run recording for non-BuildFlow tools) | N/A — consumer is BuildFlow-covered; `Record` would duplicate evidence BuildFlow already emits | pkg/provider/provider.go:67 (`toolsdk.Register` — the zero-change compliance path) | go-compliance/reporter.go:62 (`Record`), :72 (`RecordTo`); precedent consumer: md-go-validator per CHANGELOG v0.2.0 "Adoption" entry |
| Source adapters (BuildFlow evidence file, discovery daemon) | N/A — consumed by the hub, not by evidence producers | — | go-compliance/adapter.go:40 (`NewFileSource`), :46 (`NewBuildFlowFileSource`); go-compliance/source.go:12 (`Source`) |
| Hub service (realtime WebUI, SQLite cache, CLI) | N/A — dashboard concerns; this repo ships a CLI tool, not a service | — | go-compliance/hub/ (submodule; hub/go.mod: lipgloss, templ, go-daemon) |

## Gap analysis / ranked opportunities

Sorted by impact × ease, descending — all negative or moot for this consumer:

1. **Call `compliance.Record` after each run** (the md-go-validator pattern, go-compliance/reporter.go:62). Impact: negative-to-zero — the hub's discovery pipeline classifies BuildFlow-covered repos as compliant by default; a second evidence stream from the same repo invites the exact double-counting the `FoldEvidence`/window logic then has to reconcile. Ease: low effort, but it adds a go-compliance dependency to a tool whose dependency floor policy is deliberately tight. **Skip.**
2. **Expose this tool's results as a `compliance.Source`** (so the hub could read schema-2 JSON directly). Impact: marginal — BuildFlow's v1 evidence already carries these findings to the hub; a native source would only add severity fidelity. Ease: medium, and it would need upstreaming into go-compliance (a change to a pre-release lib). **Skip.**
3. **Adopt the hub dashboard as an embeddable service** (`dashboard.RegisterRoutes`) for a "fleet version-surface board". Impact: interesting someday, ease: low — the hub submodule floor is `go 1.27.2` (hub/go.mod) which would poison this module's `go 1.27` minor-form floor, an active violation of this fleet's floor policy (AGENTS.md, POISONERS.md). **Skip.**

Cost side (any adoption): (a) pre-release dependency — README.md:5 "built, pre-release", proxy publish of v0.2.0 unverified per TODO_LIST blocked row; (b) hub floor `go 1.27.2` patch-form — the exact poisoner class this tool exists to eliminate; adopting it into this repo would make the version-surface tool itself a surface violation; (c) zero removed hand-rolled code (see fleet argument).

## Fleet argument

This is the clearest case of the three: **adoption would not remove hand-rolled code, because there is none to remove — the consumer and go-compliance are adjacent layers, not competing ones.**

- **Rules engine overlap: none.** go-compliance has no rule concept; its closest analogue, `DeriveState` (go-compliance/state.go:112), computes a window-health state from run evidence, while this tool's rules (pkg/surface/surface.go:39-76) encode Go toolchain policy over a discovered surface. They share vocabulary discipline (branded types — go-compliance/branded.go mirrors this repo's `surface.Rule`/`GoVersion` convention) but no logic.
- **BuildFlow provider pattern overlap: complementary, not duplicated.** This repo *feeds* BuildFlow (toolsdk provider); go-compliance *reads* BuildFlow (v1 adapter + discovery daemon). Adding go-compliance here would put a consumer library inside a producer tool — the producer's only correct move is what it already does: emit clean findings BuildFlow can turn into evidence.
- The fleet framing ("collective improvement flows only through shared libraries") is honored here by the consumer's existing contribution path: its findings flow into go-compliance's evidence automatically through BuildFlow. If go-compliance ever needs richer version-surface evidence (e.g. a patch-floor state dimension), the right fix is a new Source adapter in go-compliance reading this tool's schema-2 JSON — an upstream addition, not a local dependency.

## RECONSIDER-WHEN

(a) go-compliance graduates from pre-release: proxy publish of v0.2.0-or-later verified, repo visibility settled, a stable tag exists — AND (b) this tool starts running OUTSIDE BuildFlow coverage (e.g. standalone fleet sweeps by an orchestrator that does not write BuildFlow evidence). At that point `compliance.Record` (go-compliance/reporter.go:62) becomes the one-liner that keeps the repo visible to the fleet dashboard, and adoption flips to yes. Also reconsider if the hub's floor drops to minor-form `go 1.27`, removing the floor-poisoning objection for any hub-side adoption.

## Verify block

```bash
rg 'go-dag-app|go-linter-sdk|go-compliance' go.mod go.sum                  # exit 1 (absent)
cd /home/lars/projects/go-compliance && git tag --list | sort -V | tail -3 # v0.1.0, v0.2.0
git log --oneline -13; git log --oneline v0.2.0..HEAD | wc -l              # 10 past tag; HEAD 01028e6
cat go.mod; head -8 hub/go.mod                                             # core vs hub floors (1.27 vs 1.27.2)
head -60 CHANGELOG.md; head -60 TODO_LIST.md                               # release intent + blocked publish rows
head -50 README.md                                                         # "built, pre-release", integration goal
grep -n '^func \|^type ' reporter.go source.go state.go v1.go fold.go evidence.go adapter.go
cd /home/lars/projects/go-version-auto-configure && grep -n 'ProviderFromSpec\|toolsdk.Register' pkg/provider/provider.go  # :46, :67
```
