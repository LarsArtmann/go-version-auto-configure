# go-dag-app — utilization review

| | |
|---|---|
| Reviewed | 2026-11-23 |
| Consumer | go-version-auto-configure @ 07d4d51 |
| Library | github.com/larsartmann/go-dag-app — not a dependency, latest local tag v0.1.1 @ HEAD c2b6c99 |
| Provenance | Group C |

## Verdict

**NOT-APPLICABLE.** go-dag-app is a DI-container bootstrap (env-gated debug tree print + `samber/do` injector lifecycle) for DAG applications that build an injector. This consumer has no DI container anywhere: `rg 'samber|do\.Injector'` over cmd/ returns nothing, and its wiring is plain constructor calls (`pkg/provider` package-level `var Provider`, blank-import contract per AGENTS.md). There is nothing for it to replace, and adopting it would add a `samber/do/v2` dependency solely to have a container nobody injects through.

It is also **not** a worker-pool library: the fleet argument below traces that suspicion to ground truth — go-dag-app owns ~100 lines of injector bootstrap (go-dag-app/container.go:75-103), zero concurrency code. The overlap the orchestrator hypothesized (go-dag-app vs cmd/'s worker pools) does not exist.

## Adoption score

Rows below: 5, N/A: 4 → A = 1. Σ = 0 (the one applicable capability is missed). Score = **0 / 100** — band **low**.

| # | Capability | Grade |
|---|---|---|
| 1 | Debug-DI env gate (`<APP>_DEBUG_DI`) | N/A — consumer has no DI tree to debug |
| 2 | Dependency-tree printer (`do.ExplainInjector` wrapper) | N/A — no injector |
| 3 | Injector bootstrap + cleanup (`NewContainer`/`NewContainerWithOpts`) | N/A — no injector |
| 4 | Custom samber/do options pass-through (HEAD commit c2b6c99) | N/A — no injector |
| 5 | Bounded parallel fan-out with sorted deterministic output | **Missed** — but the consumer already implements this itself, see below |

## Version currency

- Latest tag **v0.1.1**; only 2 tags total; initial release 2026-07-18 (go-dag-app/CHANGELOG.md, `[0.1.0] - 2026-07-18`).
- HEAD c2b6c99 is 1 commit past the tag: it adds `NewContainerWithOpts` (go-dag-app/container.go:91) for custom `do.InjectorOpts` — a small API extension, no behavioral change to existing functions.
- CHANGELOG still shows `[Unreleased]` with only docs/config/lint convergence entries — the lib is in steady, slow maintenance. TODO_LIST.md item 3 (go-dag-app/TODO_LIST.md:18-21) asks for a tag "so library-policy can pin a tag"; v0.1.1 has since been cut, so that item looks stale.
- Maturity read: young, tiny (103-line implementation file), 2 known consumers (BuildFlow, library-policy per go-dag-app/container.go:1-4). Pre-1.0.

## Capability inventory

| Capability | Status | Consumer evidence | Library evidence |
|---|---|---|---|
| `<APP>_DEBUG_DI` env-var debug gate | N/A — no DI debug surface exists here | — | go-dag-app/container.go:34-38 (`IsDebugDIEnabled`) |
| Dependency-tree printing (`do.ExplainInjector`) | N/A — no injector to explain | — | go-dag-app/container.go:49-64 (`PrintDependencyTree`) |
| Injector bootstrap with shutdown cleanup | N/A — lifecycle is Go default; exit codes in cmd/main.go | — | go-dag-app/container.go:75-103 (`NewContainer`, `NewContainerWithOpts`) |
| Custom injector options (auditlog plugin hooks) | N/A — no plugin system | — | go-dag-app/container.go:91-103 (HEAD addition) |
| Bounded worker pool, sorted output | Missed (but hand-rolled equivalently) | cmd/go-version-auto-configure/main.go:377-402 (`runSorted` — semaphore + `wg.Go`, `slices.SortFunc` for determinism), cmd/go-version-auto-configure/main.go:439-441 (`workersFor`) | — (go-dag-app has no concurrency API at all) |

## Gap analysis / ranked opportunities

Nothing to rank above trivial. The only theoretical "opportunity" — replacing `runSorted` with a shared pool primitive — is not what go-dag-app offers: it owns DI bootstrap, not concurrency. Adopting it would:

- **Buy:** nothing. The consumer's wiring is `pkg/provider` package-level `var Provider` + blank import (AGENTS.md architecture section) — deliberately container-less. DI would be pure ceremony over 5 packages of pure functions and constructors.
- **Cost:** a new `samber/do/v2 v2.1.0` dependency (go-dag-app/go.mod:5) with its own go.mod floor to track in the floor-poisoning ledger (docs/POISONERS.md governance), plus a split-brain: this fleet's other autoconfigurers do not use DI either.

## Fleet argument

The owner's framing — re-implementation means fixing the same bug N times — applies only when the hand-rolled code duplicates a shared lib. Here the trace shows the opposite:

- **Worker pools:** the consumer's `runSorted` (cmd/go-version-auto-configure/main.go:378) is the *only* pool in the codebase, generic over `I`/`R`, with sorted output as the CLI's determinism contract. go-dag-app contains no pool. Adopting go-dag-app would not remove `runSorted`.
- **DI bootstrap:** go-dag-app deduplicated the bootstrap across BuildFlow and library-policy — two apps that *inject*. This consumer does not inject, so there is no duplicate to deduplicate. Adoption would ADD a second implementation (a container) of something the consumer intentionally does not have.

Verdict on the fleet framing: correct call to skip; nothing is being fixed N times.

## RECONSIDER-WHEN

The consumer grows an actual injector-based component graph — e.g. if pkg/fix's runner abstraction (module-scoped vs workspace-scoped command dispatch) ever becomes a pluggable service set shared with BuildFlow's provider runtime. At that point `dagapp.NewContainer("GO_VERSION_AUTO_CONFIGURE")` replaces a would-be hand-rolled `*_DEBUG_DI` + shutdown scaffold, and adoption flips to trivially yes.

## Verify block

```bash
rg 'go-dag-app|go-linter-sdk|go-compliance' go.mod go.sum                 # exit 1 (absent)
cd /home/lars/projects/go-dag-app && git tag --list | sort -V | tail -3   # v0.1.0, v0.1.1
git log --oneline -5                                                       # HEAD c2b6c99, 1 past tag
git log -1 --format='%ci' v0.1.0                                           # 2026-07-18
cat go.mod                                                                 # requires samber/do/v2 v2.1.0
sed -n 1,103p container.go; head -50 CHANGELOG.md; head -30 TODO_LIST.md
rg -n 'samber|do\.Injector|sync.WaitGroup|worker' cmd/go-version-auto-configure/*.go
sed -n 370,445p cmd/go-version-auto-configure/main.go                      # runSorted / workersFor
```
