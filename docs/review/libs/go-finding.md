# github.com/larsartmann/go-finding — utilization review

Reviewed: 2026-11-23 | Consumer: go-version-auto-configure @ 07d4d51 | Library: github.com/larsartmann/go-finding pinned v1.15.1, latest local tag v1.16.0 @ HEAD 5f568e0 | Provenance: Group A.

(One daemon auto-commit, 9474ea0, touched only `scripts/citation-sweep.py` after the stamped HEAD; all citations were re-verified against the working tree.)

## Verdict

**UNDERUSED-mild.**

The consumer's integration shape is exactly the pattern the library prescribes: pkg/surface stays dependency-light with its own branded types, pkg/provider is the sole import site, and every finding that reaches BuildFlow is a real `finding.Finding` built through the shared builder. What is unused is concentrated in output/expression surface (SARIF, run propertyBag, DetectorV2 coverage, the Metadata channel) plus two small utility replacements — additive headroom, not a wrong architecture. The large untouched remainder (pipeline/, analysis/, LSP, merge/correlate) is structurally not applicable to a directive-file scanner, and marking it otherwise would be padding.

## Adoption score

Capability inventory: **N = 24** rows. Not-Applicable rows: **8**. Applicable **A = 24 − 8 = 16**.
Fully Leveraged = 7 (×1.0), Partially Used = 3 (×0.5), Missed Opportunity = 6 (×0).
Σ = 7 + 1.5 + 0 = **8.5** → score = round(100 × 8.5 / 16) = **53 / 100 — moderate band (50–69)**.
Published inputs: **7 of 16 applicable capabilities fully leveraged** (the "via bridge" rows count as leveraged: the consumer deliberately delegates finding emission to linter-autoconfigure-sdk, which is the documented composition — pkg/provider/provider.go:8-10).

The moderate score with a mild verdict is explained by structure: five of the six misses (SARIF export, run propertyBag, DetectorV2, Metadata, and half of the exec helpers) are one adoption decision away and none indicates misuse; the eighth-plus of the surface that would depress the score further is honestly N/A for this tool shape.

## Version currency

- Consumer pin: `github.com/larsartmann/go-finding v1.15.1` (go.mod:5, direct).
- Latest local tag: **v1.16.0** (repo HEAD 5f568e0 = exactly the tag, verified `git describe --tags --exact-match HEAD`). **One minor behind.**
- Gap contents (verified via `git diff --stat v1.15.1..v1.16.0` and the commits):
  - Run-level SARIF propertyBag: `Report.SetRunProperty`/`RunProperties` (go-finding/report.go:140, :155), `runs[].properties` emission in go-finding/sarif_export.go, readback `RunPropertiesFromSARIF` (go-finding/sarif_import.go:29); commit 5f568e0 "feat: run-level SARIF propertyBag".
  - Randomized SARIF round-trip losslessness property test (commit 93e6898; sarif_roundtrip/property tests — test-only).
  - Runnable DetectorV2 example (go-finding/examples/detectorv2/main.go, commit 253d967).
  - `IsHashID` signature tightened `string → ID` (go-finding/id.go:176) — not referenced by the consumer.
- Does the consumer miss anything from the gap? **No runtime behavior it currently executes changes.** The delta is entirely SARIF-layer plus examples/tests; the consumer's path (branded types, context key, bridge-built findings) is identical in both versions. Both versions require `go 1.27`; the consumer's directive is `go 1.27` — compatible.
- Recommendation: **bump opportunistically, not urgently.** Fold the v1.16.0 bump into the first gap item that needs it (SARIF export wants run propertyBag for scan-root/schema stamping). Staying one minor behind on a module whose floor is minor-form `go 1.27` carries no poisoning risk (POISONERS.md class).

## Capability inventory

