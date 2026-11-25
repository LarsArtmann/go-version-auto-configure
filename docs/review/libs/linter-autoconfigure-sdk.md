# linter-autoconfigure-sdk — utilization review

Reviewed: 2026-11-23 | Consumer: go-version-auto-configure @ 07d4d51 | Library: github.com/larsartmann/linter-autoconfigure-sdk pinned v0.8.1 (direct), latest local tag v0.8.1 @ HEAD bfe96d7 | Provenance: Group B.

> Commit-stamp note: the orchestrator stamped 07d4d51; the repo's auto-commit daemon has since moved HEAD to 2499b8d. Every consumer file cited below is byte-identical between the two (`git show 07d4d51:<file>` vs working tree, sha256 compared for pkg/provider/provider.go, pkg/surface/discover.go, pkg/surface/rules.go, pkg/fix/held.go, pkg/fix/fix.go). The linter-autoconfigure-sdk checkout was read strictly read-only (a parallel session develops `feat/audit-subpackage` there).

## Verdict

**UNDERUSED-moderate.** The consumer uses the SDK for exactly its headline purpose — `ProviderFromSpec` builds the BuildFlow Spec and owns finding emission — but leaves three adoptable capabilities on the table (`ConfigFiles`→Inputs, the `FixStrategy` override, the `WorkingDir` helper), and the SDK's bridge itself caps how much value ANY consumer can extract: `ConfigIssue` has no structured-metadata field and `ProviderSpec.Repair` returns only a string, so the consumer's richest data (dep-forced floors, carriers, modified files) is flattened into prose at the provider boundary. Group A's three bottleneck claims are all confirmed against the v0.8.1 tag and unchanged at HEAD (the bridge file is byte-identical between the two).

## Adoption score

Inventory rows N = 11, N/A rows = 6 → A = 5.
Fully = 2, Partial = 1, Missed = 2, Misused = 0.
Σ = 2×1 + 1×0.5 + 2×0 = 2.5 → **score = round(100 × 2.5 / 5) = 50 — moderate band (50–69).**

## Version currency

- **Pin vs latest local tag:** consumer pins `github.com/larsartmann/linter-autoconfigure-sdk v0.8.1` (go.mod:11) — already at the latest local tag `v0.8.1`. (Consumer AGENTS.md:9 still says "v0.8.0" — stale prose, go.mod wins.)
- **What v0.8.1 is:** a dependency-bump-only release (go-atomic-write v0.6.1, "temp-file leak fix"); the tag diff touches only go.mod/go.sum/determinism testdata, and CHANGELOG.md has no `[0.8.1]` section — the entry was never written for this tag.
- **In-flight HEAD delta** (bfe96d7, 7 commits past the tag on `feat/audit-subpackage`, summarized in CHANGELOG.md `[Unreleased]` at linter-autoconfigure-sdk/CHANGELOG.md:7-31):
  - New `audit` subpackage + `cmd/audit` (linter-autoconfigure-sdk/CHANGELOG.md:11-20): golangci-lint config-drift audit — dead exclusion rules, dead settings blocks, dead `nolint` directives; `Checker.Audit` emits `ConfigIssue`s (linter-autoconfigure-sdk/audit/audit.go:60,101).
  - First non-larsartmann direct dependency: `github.com/go-faster/yaml` v0.4.6 (linter-autoconfigure-sdk/CHANGELOG.md:21-23), carried by the audit subpackage only.
  - go-atomic-write → v0.6.2, go-finding → v1.15.1 (HEAD go.mod).
  - **Root-package API is untouched**: `autoconfigure.go`, `diff.go`, `bootstrap.go` are byte-identical between v0.8.1 and HEAD (`git diff v0.8.1..HEAD --stat -- autoconfigure.go diff.go bootstrap.go` is empty).
- **Consumer planning:** a future tag carrying the audit subpackage is purely additive for this consumer (it imports only the root package), adds no new module-graph weight (the consumer already carries go-faster/yaml v0.4.6 indirect via go-output), and the floor stays `go 1.27`. No action needed until a tag lands; nothing in flight changes the bridge.

## Capability inventory

