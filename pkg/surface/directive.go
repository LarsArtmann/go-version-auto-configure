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

// errUnknownDirectiveKind is wrapped with the kind value by the parse
// entry points.
var errUnknownDirectiveKind = errors.New("unknown directive kind")

// ParseDirective extracts the `go` directive (version and 1-based line)
// from go.mod or go.work file content. It is the single parsing entry point
// shared by discovery and by fix verification, so the fixer checks edits
// with exactly the same code that detected the violation.
func ParseDirective(kind DirectiveKind, data []byte) (string, int, error) {
	switch kind {
	case KindGoMod:
		file, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			return "", 0, fmt.Errorf("parse go.mod: %w", err)
		}

		if file.Go == nil {
			return "", 0, ErrNoDirective
		}

		return file.Go.Version, syntaxLine(file.Go.Syntax), nil
	case KindGoWork:
		file, err := modfile.ParseWork("go.work", data, nil)
		if err != nil {
			return "", 0, fmt.Errorf("parse go.work: %w", err)
		}

		if file.Go == nil {
			return "", 0, ErrNoDirective
		}

		return file.Go.Version, syntaxLine(file.Go.Syntax), nil
	default:
		return "", 0, fmt.Errorf("%w: %q", errUnknownDirectiveKind, kind)
	}
}

// ParseToolchain extracts the `toolchain` directive (version and 1-based
// line) from go.mod or go.work file content. An absent directive is normal:
// it yields an empty version and a nil error. The version keeps the `go`
// prefix as written (e.g. "go1.26.7"), matching modfile's representation.
func ParseToolchain(kind DirectiveKind, data []byte) (string, int, error) {
	switch kind {
	case KindGoMod:
		file, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			return "", 0, fmt.Errorf("parse go.mod: %w", err)
		}

		if file.Toolchain == nil {
			return "", 0, nil
		}

		return file.Toolchain.Name, syntaxLine(file.Toolchain.Syntax), nil
	case KindGoWork:
		file, err := modfile.ParseWork("go.work", data, nil)
		if err != nil {
			return "", 0, fmt.Errorf("parse go.work: %w", err)
		}

		if file.Toolchain == nil {
			return "", 0, nil
		}

		return file.Toolchain.Name, syntaxLine(file.Toolchain.Syntax), nil
	default:
		return "", 0, fmt.Errorf("%w: %q", errUnknownDirectiveKind, kind)
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
		return "", fmt.Errorf("%w: %q", errUnknownDirectiveKind, kind)
	}
}

// syntaxLine returns the 1-based line of a directive, 0 when unknown.
func syntaxLine(syntax *modfile.Line) int {
	if syntax == nil {
		return 0
	}

	return syntax.Start.Line
}