| Capability | Status | Consumer evidence | Library evidence |
| --- | --- | --- | --- |
| Branded string types (`RuleName`/`ToolName`/`FilePath`/`ID`/`GroupID`) | Fully Leveraged | pkg/provider/provider.go:95, :98, :160, :163 (boundary conversion); pkg/provider/provider_test.go:57 | go-finding/branded_types.go:10, :13, :16, :19 |
| `Finding` data model (+ `Validate`/`Equal`) | Fully Leveraged (via bridge — deliberate delegation) | pkg/provider/provider.go:46-52; tests assert `f.ToolName`/`f.Rule` at pkg/provider/provider_test.go:56-59 | go-finding/finding.go:8; bridge builds via linter-autoconfigure-sdk autoconfigure.go:450 |
| Builder API (validated construction) | Fully Leveraged (via bridge) | pkg/provider/provider.go:8-10 documents the delegation | go-finding/finding_builder.go (`NewBuilder`, used by the bridge at linter-autoconfigure-sdk autoconfigure.go:296) |
| Stable ID generation (`GenerateID`) | Fully Leveraged (via bridge — every emitted finding carries a minted ID) | pkg/provider/provider.go:46-52 (all findings go through the bridge) | go-finding/id.go:34 |
| `FixStrategy` classification (none/suggest/direct) | Fully Leveraged (via bridge: suggestion⇒suggest, else none) | pkg/provider/provider.go:146-147, :165 (suggestion populated per issue kind) | go-finding/finding.go:23; bridge rule at linter-autoconfigure-sdk autoconfigure.go:324-330 |
| Category / Confidence / Tags | Partially Used | Bridge stamps `CategoryConfiguration` + default `ConfidenceFull`; consumer sets neither, and its own taxonomy (form/alignment/toolchain/discovery, pkg/surface/rules.go) is richer than one category | go-finding/category.go, go-finding/confidence.go; bridge at linter-autoconfigure-sdk autoconfigure.go:297-302 |
| Severity scale (4 levels, `Compare`, aliases) | Partially Used | Every finding is `finding.SeverityWarning` — pkg/provider/provider.go:97, :162; policy keeps advisories at warning (pkg/provider/analyze_held_test.go:88) | go-finding/severity.go:16-19 |
| Position model (`Pos`/`FilePos`, offsets, `Range`) | Partially Used | File+line only (pkg/provider/provider.go:98-99, :163-164); column/offset/`Range` unused — directive findings are line-oriented, so the unused half is domain-justified | go-finding/position.go; bridge `Pos(file,line,1)` at linter-autoconfigure-sdk autoconfigure.go:291-295 |
| Working-dir context propagation | Fully Leveraged | pkg/provider/provider.go:242 (`finding.WorkingDirFromContext`); tests set it at pkg/provider/provider_test.go:49 | go-finding/context.go:21, :31 |
| `Detector` contract (`DetectorFunc`/`NamedDetectorFunc`) | Fully Leveraged (bridge wires the Analyze closure as a `finding.Detector` into `Spec.Detect`) | pkg/provider/provider.go:46-52, :73-75 | go-finding/detector.go:10, :18, :35; go-finding/toolsdk/spec.go:84 |
| DetectorV2 coverage evidence (`DetectResult.FilesScanned`/`SkippedModules`, `RunDetect`) | Missed Opportunity | Provider answers "clean" with zero findings (pkg/provider/provider.go:87-111) — not yet provable coverage; v1.16.0 ships a runnable example | go-finding/detect_result.go:14, :34, :49; go-finding/examples/detectorv2/main.go |
| `Finding.Metadata` extension channel | Missed Opportunity | Dep-forced facts are string-concatenated into the message: pkg/provider/provider.go:154 | go-finding/finding.go:57 (with the namespacing doctrine in the same comment block) |
| SARIF export (`ToSARIF`/`WriteSARIF`, options) | Missed Opportunity | CLI emits only the schema-2 JSON contract: cmd/go-version-auto-configure/json.go:22, :35-42 | go-finding/sarif_export.go:64, :70, :92 |
| SARIF import / round-trip | Not Applicable | Never ingests SARIF from other tools — single-tool surface scan | go-finding/sarif_import.go:16 |
| Run-level SARIF propertyBag (v1.16.0) | Missed Opportunity | Unreachable at pin v1.15.1; becomes relevant the day SARIF export lands (schema version, expect-minor, scan root per run) | go-finding/report.go:140, :155; go-finding/sarif_import.go:29 |
| Suppression model (`Suppression`, expiry, `NotSuppressed`) | Not Applicable | Accepted drift is a supply-side registry (docs/POISONERS.md), not per-finding suppression; advisory findings are deliberately always visible (docs/DOMAIN_LANGUAGE.md "Advisory") | go-finding/suppression.go:6, :16 |
| Report aggregation / summary (`Report`, `ComputeSummary`) | Not Applicable | cmd/ owns a locked schema-2 wire contract (cmd/go-version-auto-configure/json.go:14-22); provider path is in-process and BuildFlow aggregates | go-finding/report.go:84, :241 |
| Filters / sort / group / merge / diff / correlate | Not Applicable | Single detector; ordering is the CLI's (multi-root sorted output) and cross-tool merging is BuildFlow's; findings never materialize as `[]finding.Finding` inside this repo outside the bridge | go-finding/filter.go, go-finding/merge.go, go-finding/diff.go, go-finding/correlate.go |
| LSP diagnostics | Not Applicable | Not an LSP server or editor tool | go-finding/lsp.go |
| Exec helpers (`CheckBinary`/`RunCmd`) | Missed Opportunity | `fix.SelfCheck` re-implements the PATH-lookup half: pkg/fix/fix.go:132-135 (`RunCmd` itself cannot apply — it supports neither `cmd.Dir` nor env sculpting, which pkg/fix/fix.go:194-201 requires) | go-finding/detector.go:56, :68 |
| `FindingError` typed error family | Missed Opportunity | Provider and fix wrap with `fmt.Errorf` (pkg/provider/provider.go:89, :199) | go-finding/errors.go (`NewIOError` etc.) |
| Finding JSON / ndjson wire (`WriteJSON`, `FindingsFromReader`) | Not Applicable | The `--json` machine contract is the repo's own locked DTO set (cmd/go-version-auto-configure/json.go:17-22) | go-finding/json.go |
| `analysis/` sub-module (go/analysis bridge) | Not Applicable | Tool parses go.mod/go.work/Nix/YAML text, not Go ASTs | go-finding/analysis/ (module `analysis/v1.15.1`) |
| `pipeline/` sub-module (FixEngine, triage, retry, flight recorder) | Not Applicable | The fix path is external-tool edits (`go mod edit`/`go work edit`) plus a non-mutating `go mod tidy -diff` gate and re-parse verification (pkg/fix/fix.go) — inexpressible in an in-memory edit engine; one detector needs no `pipeline.Detect` fan-out | go-finding/pipeline/ (module `pipeline/v1.15.1`) |