| Capability                                                                                                                  | Status                                                                                                                                                                                                                                            | Consumer evidence                                                                                                                                                            | Library evidence                                                                                           |
| --------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| BuildFlow provider bridge (`ProviderFromSpec` → `toolsdk.Spec`)                                                             | Fully                                                                                                                                                                                                                                             | pkg/provider/provider.go:46-52 (spec), :67 (`toolsdk.Register`)                                                                                                              | linter-autoconfigure-sdk/autoconfigure.go:437-468                                                          |
| Finding emission (`ConfigIssue` → `FindingFromIssue`/`FindingsFromIssues`, auto `FixStrategySuggest`/`None`)                | Fully                                                                                                                                                                                                                                             | pkg/provider/provider.go:92-101, :159-166 (issues built; conversion runs inside the bridge)                                                                                  | linter-autoconfigure-sdk/autoconfigure.go:244-349, :525-532                                                |
| `ConfigIssue.FixStrategy` override (`FixStrategyDirect` for directly-applicable repairs)                                    | Partial                                                                                                                                                                                                                                           | pkg/provider/provider.go:159-166 — never sets `FixStrategy`; mechanical fixes fall through to suggest                                                                        | linter-autoconfigure-sdk/autoconfigure.go:273-277 (field), :312-319 (selection)                            |
| Multi-candidate discovery → `Inputs` (`ConfigFiles`)                                                                        | Missed                                                                                                                                                                                                                                            | pkg/provider/provider.go:49 sets `ConfigFile: "go.mod"` only, while Discover also reads go.work/flake.nix/CI workflows/VERSION/CHANGELOG.md (pkg/surface/discover.go:90-108) | linter-autoconfigure-sdk/autoconfigure.go:367-374 (field), :452-454 (Inputs derivation), :479-494          |
| `WorkingDir(ctx)` helper                                                                                                    | Missed                                                                                                                                                                                                                                            | pkg/provider/provider.go:241-247 hand-rolls the identical fallback                                                                                                           | linter-autoconfigure-sdk/autoconfigure.go:230-240 (doc at :228-229 says exactly "instead of hand-rolling") |
| JSON config I/O suite (`ReadConfig`/`LoadJSON`/`ParseJSON`/`MarshalJSONIndented`/`SaveJSON`/`SaveJSONBytes`, `ConfigError`) | N/A — the consumer's targets are go.mod/go.work/CI-YAML **byte-preserving text surgery**, not JSON configs; it already uses the underlying go-atomic-write primitive directly (pkg/fix/fix.go:384, pkg/fix/ci.go:39, pkg/fix/canonicalize.go:304) | —                                                                                                                                                                            | linter-autoconfigure-sdk/autoconfigure.go:99-220                                                           |
| Config diff engine (`Change`, `DiffMaps`/`DiffSets`/`DiffBlobs`, `Summary`, `FormatDiff`)                                   | N/A — no before/after config-compare need; drift is judged against the go tool's directive ranking (`CompareDirective`, docs/DOMAIN_LANGUAGE.md:11), not by diffing two config blobs                                                              | —                                                                                                                                                                            | linter-autoconfigure-sdk/diff.go:32, :43, :71, :103, :126, :147                                            |
| `FirstExisting(root, candidates...)` discovery                                                                              | N/A — surface files are fixed names found by a walk (pkg/surface/discover.go:59-80), not one-of-N config candidates                                                                                                                               | —                                                                                                                                                                            | linter-autoconfigure-sdk/autoconfigure.go:500-514                                                          |
| Bootstrap lifecycle (`BootstrapSpec[T]` / `BootstrapProviderFromSpec`)                                                      | N/A — gvac audits and repairs existing version surfaces (AGENTS.md:6); it never generates configs, so the generate-if-missing lifecycle has no application                                                                                        | —                                                                                                                                                                            | linter-autoconfigure-sdk/bootstrap.go:51, :128                                                             |
| Determinism analyzer (`determinism.NewAnalyzer` + `cmd/jsondeterminism`)                                                    | N/A — the consumer's `--json` wire DTOs are structs with fixed field order (no map-bearing marshals; encoder at cmd/go-version-auto-configure/json.go:279-281), so the nondeterministic-map-key bug class cannot bite                             | —                                                                                                                                                                            | linter-autoconfigure-sdk/determinism/determinism.go:38; linter-autoconfigure-sdk/FEATURES.md:46-51         |
| golangci-lint config-drift audit (`audit` subpackage, in flight)                                                            | N/A — exists only past the pin (branch `feat/audit-subpackage`, unreleased) and is golangci-lint-config-specific, not the Go version surface                                                                                                      | —                                                                                                                                                                            | linter-autoconfigure-sdk/audit/audit.go:43, :60, :101                                                      |

