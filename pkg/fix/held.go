package fix

import (
	"context"
	"fmt"
	"strings"

	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// HeldStrip explains why a patch-form go.mod finding keeps its mechanical
// strip withheld: `go mod tidy` would revert the rewrite while dependencies
// hold the floor. It is the finding-facing cut of AnalyzeFloors, so a
// finding consumer (the BuildFlow provider's "could not auto-fix" view) can
// explain the withholding instead of showing a bare finding.
type HeldStrip struct {
	// Floor is the dependency floor holding the directive: the directive's
	// own value at parity, the higher floor when poisoned.
	Floor surface.GoVersion
	// Carriers names the dependencies holding that floor, sorted highest
	// floor first (the same rows who-forces reports).
	Carriers []DependencyFloor
	// Poisoned reports whether the floor exceeds the directive outright:
	// tidy would re-raise above the declared version, not merely revert a
	// strip.
	Poisoned bool
}

// Reason renders why the mechanical strip was not auto-applied; it is
// appended to the held finding's message.
func (h HeldStrip) Reason() string {
	if h.Poisoned {
		return fmt.Sprintf(
			"not auto-fixed: dependencies force go %s above the declared directive (%s)",
			h.Floor,
			h.carriersText(),
		)
	}

	return fmt.Sprintf(
		"not auto-fixed: go mod tidy reverts the strip, go %s is forced by %s",
		h.Floor,
		h.carriersText(),
	)
}

// Remediation renders the replacement suggestion for a held finding: the
// mechanical rewrite stays withheld, so the honest next step is named
// instead of a strip tidy would silently revert.
func (h HeldStrip) Remediation() string {
	if h.Poisoned {
		return fmt.Sprintf(
			"raise the go directive to the forced floor (go mod edit -go=%s) or re-tag the forcers with lower floors; "+
				"who-forces reports the full matrix",
			h.Floor,
		)
	}

	return "fix supply-side: re-tag " + h.carriersText() +
		" with a major.minor-only go directive, then bump this repo; who-forces reports the full matrix"
}

// carriersText renders the carriers as module@version, comma-joined; a
// poisoned row carries its floor per carrier because those vary.
func (h HeldStrip) carriersText() string {
	parts := make([]string, 0, len(h.Carriers))

	for _, carrier := range h.Carriers {
		text := string(carrier.Module) + "@" + string(carrier.Version)
		if h.Poisoned {
			text += fmt.Sprintf(" (go %s)", carrier.Floor)
		}

		parts = append(parts, text)
	}

	return strings.Join(parts, ", ")
}

// ExplainHeldStrips resolves, read-only, which of the given patch-form
// go.mod files `go mod tidy` would hold at their current directive (the
// mechanical strip reverts), and names the dependencies carrying the floor.
// The answer reduces from AnalyzeFloors: a row at parity is held by its
// ParityFloors, a poisoned row by its PoisonerFloors; a row whose
// dependencies resolve below the directive is not held (the strip is
// auto-fixable and repair applies it).
//
// Two boundaries, both deliberate. Files whose graph cannot be listed are
// omitted: the annotation is unavailable, never guessed. And floors no
// listed dependency carries (a replace target, the standard library's own
// floor; DepForced.Cause in Apply) are invisible to `go list -m` and stay
// unannotated here; only the tidy-diff gate during Apply can name them.
func ExplainHeldStrips(
	ctx context.Context,
	root string,
	run GoCommandRunner,
	files []string,
) map[string]HeldStrip {
	if len(files) == 0 {
		return nil
	}

	rows, err := AnalyzeFloors(ctx, root, run)
	if err != nil {
		return nil
	}

	wanted := make(map[string]bool, len(files))
	for _, file := range files {
		wanted[file] = true
	}

	held := make(map[string]HeldStrip, len(files))

	for _, row := range rows {
		if !wanted[row.Path] || row.Error != "" {
			continue
		}

		if strip, ok := heldStrip(row); ok {
			held[row.Path] = strip
		}
	}

	if len(held) == 0 {
		return nil
	}

	return held
}

// heldStrip reduces one floor-matrix row to its finding-facing explanation:
// held at parity (ParityFloors carry the directive's own value) or poisoned
// outright (PoisonerFloors force a higher floor). The second return is
// false when nothing holds the strip.
func heldStrip(row ModuleFloors) (HeldStrip, bool) {
	switch {
	case row.Poisoned && len(row.PoisonerFloors) > 0:
		return HeldStrip{Floor: row.MaxDepFloor, Carriers: row.PoisonerFloors, Poisoned: true}, true
	case len(row.ParityFloors) > 0:
		return HeldStrip{Floor: row.MaxDepFloor, Carriers: row.ParityFloors}, true
	default:
		return HeldStrip{}, false
	}
}
