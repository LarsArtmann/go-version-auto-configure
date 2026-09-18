# Domain Language

The ubiquitous language of go-version-auto-configure. Terms are defined as the code uses them.

## Glossary

| Term                | Definition                                                                                                                                                             | Used in                                                             |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| Version surface     | Every location in a repository where a Go version is declared: go.mod `go` directives, go.work, flake.nix nixpkgs pins, CI `go-version:` pins                          | `pkg/surface/surface.go` (the `Surface` type)                       |
| Go directive        | The `go` line in go.mod or go.work. Semantically a floor: the minimum toolchain the module needs. Fleet policy: major.minor only                                       | `pkg/surface/parse.go` (`ParseDirective`)                           |
| Floor               | The minimum Go version something requires. A module declares one via its directive; a whole repo's is the highest across all modules                                   | `pkg/surface/surface.go` (`Surface.Floor`)                          |
| Workspace floor     | The repo-wide floor: the highest declared major.minor across all module directives, including go.work files                                                            | `Surface.Floor`, `pkg/surface/rules.go`                             |
| Form violation      | A directive written in the wrong shape (`go 1.26.7` instead of `go 1.26`). Unambiguous, so mechanically auto-fixable                                                   | `RuleGoDirectivePatchForm`, `RuleWorkDirectivePatchForm`            |
| Alignment violation | Two surface locations disagree on the minor (e.g. flake pins `go_1_26`, module declares `go 1.27`). Which side moves is a maintainer decision, so suggest-only         | `RuleNixPinBelowFloor`, `RuleCIPinBelowFloor`, `RuleCIPinPatchForm` |
| Floor poisoning     | `go mod tidy` lifts a consumer's directive to the highest dependency floor. One published library with a patch-form floor re-poisons every consumer that tidies        | AGENTS.md, TODO_LIST T1                                             |
| Poisoner            | A dependency whose own `go` floor equals the value tidy enforces — the named re-tag target of a dep-forced fix                                                         | `pkg/fix/fix.go` (`resolvePoisoners`)                               |
| Dep-forced          | Classification of a form fix that `go mod tidy` reverts because dependencies force a higher floor. Reported truthfully instead of claiming a fix that silently reverts | `pkg/fix/fix.go` (`DepForcedError`, `Result.DepForced`)             |
| Pin                 | A Go version reference outside the Go module system: a flake.nix nixpkgs pin (`go_1_26`, `buildGo126Module`) or a CI workflow `go-version:`                            | `pkg/surface/surface.go` (`Pin`, `PinSource`)                       |
| Comparable pin      | A pin stating an exact numeric version. Expressions (`${{ }}`), ranges (`1.26.x`), and words (`stable`) are non-comparable and never recorded                          | `pkg/surface/parse.go` (`parseCIPin`)                               |
| Discovery issue     | A go.mod that cannot be parsed. Reported as a finding so one broken file cannot hide drift elsewhere; never auto-fixed                                                 | `RuleGoModUnparseable`                                              |
| Mechanical fix      | An unambiguous, semantics-preserving rewrite applied via `go mod edit` / `go work edit` and verified by re-parsing with the same code that detected it                 | `pkg/fix/fix.go`, `pkg/surface/parse.go`                            |
| Supply-side re-tag  | Re-publishing a library with major.minor-only floors so consumers stop being re-poisoned. The permanent fix for floor poisoning (TODO_LIST T1)                         | TODO_LIST.md, ROADMAP.md                                            |

## Bounded contexts

- **"Floor" in this repo** always means the Go toolchain floor — not a dependency version requirement and not a Nix store path.
- **"Fix" in `pkg/fix`** means an applied-and-verified directive rewrite. A fix that tidy reverts is never counted as applied — it is dep-forced.
- **"Trigger"** (provider context) means BuildFlow's file-pattern gate for running the provider (`go`, `go.mod`, `go.work`), not a CI trigger.
