package fix

import (
	"fmt"
	"os"
	"strings"

	atomicwrite "github.com/larsartmann/go-atomic-write"
	"github.com/larsartmann/go-version-auto-configure/pkg/surface"
)

// CI-pin raises are mechanical: the rule only fires when the pin sits BELOW
// the module floor, so the rewrite is always a raise-to-floor, never a
// downgrade, and a below-floor pin is already untruthful — with
// GOTOOLCHAIN=auto (the default) the go command silently switches above the
// pin anyway. Unlike go.mod rewrites there is no dependency gate to run:
// the workflow pin is inert YAML for module resolution, so the oracle is
// re-parsing the rewritten content with the same parser discovery used to
// detect the pin.

// applyCIPinFix performs one byte-preserving go-version rewrite on a CI
// workflow, preserving the quoting style of the value as written. The
// original content is restored when verification fails.
func applyCIPinFix(abs string, fx surface.Fix) error {
	original, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("read %s: %w", abs, err)
	}

	if err := verifyCIPinDrift(original, fx); err != nil {
		return err
	}

	updated, err := rewriteCIPinValue(string(original), fx)
	if err != nil {
		return err
	}

	if err := atomicwrite.Write(abs, []byte(updated)); err != nil {
		return fmt.Errorf("write %s: %w", abs, err)
	}

	if err := verifyCIPinMatches(updated, fx); err != nil {
		if revertErr := atomicwrite.Write(abs, original); revertErr != nil {
			return fmt.Errorf("%w (AND revert of %s failed: %w)", err, abs, revertErr)
		}

		return err
	}

	return nil
}

// verifyCIPinDrift rejects the fix when the pin on disk no longer matches
// the value Analyze recorded: rewriting anyway would silently apply a
// different change than the one detected.
func verifyCIPinDrift(content []byte, fx surface.Fix) error {
	for _, pin := range surface.ParseCIPins(content, surface.FilePath(fx.File)) {
		if pin.Line != fx.Line {
			continue
		}

		if pin.Raw != fx.From {
			return fmt.Errorf(
				"%w: got go-version %s, want %s",
				errDirectiveDrifted,
				pin.Raw,
				fx.From,
			)
		}

		return nil
	}

	return fmt.Errorf(
		"%w: %s:%d carries no comparable go-version pin",
		errDirectiveDrifted,
		fx.File,
		fx.Line,
	)
}

// rewriteCIPinValue replaces the recorded pin text on the fix's line,
// wrapping the replacement in the original value's quoting style (bare,
// double, single, backtick, or a YAML flow-sequence bracket).
func rewriteCIPinValue(content string, fx surface.Fix) (string, error) {
	lines := strings.Split(content, "\n")

	if fx.Line < 1 || fx.Line > len(lines) {
		return "", fmt.Errorf(
			"%w: line %d out of range in %s (%d lines)",
			errDirectiveDrifted,
			fx.Line,
			fx.File,
			len(lines),
		)
	}

	line := lines[fx.Line-1]

	span := string(fx.From)
	start := strings.Index(line, span)

	if start < 0 {
		return "", fmt.Errorf(
			"%w: %s:%d no longer carries %s",
			errDirectiveDrifted,
			fx.File,
			fx.Line,
			span,
		)
	}

	end := start + len(span)

	lines[fx.Line-1] = line[:start] + ciQuoted(span, string(fx.To)) + line[end:]

	return strings.Join(lines, "\n"), nil
}

// ciQuoted renders newValue in the quoting style of the recorded pin text:
// a leading quote character wraps the replacement in the same character,
// anything else (bare values, bracket-stripped flow sequences) passes the
// replacement through — the surrounding bracket stays in the line.
func ciQuoted(recorded, newValue string) string {
	switch recorded[0] {
	case '"', '\'', '`':
		return string(recorded[0]) + newValue + string(recorded[0])
	default:
		return newValue
	}
}

// verifyCIPinMatches re-parses the rewritten content with the detection
// parser and reports a mismatch against the fix target.
func verifyCIPinMatches(content string, fx surface.Fix) error {
	for _, pin := range surface.ParseCIPins([]byte(content), surface.FilePath(fx.File)) {
		if pin.Line != fx.Line {
			continue
		}

		if pin.Version != fx.To {
			return fmt.Errorf(
				"%w: got go-version %s, want %s",
				errDirectiveMismatch,
				pin.Version,
				fx.To,
			)
		}

		return nil
	}

	return fmt.Errorf(
		"%w: %s:%d carries no comparable go-version pin after the rewrite",
		errDirectiveMismatch,
		fx.File,
		fx.Line,
	)
}
