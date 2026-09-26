# go-version-auto-configure

One policy for the Go toolchain version surface across the LarsArtmann fleet — detected and auto-fixed everywhere, wired into BuildFlow like every other auto-configurer.

[![CI](https://github.com/LarsArtmann/go-version-auto-configure/actions/workflows/ci.yml/badge.svg)](https://github.com/LarsArtmann/go-version-auto-configure/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Why?

A Go version number lives in many places: every `go.mod` and `go.work` directive, `flake.nix` nixpkgs pins, CI `go-version:` pins. Each location is individually correct; together they drift — the exact "three-way split brain" documented in the go-ecosystem-upgrade protocol. A fleet-wide audit (2026-09-18) found **284 of 383 modules** carrying patch versions in the `go` directive (`go 1.26.7`), Nix flakes pinning `go_1_26` while their modules declare `go 1.27`, and CI pins scattered across `1.22`–`1.27.1`.

Worse: the drift **propagates**. `go mod tidy` copies a dependency's `go` floor verbatim, so one published library with `go 1.26.7` re-poisons every consumer that tidies — per-repo fixes can never stick while supply-side roots carry patch floors. This tool exists to make that visible everywhere and mechanically fix the part that is safe to fix.

## The policy

1. `go` directives (`go.mod`, `go.work`) are **major.minor only** (`go 1.26`, never `go 1.26.7`). A patch component raises the minimum toolchain to one exact patch and breaks trailing environments (Nix, CI runners).
2. `go.work` must cover the workspace floor: its normalized target is never below the highest module floor it lists.
3. Nix and CI pins must not trail the module floor; CI patch pins are flagged because they silently diverge from the nixpkgs minor the flakes track.

**Auto-fix** applies only to unambiguous form violations — via `go mod edit` / `go work edit`, never text rewriting, and every fix is verified by re-parsing the file. **Alignment** issues (which side moves: pin or floor?) carry suggestions only; downgrades are never auto-applied.

## Install

```bash
go get github.com/larsartmann/go-version-auto-configure@latest
```

Or build from source:

```bash
git clone https://github.com/larsartmann/go-version-auto-configure
cd go-version-auto-configure
GOEXPERIMENT=jsonv2 go build -o go-version-auto-configure ./cmd/go-version-auto-configure
```

## Usage

### CLI

```bash
go-version-auto-configure check ~/projects/go-finding   # detect drift, exit 1 when found
go-version-auto-configure fix  ~/projects/go-finding    # auto-fix directive form
go-version-auto-configure fix --dry-run .               # report without touching files
go-version-auto-configure who-forces .                  # name the deps forcing each go directive
```

Built for fleet sweeps: every command accepts multiple roots (analyzed in parallel, reported sorted), `--parallel N` caps the worker pool (default: CPU count), `--quiet` runs exit-code-only on all three analysis commands, `check --expect-minor 1.27` encodes the fleet policy (any surface naming a higher minor is a policy finding), `fix` skips clean repos entirely, and `--json` emits a stable machine-readable report for CI:

```bash
go-version-auto-configure check --json ~/projects/*/ | jq '.repos[] | select(.clean == false)'
go-version-auto-configure check --quiet --expect-minor 1.27 ~/projects/*/ ; echo $?   # 0 clean, 1 findings, 2 errors
```

Scheduled enforcement as a cron job (nightly sweep, non-zero exit alerts through your usual failure channel):

```cron
17 4 * * * $HOME/go/bin/go-version-auto-configure check --quiet --expect-minor 1.27 $HOME/projects/*/ >> $HOME/.cache/gvac-sweep.log 2>&1
```

When a form fix cannot stick, the tool says so truthfully and names the dependencies that force the floor — the supply-side re-tag targets:

```text
$ go-version-auto-configure fix .
applied 0, dep-forced 1, held back 0, failed 0
  dep-forced: rewrite go.mod directive in go.mod: go 1.26.7 → go 1.26
              floor go 1.26.7 is forced by: github.com/larsartmann/go-finding, github.com/larsartmann/go-finding/toolsdk, github.com/larsartmann/linter-autoconfigure-sdk
              fix supply-side: re-tag those modules with a major.minor-only go directive, then bump consumers
```

`who-forces` reports the same matrix for every module up front — directive, highest dependency floor, and who carries it (every forcing dependency with its own floor, not just the max carriers) — without touching anything. `--allow-partial` downgrades per-module `go list` failures from exit 2 to exit 1 for fleets where some modules cannot resolve.

#### JSON contract

The `--json` documents are a stable machine contract, versioned by the top-level `"schema": 2` field: field names and presence (`omitempty`) are guaranteed, and list fields are empty, never null. Schema 2 dropped the redundant `poisoners` strings — forcing dependencies are named by `poisonerFloors` (module, version, own floor).

| Command      | Per-repo fields                                                                                                                                              |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `check`      | `root`, `error?`, `clean`, `counts` (`total`, `mechanical`, `suggested`, `discovery`), `findings` (`rule`, `message`, `file`, `line`, `suggestion?`, `fix?`) |
| `fix`        | `root`, `error?`, `applied`, `heldBack`, `depForced` (`fix`, `floor`), `failures`, `suggested`, `discovery`                                                  |
| `who-forces` | `root`, `error?`, `modules` (`path`, `kind`, `module`, `directive?`, `maxDepFloor?`, `poisonerFloors?`, `poisoned`, `error?`)                                |

### BuildFlow provider

BuildFlow discovers the provider when a consumer blank-imports it:

```go
import _ "github.com/larsartmann/go-version-auto-configure/pkg/provider"
```

The provider self-registers via `toolsdk.Register` (go-finding), built on `linter-autoconfigure-sdk`'s `ProviderSpec → ProviderFromSpec` bridge — the same integration path as `golangci-lint-auto-configure`, `oxlint-auto-configure`, and `dependabot-auto-configure`.

## The mini SDK

`pkg/surface` is the reusable half: it discovers and models the version surface (module directives, workspace, Nix pins, CI pins) and evaluates the policy rules, writing nothing. Consumers that want fleet versioning intelligence without the fixer import it directly:

```go
s, discoveryIssues, err := surface.Discover(repoRoot)
// discoveryIssues: unparseable files; reported as findings, never auto-fixed
issues := surface.Analyze(s) // policy violations; mechanical ones carry issue.Fix
```

## Relationship to the rest of the fleet

| Concern                                  | Owner                                                             |
| ---------------------------------------- | ----------------------------------------------------------------- |
| Toolchain surface drift (this repo)      | detect + mechanical fix + suggestions                             |
| go.mod hygiene (tidy, replaces, floors)  | BuildFlow `gomod-check`                                           |
| Publishing clean library versions        | `go-release` protocol (supply-side root cause of floor poisoning) |
| Propagating releases to consumers        | `go-ecosystem-upgrade` protocol + BuildFlow `update`              |
| Fleet graph, conflicts, release planning | `project-dependency-graph`                                        |

## Development

```bash
GOEXPERIMENT=jsonv2 go build ./...
GOEXPERIMENT=jsonv2 go test ./...
```

Requires Go 1.27 or newer with `GOEXPERIMENT=jsonv2` (inherited from go-finding; the fleet minor is pinned by [ADR-0001](docs/adr/0001-fleet-go-minor.md)).
