package fix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyncGoWorkDirectives_HostileParentGOWORK pins the 2026-09-25 BuildFlow
// go_line failure class: a parent GOWORK=off (direnv shells, CI workers)
// must not leak into `go work edit` — the runner pins GOWORK at the
// workspace file under dir, so the below-floor fix still applies.
func TestSyncGoWorkDirectives_HostileParentGOWORK(t *testing.T) {
	t.Setenv("GOWORK", "off")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.27\n\nuse ./a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "a", "go.mod"),
		[]byte("module example.com/a\n\ngo 1.27.1\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	res, err := SyncGoWorkDirectives(context.Background(), root, Options{}, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	if len(res.Applied) != 1 {
		t.Fatalf("expected 1 applied fix, got %d (failures: %v)", len(res.Applied), res.Failures)
	}

	content, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "go 1.27.1") {
		t.Fatalf("go.work = %q, want the floor restored to go 1.27.1", string(content))
	}
}
