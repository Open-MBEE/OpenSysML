package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runsModel declares what %runs runs: an action that draws a duration, an
// Integer, and a weighted branch, and one that draws nothing.
const runsModel = `package MC {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	private import Stochastic::*;
	private import RandomFunctions::*;
	action acquire {
		attribute total : Real = 0.0;
		attribute tries : Integer = uniformInteger(1, 3);
		attribute slow : Boolean = false;
		first start;
		then accept after uniform(1, 80) [s];
		then decide d;
		first d then fast { @Probability { p = 0.7; } }
		first d then long { @Probability { p = 0.3; } }
		action fast { assign total := total + 1.0; }
		then done;
		action long { assign total := total + 10.0; assign slow := true; }
		then done;
	}
	action fixed { attribute k : Integer = 4; first start; then done; }
}`

func runsSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(runsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	return s
}

// %runs runs the action once per run, each run's draws seeded from the seed
// given, and reports the observables named — here the clock the run completed
// at — with their distribution: extremes, mean, percentiles and a histogram.
func TestRunsReportsEachRunAndTheDistribution(t *testing.T) {
	s := runsSession(t)
	got := sweepTable(run(t, s, "%runs 6 7 MC::acquire clock"))
	want := strings.Join([]string{
		"runs MC::acquire — 6 run(s), seed 7",
		"run | clock                  | time",
		"-+-+-",
		"1   | 24.59324592607634 [s]  | <time>",
		"2   | 4.848060164697906 [s]  | <time>",
		"3   | 29.084124217381778 [s] | <time>",
		"4   | 69.36274129306246 [s]  | <time>",
		"5   | 33.04997525591596 [s]  | <time>",
		"6   | 73.76824997509146 [s]  | <time>",
		"clock: 6 run(s), min 4.848060164697906 [s], mean 39.117732805370984 [s], max 73.76824997509146 [s], p50 29.084124217381778 [s], p90 73.76824997509146 [s]",
		"  4.848..13.46 [s] ####                 1",
		"  13.46..22.08 [s]                      0",
		"  22.08..30.69 [s] #######              2",
		"  30.69..39.31 [s] ####                 1",
		"  39.31..47.92 [s]                      0",
		"  47.92..56.54 [s]                      0",
		"  56.54..65.15 [s]                      0",
		"  65.15..73.77 [s] #######              2",
		"  standing: table (observed: 6 rows)",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// Naming no observable reports every feature the action holds, in name order,
// then the clock; whole numbers bin one per bin, and a nonnumeric observable
// is tallied by value.
func TestRunsDefaultsToEveryFeatureAndTheClock(t *testing.T) {
	s := runsSession(t)
	out := sweepTable(run(t, s, "%runs 6 7 MC::acquire"))
	wants(t, out,
		"run | slow  | total | tries | clock                  | time",
		"1   | false | 1.0   | 3     | 24.59324592607634 [s]  | <time>",
		"slow: false ×6",
		"total: 6 run(s), min 1.0, mean 1.0, max 1.0, p50 1.0, p90 1.0",
		"  1.0 #################### 6",
		"tries: 6 run(s), min 1, mean 2.0, max 3, p50 2, p90 3",
		"  1 #######              2",
		"  2 #######              2",
		"  3 #######              2",
		"clock: 6 run(s), min 4.848060164697906 [s]",
	)
}

// Each run draws from a seed of its own, so a weighted branch is taken as its
// weight says over the runs rather than the same way in every run, and the
// same seed makes the same runs while another seed makes others.
func TestRunsDrawEachRunFromItsOwnSeed(t *testing.T) {
	s := runsSession(t)
	first := sweepTable(run(t, s, "%runs 20 7 MC::acquire slow"))
	wants(t, first, "slow: false ×17, true ×3")
	if again := sweepTable(run(t, s, "%runs 20 7 MC::acquire slow")); again != first {
		t.Errorf("the same seed ran differently:\n%s\nthen\n%s", first, again)
	}
	other := sweepTable(run(t, s, "%runs 20 8 MC::acquire slow"))
	if other == first {
		t.Errorf("seed 8 ran as seed 7 did:\n%s", other)
	}
	wants(t, other, "slow: ", "true ×")
}

// The model seed of each run is the one derived for it: the schedule's own
// seed, set for the choices the model leaves open, decides no draw.
func TestRunsAreTheSameUnderAnySchedule(t *testing.T) {
	s := runsSession(t)
	declared := sweepTable(run(t, s, "%runs 3 7 MC::acquire clock"))
	run(t, s, "%schedule seed:3")
	if seeded := sweepTable(run(t, s, "%runs 3 7 MC::acquire clock")); seeded != declared {
		t.Errorf("under seed:3 the runs differ:\n%s\nfrom\n%s", seeded, declared)
	}
}

// An action drawing nothing runs the same every time, and the table says so.
func TestRunsOfADeterministicActionAgree(t *testing.T) {
	s := runsSession(t)
	got := sweepTable(run(t, s, "%runs 4 7 MC::fixed"))
	want := strings.Join([]string{
		"runs MC::fixed — 4 run(s), seed 7",
		"run | k | clock   | time",
		"-+-+-+-",
		"1   | 4 | 0.0 [s] | <time>",
		"2   | 4 | 0.0 [s] | <time>",
		"3   | 4 | 0.0 [s] | <time>",
		"4   | 4 | 0.0 [s] | <time>",
		"k: 4 run(s), min 4, mean 4.0, max 4, p50 4, p90 4",
		"  4 #################### 4",
		"clock: 4 run(s), min 0.0 [s], mean 0.0 [s], max 0.0 [s], p50 0.0 [s], p90 0.0 [s]",
		"  0.0 [s] #################### 4",
		"  standing: table (observed: 4 rows)",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// A malformed %runs is refused with its usage; an observable the action does
// not hold, or one named twice, is an error naming it.
func TestRunsRefusesWhatItCannotRun(t *testing.T) {
	s := runsSession(t)
	for _, tc := range []struct{ line, want string }{
		{"%runs", runsUsage},
		{"%runs 3 7", "name the number of runs, the seed, then the action"},
		{"%runs 0 7 MC::acquire", `"0" is not a number of runs to make`},
		{"%runs -2 7 MC::acquire", `"-2" is not a number of runs to make`},
		{"%runs 3 x MC::acquire", `"x" is not a seed`},
		{"%runs 2 7 MC::Missing", "unresolved reference: MC::Missing"},
		{"%runs 3 7 MC::acquire total nope", "MC::acquire holds no feature named nope (the clock is observed as clock)"},
		{"%runs 3 7 MC::acquire total total", "observable total is named twice"},
	} {
		wants(t, run(t, s, tc.line), tc.want)
	}
}

// While a witness drives the schedule every draw is fixed, so %runs is refused
// rather than reporting one run many times.
func TestRunsRefusedUnderAReplay(t *testing.T) {
	s := runsSession(t)
	witness := filepath.Join(t.TempDir(), "none.witness")
	if err := os.WriteFile(witness, []byte("no choice points\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wants(t, run(t, s, "%replay "+witness), "schedule: replay:"+witness)
	wants(t, run(t, s, "%runs 3 7 MC::acquire"), ErrRunsReplay.Error())
}

// %help names %runs, so a reader at the prompt finds it.
func TestHelpNamesRuns(t *testing.T) {
	s := runsSession(t)
	wants(t, run(t, s, "%help"), "%runs <n> <seed> <action> [<observable>...]", "%seed [<n>|off]")
}

// %seed fixes the seed one run's draws come from under any schedule: unset, a
// run that draws is refused naming the draw; set, the run draws from it the same
// under reverse and under seed:<n>; off unsets it, and a non-number is refused.
func TestSeedFixesTheDrawsOfARun(t *testing.T) {
	s := runsSession(t)
	wants(t, run(t, s, "%seed"), "seed: off")
	wants(t, run(t, s, "%action MC::acquire"), "modeled randomness needs a seed", "uniformInteger(1, 3)", "%seed <n>")
	wants(t, run(t, s, "%seed 7"), "seed: 7")
	wants(t, run(t, s, "%seed"), "seed: 7")
	if seed, set := s.ModelSeed(); !set || seed != 7 {
		t.Errorf("ModelSeed() = %d, %v, want 7, true", seed, set)
	}
	run(t, s, "%action MC::acquire")
	reverse := run(t, s, "%continue")
	wants(t, reverse, "Action completed", "tries = 1")
	run(t, s, "%schedule seed:3")
	run(t, s, "%action MC::acquire")
	if seeded := run(t, s, "%continue"); seeded != reverse {
		t.Errorf("under seed:3 the run was\n%s\nunder reverse\n%s", seeded, reverse)
	}
	wants(t, run(t, s, "%seed x"), `"x" is not a seed`)
	wants(t, run(t, s, "%seed off"), "seed: off")
	if _, set := s.ModelSeed(); set {
		t.Error("ModelSeed() is set after seed off")
	}
}
