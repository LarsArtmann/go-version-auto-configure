# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- `who-forces` command: a per-module dependency-floor matrix naming which dependencies force (poison) each `go` directive and whether `go mod tidy` would re-raise it (`pkg/fix.AnalyzeFloors`).
- Fleet-sweep enablers: `check`/`fix`/`who-forces` accept multiple roots analyzed in parallel, and `fix` skips clean repos entirely (no go invocations) so sweeps scale with drifted repos, not repo count.
- `--json` machine-readable output for `check`, `fix`, and `who-forces` (`encoding/json/v2`, stable camelCase contract).
- `toolchain` directive coverage: `toolchain` lines in go.mod/go.work are discovered; a stale one below the same file's `go` directive surfaces as `toolchain-below-directive` (suggest-only), and a toolchain naming a newer minor raises the effective floor for Nix/CI pin alignment.
- `.editorconfig` and `.gitattributes`; dependabot config gained an explicit `open-pull-requests-limit` and an entry-level `gomod` update group.

### Changed

- Domain strings in `pkg/surface` and `pkg/fix` are now named types (`surface.Rule`, `surface.GoVersion`, `surface.ModulePath`, `fix.FailureCause`), matching go-finding's branded-type convention; the JSON wire format is unchanged.

### Fixed

- `go list -m` invocations run with `GOWORK=off` so a module's dependency graph cannot be resolved through an enclosing workspace.
- Cleared every warning-severity golangci-lint finding (159 → 0); tests run in parallel; full-mode BuildFlow (race + coverage) green.

## [0.1.0] - 2026-09-18

### Added

- Initial release candidate: Go toolchain version-surface detection and mechanical auto-fix.
  - `pkg/surface` mini SDK: discovery + policy analysis over go.mod/go.work directives, flake.nix pins, and CI go-version pins.
  - `pkg/fix`: `go mod edit` / `go work edit` appliers with re-parse verification and dry-run.
  - `pkg/provider`: BuildFlow provider self-registered through linter-autoconfigure-sdk's `ProviderFromSpec`.
  - CLI: `check`, `fix [--dry-run]`, `version`.
- Fleet audit findings that motivated the tool (2026-09-18): 284/383 modules with patch-form `go` directives; floor poisoning propagates through `go mod tidy` from published go-finding/go-atomic-write versions; go.work-below-workspace-floor breakage class found in go-finding.

### Fixed

- README install instructions: replaced the `go get` command (unrunnable while the module is untagged) with build-from-source steps; `go get` is documented as available once the first version is tagged.
