package xpect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The written files and the headline are announced on the writer the caller
// supplies, so an embedding program captures the whole run.
func TestWriteReportsAnnouncesOnTheSuppliedWriter(t *testing.T) {
	report := &Report{}
	report.summarize()

	var log strings.Builder
	if _, err := writeReports(filepath.Join(t.TempDir(), "out"), report, &log); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"wrote ", "pilot-xpect.json", "0 .xt file(s), 0 unparsed"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
}

// A suite that is not downloaded is reported as skipped on the same writer.
func TestRunReportsAbsentSuitesOnTheSuppliedWriter(t *testing.T) {
	repo := t.TempDir()
	pin := filepath.Join(repo, "scripts", "pilot-pin.sh")
	if err := os.MkdirAll(filepath.Dir(pin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pin, []byte("PILOT_TAG=\"${PILOT_TAG:-2026-05}\"\nPILOT_COMMIT=\"${PILOT_COMMIT:-fa709f28dfd49dfdb7ee83e4e19da2f57e0eb3aa}\"\nPILOT_ARTIFACT_VERSION=\"${PILOT_ARTIFACT_VERSION:-0.60.1}\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var log strings.Builder
	err := run(repo, filepath.Join(repo, "out"), 1, false, false, &log)
	if err == nil || !strings.Contains(err.Error(), "no suite found") {
		t.Fatalf("run() error = %v", err)
	}
	for _, want := range []string{"skipping kerml: ", "skipping sysml: "} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
}
