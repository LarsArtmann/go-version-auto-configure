package fix

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDebug_WorkspaceFixture(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.27\n\nuse ./a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "go.mod"), []byte("module example.com/a\n\ngo 1.27.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := SyncGoWorkDirectives(context.Background(), root, Options{}, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	t.Logf("applied=%d heldback=%d report=%s", len(res.Applied), len(res.HeldBack), res.Report())
	for _, a := range res.Applied {
		t.Logf("applied: %+v", a)
	}
	content, _ := os.ReadFile(filepath.Join(root, "go.work"))
	t.Logf("go.work after: %q", string(content))
}
