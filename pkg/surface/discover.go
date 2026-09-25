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
//
//nolint:gochecknoglobals // policy table, read-only
var skippedDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	".git":         true,
	"result":       true,
}

// errNotDirectory is wrapped with the root path by Discover.
var errNotDirectory = errors.New("surface: root is not a directory")

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
		return nil, nil, fmt.Errorf("%w: %q", errNotDirectory, root)
	}

	surf := &Surface{Root: absRoot}

	var issues []Issue

	walkErr := filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if skippedDirs[entry.Name()] {
				return filepath.SkipDir
			}

			return nil
		}

		relStr, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return relErr
		}

		issues = append(issues, surf.absorbFile(path, FilePath(relStr), entry.Name())...)

		return nil
	})
	if walkErr != nil {
		return nil, issues, fmt.Errorf("surface: walk %q: %w", root, walkErr)
	}

	return surf, issues, nil
}

// absorbFile folds one discovered file into the surface and returns any
// discovery issues it raised. It is Discover's per-file dispatch.
func (s *Surface) absorbFile(path string, rel FilePath, name string) []Issue {
	switch {
	case name == "go.mod":
		module, toolchain, issue := parseGoMod(path, rel)

		if issue != nil {
			return []Issue{*issue}
		}

		if module != nil {
			s.Modules = append(s.Modules, *module)
		}

		if toolchain != nil {
			s.Toolchains = append(s.Toolchains, *toolchain)
		}
	case name == "go.work":
		workspace, toolchain, issue := parseGoWork(path, rel)

		if issue != nil {
			return []Issue{*issue}
		}

		if workspace != nil {
			s.Modules = append(s.Modules, *workspace)
		}

		if toolchain != nil {
			s.Toolchains = append(s.Toolchains, *toolchain)
		}
	case name == "flake.nix":
		s.NixPins = append(s.NixPins, scanNixPins(path, rel)...)
	case name == "flake.lock":
		// deliberately unmodeled: the lock alone does not name a Go version
		// (resolving it needs an impure nix eval, see TODO_LIST.md T9)
	case isCIWorkflow(rel):
		s.CIPins = append(s.CIPins, scanCIPins(path, rel)...)
	}

	return nil
}

// parseGoMod extracts the module path, the `go` directive, and the
// `toolchain` directive from one go.mod.
func parseGoMod(path string, rel FilePath) (*ModuleDirective, *ToolchainDirective, *Issue) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, &Issue{
			Rule:    RuleGoModUnparseable,
			Message: fmt.Sprintf("read go.mod: %v", err),
			File:    string(rel),
		}
	}

	version, line, err := ParseDirective(KindGoMod, data)
	if errors.Is(err, ErrNoDirective) {
		return nil, nil, nil
	}

	if err != nil {
		return nil, nil, &Issue{
			Rule:    RuleGoModUnparseable,
			Message: err.Error(),
			File:    string(rel),
		}
	}

	modulePath, moduleErr := ParseModulePath(KindGoMod, data)
	if moduleErr != nil {
		return nil, nil, &Issue{
			Rule:    RuleGoModUnparseable,
			Message: moduleErr.Error(),
			File:    string(rel),
		}
	}

	return &ModuleDirective{
		Path:    string(rel),
		Kind:    KindGoMod,
		Module:  modulePath,
		Version: version,
		Line:    line,
	}, parseToolchainOf(KindGoMod, rel, data), nil
}

// parseGoWork extracts the `go` and `toolchain` directives from a go.work
// file. Parse failures surface as a discovery issue: dropping the file
// silently would hide its `go` directive from the whole analysis (a
// `toolchain local` line, which the go tool rejects, is one such case).
func parseGoWork(path string, rel FilePath) (*ModuleDirective, *ToolchainDirective, *Issue) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, &Issue{
			Rule:    RuleGoWorkUnparseable,
			Message: fmt.Sprintf("read go.work: %v", err),
			File:    string(rel),
		}
	}

	version, line, err := ParseDirective(KindGoWork, data)
	if errors.Is(err, ErrNoDirective) {
		return nil, parseToolchainOf(KindGoWork, rel, data), nil
	}

	if err != nil {
		return nil, nil, &Issue{
			Rule:    RuleGoWorkUnparseable,
			Message: err.Error(),
			File:    string(rel),
		}
	}

	workspace := &ModuleDirective{Path: string(rel), Kind: KindGoWork, Version: version, Line: line}

	return workspace, parseToolchainOf(KindGoWork, rel, data), nil
}

