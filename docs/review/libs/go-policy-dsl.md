# go-policy-dsl — utilization review

Reviewed: 2026-11-23 | Consumer: go-version-auto-configure @ 07d4d51 | Library: github.com/larsartmann/go-policy-dsl not a dependency (zero references in go.mod and go.sum), latest local tag v0.4.0 @ HEAD ee88a84 | Provenance: Group B.

> Commit-stamp note: consumer HEAD has moved past the stamped 07d4d51 to 2499b8d (auto-commit daemon); every consumer file cited below is byte-identical between the two (sha256-compared for pkg/surface/discover.go, pkg/surface/rules.go, pkg/surface/parse.go, pkg/provider/provider.go, pkg/fix/held.go, pkg/fix/fix.go).

## Verdict

**NOT-ADOPTED — and the no is justified on the library's own terms.** go-policy-dsl is a declaration language for **library governance policies** — bans, required companions, version-bounded library restrictions (go-policy-dsl/policy.go:1-8, "library governance policies (bans and companions)"; primary consumer `library-policy`, ROADMAP.md "Direction: adoption"). The consumer's "Policy decisions encoded here (do not regress)" (AGENTS.md:40-47) are a different species: relational invariants over a discovered version surface (go.work must cover the full module floor), fix-classification rules (mechanical vs suggest-only), and process policies (GOWORK=off dispatch). Nothing in the consumer's policy surface is a statement about which **libraries** may be imported. The one honest adoption candidate — declaring floor-poisoner ranges as ban policies — is surfaced and reasoned away in the fleet argument below.

## Adoption score

Inventory rows N = 8, N/A rows = 4 → A = 4.
Fully = 0, Partial = 0, Missed = 4, Misused = 0. All four Missed rows are justified reimplementations or refusals (per-row reasons in the table), so the number measures non-usage, not a defect.
Σ = 0 → **score = round(100 × 0 / 4) = 0 — low band (<50)**, as expected for NOT-ADOPTED.

## Version currency

- **Not a dependency:** `rg -n 'go-policy-dsl' go.mod go.sum` → zero matches (exit 1). No pin exists to drift.
- **Latest local tag:** v0.4.0 (tagged commit 92c07ca, 2026-11-04). HEAD ee88a84 is exactly 1 commit past the tag and is **docs-only** (CHANGELOG.md + FEATURES.md, 31 insertions — `git diff --stat v0.4.0..HEAD`): it back-fills the changelog/FEATURES prose for WhenAnyPresent, whose code is already in the tag (go-policy-dsl/builder.go:114 at v0.4.0).
- **What v0.4.0 offers** (go-policy-dsl/CHANGELOG.md:14-27): `Detection.WhenAnyPresent []string` and the fluent `Builder.WhenAnyPresent(...)` — an **environment gate**: "the policy only applies when the scan context (project) contains at least one of the listed module paths/signals… the 'has B' half of 'has A + has B = use C' combination rules" (go-policy-dsl/policy.go:110-118). Consumers evaluate it against project-level context such as go.mod content; the DSL still owns no matching semantics (go-policy-dsl/README.md:176).
- Status: pre-1.0, API unstable, breaking changes allowed in 0.x bumps (go-policy-dsl/CHANGELOG.md:8-10); zero dependencies by design (go-policy-dsl/README.md:35, :160).

## Capability inventory

Enumerated from the library's real public surface (README API tables verified against policy.go/builder.go/version.go/cve.go, CHANGELOG, FEATURES.md). "Consumer evidence" cites what the consumer does instead.