Deliberate-domain-types question (asked by the brief): pkg/surface using its own `surface.Rule`/`GoVersion`/`ModulePath`/`FilePath` instead of go-finding's types is **justified layering, not a split brain**. Verified: pkg/surface imports only stdlib + `golang.org/x/mod/modfile` (pkg/surface/surface.go:11, pkg/surface/parse.go:7) — zero go-finding imports; the sole mention of the module path is a doc-comment example (pkg/surface/surface.go:22). pkg/fix is likewise import-free of go-finding in production code (test fixtures reference the path as data only). The conversion happens in exactly one place, `toConfigIssue`/`analyzeWith` (pkg/provider/provider.go:92-101, :144-167) — the boundary the provider exists to be. This also keeps the fleet-policy door open: ROADMAP.md:45 plans to promote pkg/surface to its own submodule "mirrors go-finding/toolsdk", which would be impossible if surface imported the finding model.

## Gap analysis / ranked opportunities

Sorted by impact × ease, descending.

### 1. Structured dep-forced facts via `Finding.Metadata` (impact: high, ease: medium — needs an upstream passthrough)

What the library offers: `Finding.Metadata map[string]string` (go-finding/finding.go:57) with a documented tool-prefixed namespacing convention — built for exactly "tool-specific key-value pairs" riding findings across wire formats.

What the consumer does instead: concatenates the held-strip explanation into the message prose — `message = message + "; " + strip.Reason()` (pkg/provider/provider.go:154) — so any host that wants the carriers/floors machine-readably must parse English.

Recommended usage (blocked on the bridge — `autoconfigure.ConfigIssue` has no metadata field; push upstream, see fleet argument):

```go
// pkg/provider/provider.go — once linter-autoconfigure-sdk's ConfigIssue
// grows a Metadata passthrough:
issue := autoconfigure.ConfigIssue{
    Rule: finding.RuleName(issue.Rule), /* ... */
    Metadata: map[string]string{
        "go-version-auto-configure.held-by": strings.Join(carrierNames(issue.File), ","),
        "go-version-auto-configure.held-floor": held[issue.File].Floor,
    },
}
```

### 2. SARIF export flag in the CLI (impact: medium, ease: high)

What the library offers: `finding.NewReportFromFindings` + `Report.ToSARIF` (go-finding/report.go:99; go-finding/sarif_export.go:64) — one-call SARIF 2.1.0 emission, losslessness property-tested upstream.

What the consumer does instead: only the schema-2 JSON DTO contract (cmd/go-version-auto-configure/json.go:22) — fine for the fleet's own tooling, invisible to SARIF-native consumers (GitHub code scanning, CI annotations).

Recommended usage:

