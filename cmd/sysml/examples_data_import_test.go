package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDataImportDemo imports the demo's four result files into its model as
// the README does and checks the values the model then computes from.
func TestDataImportDemo(t *testing.T) {
	binary := buildCLI(t)
	demo := filepath.Join("..", "..", "examples", "data-import-demo")

	if _, err := exec.LookPath("python3"); err == nil {
		fresh := t.TempDir()
		gen := exec.Command("python3", filepath.Join(demo, "tools", "simulate.py"), fresh)
		if out, err := gen.CombinedOutput(); err != nil {
			t.Fatalf("simulate.py: %v\n%s", err, out)
		}
		for _, name := range []string{"mass-properties.csv", "battery-test.json", "motor-telemetry.jsonl", "overrides.tsv"} {
			want, err := os.ReadFile(filepath.Join(demo, "results", name))
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(fresh, name))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("simulate.py writes a different %s than results/ holds:\n%s", name, got)
			}
		}
	}

	out := filepath.Join(t.TempDir(), "rover-imported.sysml")
	imp := exec.Command(binary, "rover.sysml",
		"-import", "results/mass-properties.csv",
		"-import", "results/battery-test.json", "-import-map", "battery-map.json",
		"-import", "results/motor-telemetry.jsonl",
		"-import", "results/overrides.tsv",
		"-convert", "sysml", "-o", out)
	imp.Dir = demo
	output, err := imp.CombinedOutput()
	if err != nil {
		t.Fatalf("import: %v\n%s", err, output)
	}
	for _, want := range []string{
		"✓ imported 5 values into 2 elements from results/mass-properties.csv",
		"✓ imported 4 values into 2 elements from results/battery-test.json",
		"✓ imported 2 values into 2 elements from results/motor-telemetry.jsonl",
		"✓ imported 1 value into 1 element from results/overrides.tsv",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("import output is missing %q:\n%s", want, output)
		}
	}

	eval := exec.Command(binary, out, "-validate",
		"-eval", "RoverDemo::scout.specificEnergy",
		"-eval", "RoverDemo::hauler.health",
		"-eval", "RoverDemo::scout2.driveMotor.peakPower")
	output, err = eval.CombinedOutput()
	if err != nil {
		t.Fatalf("eval: %v\n%s", err, output)
	}
	for _, want := range []string{"no errors", "= 22500.0", "= Health::degraded", "= 1450.5 [W]"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("eval output is missing %q:\n%s", want, output)
		}
	}
}
