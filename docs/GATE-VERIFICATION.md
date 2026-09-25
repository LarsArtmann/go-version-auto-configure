# Gate-Verification Recipe: testing a poisoner consumer safely

How to verify that the dependency-floor gate protects a consumer without mutating the consumer's working tree. Used live 2026-09-25 on go-health-dashboard (go-health v0.4.0 era); recorded because the mutation alternative is exactly the incident class this tool exists to prevent (BuildFlow gotcha #169, the dashboard's three 2026-09-22 downgrade incidents).

## Steps

```bash
# 1. Build the tool under test from the exact source you want to verify
GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2 go build -o /tmp/gvac ./cmd/go-version-auto-configure

# 2. Copy the consumer WITHOUT its .git (cheaper, and the daemon cannot
#    auto-commit a tree it cannot see)
cp -r --no-preserve=mode,ownership ~/projects/<consumer> /tmp/<consumer>-gate-test
rm -rf /tmp/<consumer>-gate-test/.git   # use trash if you prefer

# 3. Record the directive, run the REAL fix (not --dry-run: dry-run holds
#    everything back by design and never exercises the tidy gate)
grep '^go ' /tmp/<consumer>-gate-test/go.mod
/tmp/gvac fix /tmp/<consumer>-gate-test

# 4. Assert the outcome:
#    - dep-forced path: "dep-forced 1" + floor carrier named + go.mod UNTOUCHED
#    - mechanical path: "applied 1" + directive at minor form + survives tidy
grep '^go ' /tmp/<consumer>-gate-test/go.mod

# 5. Clean up with trash, never rm -rf on anything you did not create
trash /tmp/<consumer>-gate-test
```

## Why a copy (and not `fix --dry-run`)

`--dry-run` reports what would be attempted but holds every fix back before the gate runs — it cannot distinguish "would apply cleanly" from "the dependency floor forbids this." Only a real `fix` executes the non-mutating `go mod tidy -diff` gate with atomic revert. Running that real fix on the consumer's checkout would leave a mutation window the auto-commit daemon can sweep (it has committed half-written go.mod files before — BuildFlow gotcha #152); the throwaway copy has no daemon and no `.git` to sweep.

## Preconditions

- No `buildflow` process may be mid-run in the consumer (torn go.mod state — their AGENTS rule); check `pgrep -fa buildflow` first.
- The toolchain env for building gvac: `GOTOOLCHAIN=go1.27.1 GOEXPERIMENT=jsonv2` (outside the devShell).
