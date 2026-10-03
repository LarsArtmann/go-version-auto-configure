# Poisoner Registry

The single source of truth for active patch-form Go floor poisoners in the LarsArtmann fleet. A poisoner is a published module whose `go` directive carries a patch component; every consumer's `go mod tidy` lifts its own directive to match (MVS floor propagation), defeating minor-form normalization until the poisoner re-tags.

`check` reports the consumer-side symptom (`go-directive-patch-form` + the dependency-floor gate caveat since v0.2.1); `who-forces` names the carrier; the fix is always supply-side: re-tag with a major.minor-only directive, then bump consumers.

## Active poisoners

| Module                                                           | Forced floor                    | Status                             | Notes                                                                                                                                                                                                                                                                                                                                |
| ---------------------------------------------------------------- | ------------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `golang.org/x/text`                                              | `go 1.26.0`                     | **ACCEPTED** (upstream, permanent) | Standard-library-adjacent; not ours to re-tag. Rule comparisons treat `1.26.0` as a legit floor (`--expect-minor` skips accepted zero-patch forms).                                                                                                                                                                                  |
| `encoding/json/v2` (std)                                         | toolchain patch (e.g. `1.27.1`) | **ACCEPTED** (standard library)    | Bites importers of the v2 API; the floor follows the local toolchain patch, not a module we control. Classification gap CLOSED in v0.2.3 (2026-09-27): a tidy-diff `+go X` raise that no listed module carries is now dep-forced with the forcer named (replace target / vendored module) or attributed to the std floor in `cause`. |
| `github.com/larsartmann/go-health-dashboard`                     | `go 1.27.1`                     | **PENDING re-tag**                 | Its v0.10.x tags still carry the patch-form floor (fixed on master 2026-09-25 with the go-health v0.4.1 bump); forces DiscordSync and others.                                                                                                                                                                                        |
| `github.com/larsartmann/go-cqrs-lite/*` (v4 family, ~27 modules) | `go 1.27.1`                     | **PENDING re-tag**                 | The largest carrier set (found in DiscordSync's who-forces matrix 2026-09-25; dnsblockd's vendor annotations name claiming/dedup/metaengine/sqliteengine/metaengine/record 2026-09-26).                                                                                                                                              |
| `github.com/larsartmann/cqrs-htmx/usermgmt/oauth2/v4`            | `go 1.27.1`                     | **PENDING re-tag**                 | Named by dnsblockd's vendor annotation fallback 2026-09-26 (v4.11.0).                                                                                                                                                                                                                                                                |
| `github.com/larsartmann/go-etag/*`                               | `go 1.27.1`                     | **PENDING re-tag**                 | Carried alongside the cqrs-lite family.                                                                                                                                                                                                                                                                                              |
| `github.com/larsartmann/go-sse`                                  | `go 1.27.1`                     | **PENDING re-tag**                 | Named in both DiscordSync and PMA matrices.                                                                                                                                                                                                                                                                                          |

Vendor-mode note (2026-09-25, dnsblockd; fixed in v0.2.3): when `go list -m` cannot run against a skewed `vendor/modules.txt`, floor resolution now falls back to the `## explicit; go X` annotations — the gate outcome is dep-forced with the vendored carriers named (path@version), and `who-forces` resolves the same rows instead of erroring.

## Resolved poisoners

| Module                                                                                                                    | Poisoned tags           | Resolved                | Resolution                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ------------------------------------------------------------------------------------------------------------------------- | ----------------------- | ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `github.com/larsartmann/go-health`                                                                                        | v0.4.0 (`go 1.27.1`)    | 2026-09-25, **v0.4.1**  | Flagship consumer go-health-dashboard dep-forced for 3 days (3 CI-guard incidents while the running BuildFlow binary predated the v0.2.0 gate). Re-tag + consumer bump + guard deletion; incident record in [ADR-0001 appendix](adr/0001-fleet-go-minor.md).                                                                                                                                                                                                                                                                                                     |
| go-finding, go-atomic-write, go-error-family, go-output, go-branded-id, linter-autoconfigure-sdk, sibling autoconfigurers | various patch-form tags | 2026-09-22 campaign     | See [ADR-0001](adr/0001-fleet-go-minor.md); go-output v0.38.1 stays documented-only (NOT retracted, owner decision).                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `github.com/larsartmann/go-output/*` (17-module family)                                                                   | v0.38.2 (`go 1.27.1`)   | 2026-10-02, **v0.38.3** | v0.38.2 REGRESSED to patch-form (v0.38.1 was the minor-form tag; the 2026-09-22 "v0.38.2+ is minor-form" registry note had it inverted — corrected same day from module-cache evidence). v0.38.3 ships minor-form floors AND v0.38.3 sibling pins inside the tagged tree (a v0.38.2 pin would have kept the floor alive through every submodule graph — one-time deviation from tag-then-bump). Consumer gvac bumped same day: directive back to `go 1.27`, `check` clean, tidy stable. Fleet consumers still pinning v0.38.2 keep re-poisoning until they bump. |

## Maintenance

- When `who-forces` names a new poisoner: add it to Active, open a supply-side re-tag task in its repo, link the consumer evidence here.
- When a re-tag ships: move the row to Resolved with the tag pair and date.
- This file replaces the poisoner list previously maintained inline in AGENTS.md (which now links here).

### Evidence convention (2026-10-03)

Every Resolved row cites at least one **published `.mod` verification**: the
exact tag whose `go` directive was read from the resolution path a consumer
actually uses, plus the date. Two accepted forms:

- proxy URL: `https://proxy.golang.org/<module-path>/@v/<tag>.mod` (public repos)
- fresh fetch: `go get <module>@<tag>` in a scratch module, then read
  `$(go env GOMODCACHE)/cache/download/<path>/@v/<tag>.mod` — also the
  correct form for `GOPRIVATE` repos, where resolution is direct-VCS and the
  cache file reflects the pushed tag (verified 2026-10-03 on go-output's
  submodules: the pushed v0.38.3 tags serve `go 1.27` even though
  proxy.golang.org is bypassed).

**No re-tag ships without published-`.mod` verification** — a locally-stripped
tree proves nothing about what consumers resolve (the go-output v0.38.2
registry inversion survived partly because a claim about tags was recorded
without reading one). A local `git show <tag>:<dir>/go.mod` reading is a
complement (it catches tag-vs-tree drift), never a substitute.
