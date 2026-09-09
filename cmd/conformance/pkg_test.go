package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	"github.com/Open-MBEE/OpenSysML/internal/testutil/gobuild"
)

// TestSuitePassesThroughThePublicGoAPI runs the committed suite in process over
// the pkg protocol, the way `make conformance-pkg` does, so a scenario the
// public API answers differently from the wire fails here without a service.
func TestSuitePassesThroughThePublicGoAPI(t *testing.T) {
	api, err := opensysml.New()
	if err != nil {
		t.Fatalf("opensysml.New: %v", err)
	}
	c := newPkgClient("pkg", api)
	defer c.close()

	scenarios := loadSuite(t)
	var log strings.Builder
	r := &runner{
		service:     &service{binary: "client/opensysml"},
		client:      c,
		fixtures:    filepath.Join(suiteDir, "fixtures"),
		models:      map[Model]string{},
		out:         &log,
		scenarioLog: &log,
	}
	ctx := context.Background()
	if err := r.readCapabilities(ctx); err != nil {
		t.Fatalf("readCapabilities: %v", err)
	}
	if len(r.capabilities) == 0 {
		t.Fatal("the in-process service reports no capabilities")
	}

	summary := r.runAll(ctx, scenarios, nil)
	if summary.Total != len(scenarios) {
		t.Errorf("ran %d scenarios, want %d", summary.Total, len(scenarios))
	}
	if summary.Failed > 0 || summary.Errored > 0 {
		t.Fatalf("%d failed, %d in error:\n%s", summary.Failed, summary.Errored, log.String())
	}
	// Skips are the calls the v1 public API cannot express; every one must say
	// so rather than skip silently.
	for _, result := range summary.Results {
		if result.Outcome == "skip" && result.Reason == "" {
			t.Errorf("%s skipped without a reason", result.ID)
		}
	}
	if summary.Passed == 0 {
		t.Fatal("no scenario passed")
	}
}

// TestSuitePassesOverTheWire runs the suite the way `make conformance` does,
// against a service built from the working tree, so the run's plumbing (the
// service start, the wire clients, the reports) is exercised under go test.
func TestSuitePassesOverTheWire(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and starts sysml-grpc")
	}
	binary := filepath.Join(t.TempDir(), "sysml-grpc")
	build := exec.Command("go", gobuild.Args(binary)...)
	build.Dir = filepath.Join("..", "sysml-grpc")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building sysml-grpc: %v\n%s", err, out)
	}

	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	junitPath := filepath.Join(dir, "report.xml")
	err := runSuite(options{
		dir:       suiteDir,
		binary:    binary,
		report:    report,
		junit:     junitPath,
		allowSkip: true,
		protocols: "grpc,connect,connect-json,pkg-connect",
		transport: transportConnect,
		withhold:  "strict_conformance,oslc_query",
	})
	if err != nil {
		t.Fatalf("runSuite: %v", err)
	}

	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var written Report
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("the report is not JSON: %v", err)
	}
	if written.Failed != 0 || written.Errored != 0 || written.Passed == 0 {
		t.Errorf("report: %d passed, %d failed, %d errored", written.Passed, written.Failed, written.Errored)
	}
	if len(written.Configurations) != 2 {
		t.Errorf("report has %d configurations, want the default and the withheld one", len(written.Configurations))
	}
	if _, err := os.Stat(junitPath); err != nil {
		t.Errorf("the JUnit report was not written: %v", err)
	}
}
