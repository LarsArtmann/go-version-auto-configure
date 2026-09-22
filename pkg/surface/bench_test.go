package surface

import (
	"os"
	"path/filepath"
	"testing"
)

// benchRepo builds a representative repo fixture for benchmarks: several
// modules, a workspace, toolchain lines, a flake, and CI pins.
func benchRepo(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	files := map[string]string{
		"go.mod":         "module example.com/root\n\ngo 1.26.7\n\ntoolchain go1.26.8\n",
		"sub/api/go.mod": "module example.com/sub/api\n\ngo 1.26\n",
		"sub/cli/go.mod": "module example.com/sub/cli\n\ngo 1.27\n",
		"go.work":        "go 1.26.5\n\nuse .\n\tuse ./sub/api\n\tuse ./sub/cli\n",
		"flake.nix":      "pkgs.go_1_26\n",
		".github/workflows/ci.yml": "jobs:\n  lint:\n    steps:\n" +
			"      - uses: actions/setup-go@v5\n        with:\n          go-version: 1.26.7\n",
	}

	for rel, content := range files {
		abs := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			b.Fatal(err)
		}

		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}

	return root
}

func BenchmarkDiscover(b *testing.B) {
	root := benchRepo(b)

	b.ReportAllocs()

	for b.Loop() {
		if _, _, err := Discover(root); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalyze(b *testing.B) {
	root := benchRepo(b)

	surf, discoverIssues, err := Discover(root)
	if err != nil {
		b.Fatal(err)
	}

	if len(discoverIssues) != 0 {
		b.Fatalf("fixture must be parseable, got %d issues", len(discoverIssues))
	}

	b.ReportAllocs()

	for b.Loop() {
		if issues := Analyze(surf); len(issues) == 0 {
			b.Fatal("fixture must produce findings")
		}
	}
}

func BenchmarkParseDirective(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		if _, _, err := ParseDirective(KindGoMod, []byte("module example.com/root\n\ngo 1.26.7\n")); err != nil {
			b.Fatal(err)
		}
	}
}
