package fix

// The gated rewrite of CanonicalizeGoMod: every gate judges a staged copy
// of the candidate, so the live go.mod is written exactly once and only
// after the verdict (BuildFlow#124: the pre-staging gate wrote the
// downgrade to the live file first, and concurrent go steps failed with
// "updates to go.mod needed" inside that window).

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

// TestCanonicalizeGoMod_RejectedDowngradeNeverReachesTheLiveFile is the
// BuildFlow#124 regression: the dependency gate runs while the live file
// still holds the original directive, and it judges the staged candidate.
func TestCanonicalizeGoMod_RejectedDowngradeNeverReachesTheLiveFile(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26.7\n"
	path := writeGoModFixture(t, original)
	seen := gateObservation{}

	gate := observingGate(path, &seen, func(string, []string) (string, string, error) {
		return "\ndiff current/go.mod tidy/go.mod\n-go 1.26\n+go 1.26.7\n", "", nil
	})

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		InstalledToolchain: "1.26.7",
	}, gate)
	require.NoError(t, err)

	assert.Equal(t, original, seen.live, "the gate runs while the live file still holds the original directive")
	assert.NotEqual(t, path, seen.modFile, "the gate judges a staged copy, never the live file")
	assert.Equal(t, "module example.com/norm\n\ngo 1.26\n", seen.candidate, "the gate judged the staged candidate")

	assert.True(t, res.HeldBackByDepFloor)
	assert.False(t, res.Changed)
	assert.Contains(t, res.GateDetail, "+go 1.26.7", "GateDetail is the rejecting gate's own output")
	assert.Equal(t, original, readFile(t, path), "a rejected downgrade is simply not committed")
	assertStagedCopyGone(t, seen.modFile)
}

func TestCanonicalizeGoMod_AcceptedDowngradeIsCommittedOnce(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26.7\n"
	path := writeGoModFixture(t, original)
	seen := gateObservation{}

	gate := observingGate(path, &seen, func(string, []string) (string, string, error) { return "", "", nil })

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		InstalledToolchain: "1.26.7",
	}, gate)
	require.NoError(t, err)

	assert.True(t, res.Changed)
	assert.Equal(t, original, seen.live, "even an accepted downgrade is judged on the staged copy")
	assert.Equal(t, "module example.com/norm\n\ngo 1.26\n", readFile(t, path))
	assertStagedCopyGone(t, seen.modFile)
}

func TestCanonicalizeGoMod_ExtraGateSeesStagedCandidate(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26.7\n\ntoolchain go1.26.7\n"
	path := writeGoModFixture(t, original)
	sighting := extraGateSighting{livePath: path}
	gateCalls := 0

	gate := fakeGate(func(_ string, _ []string) (string, string, error) {
		gateCalls++

		return "", "", nil
	})

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		StripToolchain:     true,
		InstalledToolchain: "1.26.7",
		ExtraGate:          sighting.approve,
	}, gate)
	require.NoError(t, err)

	assert.True(t, res.Changed)
	assert.Equal(t, 1, gateCalls, "the dependency-floor gate runs first")
	assert.Equal(t, 1, sighting.calls, "the extra gate runs once, after the dependency-floor gate")
	assert.Equal(t, filepath.Dir(path), sighting.dir)
	assert.Equal(t, "module example.com/norm\n\ngo 1.26\n", sighting.candidate)
	assert.Equal(t, original, sighting.live, "the live file is still the original while the extra gate runs")
	assert.Equal(t, sighting.candidate, readFile(t, path))
}

func TestCanonicalizeGoMod_ExtraGateRejectionWithholdsRewrite(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26.7\n"
	path := writeGoModFixture(t, original)
	sighting := extraGateSighting{livePath: path}

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		InstalledToolchain: "1.26.7",
		ExtraGate:          sighting.reject(errVetLanguageFloor),
	}, noopGate())
	require.NoError(t, err)

	assert.True(t, res.HeldBackByExtraGate)
	assert.False(t, res.HeldBackByDepFloor)
	assert.False(t, res.Changed)
	assert.Equal(t, errVetLanguageFloor.Error(), res.GateDetail)
	assert.Equal(t, original, readFile(t, path))
}

func TestCanonicalizeGoMod_DepFloorRejectionSkipsExtraGate(t *testing.T) {
	t.Parallel()

	path := writeGoModFixture(t, "module example.com/norm\n\ngo 1.26.7\n")
	sighting := extraGateSighting{livePath: path}

	gate := fakeGate(func(string, []string) (string, string, error) {
		return "\ndiff current/go.mod tidy/go.mod\n", "", nil
	})

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		InstalledToolchain: "1.26.7",
		ExtraGate:          sighting.approve,
	}, gate)
	require.NoError(t, err)

	assert.True(t, res.HeldBackByDepFloor)
	assert.False(t, res.HeldBackByExtraGate)
	assert.Zero(t, sighting.calls, "a dependency-floor rejection ends the verdict; the caller gate is skipped")
}

