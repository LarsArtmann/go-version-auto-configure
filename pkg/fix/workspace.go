// Workspace-level go.work directive syncing: the scoped entry point for
// consumers that own go.work health (BuildFlow's go-work-sync arbiter) and
// want the version-surface rules applied without touching module files.
package fix

import (
	"context"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// SyncGoWorkDirectives discovers the version surface under root, collects
// every mechanically safe go.work directive fix — patch-form strips that
// keep the workspace valid, and below-floor restorations — and applies
// them. Module go.mod fixes are deliberately out of scope here: the go.work
// file must cover every module, so its rules run after module work, as the
// arbiter step.
//
// A repo without a go.work yields an empty Result. DryRun holds the fixes
// back (Result.HeldBack); DepForced never occurs for go.work (raising a
// floor is always resolvable). run is the go command runner; nil uses the
// production exec runner.
func SyncGoWorkDirectives(ctx context.Context, root string, opts Options, run GoCommandRunner) (*Result, error) {
	s, _, err := surface.Discover(root)
	if err != nil {
		return nil, err
	}

	var fixes []surface.Fix

	for _, issue := range surface.Analyze(s) {
		if issue.Fix == nil || issue.Fix.Kind != surface.KindGoWork {
			continue
		}

		fixes = append(fixes, *issue.Fix)
	}

	if len(fixes) == 0 {
		return &Result{}, nil
	}

	return Apply(ctx, root, fixes, opts, run)
}
