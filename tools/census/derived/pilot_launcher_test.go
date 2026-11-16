package derived

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/exporter"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// stubBridge writes a shell launcher that writes a bridge document for each of
// written under --out, prints stderr and exits with status.
func stubBridge(t *testing.T, stderr string, status int, written ...string) string {
	t.Helper()
	var script strings.Builder
	script.WriteString("#!/bin/sh\nwhile [ $# -gt 0 ]; do\n\tcase \"$1\" in\n\t--out) out=\"$2\"; shift ;;\n\tesac\n\tshift\ndone\n")
	for _, rel := range written {
		path := "\"$out/" + rel + ".json\""
		script.WriteString("mkdir -p \"$(dirname " + path + ")\"\n")
		script.WriteString("printf '%s' '{\"elements\":[]}' > " + path + "\n")
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

func exportStub(t *testing.T, launcher string) (*PilotCensus, error) {
	t.Helper()
	return runExporter(launcher, t.TempDir(), t.TempDir(), stubFiles, time.Minute, newMetaclasses())
}

func TestRunExporterRefusesTheLauncherSetupStatus(t *testing.T) {
	_, err := exportStub(t, stubBridge(t, "Error: pilot jar not found at /gone.jar", exporter.SetupStatus))
	requireNotRun(t, err, "pilot jar not found at /gone.jar")
}

func TestRunExporterRefusesExitOneWithNothingWritten(t *testing.T) {
	_, err := exportStub(t, stubBridge(t, "", 1))
	requireNotRun(t, err, "no document written")
}

func TestRunExporterRefusesExitOneWithALauncherError(t *testing.T) {
	_, err := exportStub(t, stubBridge(t, "Error: Could not find or load main class ExportJSON", 1, "a.sysml"))
	requireNotRun(t, err, "Could not find or load main class")
}

// The bridge exits 1 when it failed on some files: those, and only those, are the pilot's failures.
func TestRunExporterRecordsAPartialFailurePerFile(t *testing.T) {
	census, err := exportStub(t, stubBridge(t, "sub/b.sysml: unresolved reference to Nowhere", 1, "a.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(census.Failed, []string{"sub/b.sysml"}) {
		t.Fatalf("failed = %v, want only sub/b.sysml", census.Failed)
	}
}

// stubRepo is a checkout holding one root, testdata, whose files a run exports.
func stubRepo(t *testing.T) options {
	t.Helper()
	real, err := repo.Choose("")
	if err != nil {
		t.Fatal(err)
	}
	pin, err := os.ReadFile(filepath.Join(real, "scripts", "pilot-pin.sh"))
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
		if err := writeFile(path, content); err != nil {
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

func committedPath(opts options) string {
	return filepath.Join(opts.repo, filepath.FromSlash(CommittedBaseline))
}

// A bridge that exits 0 and writes nothing fails every file: -update records no such baseline.
func TestUpdateRefusesARootThePilotFailedWhole(t *testing.T) {
	opts := stubRepo(t)
	opts.launcher, opts.update = stubBridge(t, "", 0), true
	requireRefusedRoot(t, run(opts))
	if _, err := os.Stat(committedPath(opts)); !os.IsNotExist(err) {
		t.Fatalf("want no baseline recorded, got %v", err)
	}
}

// A committed all-pilotFailed baseline that the same broken exporter reproduces does not pass -check.
func TestCheckRefusesARootThePilotFailedWhole(t *testing.T) {
	opts := stubRepo(t)
	opts.launcher = stubBridge(t, "", 0)
	if err := run(opts); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(opts.out, toolName+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal(content, &report); err != nil {
		t.Fatal(err)
	}
	head, err := committedHeader(&report)
	if err != nil {
		t.Fatal(err)
	}
	committed := committedPath(opts)
	if err := baseline.Write(committed, head, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(recordsPath(committed), string(encodeRecords(&report))); err != nil {
		t.Fatal(err)
	}
	opts.check = true
	requireRefusedRoot(t, run(opts))
}
