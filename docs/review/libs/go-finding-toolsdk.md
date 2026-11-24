# github.com/larsartmann/go-finding/toolsdk — utilization review

Reviewed: 2026-11-23 | Consumer: go-version-auto-configure @ 07d4d51 | Library: github.com/larsartmann/go-finding/toolsdk pinned v1.16.0, latest local tag toolsdk/v1.16.0 @ HEAD 5f568e0 | Provenance: Group A.

(One daemon auto-commit, 9474ea0, touched only `scripts/citation-sweep.py` after the stamped HEAD; citations re-verified against the working tree.)

## Verdict

**ADOPTED.**

This is the intended integration, executed at parity: package-level `var Provider = toolsdk.Register(...)` (the blank-import contract), Trigger/HealthCheck/Options layered on top of the linter-autoconfigure-sdk bridge, dry-run and options context honored in the repair/detect closures, and the registry presence test-pinned. The two real findings are an under-declared `Inputs` read set (the one consequential gap — BuildFlow under-triggers on flake.nix/workflow changes) and a hand-rolled bool read that the SDK's typed accessor does better. Neither changes the shape: the SDK is being used the way it was designed to be used.

## Adoption score

Capability inventory: **N = 19** rows. Not-Applicable rows: **8**. Applicable **A = 19 − 8 = 11**.
Fully Leveraged = 7 (×1.0), Partially Used = 2 (×0.5), Missed Opportunity = 2 (×0), Misused = 0.
Σ = 7 + 1.0 + 0 = **8.0** → score = round(100 × 8.0 / 11) = **73 / 100 — high band (70–84)**.
Published inputs: **7 of 11 applicable capabilities fully leveraged.**

## Version currency

- Consumer pin: `github.com/larsartmann/go-finding/toolsdk v1.16.0` (go.mod:6, direct).
- Latest local tag: **toolsdk/v1.16.0** (`git tag --list 'toolsdk/*'` → …, toolsdk/v1.15.0, toolsdk/v1.16.0). **At parity — no action.**
- Consistency note: toolsdk v1.16.0's own go.mod requires `github.com/larsartmann/go-finding v1.15.1` (go-finding/toolsdk/go.mod:5) — so the consumer's root-pin v1.15.1 + toolsdk-pin v1.16.0 is exactly the version lattice the library ships with. Nothing to reconcile.
- toolsdk diff v1.15.1→v1.16.0 exists in the repo (`git log v1.15.1..v1.16.0 -- toolsdk`) but the module is consumed at its own tag; no newer toolsdk tag exists locally.

## Capability inventory