## Gap analysis / ranked opportunities

### 1. Declare the full version surface via `ConfigFiles` (consumer-only fix, live today)

- **Library offers:** `ProviderSpec.ConfigFiles` — "An existing file under ANY of these names means the project already carries a config… ProviderFromSpec derives Inputs from ConfigFiles when set" (linter-autoconfigure-sdk/autoconfigure.go:367-374); derivation at :452-454 and :479-494. Shipped in v0.4.0 precisely so "BuildFlow re-runs the tool when any of them appears or changes" (linter-autoconfigure-sdk/autoconfigure.go:417-421).
- **Consumer does instead:** sets only `ConfigFile: "go.mod"` (pkg/provider/provider.go:49), so the bridge derives `Inputs = ["go.mod"]` — while `Discover` dispatches on six file classes: go.mod/go.work, flake.nix, `.github/workflows/*` CI files, VERSION, CHANGELOG.md (pkg/surface/discover.go:90-108). The manual layering `spec.Trigger = toolsdk.OnFiles("go", "go.mod", "go.work")` (pkg/provider/provider.go:57) compensates for re-run gating on go.work only. `Inputs` is what toolsdk uses to "derive data-flow edges and to gate the tool on file presence" (go-finding/toolsdk@v1.16.0/spec.go:60-62): a producer that rewrites flake.nix or a CI workflow is invisible to BuildFlow's ordering around this tool.
- **Impact:** high — correct DAG edges and presence-gating for the whole surface, the tool's actual read contract; zero code risk (additive field).
- **Example:**

```go
spec, err := autoconfigure.ProviderFromSpec(autoconfigure.ProviderSpec{
	Name:        toolName,
	Description: description,
	ConfigFile:  finding.FilePath("go.mod"),
	ConfigFiles: []finding.FilePath{
		"go.mod", "go.work", "flake.nix",
		"VERSION", "CHANGELOG.md",
		".github/workflows/ci.yml", ".github/workflows/release.yml", // or a host glob
	},
	Analyze: analyze,
	Repair:  repair,
})
```

### 2. Add a `Metadata` passthrough to `ConfigIssue` (upstream contribution — the fleet bottleneck)

- **Library offers:** nothing — `ConfigIssue` ends at `FixStrategy` (linter-autoconfigure-sdk/autoconfigure.go:247-278; no metadata/map field), and `FindingFromIssue` never touches metadata (linter-autoconfigure-sdk/autoconfigure.go:288-332). The SDK's own README concedes the gap: golangci-lint-auto-configure keeps linter-recommendation findings app-side because "those carry domain metadata ConfigIssue does not model" (linter-autoconfigure-sdk/README.md:272-274). The downstream channel already exists: `finding.Finding.Metadata map[string]string` (go-finding@v1.15.1/finding.go:57) and `Builder.WithMetadata` (go-finding@v1.15.1/finding_builder.go:130).
- **Consumer does instead:** string-concatenates structured facts into the message: `message = message + "; " + strip.Reason()` and `suggestion = strip.Remediation()` (pkg/provider/provider.go:152-156), where `strip` is a structured `fix.HeldStrip{Floor, Carriers []DependencyFloor, Poisoned}` (pkg/fix/held.go:16-27). BuildFlow consumers must parse prose to recover floor/carriers/poisoned.
- **Impact:** high — unlocks structured dep-forced facts for this consumer's flagship 2026-11-17 annotation feature and for every future auto-configurer; the SDK is pre-1.0 (breaking changes acceptable, README:284-286).
- **Example** (upstream, ~2 lines + tests):

```go
// linter-autoconfigure-sdk/autoconfigure.go — ConfigIssue
Metadata map[string]string // forwarded to finding.Metadata verbatim

// FindingFromIssue, after building:
if len(issue.Metadata) > 0 {
	builder = builder.WithMetadata(issue.Metadata)
}

// consumer pkg/provider/provider.go:159-166 — then becomes
return autoconfigure.ConfigIssue{
	..., // unchanged
	Metadata: map[string]string{
		"go-version-auto-configure.dep-forced": "true",
		"go-version-auto-configure.floor":      string(strip.Floor),
		"go-version-auto-configure.poisoned":   strconv.FormatBool(strip.Poisoned),
	},
}
```

### 3. Structured repair passthrough (upstream contribution)

