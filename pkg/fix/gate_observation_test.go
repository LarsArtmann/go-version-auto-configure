package fix

// Gate observation helpers for the staged-rewrite tests (fix_staged_test.go,
// canonicalize_staged_test.go, canonicalize_e2e_test.go): what a gate saw
// of the module at the moment it ran.

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Canned gate failures of the staged-gate tests.
var (
	errVetLanguageFloor = errors.New("go vet: stdversion: fmt.Println requires go1.27 or later")
	errNoMainModule     = errors.New("go: cannot find main module")
)

// gateObservation is what the dependency gate saw of the module at the
// moment it ran.
type gateObservation struct {
	// live is the live go.mod content at gate time.
	live string
	// modFile is the -modfile argument; empty when no gate ran.
	modFile string
	// candidate is the staged content the gate judged; empty when no gate ran.
	candidate string
}

// modFileArg returns the value of the -modfile flag in args, or "".
func modFileArg(args []string) string {
	for _, arg := range args {
		if value, found := strings.CutPrefix(arg, "-modfile="); found {
			return value
		}
	}

	return ""
}

// observingGate wraps reply into a dependency gate that records the live
// go.mod content and the -modfile argument at the moment it runs.
func observingGate(
	goModPath string,
	seen *gateObservation,
	reply func(dir string, args []string) (string, string, error),
) SplitRunner {
	return fakeGate(func(dir string, args []string) (string, string, error) {
		if len(args) == 0 || args[0] != "mod" {
			return reply(dir, args)
		}

		content, _ := os.ReadFile(goModPath)

		seen.live = string(content)
		seen.modFile = modFileArg(args)
		seen.candidate = readFileForGate(seen.modFile)

		return reply(dir, args)
	})
}

// extraGateSighting is what an ExtraGate observed when it ran; livePath is
// the go.mod whose live content the gate records beside its arguments.
type extraGateSighting struct {
	livePath  string
	calls     int
	dir       string
	candidate string
	live      string
}

// record stores the gate's arguments and what it saw of the module.
func (s *extraGateSighting) record(dir, modFile string) {
	s.calls++
	s.dir = dir
	s.candidate = readFileForGate(modFile)
	s.live = readFileForGate(s.livePath)
}

// approve records the gate's arguments and approves the candidate.
func (s *extraGateSighting) approve(_ context.Context, dir, modFile string) error {
	s.record(dir, modFile)

	return nil
}

// reject returns an ExtraGate that records its call and refuses the
// candidate with cause.
func (s *extraGateSighting) reject(cause error) ExtraGate {
	return func(_ context.Context, dir, modFile string) error {
		s.record(dir, modFile)

		return cause
	}
}

func readFileForGate(path string) string {
	content, _ := os.ReadFile(path)

	return string(content)
}

// readFile is os.ReadFile with the content attached to the test's failure.
func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}

// assertStagedCopyGone checks that the staged copy a gate observed no
// longer exists.
func assertStagedCopyGone(t *testing.T, modFile string) {
	t.Helper()

	_, statErr := os.Stat(modFile)

	assert.ErrorIs(t, statErr, os.ErrNotExist, "the staged copy is removed once the verdict is in")
}
