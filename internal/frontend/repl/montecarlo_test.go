package repl

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// monteCarloModel declares Simulation::MonteCarlo cases over a part whose behaviors
// draw a Real, a duration, a string, or fail, and an ordinary analysis case.
const monteCarloModel = `package MC {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	private import RealFunctions::floor;
	private import ISQ::*;
	private import SI::*;
	part def Probe {
		attribute t : Real;
		attribute elapsed : DurationValue;
		attribute label : String;
		action settle { first start; then assign t := uniform(1.0, 5.0); then done; }
		action time { first start; then assign elapsed := uniform(1.0, 5.0) [s]; then done; }
		action name { first start; then assign label := "x"; then done; }
		action crash { first start; then assign t := 1.0 / 0.0; then done; }
		action flake { first start; then assign t := 1.0 / floor(uniform(0.0, 2.0)); then assign label := "x"; then done; }
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
	analysis def Timed :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.time;
		attribute :>> observed : DurationValue = analysed.elapsed;
		return Mean : DurationValue = mean;
		out Deviation : DurationValue[0..1] = deviation;
	}
	analysis def Named :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.name;
		attribute :>> observed : String = analysed.label;
		return Mean = mean;
	}
	analysis def Crashing :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.crash;
		attribute :>> observed : Real = analysed.t;
		return Mean : Real = mean;
	}
	analysis def Flaky :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.flake;
		attribute :>> observed : String = analysed.label;
		return Mean = mean;
	}
	analysis def Checked :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.settle;
		attribute :>> observed : Real = analysed.t;
		assert constraint { observed < 3.0 }
		assert constraint { mean > 0.0 }
		return Mean : Real = mean;
		out OutOfSpec : Natural = outOfSpec;
	}
	analysis def Branching :> Simulation::MonteCarlo {
		subject analysed : Probe;
		perform action run ::> analysed.settle;
		attribute :>> observed : Real = analysed.t;
		if mean < 3.0 {
			out Statistic : Real = 0.0 - mean;
		}
		return Statistic : Real = mean;
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

// statistic reads the number a "name = value" line of a concluded case reports,
// a quantity's magnitude included.
func statistic(t *testing.T, out, name string) float64 {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), name+" = "); ok {
			rest, _, _ = strings.Cut(rest, " [")
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

// A result reached inside a branch is a result, not a step of every run: the branch
// reads the sample's mean once, at the conclusion, as the ordinary body's end would.
func TestRunsConcludeAResultNestedInABranch(t *testing.T) {
	s := monteCarloSession(t)
	out := run(t, s, "%runs 4 7 MC::Branching MC::probe")
	wants(t, out, "✓ MC::Branching over 4 run(s)")
	if strings.Contains(out, "error") {
		t.Fatalf("a run evaluated the branch over the unbound mean:\n%s", out)
	}
	mean := statistic(t, out, "mean")
	want := mean
	if mean < 3.0 {
		want = -mean
	}
	if got := statistic(t, out, "Statistic"); got != want {
		t.Errorf("Statistic = %v over a mean of %v, want %v", got, mean, want)
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

// A check decided run by run counts the runs it failed in as outOfSpec, the last run's
// no more than the others'; only a check of the sample decides the conclusion.
func TestRunsCountPerRunChecksWithoutJudgingTheLastRunTwice(t *testing.T) {
	s := monteCarloSession(t)
	out := run(t, s, "%runs 5 7 MC::Checked MC::probe")
	wants(t, out, "✓ MC::Checked over 5 run(s)", "assertion mean > 0.0: satisfied")
	if strings.Contains(strings.SplitN(out, "✓ MC::Checked", 2)[1], "observed < 3.0") {
		t.Errorf("the conclusion judges the per-run check again:\n%s", out)
	}
	var last bool
	var failed int
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Split(line, "|"); len(fields) > 1 {
			if v, err := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64); err == nil {
				last = v >= 3.0
				if last {
					failed++
				}
			}
		}
	}
	if !last || failed == 5 {
		t.Fatalf("the sample proves nothing: %d run(s) out of spec, the last %v:\n%s", failed, last, out)
	}
	if got := statistic(t, out, "OutOfSpec"); got != float64(failed) {
		t.Errorf("OutOfSpec = %v, want the %d run(s) observing 3.0 or more", got, failed)
	}
}

// A case observing a quantity concludes in its unit: Mean and Deviation are quantities
// over the magnitudes the runs observed.
func TestRunsConcludeAQuantityObservedCaseInItsUnit(t *testing.T) {
	s := monteCarloSession(t)
	out := run(t, s, "%runs 3 7 MC::Timed MC::probe")
	wants(t, out, "✓ MC::Timed over 3 run(s)", "Mean = ", "Deviation = ")
	var observed []float64
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 2 {
			continue
		}
		magnitude, unit, isQuantity := strings.Cut(strings.TrimSpace(fields[1]), " ")
		if v, err := strconv.ParseFloat(magnitude, 64); err == nil && isQuantity && unit == "[s]" {
			observed = append(observed, v)
		}
	}
	if len(observed) != 3 {
		t.Fatalf("the table holds %d observation(s) in seconds, want 3:\n%s", len(observed), out)
	}
	mean := (observed[0] + observed[1] + observed[2]) / 3
	if got := statistic(t, out, "Mean"); math.Abs(got-mean) > 1e-12 {
		t.Errorf("Mean = %v, want the mean %v of %v", got, mean, observed)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if (strings.HasPrefix(line, "Mean = ") || strings.HasPrefix(line, "Deviation = ")) && !strings.HasSuffix(line, " [s]") {
			t.Errorf("%q is not a quantity in seconds", line)
		}
	}
}

// Every run failing keeps every run's row and error in the table; the case fails and is
// reported unconcluded rather than refused as a sample of nothing.
func TestRunsAllFailingKeepTheirRows(t *testing.T) {
	s := monteCarloSession(t)
	out := run(t, s, "%runs 3 7 MC::Crashing MC::probe")
	wants(t, out, "run | time", "| error", "error 1: ", "error 2: ", "error 3: ", "division by zero",
		"✗ MC::Crashing: no run completed, so the case is not concluded")
	if strings.Contains(out, "no run observed anything") {
		t.Errorf("the failed runs are reported as a sample of nothing:\n%s", out)
	}
}

// Runs observing no number keep their table too; the refusal names the run and what it observed.
func TestRunsObservingNoNumberKeepTheirRows(t *testing.T) {
	s := monteCarloSession(t)
	wants(t, run(t, s, "%runs 2 7 MC::Named MC::probe"), "run | observed", `| "x"`,
		"? MC::Named: invalid observation: run 1 of MC::Named observed \"x\", not a number")
}

// A failed run fails the table whatever comes of the sample: a completed run observing
// no number leaves the case unconcluded under the failed run's verdict, not in place of it.
func TestRunsFailedRunsOutweighAnUnconcludedSample(t *testing.T) {
	s := monteCarloSession(t)
	seed := uint64(7)
	v := s.RunMonteCarlo("MC::Flaky MC::probe", 4, &seed)
	out := strings.Join(v.Lines, "\n")
	var failed, completed int
	for _, row := range v.Rows {
		if row.Error != "" {
			failed++
		} else {
			completed++
		}
	}
	if failed == 0 || completed == 0 {
		t.Fatalf("%d run(s) failed and %d completed; the sample proves nothing:\n%s", failed, completed, out)
	}
	wants(t, out, "division by zero", "✗ MC::Flaky: invalid observation", `observed "x", not a number`)
	if v.Status != VerdictFails {
		t.Errorf("status %v, want %v: run(s) failed\n%s", v.Status, VerdictFails, out)
	}
}

// The sample names a refused observation by the number of the run that made it,
// the row's own, not its place among the runs that completed after earlier ones failed.
func TestRunsRefusedObservationNamesTheRunByItsRow(t *testing.T) {
	s := monteCarloSession(t)
	for seed := uint64(1); seed <= 64; seed++ {
		v := s.RunMonteCarlo("MC::Flaky MC::probe", 6, &seed)
		first := ""
		for _, row := range v.Rows {
			if row.Error == "" {
				for _, in := range row.Inputs {
					if in.Name == runtime.RunParam {
						first = in.Value
					}
				}
				break
			}
		}
		if first == "" || first == "1" {
			continue
		}
		out := strings.Join(v.Lines, "\n")
		wants(t, out, "run "+first+" of MC::Flaky observed \"x\", not a number")
		if strings.Contains(out, "run 1 of MC::Flaky") {
			t.Errorf("the refusal numbers the completed run by its position:\n%s", out)
		}
		return
	}
	t.Fatal("no seed failed the first run before one completed")
}

// A count the sweep budget does not allow, or no count at all, is refused as the
// plan is validated, before any run is made or anything is sized by the count.
func TestRunsRefusesACountBeyondTheBudget(t *testing.T) {
	s := monteCarloSession(t)
	seed := uint64(7)
	beyond := s.RunMonteCarlo("MC::Mc MC::probe", math.MaxInt64, &seed)
	if beyond.Status != VerdictUnresolved || !strings.Contains(strings.Join(beyond.Lines, "\n"), "run(s) per sweep") {
		t.Errorf("%d runs: %v %q, want the sweep budget refused", int64(math.MaxInt64), beyond.Status, beyond.Lines)
	}
	for _, count := range []int64{0, -1} {
		none := s.RunMonteCarlo("MC::Mc MC::probe", count, &seed)
		if none.Status != VerdictUnresolved || !strings.Contains(strings.Join(none.Lines, "\n"), "runs nothing; ask for at least one") {
			t.Errorf("%d runs: %v %q, want the count refused", count, none.Status, none.Lines)
		}
	}
}

// An analysis case that does not specialize Simulation::MonteCarlo is not run
// repeatedly, and the refusal names the library case it would have to be.
func TestRunsRefusesAnOrdinaryAnalysisCase(t *testing.T) {
	s := monteCarloSession(t)
	wants(t, run(t, s, "%runs 3 7 MC::Plain MC::probe"), "MC::Plain", "Simulation::MonteCarlo")
}