// parseToolchainOf extracts the `toolchain` directive from already-read
// file content; a file without one yields nil.
func parseToolchainOf(kind DirectiveKind, rel FilePath, data []byte) *ToolchainDirective {
	version, line, err := ParseToolchain(kind, data)
	if err != nil || version == "" {
		return nil
	}

	return &ToolchainDirective{Path: string(rel), Kind: kind, Version: version, Line: line}
}

// nixGoPinRe matches nixpkgs Go references: go_1_26, go_1_27, and the
// buildGo126Module builder function names (3-digit encoding of major.minor).
var nixGoPinRe = regexp.MustCompile(`\bgo_([0-9]+)_([0-9]+)\b|\bbuildGo([0-9]{3})Module\b`)

// readLines returns path's content split into lines, or nil when the file
// cannot be read: a missing flake.nix or workflow simply carries no pins.
func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	return strings.Split(string(data), "\n")
}

// stripNixComments removes Nix comment text from every line while preserving
// the line count (pin line numbers must stay faithful to the source file).
// Handled: `#` line comments and `/* ... */` block comments, both only
// outside strings — a `#` inside a quoted URL is not a comment, and quoted
// pin-bearing text stays scannable. Double-quoted strings honor backslash
// escapes; indented strings (” ... ”) toggle on the two-quote token
// without deeper interpolation handling (pins never appear interpolated in
// practice, and a wrong guess inside a string can only under-report a pin,
// never invent one). Discovered via go-health's flake.nix: a comment
// explaining why the pin IS go_1_27 mentioned go_1_26 and was reported as
// the pin itself.
func stripNixComments(lines []string) []string {
	stripped := make([]string, len(lines))

	inBlock, inDouble, inIndented := false, false, false

	for i, line := range lines {
		var out strings.Builder

		for j := 0; j < len(line); {
			switch {
			case inBlock:
				if strings.HasPrefix(line[j:], "*/") {
					inBlock = false
					j += 2
				} else {
					j++
				}
			case inDouble:
				if line[j] == '\\' && j+1 < len(line) {
					out.WriteByte(line[j])
					out.WriteByte(line[j+1])
					j += 2
				} else {
					if line[j] == '"' {
						inDouble = false
					}
					out.WriteByte(line[j])
					j++
				}
			case inIndented:
				if strings.HasPrefix(line[j:], "''") {
					inIndented = false
					out.WriteString("''")
					j += 2
				} else {
					out.WriteByte(line[j])
					j++
				}
			case strings.HasPrefix(line[j:], "/*"):
				inBlock = true
				j += 2
			case strings.HasPrefix(line[j:], "''"):
				inIndented = true
				out.WriteString("''")
				j += 2
			case line[j] == '"':
				inDouble = true
				out.WriteByte('"')
				j++
			case line[j] == '#':
				j = len(line)
			default:
				out.WriteByte(line[j])
				j++
			}
		}

		stripped[i] = out.String()
	}

	return stripped
}

// scanNixPins extracts every nixpkgs Go pin with its line number.
func scanNixPins(path string, rel FilePath) []Pin {
	var pins []Pin

	for lineNo, line := range stripNixComments(readLines(path)) {
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
				Path:    string(rel),
				Version: GoVersion(parsed.String()),
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
func scanCIPins(path string, rel FilePath) []Pin {
	var pins []Pin

	for lineNo, line := range readLines(path) {
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
			Path:    string(rel),
			Version: GoVersion(parsed.String()),
			Raw:     GoVersion(raw),
			Line:    lineNo + 1,
			Source:  PinCI,
		})
	}

	return pins
}

// isCIWorkflow reports whether rel points into .github/workflows.
func isCIWorkflow(rel FilePath) bool {
	return strings.HasPrefix(string(rel), ".github/workflows/") &&
		(strings.HasSuffix(string(rel), ".yml") || strings.HasSuffix(string(rel), ".yaml"))
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