| Capability | Status | Consumer evidence | Library evidence |
| --- | --- | --- | --- |
| `Spec` + `Register` self-registration contract (panic-on-invalid at startup) | Fully Leveraged | pkg/provider/provider.go:40 (`var Provider = mustProvider()`), :45-68, :67 (`toolsdk.Register(spec)`); registry presence asserted at pkg/provider/provider_test.go:28-38 | go-finding/toolsdk/spec.go:16; go-finding/toolsdk/registry.go:31, :89-118 |
| `Trigger` declaration + `OnFiles` constructor + language vocabulary (`ValidLanguages`, `Trigger.Validate`) | Fully Leveraged | pkg/provider/provider.go:57 (`toolsdk.OnFiles("go", "go", "go.mod", "go.work")`); test asserts Language/Files at pkg/provider/provider_test.go:25-26 | go-finding/toolsdk/spec.go:97-173; go-finding/toolsdk/triggers.go:27 |
| Go trigger presets `OnGoFiles`/`OnGoModule` | Not Applicable | — | Reason: presets match `**/*.go`-shaped linters; this tool triggers on module/workspace manifest files plus a generic "go" pattern, which only `OnFiles` expresses (pkg/provider/provider.go:57). go-finding/toolsdk/triggers.go:7, :17 |
| `ModuleFanOut` (run once per Go module) | Not Applicable | — | Reason: Discover intentionally analyzes the whole repository surface in one pass — the workspace floor and full module floor span modules by definition (docs/DOMAIN_LANGUAGE.md "Full module floor"; pkg/surface/surface.go), so per-module fan-out would fragment the very state the tool computes. go-finding/toolsdk/spec.go:45 |
| `SwitchCases` conditional repair paths | Not Applicable | — | Reason: single repair path by design — the dep-forced branch is computed inside analyze/repair (pkg/provider/provider.go:104, :152-157), not split into DAG nodes. go-finding/toolsdk/spec.go:58, :181 |
| `DependsOn` DAG ordering | Not Applicable | — | Reason: no ordering relationship with other BuildFlow tools; the version-surface scan is standalone. go-finding/toolsdk/spec.go:29 |
| `Inputs` read-set declaration | Partially Used | Only `go.mod` declared, inherited from the bridge's `ConfigFile` (pkg/provider/provider.go:49 → linter-autoconfigure-sdk/autoconfigure.go:452); Discover actually reads go.work + flake.nix (pkg/surface/discover.go:92-94) and `.github/workflows/*.y{a,}ml` (pkg/surface/discover.go:485-486), and the consumer never overrides `spec.Inputs` after the bridge (pkg/provider/provider.go:46-67) | go-finding/toolsdk/spec.go:60-62 |
| `Outputs` write-contract | Missed Opportunity | Absent: repair rewrites go.mod/go.work/CI workflow files (pkg/provider/provider.go:172-203 → pkg/fix) but declares no `Outputs`, so hosts cannot order readers after it or key result caches on its write set | go-finding/toolsdk/spec.go:64-71 |
| `Option` declaration + declaration validation (kinds, defaults, duplicate panic) | Fully Leveraged | pkg/provider/provider.go:59-65 (`respect_patch_floor`, bool, default false, description); declaration pinned at pkg/provider/provider_test.go:222-239 | go-finding/toolsdk/options.go:44-92; enforced at go-finding/toolsdk/registry.go:102-113 |
| Typed option accessors (`BoolFromContext`/`StringFromContext`/`IntFromContext`) | Missed Opportunity | Hand-rolled type assertion instead: pkg/provider/provider.go:218-221 (`values[providerOptionRespectPatchFloor].(bool)`; a wrong-typed value silently reads as unset rather than erroring) | go-finding/toolsdk/options.go:142, :149, :156-158 |
| Options context plumbing (`WithOptions`/`OptionsFromContext`, snapshot semantics) | Fully Leveraged | pkg/provider/provider.go:213 (read); set in tests at pkg/provider/provider_test.go:259 | go-finding/toolsdk/options.go:109, :123 |
| `Spec.ValidateOptions` (consumer-side value checking) | Not Applicable | — | Reason: that is the executing host's job (BuildFlow validates values against declarations before the tool runs); this repo authors specs, it does not execute foreign ones. go-finding/toolsdk/options.go:185-200 |
| Dry-run context contract (`WithDryRun`/`DryRunFromContext`) | Fully Leveraged | pkg/provider/provider.go:197 (`fix.Options{DryRun: toolsdk.DryRunFromContext(ctx)}`); behavior pinned at pkg/provider/provider_test.go:122-138 | go-finding/toolsdk/dryrun.go:10, :17 |
| `HealthCheck` (external-dependency gate) | Fully Leveraged | pkg/provider/provider.go:58 (`spec.HealthCheck = fix.SelfCheck`; implementation at pkg/fix/fix.go:131-147) | go-finding/toolsdk/spec.go:90-92 |
| `RepairResult` provenance channel (`ModifiedFiles` + `Facts`) | Partially Used | Only `Description` flows: repair returns `res.Report()` prose (pkg/provider/provider.go:202) and the bridge discards structure (linter-autoconfigure-sdk/autoconfigure.go:457-465), although `fix.Result` already holds `Applied`/`HeldBack`/`DepForced` per file (pkg/fix/fix.go:33-44) | go-finding/toolsdk/spec.go:224-237, :206, :232, :236 |
| `All()` discovery (BuildFlow startup enumeration) | Fully Leveraged | pkg/provider/provider_test.go:28-37 (asserts the provider is in the process-global registry) | go-finding/toolsdk/registry.go:54-58 |
| Registry test isolation (`ResetForTest`/`SnapshotForTest`/`RestoreForTest`) | Not Applicable | — | Reason: consumer tests only scan `All()` and never mutate the process-global registry, so isolation helpers have nothing to isolate. go-finding/toolsdk/registry.go:62, :74, :81 |
| `EnsureContext` nil-context guard | Not Applicable | — | Reason: the bridge already normalizes nil contexts around the consumer's closures (linter-autoconfigure-sdk/autoconfigure.go:460-465); the provider's Detect/Repair always receive non-nil ctx from BuildFlow and the CLI. go-finding/toolsdk/registry.go:123 |
| `RaceEnabled` build-tag constant | Not Applicable | — | Reason: exists for tests that must skip under `-race`; the consumer has no race-conditional behavior. go-finding/toolsdk/race_on.go / race_off.go |

