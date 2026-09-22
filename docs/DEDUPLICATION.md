# Duplication Baseline

Reference for future `art-dupl` runs: the accepted, intentional similarity this
repo tolerates, so reports can be diffed against a known list instead of
re-litigated every sweep.

Invocation (run RAW first; type-aware mode fails transiently if the tree does
not compile):

```bash
art-dupl --sort total-tokens -t 1 --type-aware
```

## History

- 2026-09-22 (dedup session): two clone groups found; `readLines` extracted in
  `pkg/surface/discover.go`, and the check/fix/who-forces flag skeleton was
  first deduplicated locally (`runFlags`/`newRunFlagSet`/`parseRoots`).
- 2026-09-22 (cmdguard migration): the whole skeleton class was deleted by
  migrating `cmd/` to `github.com/larsartmann/cmdguard/v4`. Post-migration
  `-t 1` scan: zero harmful clones in `cmd/`; the only remaining `cmd/`
  similarity is the two-line `case` tail shared by the three
  `exitFrom*` functions (intentional — each maps a different result type onto
  the 0/1/2 contract).

## Accepted groups (as of 2026-09-22, `-t 1`)

All remaining groups are 2-line intentional similarity, none actionable:

| Where | Shape | Why accepted |
|-------|-------|--------------|
| `cmd/` `exitFromAnalyses`/`exitFromOutcomes`/`exitFromFloors` tails | `switch { case errored/fails: … case findings/poisoned: … }` | Each maps a distinct result type onto the shared exit contract; forcing them through one abstraction would hide the per-command semantics this file is meant to show. |
| `pkg/surface/surface.go` version-collection appends | `versions = append(…)` | Two lines over different types (`m.Version` vs `tc.Version`). |
| `pkg/surface/rules.go` minor-exceeds guards | identical guard over different vertex types | Table-driven candidate, but the types differ; a helper would need generics for two call sites. |
| `pkg/surface/discover.go` toolchain nil-guards | `if toolchain != nil` twice | Distinct handling bodies; shared only in the guard token. |
| `pkg/surface/directive.go` default branches | `default:` switch tails | Trivial. |
| (138 further non-actionable groups at higher thresholds) | — | Below noise floor. |

Rule of thumb: clone reports are lower bounds — flag-order or type differences
hide real duplicates (the who-forces pool was a third instance art-dupl never
flagged). After any refactor, scan for the pattern manually before declaring
victory.
