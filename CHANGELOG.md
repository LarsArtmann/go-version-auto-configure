# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- Initial release candidate: Go toolchain version-surface detection and mechanical auto-fix.
  - `pkg/surface` mini SDK: discovery + policy analysis over go.mod/go.work directives, flake.nix pins, and CI go-version pins.
  - `pkg/fix`: `go mod edit` / `go work edit` appliers with re-parse verification and dry-run.
  - `pkg/provider`: BuildFlow provider self-registered through linter-autoconfigure-sdk's `ProviderFromSpec`.
  - CLI: `check`, `fix [--dry-run]`, `version`.
- Fleet audit findings that motivated the tool (2026-09-18): 284/383 modules with patch-form `go` directives; floor poisoning propagates through `go mod tidy` from published go-finding/go-atomic-write versions; go.work-below-workspace-floor breakage class found in go-finding.
