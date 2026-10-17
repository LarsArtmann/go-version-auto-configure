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
- 2026-10-17 (docs-health re-scan, art-dupl 0.9.0): 228 clone groups, 7 above
  the noise floor (202 non-actionable + 19 filtered/suppressed), every shown
  group 2-3 line intentional similarity. New since 2026-09-22: the
  `nixCommentScanner` per-token writers repeat `out.WriteByte(line[j])` /
  `out.WriteString("''")` (the v0.2.2 split that satisfied gocognit/wsl). No
  harmful clones; the delta is all intentional.

## Accepted groups (as of 2026-10-17, `-t 1`)

All remaining groups are 2-line intentional similarity, none actionable:

| Where                                                               | Shape                                                        | Why accepted                                                                                                                                                          |
| ------------------------------------------------------------------- | ------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/` `exitFromAnalyses`/`exitFromOutcomes`/`exitFromFloors` tails | `switch { case errored/fails: … case findings/poisoned: … }` | Each maps a distinct result type onto the shared exit contract; forcing them through one abstraction would hide the per-command semantics this file is meant to show. |
| `pkg/surface/surface.go` version-collection appends                 | `versions = append(…)`                                       | Two lines over different types (`m.Version` vs `tc.Version`).                                                                                                         |
| `pkg/surface/rules.go` minor-exceeds guards                         | identical guard over different vertex types                  | Table-driven candidate, but the types differ; a helper would need generics for two call sites.                                                                        |
| `pkg/surface/discover.go` toolchain nil-guards                      | `if toolchain != nil` twice                                  | Distinct handling bodies; shared only in the guard token.                                                                                                             |
| `pkg/surface/directive.go` default branches                         | `default:` switch tails                                      | Trivial.                                                                                                                                                              |
| `pkg/surface/discover.go` `nixCommentScanner` token writers         | `out.WriteByte(line[j])`, `out.WriteString("''")`             | The v0.2.2 per-token split satisfied gocognit/wsl; the writes repeat per token type over different branches. Folding them back reintroduces the complexity the split removed. |
| `cmd/` exit-contract state flags + returns                          | `failed/errored = true`, `return exitFindings`              | Same family as the `exitFrom*` tails: three result types onto the 0/1/2 contract.                                                                                    |
| `pkg/surface/rules.go` / `release.go` short guards                  | `if err != nil {`, `if !ok {`                               | Trivial two-line guards over different values; abstraction would obscure.                                                                                            |
| `pkg/provider/provider.go` detect/repair prologue                   | `root := workingDir(ctx)`                                    | Same two-line prologue in two entry points; a helper would thread the same context for no gain.                                                                      |
| (202 further non-actionable groups at higher thresholds)            | —                                                            | Below noise floor.                                                                                                                                                    |

Rule of thumb: clone reports are lower bounds — flag-order or type differences
hide real duplicates (the who-forces pool was a third instance art-dupl never
flagged). After any refactor, scan for the pattern manually before declaring
victory.
