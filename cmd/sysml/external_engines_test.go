package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/testutil/gobuild"
)

var (
	standinOnce sync.Once
	standinPath string
	standinErr  error
)

// engineStandin builds the stand-in engine of the analysis package's tests once per test binary.
func engineStandin(t *testing.T) string {
	t.Helper()
	standinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "enginestandin")
		if err != nil {
			standinErr = err
			return
		}
		standinPath = filepath.Join(dir, "enginestandin")
		build := exec.Command("go", gobuild.Args(standinPath)...)
		build.Dir = filepath.Join("..", "..", "internal", "core", "analysis", "testdata", "enginestandin")
		if out, err := build.CombinedOutput(); err != nil {
			standinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if standinErr != nil {
		t.Fatalf("building the stand-in engine: %v", standinErr)
	}
	return standinPath
}

// recordingManifest writes a manifest of two engine entries whose commands are scripts that
// append their name to record before speaking the protocol as the stand-in engine.
func recordingManifest(t *testing.T) (dir, record string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the recording command is a shell script")
	}
	dir = t.TempDir()
	record = filepath.Join(dir, "record")
	for _, name := range []string{"alpha", "beta"} {
		script := "#!/bin/sh\necho " + name + " >> " + record + "\n" +
			`ENGINE_STANDIN_DESCRIBE='{"name":"` + name + `","version":"1.0.0","protocol":1,"answers":["holds"]}' ` +
			"exec " + engineStandin(t) + "\n"
		if err := os.WriteFile(filepath.Join(dir, name+".sh"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		entry := `{"kind":"engine","name":"` + name + `","version":"1.0.0","command":["` + name + `.sh"],` +
			`"protocol":1,"answers":["holds"],"model":["sources"],"witness":"schedule","authority":"bounded"}`
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(analysis.ToolsEnv, "")
	t.Setenv(analysis.EnginesEnv, dir)
	return dir, record
}

// spawned reads the names the recording scripts appended, in order.
func spawned(t *testing.T, record string) []string {
	t.Helper()
	data, err := os.ReadFile(record)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

// TestEnginesSpawnsNothingAndProbeSpawnsEachEntryOnce checks that -engines lists a manifest
// engine with its kind, protocol, authority and resolved command without starting it, and that
// -engines -probe starts each entry exactly once and reports the handshake.
func TestEnginesSpawnsNothingAndProbeSpawnsEachEntryOnce(t *testing.T) {
	binary := buildCLI(t)
	dir, record := recordingManifest(t)

	out := run(t, binary, "-engines")
	if got := spawned(t, record); len(got) != 0 {
		t.Fatalf("-engines spawned %v", got)
	}
	for _, want := range []string{
		"alpha    engine    stdio/1   bounded    holds", "beta     engine    stdio/1   bounded    holds",
		"ready (alpha 1.0.0 at " + filepath.Join(dir, "alpha.sh") + ")",
		"alpha 1.0.0: engine from " + filepath.Join(dir, "alpha.json") + ", runs " + filepath.Join(dir, "alpha.sh") + ", not admitted",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("-engines is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "describe agrees") {
		t.Errorf("-engines reports a handshake it did not make:\n%s", out)
	}

	out = run(t, binary, "-engines", "-probe")
	got := spawned(t, record)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("-engines -probe spawned %v, want each entry once", got)
	}
	for _, want := range []string{"ready (alpha 1.0.0 at " + filepath.Join(dir, "alpha.sh") + "; describe agrees)", "ready (beta 1.0.0 at"} {
		if !strings.Contains(out, want) {
			t.Errorf("-engines -probe is missing %q:\n%s", want, out)
		}
	}
}

// TestProbeGoesWithEngines checks that -probe alone is a usage error naming what it does.
func TestProbeGoesWithEngines(t *testing.T) {
	binary := buildCLI(t)
	got := check(t, binary, engineModel, "-probe")
	wantReport(t, got, 2, "-probe goes with -engines")
}

// TestEnginesUnderTheWorkspaceIsNotRead checks that a manifest directory under the working
// directory the models are read from is refused at startup, before any flag is acted on.
func TestEnginesUnderTheWorkspaceIsNotRead(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	manifest := filepath.Join(dir, "engines")
	if err := os.Mkdir(manifest, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(analysis.ToolsEnv, "")
	t.Setenv(analysis.EnginesEnv, manifest)

	cmd := exec.Command(binary, "-engines")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "is under the workspace") || !strings.Contains(string(out), analysis.EnginesEnv) {
		t.Fatalf("-engines under the workspace: %v\n%s", err, out)
	}
}

// raceModel is an action with a schedule choice, so a question about it under -schedule
// explore is one an external engine answering outcomes covers.
const raceModel = `package Mission {
    private import ScalarValues::*;
    action race {
        attribute x : Integer = 0;
        first start;
        fork split;
        action left { assign x := 1; }
        action right { assign x := 2; }
        join sync;
        done;
        succession first start then split;
        succession first split then left;
        succession first split then right;
        succession first left then sync;
        succession first right then sync;
        succession first sync then done;
    }
}
`

// TestExternalWitnessedViolationFailsTheCheck checks that a violation an external engine
// reports, replayed by the host to the move it names, fails the check as the check engine's
// own would, and that the two agree under -engine all.
func TestExternalWitnessedViolationFailsTheCheck(t *testing.T) {
	binary := buildCLI(t)
	dir, _ := recordingManifest(t)
	entry := `{"kind":"engine","name":"alpha","version":"1.0.0","command":["alpha.sh"],"protocol":1,` +
		`"answers":["holds"],"subjects":["action"],"model":["sources"],"witness":"schedule","authority":"bounded"}`
	if err := os.WriteFile(filepath.Join(dir, "alpha.json"), []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		`ENGINE_STANDIN_DESCRIBE='{"name":"alpha","version":"1.0.0","protocol":1,"answers":["holds"],"subjects":["action"]}' ` +
		"exec " + engineStandin(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "alpha.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENGINE_STANDIN_RESULT", `{"claim":"violated","strength":"witnessed",`+
		`"witness":{"schedules":["step 3: 2@a first of 2@a, 3@b"]}}`)

	got := check(t, binary, tankModel, "-engine", "all", "-instantiate", "Plant::tank",
		"-action", "Plant::Tank::fill Plant::tank", "-check-property", "Plant::Tank::low")
	wantReport(t, got, 1, `✗ Action Plant::Tank::fill: at its end (engine "alpha", replayed): `+
		"`Plant::Tank::low` evaluates false there",
		"witness: step 3: 2@a first of 2@a, 3@b",
		"standing: violated (witnessed: witness of 1 choice replayed); all: alpha violated (witnessed), beta violated (witnessed), check violated (witnessed)")
	rejectReport(t, got, "could not be checked")
}

// TestProgressGoesToStandardError checks that what an external engine reports while it runs is
// TestModelSeedReachesTheExternalEngine checks that the seed -seed names goes to an external
// engine on the checker's question as `modelSeed`, 0 as much as any other seed, apart from
// `schedule`, and that no `modelSeed` is written when -seed is not given.
func TestModelSeedReachesTheExternalEngine(t *testing.T) {
	binary := buildCLI(t)
	dir, _ := recordingManifest(t)
	entry := `{"kind":"engine","name":"alpha","version":"1.0.0","command":["alpha.sh"],"protocol":1,` +
		`"answers":["holds"],"subjects":["action"],"model":["sources"],"witness":"schedule","authority":"bounded"}`
	if err := os.WriteFile(filepath.Join(dir, "alpha.json"), []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		`ENGINE_STANDIN_DESCRIBE='{"name":"alpha","version":"1.0.0","protocol":1,"answers":["holds"],"subjects":["action"]}' ` +
		"exec " + engineStandin(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "alpha.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENGINE_STANDIN_RESULT", `{"claim":"holds","strength":"bounded"}`)
	cases := []struct {
		name  string
		seed  []string
		want  string
		unset bool
	}{
		{name: "seed 0", seed: []string{"-seed", "0"}, want: `"schedule":"explore","modelSeed":0,"free"`},
		{name: "seed 11", seed: []string{"-seed", "11"}, want: `"schedule":"explore","modelSeed":11,"free"`},
		{name: "no seed", want: `"schedule":"explore","free"`, unset: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wire := t.TempDir()
			t.Setenv("ENGINE_STANDIN_WIRE", wire)
			args := append([]string{"-engine", "all", "-instantiate", "Plant::tank",
				"-action", "Plant::Tank::fill Plant::tank", "-check-property", "Plant::Tank::low"}, c.seed...)
			got := check(t, binary, tankModel, args...)
			wantReport(t, got, 1, `alpha not covered (engine "alpha" reports holds`)
			lines := hostLines(t, wire)
			question := ""
			for _, line := range lines {
				if strings.Contains(line, `"method":"run"`) {
					question = line
				}
			}
			if question == "" {
				t.Fatalf("no run went to alpha; the host wrote:\n%s", strings.Join(lines, "\n"))
			}
			if !strings.Contains(question, c.want) {
				t.Errorf("the question does not carry %s:\n%s", c.want, question)
			}
			if c.unset && strings.Contains(question, "modelSeed") {
				t.Errorf("the question names a model seed without -seed:\n%s", question)
			}
		})
	}
}

// hostLines is every line the host wrote to the stand-ins that recorded under dir.
func hostLines(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.wire"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if rest, ok := strings.CutPrefix(line, "< "); ok {
				lines = append(lines, rest)
			}
		}
	}
	return lines
}

// TestProgressGoesToStandardError checks that what an external engine reports while it runs is
// printed to standard error, one line naming the engine per coalesced report, and that -quiet
// prints none; the verdict itself is on standard output either way.
func TestProgressGoesToStandardError(t *testing.T) {
	binary := buildCLI(t)
	dir, _ := recordingManifest(t)
	entry := `{"kind":"engine","name":"alpha","version":"1.0.0","command":["alpha.sh"],"protocol":1,` +
		`"answers":["holds","outcomes"],"model":["sources"],"witness":"schedule","authority":"bounded"}`
	if err := os.WriteFile(filepath.Join(dir, "alpha.json"), []byte(entry), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n" +
		`ENGINE_STANDIN_DESCRIBE='{"name":"alpha","version":"1.0.0","protocol":1,"answers":["holds","outcomes"]}' ` +
		"exec " + engineStandin(t) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "alpha.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENGINE_STANDIN_PROGRESS", "5")

	got := check(t, binary, raceModel, "-action", "Mission::race", "-schedule", "explore", "-engine", "alpha")
	wantReport(t, got, 2, "? Action Mission::race could not be checked", `engine "alpha" reports no claim`)
	if !strings.Contains(got.stderr, "engine alpha: runs 5, depth 5\n") || strings.Contains(got.stdout, "engine alpha: runs") {
		t.Errorf("progress is not on standard error alone:\nstdout:\n%s\nstderr:\n%s", got.stdout, got.stderr)
	}

	got = check(t, binary, raceModel, "-quiet", "-action", "Mission::race", "-schedule", "explore", "-engine", "alpha")
	wantReport(t, got, 2, `engine "alpha" reports no claim`)
	if strings.Contains(got.output(), "engine alpha: runs") {
		t.Errorf("-quiet printed progress:\n%s", got.output())
	}
}