func TestCanonicalizeGoMod_ToolchainOnlyStripRunsExtraGate(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26\n\ntoolchain go1.26.7\n"
	path := writeGoModFixture(t, original)
	sighting := extraGateSighting{livePath: path}

	gate := fakeGate(func(string, []string) (string, string, error) {
		return "", "", errGateMustNotRun
	})

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		StripToolchain:     true,
		InstalledToolchain: "1.26.7",
		ExtraGate:          sighting.approve,
	}, gate)
	require.NoError(t, err)

	assert.True(t, res.Changed)
	assert.Equal(
		t,
		1,
		sighting.calls,
		"a stripped toolchain line can change which toolchain builds, so the caller gate still runs",
	)
	assert.Equal(t, "module example.com/norm\n\ngo 1.26\n", sighting.candidate)
	assert.Equal(t, sighting.candidate, readFile(t, path))
}

func TestCanonicalizeGoMod_DryRunRunsNoGate(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26.7\n\ntoolchain go1.26.7\n"
	path := writeGoModFixture(t, original)
	sighting := extraGateSighting{livePath: path}
	gateCalls := 0

	gate := fakeGate(func(string, []string) (string, string, error) {
		gateCalls++

		return "", "", nil
	})

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		DryRun:             true,
		StripToolchain:     true,
		InstalledToolchain: "1.26.7",
		ExtraGate:          sighting.approve,
	}, gate)
	require.NoError(t, err)

	assert.True(t, res.HeldBack)
	assert.Zero(t, gateCalls, "a dry run stages nothing, so no gate can run")
	assert.Zero(t, sighting.calls)
	assert.Equal(t, original, readFile(t, path))
}

// TestCanonicalizeGoMod_LiveFileChangedDuringGatesIsNotClobbered pins the
// verified commit: the verdict was earned for the content that was read, so
// a go.mod rewritten by someone else while the gates ran is left alone.
func TestCanonicalizeGoMod_LiveFileChangedDuringGatesIsNotClobbered(t *testing.T) {
	t.Parallel()

	original := "module example.com/norm\n\ngo 1.26.7\n"
	path := writeGoModFixture(t, original)
	otherWriter := "module example.com/norm\n\ngo 1.26.7\n\n// grouped by a concurrent go mod tidy\n"

	res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
		InstalledToolchain: "1.26.7",
		ExtraGate: func(context.Context, string, string) error {
			require.NoError(t, os.WriteFile(path, []byte(otherWriter), 0o644))

			return nil
		},
	}, noopGate())
	require.NoError(t, err)

	assert.True(t, res.Skipped)
	assert.Contains(t, res.SkipReason, "another writer")
	assert.False(t, res.Changed)
	assert.Equal(t, otherWriter, readFile(t, path), "the other writer's file stands")
}

// TestCanonicalizeGoMod_StagedCopyNeverOutlivesTheCall covers every verdict
// path: whatever the gates decide, no staged copy is left behind.
func TestCanonicalizeGoMod_StagedCopyNeverOutlivesTheCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		dryRun          bool
		gateDiff        string
		gateErr         error
		wantChanged     bool
		wantHeldByFloor bool
	}{
		{name: "accepted", wantChanged: true},
		{name: "dependency floor rejects", gateDiff: "diff", wantHeldByFloor: true},
		{name: "dependency gate fails", gateErr: errNoMainModule, wantHeldByFloor: true},
		{name: "dry run runs no gate", dryRun: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeGoModFixture(t, "module example.com/norm\n\ngo 1.26.7\n")
			seen := gateObservation{}

			gate := observingGate(path, &seen, func(string, []string) (string, string, error) {
				return tt.gateDiff, "", tt.gateErr
			})

			res, err := CanonicalizeGoMod(context.Background(), path, CanonicalizeOptions{
				DryRun:             tt.dryRun,
				InstalledToolchain: "1.26.7",
			}, gate)
			require.NoError(t, err)

			assert.Equal(t, tt.wantChanged, res.Changed)
			assert.Equal(t, tt.wantHeldByFloor, res.HeldBackByDepFloor)
			assert.Equal(
				t,
				[]string{"go.mod"},
				dirEntryNames(t, filepath.Dir(path)),
				"the module directory never holds a staged copy",
			)

			if tt.dryRun {
				assert.Empty(t, seen.modFile)

				return
			}

			require.NotEmpty(t, seen.modFile)
			assertStagedCopyGone(t, seen.modFile)
		})
	}
}
