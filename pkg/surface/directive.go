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
