# go-linter-sdk — utilization review

| | |
|---|---|
| Reviewed | 2026-11-23 |
| Consumer | go-version-auto-configure @ 07d4d51 |
| Library | github.com/larsartmann/go-linter-sdk — not a dependency, latest local tag v0.6.1 @ HEAD 1694dcb |
| Provenance | Group C |
| Format (M17, 2026-11-24) | Compact grade format declared canonical for NOT-ADOPTED reviews — the four-column inventory below carries the evidence |

## Verdict

**NOT-ADOPTED (justified).** No split-brain risk — that was the first thing checked, and it clears: go-linter-sdk is not a competing findings model. It is a scaffolding layer *on top of* go-finding (go-linter-sdk/go.mod:5 requires go-finding v1.15.1, the exact version this consumer pins; go-linter-sdk/README.md:3 "building linters that plug into a finding-based ecosystem"; go-linter-sdk/README.md:136-171 shows its own BuildFlow recipe routes through `toolsdk.Register` — the same terminal API this consumer's provider already reaches). Adopting it would be additive to the adopted stack, not a replacement.

The justification for not adopting is shape, not allegiance: go-linter-sdk packages the *rule interface + registry + directory-walking execution* tier (`RuleFunc.Check(ctx context.Context, dir string)`, go-linter-sdk/rule.go:306; `Registry.Run`, go-linter-sdk/registry.go:218). This consumer's check tier is fundamentally different: one pure function `surface.Analyze(surf *Surface, opts ...) []Issue` (pkg/surface/rules.go:30) over a discovered version surface, single-pass, shared by detection AND fix verification (the "fix is checked with exactly the code that detected it" contract, AGENTS.md). Wrapping `Analyze` as a `RuleFunc` would add a registry, an `ID()/Name()/Category()` ceremony layer, and a `(ctx, dir)` walk the domain does not have (Discover already walked; rules run over the in-memory `Surface`). No hand-rolled registry, no per-rule dispatch, no converter — the three things the SDK eliminates do not exist here.

## Adoption score

Rows below: 6, N/A: 4 → A = 2. The 2 applicable rows grade what the consumer does instead. `Analyze` over a pre-built Surface covers finding emission + exit-code semantics fully in its own idiom (Fully for capability 5 is generous but honest: findings ARE go-finding-shaped downstream via the provider bridge, and exit codes are the cmd/ contract). Score = round(100 × (0.5 + 1) / 2) = **75 / 100** — band **high**. The consumer is not missing capability; it just serves the last mile differently.

| # | Capability | Grade |
|---|---|---|---|
| 1 | `Rule` interface with identity/validation | N/A — rules are named `Rule` string constants + plain helper funcs, no dynamic rule set |
| 2 | `Registry` (register/get/enable-disable/run) | N/A — one fixed policy function, nothing to register |
| 3 | Detector adapters to go-finding (`DetectorFromRegistry`, `DetectV2`) | N/A — provider path goes through linter-autoconfigure-sdk `ProviderFromSpec` → `toolsdk.Register` instead (pkg/provider/provider.go:46, :67) |
| 4 | `RuleSet`/`Filter` enable-disable plumbing | N/A — policy knobs are `AnalyzeOption`s (`WithExpectedMinor`, `WithRespectPatchFloor`, pkg/surface/rules.go:77, :97) |
| 5 | Findings emitted as `finding.Finding` (no converter) | Fully — consumer emits its own `Issue` in pkg/surface but the provider bridge converts once at the boundary into go-finding findings (pkg/provider/provider.go:46-67); the SDK's zero-converter ideal is met one layer up |
| 6 | Exit-code mapping | Fully — consumer's own contract: 0 clean / 1 findings / 2 hard errors, locked in cmd/go-version-auto-configure/contract_test.go:17 and depth tests |

## Version currency

- Latest tag **v0.6.1** (2026-11-20), 9 tags total — healthy cadence. HEAD 1694dcb (2026-11-22) is 8 commits past, all docs/tooling-tier auto-commits; the last one fixes dangling DOMAIN_LANGUAGE citations (1694dcb "docs: fix dangling code citations in DOMAIN_LANGUAGE"). No exported-API drift since the tag.
- CHANGELOG `[0.6.1] - 2026-11-19` documents the slot decision as PATCH ("no exported SDK surface changed") — disciplined semver hygiene. README status line: "Early, adopted, proven", 100% library coverage measured 2026-11-19, six fleet repos already import it.
- Maturity read: this is the *most production-mature* of the three audited libs — real consumers, measured coverage, release discipline. Not adopting it is a shape decision, not a quality call.

## Capability inventory

| Capability | Status | Consumer evidence | Library evidence |
|---|---|---|---|
| `Rule` interface: ID/Name/Category/Severity/Check + `RuleFunc` struct literal | N/A — no dynamic rule enumeration; policy is one function | pkg/surface/rules.go:30 (`Analyze`); pkg/surface/surface.go:39-76 (branded `Rule` constants) | go-linter-sdk/rule.go:115 (`Rule`), :158 (`RuleFunc`), :169 (`RuleMeta` + `Validate` :216) |
| Opt-in rule wrapper | N/A — rule enablement is flags (`--respect-patch-floor`, `--expect-minor`) | pkg/surface/rules.go:77-95, :97-111 | go-linter-sdk/rule.go:275 (`OptIn`) |
| `Registry` with register/get/deregister + `Run` (fail-fast or `ContinueOnError`) | N/A — nothing to registry-drive | pkg/surface/rules.go:42-62 (fixed rule list inside `Analyze`) | go-linter-sdk/registry.go:39 (`NewRegistry`), :218 (`Run`), :194 (`ContinueOnError`) |
| go-finding Detector adapters, `DetectV2` coverage evidence | N/A — BuildFlow wiring via ProviderSpec bridge | pkg/provider/provider.go:46 (`autoconfigure.ProviderFromSpec`), :67 (`toolsdk.Register`) | go-linter-sdk/registry.go:294 (`DetectorFromRegistry`), :319 (`DetectV2`), :362 (`DetectorsFromRegistry`) |
| Exit-code mapping from report/findings | Fully — own cmdguard-integrated contract | cmd/go-version-auto-configure/contract_test.go:17 (exit contract comment), cmd/go-version-auto-configure/json.go (wire DTOs) | go-linter-sdk/registry.go:415 (`ExitCodeFromReport`), :426, :512 (`ExitCodeByConfidence`) |
| Rule enable/disable selection | N/A — flags not rule IDs | pkg/surface/rules.go:77, :97 (`AnalyzeOption`s) | go-linter-sdk/registry.go:450 (`FilterRules`), :479 (`SelectRules`); go-linter-sdk/filter.go:17 (`Filter`) |
| Findings emitted as `finding.Finding` (zero converter) | Fully (own idiom, single boundary conversion) | pkg/surface/surface.go:301 (`Issue`); conversion once in the provider bridge | go-linter-sdk/rule.go:306 (`Check` returns `[]finding.Finding`), :329 (`NewFinding` builder) |
| RuleSet grouping + batch Check | N/A — no grouped sub-runs | — | go-linter-sdk/ruleset.go:33 (`NewRuleSet`), :98 (`Check`) |

## Gap analysis / ranked opportunities

Sorted by impact × ease, descending — all three net negative for THIS consumer:

1. **Wrap `Analyze` as one `RuleFunc` + `DetectorFromRegistry` to enter the go-finding/pipeline path** — Impact: low (the consumer already reaches toolsdk via the linter-autoconfigure-sdk bridge; BuildFlow gets its Spec either way). Ease: medium (must re-shape `Analyze`'s option policy into `RuleMeta` fields or hide it inside one rule). Cost: a new dependency whose only job is to re-wrap a function the consumer already wraps once. **Skip.**
2. **Use `Registry` for per-rule enable/disable** (`--only go-directive-patch-form`-style CLI). Impact: moderate for fleet users, but current UX keys off flags mapped to policy options, and the rule set is 8 constants — a `SelectRules` call would be cosmetic. Ease: high. Cost: dep floor tracking (go-linter-sdk itself requires go 1.27 + go-finding — floor-compatible today) and a second way to express the same selection. **Skip until users ask for rule-ID CLI selection.**
3. **Adopt `ExitCodeFromReport`/`ExitCodeByConfidence`** — Impact: zero (consumer's exit contract is richer: `fix` exits 0 on all-dep-forced, `who-forces` has a poisoned exit; AGENTS.md). Ease: n/a. **Skip — adopting would WEAKEN the contract.**

Cost side of any adoption: dependency floor `go 1.27` + `go-finding v1.15.1` (go-linter-sdk/go.mod:5) — compatible with this repo's floor policy today, but it adds a fourth SDK-ish module (go-finding, toolsdk, linter-autoconfigure-sdk, go-linter-sdk) to the floor-poisoning ledger for capabilities the consumer does not exercise.

## Fleet argument

The framing cuts both ways here, and honesty requires stating both:

- **What adoption would remove:** nothing. The SDK removes (a) rule interfaces, (b) registries, (c) converters. The consumer has none of the three — its "registry" is a fixed function body (pkg/surface/rules.go:42-62) and its conversion is the single deliberate bridge in pkg/provider/provider.go:46-67 that ALSO encodes the dep-forced annotation behavior (that logic is not SDK-expressible).
- **What adoption would add:** a second description of "how this tool detects things". The consumer's six sibling autoconfigurers and BuildFlow already share the linter-autoconfigure-sdk + toolsdk composition — that IS the fleet's shared layer for this shape of tool. go-linter-sdk's shared layer serves a different shape: standalone `Check(ctx, dir)` linters (branching-flow, erraudit, go-structure-linter per go-linter-sdk/README.md:19-27). Two shared layers, two tool shapes; importing both in one repo would make it the only repo carrying both.
- If go-version-auto-configure ever spawns sub-linters that walk directories independently (e.g. the deferred flake.lock parse, TODO_LIST T9, as an opt-in BuildFlow step), THAT is a shape match for `RuleFunc` + `DetectorFromRegistry` — but it would live in its own module, not this one.

## RECONSIDER-WHEN

Any of: (a) the rule set grows enough that users demand rule-ID-level CLI selection (`--enable/--disable` per `surface.Rule` constant) — then `SelectRules`/`Filter` (go-linter-sdk/registry.go:479) plus a `Registry` becomes genuinely cheaper than hand-rolling flag plumbing; (b) a second detector module (flake.lock pin analysis, T9) is built that wants per-rule parallelism via go-finding/pipeline — then `DetectorsFromRegistry` is the right bus; (c) the fleet converges the autoconfigurers themselves onto go-linter-sdk — follow the fleet, do not stay solo.

## Verify block

```bash
rg 'go-dag-app|go-linter-sdk|go-compliance' go.mod go.sum                                   # exit 1 (absent)
cd /home/lars/projects/go-linter-sdk && git tag --list | sort -V | tail -4                   # v0.4.1..v0.6.1 (9 total)
git log --oneline -12; git log --oneline v0.6.1..HEAD | wc -l                                # 8 past tag; HEAD 1694dcb
git log -1 --format='%ci' v0.6.1                                                            # 2026-11-20
cat go.mod                                                                                   # go-finding v1.15.1, go 1.27
grep -n '^func \|^type ' rule.go registry.go ruleset.go filter.go errors.go
head -80 README.md; sed -n 136,180p README.md; head -20 CHANGELOG.md
grep -n 'go-finding\|toolsdk' README.md docs/DOMAIN_LANGUAGE.md
cd /home/lars/projects/go-version-auto-configure && sed -n 1,130p pkg/surface/rules.go
grep -n 'type Issue' pkg/surface/surface.go                                                  # :301
grep -n 'ProviderFromSpec\|toolsdk.Register' pkg/provider/provider.go                        # :46, :67
sed -n 136,171p cmd/go-version-auto-configure/contract_test.go equivalents (exit contract: cmd/go-version-auto-configure/contract_test.go:17)
```