```go
// cmd/go-version-auto-configure — a --sarif flag beside --json:
fs := make([]finding.Finding, 0)
for _, a := range analyses {
    for _, iss := range a.report.all() {
        f, _ := autoconfigure.FindingFromIssue(finding.ToolName(toolName), provider.ToConfigIssue(iss, nil))
        fs = append(fs, f)
    }
}
rep := finding.NewReportFromFindings(finding.ToolInfo{Name: toolName, Version: version.Version}, fs)
rep.ComputeSummary()
sarif, err := rep.ToSARIF()
```

Bump the pin to v1.16.0 in the same change and stamp `rep.SetRunProperty("go-version-auto-configure/schema", "2")` (go-finding/report.go:140).

### 3. DetectorV2 coverage evidence (impact: medium, ease: medium)

What the library offers: `DetectorV2.DetectV2` returning `DetectResult{Findings, FilesScanned, SkippedModules}` (go-finding/detect_result.go:14, :34) — turns "0 findings" into checkable coverage; v1.16.0 ships a runnable example (go-finding/examples/detectorv2/main.go).

What the consumer does instead: plain `Detector` via the bridge (pkg/provider/provider.go:73-75); a clean repo answers with an empty slice and no evidence.

Recommended usage: implement `DetectV2` on the provider's detector type, fed from `Discover`'s walk stats (pkg/surface/discover.go already knows every file it examined and the vendor/result dirs it skipped). Value is host-gated: it pays off when BuildFlow starts calling `RunDetect` (go-finding/detect_result.go:49) — verify the host's consumption before building it.

### 4. `CheckBinary` in `SelfCheck` (impact: low, ease: high)

What the library offers: `finding.CheckBinary("go")` (go-finding/detector.go:56) — the standardized PATH lookup with a typed `NewIOError`.

What the consumer does instead: hand-rolled `exec.LookPath` (pkg/fix/fix.go:132-135). The second half of SelfCheck (`go env GOVERSION`, pkg/fix/fix.go:137-144) is beyond the helper and stays. (`RunCmd` stays custom: it lacks `cmd.Dir` and env sculpting, which pkg/fix/fix.go:194-201 requires.)

```go
goBin, err := finding.CheckBinary("go")
if err != nil { return fmt.Errorf("fix: %w", err) }
```

### 5. `FindingError` for provider failure paths (impact: low, ease: medium)

What the library offers: categorized, position-carrying errors (`NewIOError`, `NewParseError`, … go-finding/errors.go) with `errors.Is`-able sentinels.

What the consumer does instead: `fmt.Errorf` wrapping (pkg/provider/provider.go:89, :199). Mild value — BuildFlow currently renders the string — worth doing only when a host starts branching on error categories.

## Fleet argument

The owner's framing: many projects, limited time — if every repo re-implements the same plumbing, the fleet spends its days fixing N copies of the same bug and no project benefits from collective improvement.

What this consumer already gains from go-finding's shared surface:

- **Free BuildFlow presence.** Registration, trigger vocabulary, options plumbing, dry-run and health-check contracts are the library's to evolve; a BuildFlow-side improvement to how specs are consumed lands here with zero code (pkg/provider/provider.go:57-67).
- **The canonical working-dir key.** `finding.WithWorkingDir`/`WorkingDirFromContext` (go-finding/context.go:21, :31) exists precisely so external detectors read BuildFlow's module fan-out without importing a consumer-specific package — the provider's `workingDir` (pkg/provider/provider.go:241-247) is three lines instead of a coupling.
- **One finding model, minted identities, one SARIF implementation.** IDs, validation, fix-strategy semantics and (when adopted) SARIF are fixed once for every tool in the fleet; this repo's findings are already indistinguishable from any other go-finding tool's at the BuildFlow boundary.

What it should push upstream instead of hand-rolling:

