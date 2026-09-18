# Features

Status legend: DONE / PARTIALLY DONE / PLANNED / WORTH CONSIDERING

## DONE

- **Surface discovery** — walks a repo and models every Go version declaration: go.mod `go` directives (all modules), go.work, flake.nix nixpkgs pins (attribute + builder forms), CI `go-version:` pins (workflow YAML, expression/range pins skipped as non-comparable)
- **Policy analysis** — patch-form violations on go.mod/go.work directives; go.work below workspace floor; Nix pins below module floor; CI pins below floor; CI patch-form pins
- **Mechanical auto-fix** — directive form normalization via `go mod edit` / `go work edit`; go.work target never below workspace floor; every fix verified by re-parsing AND by surviving `go mod tidy`; dry-run mode
- **Dep-forced floor detection** — when tidy reverts a strip, the tool resolves the dependencies whose own floors force the value (`go list -m`) and names them as the supply-side re-tag targets, instead of reporting a fix that silently reverts
- **Suggestion engine** — alignment issues (pin vs floor) carry actionable suggestions; downgrades never auto-applied
- **BuildFlow provider** — self-registering `toolsdk.Spec` via linter-autoconfigure-sdk's `ProviderFromSpec`; Detect + Repair; honors BuildFlow dry-run context; HealthCheck verifies the go binary
- **CLI** — `check` (exit 1 on drift), `fix` (+`--dry-run`), `version`
- **Test suite** — surface (parse/discover/analyze incl. workspace-floor regression), fix (edit verification, subdirectory modules, failure reporting), provider (registration, detect/repair/dry-run), CLI end-to-end

## PLANNED

- Supply-side convergence campaign (see TODO_LIST.md)
- BuildFlow blank-import wiring (BuildFlow dev task)
- JSON output for CI/machines

## WORTH CONSIDERING

- Promote `pkg/surface` to its own submodule when a second repo imports it (mirrors go-finding/toolsdk)
- gomod-checker upstream rule: detect when `go mod tidy` re-poisons a directive after a form fix (the "tidy revert" class)
- Read version pins from `.tool-versions`/`mise.toml`/Dockerfiles (Tier-2 surface per version-surface.md)
- VERSION/CHANGELOG/git-tag release-authority drift detection (Layer-B versioning; project-dependency-graph has partial analysis)