| Capability | Status | Consumer evidence (instead) | Library evidence |
| --- | --- | --- | --- |
| Ban policies (fluent `Ban` builder → `PolicySpec`) | N/A — the consumer audits its own version surface, never governs which libraries may be imported | — (no library-ban function exists in any pkg/) | go-policy-dsl/builder.go:26; go-policy-dsl/policy.go:168-198 |
| Companion enforcement (`RequiresCompanion`, `AsCompanionOnly`, `CompanionSpec`) | N/A — no companion-library concept in the version-surface domain | — | go-policy-dsl/builder.go:200, :208; go-policy-dsl/policy.go:130-137, :190-197 |
| Version bounds on policies (`Version`, `VersionRange`, `Validate`) | Missed — justified reimplementation: the consumer has its own directive-ranking model, `CompareDirective`/`directivePartAt` (pkg/surface/parse.go:97-118, :139-154), where the bare minor ranks BELOW patch-zero of the same minor; the DSL's semver-lite cannot express that (see gap 1) | pkg/surface/parse.go:11, :36 (own `majorMinor`), :97, :144 | go-policy-dsl/policy.go:181-188 ("NOT the Go toolchain version"); go-policy-dsl/version.go:32-38, :44-48; go-policy-dsl/policy.go:239-245 |
| Detection declaration (`ImportPatterns`/`GoModPatterns`/`ExcludeIf*`/`RequireIfContains`/`WhenAnyPresent`) | Missed — justified: the consumer's detection is a full file walk + directive parsing (pkg/surface/discover.go:40-108), and the DSL explicitly owns no matching semantics ("patterns are opaque strings… the DSL owns no matching semantics", README) | pkg/surface/discover.go:59-80 (walk), :90-108 (dispatch) | go-policy-dsl/policy.go:84-119; go-policy-dsl/README.md:176 |
| Severity / Category vocabulary | Missed — justified: the consumer stamps `finding.SeverityWarning` directly from go-finding (pkg/provider/provider.go:97, :162); the DSL's enums are deliberately NOT finding's types and would require the boundary bridge the DSL's own README prescribes | pkg/provider/provider.go:97, :162 | go-policy-dsl/policy.go:29-82; go-policy-dsl/README.md:162-174 |
| Environment gate (`WhenAnyPresent`, v0.4.0) | Missed — justified: the consumer's run-time gates are numeric thresholds, already typed compile-time options — `WithExpectedMinor` (fleet minor), `WithRespectPatchFloor` (installed toolchain) — not module-presence signals | pkg/surface/rules.go:78-85, :96-102 | go-policy-dsl/policy.go:110-118; go-policy-dsl/builder.go:114 |
| CVE tagging (`NewCVE`, `WithCVEs`) | N/A — no CVE concept anywhere in the version-surface domain | — | go-policy-dsl/cve.go:26; go-policy-dsl/builder.go:180 |
| Structural validation (`PolicySpec.Validate` → `*InvertedVersionRangeError`) | N/A — the DSL's only invariant (library version-range ordering) has no consumer counterpart; the consumer validates its option inputs at construction with named errors (e.g. `WithExpectedMinor` rejects unparseable minors, pkg/surface/rules.go:78-85) | pkg/surface/rules.go:78-85 | go-policy-dsl/policy.go:200-245 |

## Gap analysis / ranked opportunities

For a NOT-ADOPTED library, "gaps" are the places adoption was considered and declined, plus the one live candidate. Ranked by impact × ease:

### 1. Declaring floor-poisoner ranges as ban policies — the honest adoption candidate, declined on charter grounds

- **Library offers:** exactly this shape: `policydsl.Ban("golog").Because(...).VersionRange(nil, &maxV).GoModPatterns("github.com/foo/golog").Spec()` (go-policy-dsl/README.md:74-85) — a version-bounded ban on a module path, deny-by-default Mode so a typo can never disable it (go-policy-dsl/policy.go:144-150).
- **Consumer does instead:** maintains the poisoner registry as curated prose + data (docs/POISONERS.md, "the single source of truth", AGENTS.md:9), detects live floor carriers mechanically via `go list -m` (`AnalyzeFloors`, pkg/fix/who.go) and `ExplainHeldStrips` (pkg/fix/held.go:93-130), and resolves poisoners supply-side by re-tagging (AGENTS.md:51-55) rather than banning them downstream.
- **Impact if adopted:** low — the fleet's poisoner problem is already solved at the source (the supply-side campaign shipped 2026-09-22; AGENTS.md:53), every named poisoner has a resolved re-tag, and gvac's charter is detect-and-fix drift, not gate dependencies on import. A ban layer would duplicate POISONERS.md as a second source of truth and change the exit contract (findings vs bans). If the fleet ever wants hard enforcement, the natural home is the DSL's primary consumer (`library-policy`) importing gvac's floor data — not gvac importing the DSL.
- **Example of the shape that would be adopted there, not here:**

```go
max, _ := policydsl.ParseVersion("0.5.99")
var PoisonerGoAtomicWrite = policydsl.Ban("go-atomic-write v0.5.x").
    Because("patch-form go floor re-poisons every consumer tidy (AGENTS.md floor-poisoning)").
    VersionRange(nil, &max).
    GoModPatterns("github.com/larsartmann/go-atomic-write").
    Spec()
```

### 2. Version model — the consumer's core semantics cannot be expressed (hard blocker, correctly refused)

