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
