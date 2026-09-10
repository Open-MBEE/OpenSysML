package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pilotDynamicsModel is the pilot corpus file the dynamics analysis is declared
// in, and pilotRequireEnv turns its absence into a failure, as CI does.
const (
	pilotDynamicsModel = "../../examples/pilot-corpora/sysml-validation/10-Analysis and Trades/10d-Dynamics Analysis.sysml"
	pilotRequireEnv    = "OPENSYSML_REQUIRE_PILOT_CORPORA"
)

// pilotDynamicsSubject binds a vehicle for the analysis to run against, which
// the corpus file states no object of.
const pilotDynamicsSubject = `package Subject10d {
    private import '10d-Dynamics Analysis'::VehicleModel::*;
    part car : Vehicle { attribute :>> mass = 1000 [SI::kg]; }
}
`

// pilotDynamicsCase is the analysis run, with every argument but the swept one
// bound as a command line binds them; powerProfile is `ISQ::power[*]`, so distinct.
const pilotDynamicsCase = `'10d-Dynamics Analysis'::AnalysisModel::DynamicsAnalysis(` +
	`powerProfile=(1000.0 [SI::W], 1300.0 [SI::W], 1700.0 [SI::W]), ` +
	`initialPosition=0.0 [SI::m], %s) Subject10d::car`

// runPilotDynamics sweeps or samples the pilot's dynamics analysis over the
// corpus file and a subject of its own.
func runPilotDynamics(t *testing.T, binary string, args ...string) runOutcome {
	t.Helper()
	if _, err := os.Stat(pilotDynamicsModel); err != nil {
		if os.Getenv(pilotRequireEnv) != "" {
			t.Fatalf("%s is set but the pilot corpus is absent: %v", pilotRequireEnv, err)
		}
		t.Skip("pilot corpora absent; run ./scripts/download-pilot-corpora.sh")
	}
	subject := filepath.Join(t.TempDir(), "subject.sysml")
	if err := os.WriteFile(subject, []byte(pilotDynamicsSubject), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, append(args, pilotDynamicsModel, subject)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	got := runOutcome{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		got.status = exit.ExitCode()
	default:
		t.Fatalf("%v: %v\n%s", args, err, got.output())
	}
	return got
}

// TestSweepPilotDynamicsAnalysis sweeps the pilot's dynamics analysis over its
// initial speed: each speed is one ordinary run, and the speed the model's
// acceleration calc divides by zero at is that row's typed error rather than an
// abort of the table.
func TestSweepPilotDynamicsAnalysis(t *testing.T) {
	binary := buildCLI(t)

	got := runPilotDynamics(t, binary, "-instantiate", "Subject10d::car",
		"-analysis", strings.Replace(pilotDynamicsCase, "%s", "deltaT=1.0 [SI::s]", 1),
		"-sweep", "initialSpeed=0.0 [SI::'m/s']..2.0 [SI::'m/s']:1.0 [SI::'m/s']")
	if got.status != 1 {
		t.Errorf("exit status = %d, want 1 for the table with a failed run\n%s", got.status, got.output())
	}
	table := sweepTable(got.output())
	for _, want := range []string{
		"sweep 10d-Dynamics Analysis::AnalysisModel::DynamicsAnalysis — 3 run(s)",
		"initialSpeed",
		"accelerationProfile",
		"0.0 [SI::'m/s']",
		"division by zero",
		"[1.0 [SI::W/(SI::'m/s'*SI::kg)], 0.65 [SI::W/(SI::'m/s'*SI::kg)]]",
		"[0.5 [SI::W/(SI::'m/s'*SI::kg)], 0.52 [SI::W/(SI::'m/s'*SI::kg)]]",
	} {
		if !strings.Contains(table, want) {
			t.Errorf("table is\n%s\nwant it to carry %q", table, want)
		}
	}
}

// TestSamplesPilotDynamicsAnalysis draws the pilot's dynamics analysis over its
// time step: the drawn quantities carry the unit the range's start does, and the
// same seed draws the same table.
func TestSamplesPilotDynamicsAnalysis(t *testing.T) {
	binary := buildCLI(t)

	draw := func(seed string) string {
		got := runPilotDynamics(t, binary, "-instantiate", "Subject10d::car",
			"-analysis", strings.Replace(pilotDynamicsCase, "%s", "initialSpeed=1.0 [SI::'m/s']", 1),
			"-sweep", "deltaT=0.5 [SI::s]..2.0 [SI::s]", "-samples", "3", "-seed", seed)
		if got.status != 0 {
			t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
		}
		return sweepTable(got.output())
	}
	first, again := draw("42"), draw("42")
	if first != again {
		t.Errorf("seed 42 drew\n%s\nthen\n%s", first, again)
	}
	for _, want := range []string{
		"samples 10d-Dynamics Analysis::AnalysisModel::DynamicsAnalysis — 3 run(s), seed 42",
		"1.7382087604970673 [SI::s]",
		"0.5642299270421454 [SI::s]",
		"1.664109574567931 [SI::s]",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("table is\n%s\nwant it to carry %q", first, want)
		}
	}
	if draw("43") == first {
		t.Errorf("seed 43 drew what seed 42 did:\n%s", first)
	}
}
