package fix

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkAnalyzeFloors measures the who-forces per-module path: discovery,
// directive parsing, the dependency-floor fold (accumulateFloor), and the row
// finalize (finalizeFloors) — the hot path reshaped after the T11 baseline
// (2026-09-22) and, per TODO_LIST T16, never re-measured. The graph is
// synthetic (200 dependencies, a quarter of them at the directive's floor, so
// the parity branch of finalizeFloors is exercised) and the runner is faked,
// keeping the benchmark hermetic: no go tool, no network, deterministic.
func BenchmarkAnalyzeFloors(b *testing.B) {
	const depCount = 200

	var list strings.Builder

	for i := range depCount {
		floor := "1.26"
		if i%4 == 0 {
			floor = "1.27.1"
		}

		fmt.Fprintf(&list, "%s\tgithub.com/example/dep%03d\tv1.0.%d\n", floor, i, i)
	}

	root := b.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/m\n\ngo 1.27.1\n"),
		0o644,
	); err != nil {
		b.Fatal(err)
	}

	run := fakeList(list.String())
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		rows, err := AnalyzeFloors(ctx, root, run)
		if err != nil {
			b.Fatal(err)
		}

		if len(rows) != 1 {
			b.Fatalf("rows = %d, want 1", len(rows))
		}
	}
}