1. **`ConfigIssue.Metadata` passthrough in linter-autoconfigure-sdk** (gap #1) — the prose-concatenation is a workaround for a missing bridge field; growing the bridge fixes every auto-configurer at once, not just this repo.
2. **Structured `RepairResult` passthrough** — the bridge's `ProviderSpec.Repair` returns only a string (linter-autoconfigure-sdk autoconfigure.go:457-465), stranding `ModifiedFiles`/`Facts`; see the toolsdk companion report.
3. **DetectorV2 consumption in BuildFlow** — the coverage-evidence payoff (gap #3) is host-side; requesting it upstream unblocks every provider, this one included.

## RECONSIDER-WHEN

- CI or code scanning asks this tool for SARIF → adopt gap #2 and bump to v1.16.0 in the same change (run propertyBag becomes immediately useful); verdict moves toward ADOPTED.
- linter-autoconfigure-sdk grows a `ConfigIssue.Metadata` passthrough → re-rate gap #1 from "blocked upstream" to a local one-liner; the Metadata row flips to leveraged.
- BuildFlow starts consuming `DetectorV2`/`RunDetect` → gap #3 becomes high-impact; implement DetectV2 from Discover's walk stats.
- A stale-finding incident in BuildFlow is traced to the provider's under-declared read set → that is the toolsdk report's gap #1, fixed locally in minutes.
- pkg/surface is promoted to its own submodule (ROADMAP.md:45) → re-examine whether surface should then re-export or alias go-finding's branded types instead of mirroring them.

## Verify block

Commands run (consumer at /home/lars/projects/go-version-auto-configure, library at /home/lars/projects/go-finding):

```bash
# context + usage sites
cat AGENTS.md; cat docs/DOMAIN_LANGUAGE.md
rg -n 'larsartmann/go-finding' /home/lars/projects/go-version-auto-configure --type go
rg -n 'finding\.[A-Z]' /home/lars/projects/go-version-auto-configure --type go -o | sort | uniq -c
rg -n 'toolsdk\.[A-Z]' /home/lars/projects/go-version-auto-configure --type go -o | sort | uniq -c
cat go.mod                                        # pins v1.15.1 / toolsdk v1.16.0

# library state + version gap
cd /home/lars/projects/go-finding && git log --oneline -5
git describe --tags --exact-match HEAD            # → v1.16.0
git tag --list | tail -20
git diff --stat v1.15.1..v1.16.0 -- '*.go' ':!**/*_test.go' ':!toolsdk' ':!analysis' ':!pipeline' ':!cmd'
git log --oneline v1.15.1..v1.16.0 --no-merges | rg -v 'auto-commit'
git diff v1.15.1..v1.16.0 -- id.go
sed -n '13,56p' CHANGELOG.md                      # [Unreleased] = v1.16.0 content

# library public surfaces
GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2 go doc -all .            | rg '^(type|func) ' | sort -u   # root
cd toolsdk   && GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2 go doc -all . | rg '^(type|func|const|var) ' | sort -u
cd analysis  && GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2 go doc -all . | rg '^(type|func) ' | sort -u
cd pipeline  && GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2 go doc -all . | rg '^(type|func) ' | sort -u

# citation spot-checks (file:line)
rg -n 'type (ID|RuleName|ToolName|FilePath)' branded_types.go; grep -n 'func WithWorkingDir\|func WorkingDirFromContext' context.go
grep -n 'func CheckBinary\|func RunCmd' detector.go
grep -n 'SeverityInfo|SeverityWarning' severity.go   # → severity.go:16-19
grep -n 'func (r \*Report) ToSARIF\|SetRunProperty\|RunProperties' report.go sarif_export.go sarif_import.go
grep -n 'type Suppression' suppression.go; grep -n 'func GenerateID' id.go
grep -n 'func OnGoFiles\|func OnFiles' toolsdk/triggers.go; grep -n 'func WithDryRun\|func DryRunFromContext' toolsdk/dryrun.go

# consumer side
git -C /home/lars/projects/go-version-auto-configure rev-parse --short=7 HEAD   # 9474ea0 (daemon commit over stamped 07d4d51)
git -C /home/lars/projects/go-version-auto-configure show --stat 9474ea0        # touches only scripts/citation-sweep.py
rg -n 'flake.nix|\.github|workflows|go.work|go.mod' pkg/surface/discover.go
rg -n 'workflows|\.yml|\.yaml' pkg/surface/discover.go pkg/surface/parse.go
rg -n 'func SelfCheck|func EditRunner|GOWORK|GOTOOLCHAIN' pkg/fix/fix.go
rg -n 'type Result|func \(r Result\) Report' pkg/fix/fix.go
rg -n 'type Suppression' /home/lars/projects/go-finding/suppression.go
rg -n -i 'sarif|toolsdk|go-finding|detectorv2' TODO_LIST.md ROADMAP.md          # only ROADMAP.md:45 (surface submodule promotion)

# bridge behavior (pins the "via bridge" rows)
rg -n 'ProviderFromSpec|discoveryCandidates|FindingFromIssue|issueDetector' \
  ~/go/pkg/mod/github.com/larsartmann/linter-autoconfigure-sdk@v0.8.1/autoconfigure.go
```

Every file:line cited above was read in the working tree during this audit (view/grep on the exact line), not inferred.