- **Library offers:** `ProviderSpec.Repair func(ctx) (string, error)` — description string only (linter-autoconfigure-sdk/autoconfigure.go:382); the bridge wraps it as `toolsdk.RepairResult{Description: description}` and drops everything else(linter-autoconfigure-sdk/autoconfigure.go:456-465). toolsdk's `RepairResult` carries a structured provenance channel — `ModifiedFiles []string` and `Facts []Fact`, documented as "a structured provenance channel… so hosts can surface context without string-matching Description" (go-finding/toolsdk@v1.16.0/spec.go:218-237).
- **Consumer does instead:** `repair` returns `res.Report()`, a rendered string (pkg/provider/provider.go:172-203; `Result.Report()` at pkg/fix/fix.go:72-120), discarding the structured `fix.Result{Applied, HeldBack, DepForced{Floor, Poisoners, Cause}, Failures}` (pkg/fix/fix.go:33-60) — including which files were rewritten, exactly what `ModifiedFiles` exists to carry.
- **Impact:** medium-high — the consumer's dep-forced classification (its most distinctive fix-layer feature) reaches BuildFlow only as prose; every auto-configurer with real repair telemetry re-hits this wall.
- **Example** (upstream, additive to keep the string field working): add an optional richer repair shape the bridge prefers when set —

```go
type ProviderSpec struct {
	...
	Repair        func(ctx context.Context) (string, error)          // unchanged
	RepairResult  func(ctx context.Context) (RepairOutput, error)    // new: when non-nil, wins
}

type RepairOutput struct {
	Description    string
	ModifiedFiles  []string
	Facts          map[string]string
}
// bridge: RepairResult → toolsdk.RepairResult{Description, ModifiedFiles, Facts: toFacts(...)}
```

### 4. Stamp `FixStrategyDirect` on mechanical fixes (consumer-only fix, live today)

- **Library offers:** `ConfigIssue.FixStrategy *finding.FixStrategy` — "Set it for issues whose repair is directly applicable (FixStrategyDirect) rather than suggestion-only" (linter-autoconfigure-sdk/autoconfigure.go:273-277), selected in `FindingFromIssue` (linter-autoconfigure-sdk/autoconfigure.go:312-319).
- **Consumer does instead:** never sets it (pkg/provider/provider.go:159-166), so a mechanical raise-to-floor (an issue with `Fix != nil`) and a maintainer-decision suggestion both land as `FixStrategySuggest` — indistinguishable to BuildFlow.
- **Impact:** medium — lets BuildFlow present the provider's repairable class honestly; aligns the detect strategy with the repair contract (the provider repairs exactly the `Fix != nil` set).
- **Example:**

```go
direct := finding.FixStrategyDirect
ci := autoconfigure.ConfigIssue{ ..., }
if issue.Fix != nil {
	ci.FixStrategy = &direct
}
```

### 5. Use `WorkingDir(ctx)` instead of the hand-rolled twin

- **Library offers:** `WorkingDir` — "Analyze/Repair closures should resolve config paths through this helper instead of hand-rolling the fallback" (linter-autoconfigure-sdk/autoconfigure.go:224-240).
- **Consumer does instead:** `workingDir(ctx)` with byte-for-byte the same logic (pkg/provider/provider.go:241-247, including the same doc rationale "mirroring the other BuildFlow providers").
- **Impact:** low — removes a duplicated convention whose correctness (nil-context safety, v0.4.1 CHANGELOG) is then owned by the SDK.
- **Example:** delete `workingDir` and call `autoconfigure.WorkingDir(ctx)` at pkg/provider/provider.go:80 and :173.

## Fleet argument

This repo is the newest member of the auto-configure family (AGENTS.md:5 lists golangci-lint, oxlint, dependabot, and the Go version surface), and pkg/provider is the one place the family's BuildFlow wiring is shared. The owner's framing cuts both ways here:

