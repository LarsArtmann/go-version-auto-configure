package surface

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// skippedDirs are never walked: vendored and generated trees do not declare
// this repository's own toolchain floor.
var skippedDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"result":       true,
}

// Discover walks root and returns the repository's Go version surface:
// every go.mod `go` directive, go.work `go` directive, flake.nix nixpkgs Go
// pin, and CI workflow `go-version:` pin. Unparseable go.mod files surface
// as issues (RuleGoModUnparseable) instead of aborting the walk, so one bad
// fixture never hides the drift in a hundred real modules.
func Discover(root string) (*Surface, []Issue, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, fmt.Errorf("surface: resolve root %q: %w", root, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("surface: stat root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("surface: root %q is not a directory", root)
	}

	s := &Surface{Root: absRoot}
	var issues []Issue

	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if skippedDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return relErr
		}
		switch {
		case name == "go.mod":
			module, toolchain, issue := parseGoMod(path, rel)
			if issue != nil {
				issues = append(issues, *issue)
				return nil
			}
			if module != nil {
				s.Modules = append(s.Modules, *module)
			}
			if toolchain != nil {
				s.Toolchains = append(s.Toolchains, *toolchain)
			}
		case name == "go.work":
			workspace, toolchain := parseGoWork(path, rel)
			if workspace != nil {
				s.Modules = append(s.Modules, *workspace)
			}
			if toolchain != nil {
				s.Toolchains = append(s.Toolchains, *toolchain)
			}
		case name == "flake.nix":
			s.NixPins = append(s.NixPins, scanNixPins(path, rel)...)
		case name == "flake.lock":
			return nil
		case isCIWorkflow(rel):
			s.CIPins = append(s.CIPins, scanCIPins(path, rel)...)
		}
		return nil
	})
	if walkErr != nil {
		return nil, issues, fmt.Errorf("surface: walk %q: %w", root, walkErr)
	}

	return s, issues, nil
}

// parseGoMod extracts the module path, the `go` directive, and the
// `toolchain` directive from one go.mod.
func parseGoMod(path, rel string) (*ModuleDirective, *ToolchainDirective, *Issue) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, &Issue{Rule: RuleGoModUnparseable, Message: fmt.Sprintf("read go.mod: %v", err), File: rel}
	}
	version, line, err := ParseDirective(KindGoMod, data)
	if errors.Is(err, ErrNoDirective) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, &Issue{
			Rule:    RuleGoModUnparseable,
			Message: err.Error(),
			File:    rel,
		}
	}
	modulePath, moduleErr := ParseModulePath(KindGoMod, data)
	if moduleErr != nil {
		return nil, nil, &Issue{
			Rule:    RuleGoModUnparseable,
			Message: moduleErr.Error(),
			File:    rel,
		}
	}
	return &ModuleDirective{
		Path:    rel,
		Kind:    KindGoMod,
		Module:  modulePath,
		Version: version,
		Line:    line,
	}, parseToolchainOf(KindGoMod, rel, data), nil
}

// parseGoWork extracts the `go` and `toolchain` directives from a go.work
// file.
func parseGoWork(path, rel string) (*ModuleDirective, *ToolchainDirective) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	version, line, err := ParseDirective(KindGoWork, data)
	if err != nil {
		return nil, nil
	}
	workspace := &ModuleDirective{Path: rel, Kind: KindGoWork, Version: version, Line: line}
	return workspace, parseToolchainOf(KindGoWork, rel, data)
}

// parseToolchainOf extracts the `toolchain` directive from already-read
// file content; a file without one yields nil.
func parseToolchainOf(kind DirectiveKind, rel string, data []byte) *ToolchainDirective {
	version, line, err := ParseToolchain(kind, data)
	if err != nil || version == "" {
		return nil
	}
	return &ToolchainDirective{Path: rel, Kind: kind, Version: version, Line: line}
}

// nixGoPinRe matches nixpkgs Go references: go_1_26, go_1_27, and the
// buildGo126Module builder function names (3-digit encoding of major.minor).
var nixGoPinRe = regexp.MustCompile(`\bgo_([0-9]+)_([0-9]+)\b|\bbuildGo([0-9]{3})Module\b`)

// scanNixPins extracts every nixpkgs Go pin with its line number.
func scanNixPins(path, rel string) []Pin {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pins []Pin
	for lineNo, line := range strings.Split(string(data), "\n") {
		for _, match := range nixGoPinRe.FindAllStringSubmatch(line, -1) {
			var parsed majorMinor
			switch {
			case match[1] != "":
				parsed = majorMinor{Major: atoi(match[1]), Minor: atoi(match[2])}
			case match[3] != "":
				digits := match[3]
				parsed = majorMinor{Major: atoi(digits[:1]), Minor: atoi(digits[1:])}
			default:
				continue
			}
			pins = append(pins, Pin{
				Path:    rel,
				Version: parsed.String(),
				Line:    lineNo + 1,
				Source:  PinNixFlake,
			})
		}
	}
	return pins
}

// ciGoVersionRe captures the value of a `go-version:` key on one line.
var ciGoVersionRe = regexp.MustCompile(`^\s*go-version:\s*(.+?)\s*$`)

// scanCIPins extracts every comparable CI go-version pin. Expression pins
// (${{ matrix.go }}) and ranges are skipped: they carry no fixed floor.
func scanCIPins(path, rel string) []Pin {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pins []Pin
	for lineNo, line := range strings.Split(string(data), "\n") {
		match := ciGoVersionRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		raw := match[1]
		if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
			raw = strings.Trim(raw, "[]")
		}
		parsed, ok := parseCIPin(raw)
		if !ok {
			continue
		}
		pins = append(pins, Pin{
			Path:    rel,
			Version: parsed.String(),
			Raw:     raw,
			Line:    lineNo + 1,
			Source:  PinCI,
		})
	}
	return pins
}

// isCIWorkflow reports whether rel points into .github/workflows.
func isCIWorkflow(rel string) bool {
	return strings.HasPrefix(rel, ".github/workflows/") &&
		(strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml"))
}

// atoi is strconv.Atoi without the error path; callers only pass digit
// runs matched by the pin regexes.
func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