## Gap analysis / ranked opportunities

Sorted by impact × ease, descending.

### 1. Declare the full `Inputs` read set (impact: high, ease: high — one line)

What the library offers: `Spec.Inputs` — "the file patterns this tool reads. Used to derive data-flow edges and to gate the tool on file presence" (go-finding/toolsdk/spec.go:60-62). The bridge seeds it from `ConfigFile` only (linter-autoconfigure-sdk/autoconfigure.go:452), and `ConfigFiles` would be the wrong tool here (it also means "discovery candidates" for config-file tools).

What the consumer does instead: declares only `go.mod` (pkg/provider/provider.go:49) while `surface.Discover` also walks go.work, flake.nix, `.github/workflows/*.yml|*.yaml`, VERSION and CHANGELOG.md (pkg/surface/discover.go:92-94, :485-486). Consequence: a flake.nix pin change or a workflow edit does not re-trigger the tool in hosts keying on Inputs — stale findings exactly where the drift lives.

Recommended usage (layered the same way `Trigger` already is, pkg/provider/provider.go:57):

```go
spec.Trigger = toolsdk.OnFiles("go", "go", "go.mod", "go.work")
spec.HealthCheck = fix.SelfCheck
spec.Inputs = []string{
    "go.mod", "go.work", "flake.nix",
    ".github/workflows/*.yml", ".github/workflows/*.yaml",
    "VERSION", "CHANGELOG.md",
}
```

### 2. Declare `Outputs` for the repair write set (impact: medium, ease: high — one line)

What the library offers: `Spec.Outputs` — the tool's write contract; hosts use it for producer ordering and result-cache keying (go-finding/toolsdk/spec.go:64-71).

What the consumer does instead: nothing — repair mutates go.mod/go.work/CI workflow files (pkg/provider/provider.go:172-203; pkg/fix) with an empty Outputs, so hosts must treat it as write-invisible.

```go
spec.Outputs = []string{"go.mod", "go.work", ".github/workflows/*.yml", ".github/workflows/*.yaml"}
```

### 3. Read the option through `BoolFromContext` (impact: low-medium, ease: trivial)

What the library offers: `toolsdk.BoolFromContext(ctx, name, fallback)` (go-finding/toolsdk/options.go:156-158) — same miss-yields-fallback semantics plus a typed `ErrOptionKindMismatch` error path (go-finding/toolsdk/options.go:21-23).

What the consumer does instead: a raw map type assertion (pkg/provider/provider.go:218-221) in which a wrongly-typed config value silently behaves as unset — the exact "config typo fails loudly" failure the SDK's option sentinels exist to prevent.

```go
respect, err := toolsdk.BoolFromContext(ctx, providerOptionRespectPatchFloor, false)
if err != nil {
    return nil, fmt.Errorf("%s: %w", toolName, err)
}
if !respect {
    return nil, nil
}
```

### 4. Populate `RepairResult.ModifiedFiles`/`Facts` from `fix.Result` (impact: medium, ease: medium — needs a bridge passthrough)

