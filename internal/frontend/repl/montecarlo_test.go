package repl

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// monteCarloModel declares a Simulation::MonteCarlo case over a part whose
// behavior draws the value the case observes, and an ordinary analysis case.
const monteCarloModel = `package MC {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	part def Probe {
		attribute t : Real;
		action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
	}
	individual def probe :> Probe;
	analysis def Mc :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.settle;
		attribute :>> observed : Real = analysed.t;
		return Mean : Real = mean;
		out Deviation : Real[0..1] = deviation;
		out N : Natural = runs;
	}
	analysis def Plain { subject analysed : Probe; return k : Integer = 1; }
}`

func monteCarloSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(monteCarloModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%instantiate MC::probe"), "")
	return s
}

// statistic reads the number a "name = value" line of a concluded case reports.
func statistic(t *testing.T, out, name string) float64 {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), name+" = "); ok {
			v, err := strconv.ParseFloat(rest, 64)
			if err != nil {
				t.Fatalf("%s = %q is not a number", name, rest)
			}
			return v
		}
	}
	t.Fatalf("no %s in\n%s", name, out)
	return 0
}

// %runs of a Simulation::MonteCarlo case tables what each seeded fresh run observed and
// concludes the case once: Mean their arithmetic mean, Deviation their sample deviation.
func TestRunsConcludesAMonteCarloCaseOverItsSample(t *testing.T) {
	s := monteCarloSession(t)
	out := run(t, s, "%runs 4 7 MC::Mc MC::probe")
	wants(t, out, "runs MC::Mc — 4 run(s), seed 7", "run | observed", "✓ MC::Mc over 4 run(s)", "N = 4")
	var observed []float64
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 2 {
			continue
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64); err == nil {
			observed = append(observed, v)
		}
	}
	if len(observed) != 4 {
		t.Fatalf("the table holds %d observation(s), want 4:\n%s", len(observed), out)
	}
	var sum float64
	for _, v := range observed {
		sum += v
	}
	mean := sum / 4
	var squares float64
	for _, v := range observed {
		squares += (v - mean) * (v - mean)
	}
	if got := statistic(t, out, "Mean"); math.Abs(got-mean) > 1e-12 {
		t.Errorf("Mean = %v, want the mean %v of %v", got, mean, observed)
	}
	if got := statistic(t, out, "Deviation"); math.Abs(got-math.Sqrt(squares/3)) > 1e-12 {
		t.Errorf("Deviation = %v, want the sample deviation %v of %v", got, math.Sqrt(squares/3), observed)
	}
	for i := 1; i < len(observed); i++ {
		if observed[i] == observed[0] {
			t.Errorf("runs 1 and %d observed the same %v; each run draws from its own seed", i+1, observed[0])
		}
	}
	if again := run(t, s, "%runs 4 7 MC::Mc MC::probe"); statistic(t, again, "Mean") != mean {
		t.Errorf("the same seed concluded another Mean:\n%s", again)
	}
}

// One run has a mean and no deviation: the case's optional Deviation is empty and
// the case still concludes.
func TestRunsOfOneLeaveTheDeviationEmpty(t *testing.T) {
	s := monteCarloSession(t)
	out := run(t, s, "%runs 1 7 MC::Mc MC::probe")
	wants(t, out, "✓ MC::Mc over 1 run(s)", "N = 1", "Deviation = null")
	if strings.Contains(out, "never assigned") {
		t.Errorf("one run does not conclude:\n%s", out)
	}
}

// An analysis case that does not specialize Simulation::MonteCarlo is not run
// repeatedly, and the refusal names the library case it would have to be.
func TestRunsRefusesAnOrdinaryAnalysisCase(t *testing.T) {
	s := monteCarloSession(t)
	wants(t, run(t, s, "%runs 3 7 MC::Plain MC::probe"), "MC::Plain", "Simulation::MonteCarlo")
}