- **Consumer-side rows (1, 4, 5) are minutes of work now** and remove exactly the kind of per-repo re-implementation the SDK exists to kill — the hand-rolled `workingDir` is already a byte-twin of the SDK helper, which is the "fixing the same bug N times" pattern in miniature (the SDK fixed a nil-ctx panic in exactly this helper at v0.4.1; the consumer's twin has that fix only by luck of copying).
- **The two bridge gaps (2, 3) are where collective improvement actually flows.** Every auto-configurer that carries structured facts — golangci's per-linter metadata (already documented as pushed app-side, linter-autoconfigure-sdk/README.md:272-274), this repo's dep-forced floors and carriers, any future tool's modified-file lists — currently flattens them into `Message`/`Description` prose, because `ConfigIssue` has no metadata field and `Repair` returns a string. Fixing that once in the SDK fixes it for all N tools; leaving it means each tool re-invents a side-channel. The channel downstream is already built and tested (`finding.Finding.Metadata`, `Builder.WithMetadata`, `toolsdk.RepairResult.ModifiedFiles/Facts`) — the SDK bridge is the only missing segment. The SDK is pre-1.0 by its own README ("breaking changes remain acceptable until v1"), so the window is now; the in-flight `audit` subpackage proves the SDK team actively generalizes per-repo drift checks into the library, and its `Checker.Audit` returns `[]ConfigIssue` — it will hit the same metadata wall on landing.

## RECONSIDER-WHEN

The verdict flips to ADOPTED when: (a) the consumer sets `ConfigFiles` to the full version surface, stamps `FixStrategyDirect` on mechanical issues, and calls `WorkingDir` (rows 3-5 → Fully: score 100, very high band), and — the decisive condition — (b) the SDK ships a metadata-capable `ConfigIssue` and a structured repair passthrough that this provider actually consumes (dep-forced facts as `Metadata`, fix telemetry as `ModifiedFiles`/`Facts`). Absent (b), the ceiling for ANY consumer of this bridge is moderate: the library owns the emission path but strands the structured data. Revisit also when the `feat/audit-subpackage` branch tags — not for its golangci functionality, but because that release cycle is the natural vehicle for the bridge changes above.

## Verify block

Commands run (read-only on the library; orchestrator can replay):

```bash
# consumer grounding
cat /home/lars/projects/go-version-auto-configure/go.mod                       # v0.8.1 direct pin
rg -n 'larsartmann/linter-autoconfigure-sdk' /home/lars/projects/go-version-auto-configure --type go
sed -n '1,248p' /home/lars/projects/go-version-auto-configure/pkg/provider/provider.go
sed -n '88,108p' /home/lars/projects/go-version-auto-configure/pkg/surface/discover.go
sed -n '200,320p;440,591p' /home/lars/projects/go-version-auto-configure/pkg/surface/rules.go
sed -n '16,78p' /home/lars/projects/go-version-auto-configure/pkg/fix/held.go
sed -n '23,120p' /home/lars/projects/go-version-auto-configure/pkg/fix/fix.go
rg -n 'atomicwrite' /home/lars/projects/go-version-auto-configure/pkg/fix/fix.go /home/lars/projects/go-version-auto-configure/pkg/fix/ci.go /home/lars/projects/go-version-auto-configure/pkg/fix/canonicalize.go
rg -n 'map\[|MarshalEncode' /home/lars/projects/go-version-auto-configure/cmd/go-version-auto-configure/json.go

# library state (read-only)
git -C /home/lars/projects/linter-autoconfigure-sdk log --oneline v0.8.1..HEAD
git -C /home/lars/projects/linter-autoconfigure-sdk diff --stat v0.8.1..HEAD -- autoconfigure.go diff.go bootstrap.go   # empty ⇒ bridge identical
git -C /home/lars/projects/linter-autoconfigure-sdk show v0.8.1 --stat
sed -n '244,349p;353,533p' /home/lars/projects/linter-autoconfigure-sdk/autoconfigure.go
sed -n '7,31p' /home/lars/projects/linter-autoconfigure-sdk/CHANGELOG.md
sed -n '260,280p' /home/lars/projects/linter-autoconfigure-sdk/README.md
rg -n 'func NewAnalyzer' /home/lars/projects/linter-autoconfigure-sdk/determinism/determinism.go

# downstream channel the bridge strands
sed -n '216,237p' "$HOME/go/pkg/mod/github.com/larsartmann/go-finding/toolsdk@v1.16.0/spec.go"
sed -n '44,58p' "$HOME/go/pkg/mod/github.com/larsartmann/go-finding@v1.15.1/finding.go"
rg -n 'func \(b \*Builder\) WithMetadata' "$HOME/go/pkg/mod/github.com/larsartmann/go-finding@v1.15.1/finding_builder.go"
```

Cross-repo citations in this report are lib-qualified (`linter-autoconfigure-sdk/…`, `go-finding/toolsdk@v1.16.0/…`); consumer citations are repo-relative. Every cited line was read before writing.
