# scripts/

Maintenance scripts. Wired into `nix run .#docs-gate` where noted.

## citation-sweep.py

Ship gate for `docs/review/libs/*.md` utilization reports: extracts every
`path:line` citation and verifies file existence + line range against the
consumer repo, the audited library checkouts (`~/projects/<lib>`), and the Go
module cache (`GOMODCACHE`, default `~/go/pkg/mod`) for pinned-version cites.

Citation forms (en-dash ranges normalized):

| Form | Example | Resolves against |
| --- | --- | --- |
| absolute | `/home/lars/projects/go-finding/finding.go:42` | as-is |
| consumer-relative | `pkg/provider/provider.go:30` | this repo |
| lib-qualified | `go-finding/finding.go:42` | `~/projects/<lib>` |
| module-cache (pinned) | `go-finding@v1.16.0/finding.go:42`, `github.com/larsartmann/go-finding@v1.16.0/finding.go:42`, `go-finding/toolsdk@v1.16.0/spec.go:60` | module cache — the PINNED version, not checkout HEAD |
| bare basename | `finding.go:42` | unique match fallback (tracked, reported) |

Flags:

- (default) one-line count; exit 1 + stderr DEFECT lines on any miss (hard gate)
- `--summary` — per-report table + the basename-fallback list (uniqueness-fragile cites; keep this at zero by lib-qualifying)
- `--json` — machine-readable output for CI
- `--self-test` — fixture tests for the resolvers (exit 0 = all pass)

Wired as hard check 4 in `docs-gate.sh`. Baseline: 360 citations, 0 defects,
0 basename fallbacks (M18, 2026-11-24).