- **Library offers:** semver-lite `Version{Major, Minor, Patch}` with strict three-component parsing — `"1.2"` is `ErrInvalidVersion` (go-policy-dsl/version.go:44-48) — and `VersionMin/VersionMax` documented as constraining the **library** version, "NOT the Go toolchain version" (go-policy-dsl/policy.go:181-188).
- **Consumer does instead:** `CompareDirective` + `directivePartAt`, where an absent patch component ranks as −1 so `go 1.26 < go 1.26.0` under the go tool's directive ranking (pkg/surface/parse.go:139-154; docs/DOMAIN_LANGUAGE.md:11 "Directive coverage"). That bare-minor-below-patch-zero rule IS the consumer's headline policy axis (patch-form detection, go.work floor coverage at pkg/surface/rules.go:316-322), and semver collapses it: in a three-component model `1.26` would equal `1.26.0`, which the go tool explicitly rejects ("module m listed in go.work file requires go >= 1.26.0, but go.work lists go 1.26", quoted at pkg/surface/rules.go:233-234).
- **Impact:** decisive — no mapping exists without forking the DSL's Version; adopting it would silently equate forms the go tool distinguishes.
- **No code example by design**: the correct example is the consumer's existing `directivePartAt` (pkg/surface/parse.go:144-154), which stays.

### 3. Severity/Category as a shared policy vocabulary — declined as an indirection with no buyer

- **Library offers:** typed `Severity` (8 levels) and `Category` (9 axes) enums, deliberately decoupled from any consumer's types with a boundary-bridge pattern (go-policy-dsl/policy.go:29-82; go-policy-dsl/README.md:162-174).
- **Consumer does instead:** severity is a per-rule constant in go-finding's own vocabulary (`finding.SeverityWarning` at pkg/provider/provider.go:97, :162), and classification ("advisory, never error-severity" for dep-forced states, docs/DOMAIN_LANGUAGE.md:40) is deliberate policy already encoded in tests and wire goldens (AGENTS.md:87).
- **Impact:** negligible — a second severity enum would add a translation layer between two vocabularies the consumer never needs to reconcile (it emits findings directly via linter-autoconfigure-sdk; see the sibling report).
- **Example (what adoption would cost, for nothing gained):**

```go
// would be needed at every emission site — bridging two enums that say the same thing
func toFindingSeverity(s policydsl.Severity) finding.Severity { ... } // go-policy-dsl/README.md:165-173 verbatim
```

### 4. `WhenAnyPresent` environment gates — different problem, already solved better in-domain

- **Library offers:** project-level presence gates for combination rules ("has A + has B → use C", go-policy-dsl/policy.go:110-118).
- **Consumer does instead:** its two environment-sensitive behaviors are threshold options, not presence signals: `--respect-patch-floor` compares the discovered directive against the installed toolchain (pkg/surface/rules.go:96-102, gate applied at :220-222), and `--expect-minor` compares minors (pkg/surface/rules.go:78-85). Both are compile-time-typed Go functions with validating constructors — the same "policies as plain Go values" virtue the DSL sells (go-policy-dsl/README.md:161), already achieved.
- **Impact:** none — the shapes don't match (numeric comparison vs string presence).

## Fleet argument

The owner's framing — many projects, limited time, collective improvement only through shared libraries — demands the question be asked seriously here, because the consumer literally has a section named "Policy decisions encoded here (do not regress)" (AGENTS.md:40-47). I read all six decisions against the DSL's surface:

1. *Form fixes/CI-pin raises mechanical, others suggest-only* (AGENTS.md:42) — a **fix-classification** rule. The consumer encodes it structurally: mechanical issues carry a `*Fix` (pkg/surface/rules.go:245-257, :286-303, :537-549), suggest-only ones carry prose `Suggestion` (:496-505, :555-564). The DSL's nearest axis, `Mode` (ban vs companion-only, go-policy-dsl/policy.go:151-163), classifies what a policy ENFORCES on a library — it has no fix-strategy concept; linter-autoconfigure-sdk's `ConfigIssue.FixStrategy` (see the sibling report) is the correct shared lever for this axis.
2. *go.work respects the FULL module floor* (AGENTS.md:43) — a **relational invariant** between two discovered values, computed per-repo by `FullModuleFloor()` + `floorCoveredByDirective` (pkg/surface/rules.go:241-243, :316-322). Not a declarable pattern set; it needs the discovery engine behind it, which the DSL explicitly does not provide (go-policy-dsl/README.md:195 "You need a consumer… that reads PolicySpec and implements the matching").
3. *`GOWORK=off` dispatch* (AGENTS.md:44) — process policy in the fix runner (`moduleScoped`, AGENTS.md:44), not a data policy at all.
4. *Unparseable files become findings* (AGENTS.md:45) — error-routing policy (pkg/surface/discover.go:38-39, :177-183).
5. *Skip vendor/node_modules/.git/result/testdata* (AGENTS.md:46; pkg/surface/discover.go:18-29) — walk policy.
6. *Toolchain modeled, not stripped, with an explicit `StripToolchain` exception* (AGENTS.md:47) — already a typed option on the fix entry point, the consumer-side equivalent of what a policy knob would buy.

