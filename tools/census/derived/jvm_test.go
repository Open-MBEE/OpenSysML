package derived

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
)

// fakeLauncher writes a launcher answering --jvm with the given output.
func fakeLauncher(t *testing.T, answer string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the launcher is a POSIX shell script")
	}
	path := filepath.Join(t.TempDir(), "export-json")
	if err := writeFile(path, "#!/bin/sh\n[ \"$1\" = --jvm ] || exit 3\nprintf '%s\\n' '"+answer+"'\n"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil { // #nosec G302 -- the test runs it
		t.Fatal(err)
	}
	return path
}

func TestUpdateRefusesAnUndeclaredJVM(t *testing.T) {
	var log bytes.Buffer
	err := underDeclaredJVM(options{launcher: fakeLauncher(t, "25"), update: true, log: &log}, "21")
	if err == nil {
		t.Fatal("-update recorded under a JVM other than the declared one")
	}
	for _, want := range []string{"Java 25", "declares Java 21", baseline.PinPath} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
	if err := underDeclaredJVM(options{launcher: fakeLauncher(t, "21"), update: true, log: &log}, "21"); err != nil {
		t.Errorf("-update refused the declared JVM: %v", err)
	}
	if log.Len() != 0 {
		t.Errorf("a refusal or the declared JVM logged: %s", log.String())
	}
}

func TestACompareUnderAnUndeclaredJVMSaysSo(t *testing.T) {
	var log bytes.Buffer
	if err := underDeclaredJVM(options{launcher: fakeLauncher(t, "25"), check: true, log: &log}, "21"); err != nil {
		t.Fatalf("a comparing run refused another JVM, whose records must reproduce: %v", err)
	}
	if !strings.Contains(log.String(), "Java 25, not the Java 21") {
		t.Errorf("log does not name the two JVMs: %s", log.String())
	}
}

func TestExporterJVMRefusesALauncherThatNamesNone(t *testing.T) {
	for _, answer := range []string{"", "Usage: ExportJSON --root DIR", "21.0.4"} {
		if jvm, err := exporterJVM(fakeLauncher(t, answer)); err == nil || !strings.Contains(err.Error(), "download-pilot-json-exporter.sh --force") {
			t.Errorf("answer %q: got %q, %v; want a refusal naming the re-provisioning command", answer, jvm, err)
		}
	}
	if _, err := exporterJVM(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("an absent launcher named a JVM")
	}
}

func TestCheckFailsOnARecordFromAnotherJVM(t *testing.T) {
	record := baseline.Record{PilotTag: "t", PilotCommit: "c", PilotArtifact: "a", Errata: "sha256:0", PilotJava: "21",
		Inputs: []baseline.Input{{Name: "training", Dir: "d", Origin: baseline.OriginPinned, Files: 1, Digest: "sha256:1"}}}
	for _, recorded := range []string{"25", ""} {
		other := record
		other.PilotJava = recorded
		head, err := committedHeader(&Report{Provenance: other})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "baseline.json")
		if err := writeFile(path, string(head)); err != nil {
			t.Fatal(err)
		}
		err = baseline.CheckCommitted(path, refreshCommand, record)
		if err == nil || !strings.Contains(err.Error(), "provenance.pilotJava") || !strings.Contains(err.Error(), "first on PATH") {
			t.Errorf("recorded under %q, declared 21: %v", recorded, err)
		}
	}
	head, err := committedHeader(&Report{Provenance: record})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := writeFile(path, string(head)); err != nil {
		t.Fatal(err)
	}
	if err := baseline.CheckCommitted(path, refreshCommand, record); err != nil {
		t.Errorf("a record from the declared JVM fails: %v", err)
	}
}

func TestUpdateRefusesToReuseAColumnFromAnotherJVM(t *testing.T) {
	for _, recorded := range []string{"25", ""} {
		err := reusable(&Report{Provenance: baseline.Record{PilotJava: recorded}}, "21")
		if err == nil || !strings.Contains(err.Error(), "declares Java 21") {
			t.Errorf("a column recorded under %q was re-recorded as Java 21's: %v", recorded, err)
		}
	}
	if err := reusable(&Report{Provenance: baseline.Record{PilotJava: "21"}}, "21"); err != nil {
		t.Errorf("the declared JVM's column is refused: %v", err)
	}
	if err := reusable(nil, "21"); err != nil {
		t.Errorf("an absent baseline, which lends no column, is refused: %v", err)
	}
}