What the library offers: the structured provenance channel on `RepairResult` — `ModifiedFiles` ("informational: no consumer gates on the list") and `Facts` key/value pairs such as `"baseline-failed" = "true"` (go-finding/toolsdk/spec.go:224-237, :206), explicitly designed so hosts "surface context without string-matching Description".

What the consumer does instead: collapses `fix.Result` (which already carries `Applied []surface.Fix` with `.File`, `HeldBack`, `DepForced []DepForced` — pkg/fix/fix.go:33-44) into the `res.Report()` prose string (pkg/provider/provider.go:202); the bridge then wraps only the string (linter-autoconfigure-sdk/autoconfigure.go:457-465). A dep-forced count as a Fact and the rewritten files as ModifiedFiles would give BuildFlow's fix-measurement view real structure.

Recommended usage (blocked on the bridge's `ProviderSpec.Repair` returning only a string — see fleet argument):

```go
// once linter-autoconfigure-sdk's ProviderSpec.Repair can return
// (description, files, facts) or the provider wires RepairerFunc directly:
res, err := fix.Apply(ctx, root, fixes, fix.Options{DryRun: toolsdk.DryRunFromContext(ctx)}, nil)
return toolsdk.RepairResult{
    Description:   res.Report(),
    ModifiedFiles: appliedFiles(res),               // from res.Applied[i].File
    Facts: []toolsdk.Fact{
        {Key: "dep-forced-count", Value: strconv.Itoa(len(res.DepForced))},
        {Key: "dry-run", Value: strconv.FormatBool(dryRun)},
    },
}, nil
```

## Fleet argument

The owner's framing: many projects, limited time — every re-implementation in a fleet repo is a bug that has to be fixed N times, and no project benefits when the same plumbing is rebuilt per repo.

What this consumer gains from toolsdk's shared surface:

- **BuildFlow integration as configuration, not code.** Registration, trigger vocabulary, option declarations, dry-run and health-check semantics are all contract fields the host consumes (go-finding/toolsdk/spec.go:16-93). Every host-side improvement to how specs are driven — fan-out wrappers, option validation, cache semantics — reaches this repo with zero code changes.
- **The self-registration contract.** `var Provider = toolsdk.Register(spec)` (pkg/provider/provider.go:40, :67) is the same blank-import pattern as the sibling auto-configurers; a new BuildFlow feature that enumerates `All()` picks this tool up without any per-repo wiring.
- **Canonical context keys.** Options and dry-run ride shared context keys (go-finding/toolsdk/options.go:94-95, dryrun key in go-finding/toolsdk/dryrun.go) — the repo writes no key-convention code of its own to drift from the fleet.
- **A validated spec at startup.** `Register`'s panic-on-invalid contract (go-finding/toolsdk/registry.go:89-118) means a malformed Trigger or Option declaration fails at import time, not mid-pipeline — the consumer gets that enforcement for free.

What it should push upstream instead of hand-rolling:

1. **Structured `RepairResult` passthrough in linter-autoconfigure-sdk** (gap #4): `ProviderSpec.Repair` returning only `string` is the single thing stranding `ModifiedFiles`/`Facts`; growing the bridge fixes every auto-configurer in the fleet at once.
2. **An `Inputs` escape hatch on the bridge** (gap #1): a `ProviderSpec.Inputs []string` override (empty = derive from ConfigFiles, as today) would let file-surface tools like this one declare their true read set without post-mutating the returned Spec the way pkg/provider/provider.go:57 already does for Trigger — turning a local convention into a supported one.
3. **Host-side consumption of run-level facts** (with the root-module report's SARIF/propertyBag items): dep-forced counts are fleet telemetry; if BuildFlow surfaced `Facts` per run, the poisoner campaign's progress (docs/POISONERS.md, ADR-0001) would be measurable from pipeline data instead of per-repo sweeps.

## RECONSIDER-WHEN

- A stale-finding or missed-rerun incident in BuildFlow is traced to a flake.nix/workflow change → gap #1 (Inputs) becomes an incident fix; apply same-day, it is one line.
- BuildFlow documents `Outputs`-keyed result caching or producer ordering as load-bearing → gap #2 flips from nice-to-have to required for cache correctness.
- A second tool option is added (e.g. an `expect_minor` string option mirroring the CLI's `--expect-minor`) → adopt `BoolFromContext`/`StringFromContext` as part of that change (gap #3), so kind mismatches fail loudly from the first day.
- linter-autoconfigure-sdk grows `RepairResult`/`Inputs` passthroughs → gaps #4/#1 collapse to local one-liners and the score re-rates toward very high.
- BuildFlow starts consuming `ModuleFanOut` semantics for version-surface tools (a repo-wide floor registry, say) → re-examine the ModuleFanOut N/A; until the floor model fragments per module, it stays off.

## Verify block

Commands run (consumer at /home/lars/projects/go-version-auto-configure, library at /home/lars/projects/go-finding):

```bash
# usage sites
rg -n 'larsartmann/go-finding' /home/lars/projects/go-version-auto-configure --type go
rg -n 'toolsdk\.[A-Z]' /home/lars/projects/go-version-auto-configure --type go -o | sort | uniq -c
cat go.mod                                          # toolsdk v1.16.0 direct (line 6)

# library state + tags
cd /home/lars/projects/go-finding && git log --oneline -5
git describe --tags --exact-match HEAD              # → v1.16.0
git tag --list 'toolsdk/*' | tail -5                # → … toolsdk/v1.16.0
git log --oneline v1.15.1..v1.16.0 -- toolsdk
cat go-finding/toolsdk/go.mod                       # requires go-finding v1.15.1

# public surface
cd /home/lars/projects/go-finding/toolsdk && \
  GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2 go doc -all . | rg '^(type|func|const|var) ' | sort -u

# citation spot-checks (file:line)
grep -n 'ModuleFanOut\|SwitchCases\|Inputs \[\]string\|Outputs \[\]string\|HealthCheck\|type Spec\|type Trigger' \
  /home/lars/projects/go-finding/toolsdk/spec.go
grep -n 'type RepairResult\|ModifiedFiles\|Facts \[\]Fact\|type Fact' /home/lars/projects/go-finding/toolsdk/spec.go
grep -n 'func Register\|func All\|func ResetForTest\|func SnapshotForTest\|func RestoreForTest\|func EnsureContext' \
  /home/lars/projects/go-finding/toolsdk/registry.go
grep -n 'func BoolFromContext\|func StringFromContext\|func IntFromContext\|func WithOptions\|func OptionsFromContext\|func (s Spec) ValidateOptions\|ErrOptionKindMismatch' \
  /home/lars/projects/go-finding/toolsdk/options.go
grep -n 'func WithDryRun\|func DryRunFromContext' /home/lars/projects/go-finding/toolsdk/dryrun.go
grep -n 'func OnGoFiles\|func OnGoModule\|func OnFiles\|func AnyLanguage' /home/lars/projects/go-finding/toolsdk/triggers.go

# consumer side (read the cited lines in full)
# pkg/provider/provider.go:40-67, :95-99, :144-167, :172-203, :212-236, :241-247
# pkg/provider/provider_test.go:18-38, :122-138, :222-239, :246-268
# pkg/fix/fix.go:33-44 (Result), :131-147 (SelfCheck)
rg -n 'go.mod", name == "go.work"|flake.nix|isCIWorkflow|\.yml' /home/lars/projects/go-version-auto-configure/pkg/surface/discover.go

# bridge behavior (Inputs seeding, Repair wrapping, EnsureContext)
rg -n 'ProviderFromSpec|discoveryCandidates|RepairerFunc|EnsureContext|converted.Inputs' \
  ~/go/pkg/mod/github.com/larsartmann/linter-autoconfigure-sdk@v0.8.1/autoconfigure.go

# consumer HEAD sanity (daemon moved it past the stamped sha)
git -C /home/lars/projects/go-version-auto-configure log --oneline -3
git -C /home/lars/projects/go-version-auto-configure show --stat 9474ea0
```

Every file:line cited above was read in the working tree during this audit (view/grep on the exact line), not inferred.
