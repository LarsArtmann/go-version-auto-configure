package surface

import (
	"errors"
	"fmt"

	"golang.org/x/mod/modfile"
)

// ErrNoDirective reports a well-formed file that declares no `go`
// directive. Callers treat it as "nothing to check", not an error: missing
// directives are a separate concern owned by BuildFlow's gomod-checker.
var ErrNoDirective = errors.New("surface: no go directive")

// ParseDirective extracts the `go` directive (version and 1-based line)
// from go.mod or go.work file content. It is the single parsing entry point
// shared by discovery and by fix verification, so the fixer checks edits
// with exactly the same code that detected the violation.
func ParseDirective(kind DirectiveKind, data []byte) (version string, line int, err error) {
	switch kind {
	case KindGoMod:
		file, parseErr := modfile.Parse("go.mod", data, nil)
		if parseErr != nil {
			return "", 0, fmt.Errorf("parse go.mod: %w", parseErr)
		}
		if file.Go == nil {
			return "", 0, ErrNoDirective
		}
		return file.Go.Version, syntaxLine(file.Go.Syntax), nil
	case KindGoWork:
		file, parseErr := modfile.ParseWork("go.work", data, nil)
		if parseErr != nil {
			return "", 0, fmt.Errorf("parse go.work: %w", parseErr)
		}
		if file.Go == nil {
			return "", 0, ErrNoDirective
		}
		return file.Go.Version, syntaxLine(file.Go.Syntax), nil
	default:
		return "", 0, fmt.Errorf("unknown directive kind %q", kind)
	}
}

// syntaxLine returns the 1-based line of a directive, 0 when unknown.
func syntaxLine(syntax *modfile.Line) int {
	if syntax == nil {
		return 0
	}
	return syntax.Start.Line
}

// ParseToolchain extracts the `toolchain` directive (version and 1-based
// line) from go.mod or go.work file content. An absent directive is normal:
// it yields an empty version and a nil error. The version keeps the `go`
// prefix as written (e.g. "go1.26.7"), matching modfile's representation.
func ParseToolchain(kind DirectiveKind, data []byte) (version string, line int, err error) {
	switch kind {
	case KindGoMod:
		file, parseErr := modfile.Parse("go.mod", data, nil)
		if parseErr != nil {
			return "", 0, fmt.Errorf("parse go.mod: %w", parseErr)
		}
		if file.Toolchain == nil {
			return "", 0, nil
		}
		return file.Toolchain.Name, syntaxLine(file.Toolchain.Syntax), nil
	case KindGoWork:
		file, parseErr := modfile.ParseWork("go.work", data, nil)
		if parseErr != nil {
			return "", 0, fmt.Errorf("parse go.work: %w", parseErr)
		}
		if file.Toolchain == nil {
			return "", 0, nil
		}
		return file.Toolchain.Name, syntaxLine(file.Toolchain.Syntax), nil
	default:
		return "", 0, fmt.Errorf("unknown directive kind %q", kind)
	}
}

// ParseModulePath extracts the module path from go.mod content. go.work
// declares no module and yields an empty path.
func ParseModulePath(kind DirectiveKind, data []byte) (string, error) {
	switch kind {
	case KindGoMod:
		file, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			return "", fmt.Errorf("parse go.mod: %w", err)
		}
		if file.Module == nil {
			return "", nil
		}
		return file.Module.Mod.Path, nil
	case KindGoWork:
		return "", nil
	default:
		return "", fmt.Errorf("unknown directive kind %q", kind)
	}
}
