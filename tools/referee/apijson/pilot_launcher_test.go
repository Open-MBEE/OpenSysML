package apijson

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/exporter"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
	"github.com/Open-MBEE/OpenSysML/tools/referee/baseline"
)

// stubBridge writes a shell launcher that writes a bridge document for each of
// written under --out, prints stderr and exits with status.
func stubBridge(t *testing.T, stderr string, status int, written ...string) string {
	t.Helper()
	var script strings.Builder
	script.WriteString("#!/bin/sh\nif [ \"$1\" = --jvm ]; then echo 21; exit 0; fi\nwhile [ $# -gt 0 ]; do\n\tcase \"$1\" in\n\t--out) out=\"$2\"; shift ;;\n\tesac\n\tshift\ndone\n")
	for _, rel := range written {
		path := "\"$out/" + rel + ".json\""
		script.WriteString("mkdir -p \"$(dirname " + path + ")\"\n")
		script.WriteString("printf '%s' '{\"file\":\"" + rel + "\",\"elements\":[],\"keys\":{}}' > " + path + "\n")
	}
	if stderr != "" {
		script.WriteString("printf '%s\\n' '" + stderr + "' >&2\n")
	}
	script.WriteString("exit " + strconv.Itoa(status) + "\n")
	launcher := filepath.Join(t.TempDir(), "export-json")
	if err := os.WriteFile(launcher, []byte(script.String()), 0o700); err != nil { // #nosec G306 -- an executable test stub
		t.Fatal(err)
	}
	return launcher
}

func requireNotRun(t *testing.T, err error, cause string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "did not run") || !strings.Contains(err.Error(), cause) || !strings.Contains(err.Error(), exporter.ProvisionCommand) {
		t.Fatalf("want a did-not-run error naming %q and %s, got %v", cause, exporter.ProvisionCommand, err)
	}
}

var stubFiles = []string{"a.sysml", "sub/b.sysml"}

func TestRunPilotRefusesTheLauncherSetupStatus(t *testing.T) {
	launcher := stubBridge(t, "Error: pilot jar not found at /gone.jar", exporter.SetupStatus)
	_, err := runPilot(launcher, t.TempDir(), nil, stubFiles, time.Minute)
	requireNotRun(t, err, "pilot jar not found at /gone.jar")
}

func TestRunPilotRefusesExitOneWithNothingWritten(t *testing.T) {
	launcher := stubBridge(t, "log4j: no appenders", 1)
	_, err := runPilot(launcher, t.TempDir(), nil, stubFiles, time.Minute)
	requireNotRun(t, err, "no document written")
}

func TestRunPilotRefusesExitOneWithALauncherError(t *testing.T) {
	launcher := stubBridge(t, "Error: Could not find or load main class io.opensysml.pilot.ExportJSON", 1, "a.sysml")
	_, err := runPilot(launcher, t.TempDir(), nil, stubFiles, time.Minute)
	requireNotRun(t, err, "Could not find or load main class")
}

// The bridge's own status 1 with some documents written stays a per-file failure.
func TestRunPilotRecordsAPartialFailurePerFile(t *testing.T) {
	launcher := stubBridge(t, "sub/b.sysml: read: unresolved reference to Nowhere", 1, "a.sysml")
	result, err := runPilot(launcher, t.TempDir(), nil, stubFiles, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 1 || result.Failed[0] != "sub/b.sysml" || !result.file("sub/b.sysml").PilotFailed || result.file("a.sysml").PilotFailed {
		t.Fatalf("want sub/b.sysml alone pilotFailed, got %q", result.Failed)
	}
}

// The launcher download-pilot-json-exporter.sh writes exits with the setup
// status, not the bridge's 1, when its jar is absent.
func TestTheLauncherExitsWithTheSetupStatusWithoutItsJar(t *testing.T) {
	root, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(root, "scripts", "download-pilot-json-exporter.sh"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := strings.Cut(string(script), "<<'EOF'\n")
	body, _, ok := strings.Cut(body, "\nEOF\n")
	if !ok {
		t.Fatal("download-pilot-json-exporter.sh writes no launcher heredoc")
	}
	launcher := filepath.Join(t.TempDir(), "pilot-json-exporter", "export-json")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte(strings.ReplaceAll(body, "__PILOT_ARTIFACT_VERSION__", "0.0.0")+"\n"), 0o700); err != nil { // #nosec G306 -- the launcher under test
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	// #nosec G204 -- the launcher under test, in a temporary directory.
	command := exec.Command(launcher, "--out", t.TempDir(), "a.sysml")
	command.Stderr = &stderr
	var exit *exec.ExitError
	if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != exporter.SetupStatus || !strings.HasPrefix(stderr.String(), "Error:") {
		t.Fatalf("want exit %d with an Error: line, got %v: %s", exporter.SetupStatus, err, stderr.String())
	}
	_, err = runPilot(launcher, t.TempDir(), nil, []string{"a.sysml"}, time.Minute)
	requireNotRun(t, err, "Error:")
}

// stubRepo is a checkout holding one root, testdata, whose files a run exports.
func stubRepo(t *testing.T) options {
	t.Helper()
	checkout, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	pin, err := os.ReadFile(filepath.Join(checkout, "scripts", "pilot-pin.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for path, content := range map[string]string{
		"scripts/pilot-pin.sh":       string(pin),
		bridgeSource:                 "class ExportJSON {}\n",
		"libs/L.sysml":               "library package L;\n",
		"tests/testdata/a.sysml":     "package A { part def P; }\n",
		"tests/testdata/sub/b.sysml": "package B { part def Q; }\n",
	} {
		path = filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return options{repo: dir, out: t.TempDir(), libraries: filepath.Join(dir, "libs"), timeout: time.Minute, log: io.Discard}
}

func requireRefusedRoot(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "failed on every file of testdata (2 files)") || !strings.Contains(err.Error(), exporter.ProvisionCommand) {
		t.Fatalf("want the all-pilotFailed testdata root refused, got %v", err)
	}
}

// A bridge that exits 0 and writes nothing fails every file: -update records no such baseline.
func TestUpdateRefusesARootThePilotFailedWhole(t *testing.T) {
	opts := stubRepo(t)
	opts.launcher, opts.update = stubBridge(t, "", 0), true
	requireRefusedRoot(t, run(opts))
	if _, err := os.Stat(committedIndex(opts.repo)); !os.IsNotExist(err) {
		t.Fatalf("want no index recorded, got %v", err)
	}
}

// A committed all-pilotFailed baseline that the same broken exporter reproduces does not pass -check.
func TestCheckRefusesARootThePilotFailedWhole(t *testing.T) {
	opts := stubRepo(t)
	opts.launcher = stubBridge(t, "", 0)
	record, err := provenance(opts.repo, opts.libraries)
	if err != nil {
		t.Fatal(err)
	}
	var fragments []Fragment
	for _, file := range stubFiles {
		fragments = append(fragments, Fragment{Root: "testdata", File: file, Summary: FileSummary{PilotFailed: true}, Rows: map[string]Counts{}})
	}
	if err := baseline.Write(committedDir(opts.repo), Committed{Provenance: record, Summary: fold(fragments), Fragments: fragments}, io.Discard); err != nil {
		t.Fatal(err)
	}
	opts.check = true
	requireRefusedRoot(t, run(opts))
}
