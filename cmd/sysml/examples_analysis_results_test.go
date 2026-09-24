package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAnalysisResultsExample runs the analysis-results demo end to end: the
// rendered report matches its committed output, the stale-records query
// returns exactly the record the model moved away from, and the analysis
// still prints the fuelLeft the scout record saved.
func TestAnalysisResultsExample(t *testing.T) {
	binary := buildCLI(t)
	examples := filepath.Join("..", "..", "examples", "analysis-results-demo")
	source := filepath.Join(examples, "lander-results.sysml")

	committed, err := os.ReadFile(filepath.Join(examples, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "report.md")
	render := exec.Command(binary, source, "-render-document", "Reporting::AnalysisReport", "-o", out)
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(committed) {
		t.Errorf("rendered example differs from examples/analysis-results-demo/report.md:\n%s", written)
	}

	stale := exec.Command(binary, source, "-run-query", "Reporting::StaleRuns root=results")
	output, err := stale.CombinedOutput()
	if err != nil {
		t.Fatalf("stale query: %v\n%s", err, output)
	}
	for _, want := range []string{
		"returned 1 row",
		"Row 1: Results::results::relayRun",
		"drift = 30.0",
		"liveFuelLeft = 110.0",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("stale query output is missing %q:\n%s", want, output)
		}
	}

	// The recorded baseline must match what the case still prints; if the
	// model moves so the printed value differs from scoutRun's fuelLeft, this
	// catches the record silently drifting.
	run := exec.Command(binary, source, "-analysis", "Descent::scoutBudget")
	output, err = run.CombinedOutput()
	if err != nil {
		t.Fatalf("analysis: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "fuelLeft = 130.0") {
		t.Errorf("scoutBudget no longer prints the recorded fuelLeft = 130.0:\n%s", output)
	}
}