So the justified no is: **the DSL solves library governance; this repo's policies are surface invariants, fix classifications, and process rules.** Forcing them into `PolicySpec` would mean stuffing each rule into `Detection` pattern lists the DSL never evaluates, with the real logic still living in pkg/surface — the worst of both worlds (indirection without shared execution). The DSL's own "When NOT to use" section names this exact case: "You need a detection engine… The DSL declares what a policy is, not how to detect violations" (go-policy-dsl/README.md:195).

The honest yes-that-isn't: the fleet's **poisoner registry** (docs/POISONERS.md) is conceptually a version-bounded ban list, and `Ban(...).VersionRange(...).GoModPatterns(...)` is its exact DSL shape (gap 1). But enforcement belongs in the DSL's actual consumer (`library-policy`, go-policy-dsl/policy.go:4-5), driven by floor data gvac already exposes (`AnalyzeFloors`); embedding it in gvac would duplicate the registry and blur the tool's charter (detect-and-fix, exit 0 on dep-forced states — AGENTS.md:38). Collective improvement here means gvac publishing the floor matrix and library-policy declaring the bans — each library doing its own job.

## RECONSIDER-WHEN

Any of these flips the verdict toward adoption:

1. The fleet charters **dependency-governance enforcement** (fail builds on poisoner ranges instead of just reporting carriers): gvac — or more likely library-policy calling gvac's `AnalyzeFloors` — declares those bans via `policydsl.Ban(...).VersionRange(...)`, and the two registries (POISONERS.md prose, PolicySpec values) are unified with the DSL as the source of truth.
2. go-policy-dsl grows a **Go-directive version kind** that models the go tool's ranking (bare minor strictly below patch-zero of the same minor — today `ParseVersion("1.27")` is `ErrInvalidVersion`, go-policy-dsl/version.go:46-47, and `PolicySpec.VersionMin/Max` are documented as not-Go-toolchain, go-policy-dsl/policy.go:182-183). Without that, the consumer's core ordering can never live in the DSL.
3. A cross-fleet mandate for **uniform severity/category reporting** across the auto-configurers arrives and go-finding does not grow its own vocabulary for it — then the DSL's enums (with their README-prescribed boundary bridge) are the ready-made shared table.

## Verify block

Commands run (read-only on the library; orchestrator can replay):

```bash
# not-a-dependency proof (run from the consumer root)
cd /home/lars/projects/go-version-auto-configure
rg -n 'go-policy-dsl' go.mod go.sum          # zero matches, exit 1
cat go.mod                                    # full require list, absent

# consumer policy encoding (what the DSL would have to express)
sed -n '40,47p' /home/lars/projects/go-version-auto-configure/AGENTS.md
sed -n '30,59p;78,102p;200,262p;316,331p;483,569p' /home/lars/projects/go-version-auto-configure/pkg/surface/rules.go
sed -n '90,154p' /home/lars/projects/go-version-auto-configure/pkg/surface/parse.go
sed -n '40,108p' /home/lars/projects/go-version-auto-configure/pkg/surface/discover.go
sed -n '93,130p' /home/lars/projects/go-version-auto-configure/pkg/fix/held.go

# library state and surface (read-only)
git -C /home/lars/projects/go-policy-dsl log --oneline v0.4.0..HEAD      # ee88a84 docs-only
git -C /home/lars/projects/go-policy-dsl diff --stat v0.4.0..HEAD        # CHANGELOG.md + FEATURES.md only
git -C /home/lars/projects/go-policy-dsl show v0.4.0:builder.go | rg -n WhenAnyPresent
sed -n '1,8p;84,119p;139,198p;200,245p' /home/lars/projects/go-policy-dsl/policy.go
sed -n '44,48p;101,121p' /home/lars/projects/go-policy-dsl/version.go
sed -n '160,195p' /home/lars/projects/go-policy-dsl/README.md
sed -n '14,27p' /home/lars/projects/go-policy-dsl/CHANGELOG.md
cat /home/lars/projects/go-policy-dsl/ROADMAP.md
```

Cross-repo citations are lib-qualified (`go-policy-dsl/…`); consumer citations are repo-relative. Every cited line was read before writing.
