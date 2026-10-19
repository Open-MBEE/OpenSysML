package repl

import (
	"strconv"
	"strings"
	"testing"
)

// twoDrawsModel gives every argument surface a behavior with two Real inputs
// whose difference shows whether its two arguments drew the same value.
const twoDrawsModel = `package Two {
	private import ScalarValues::*;
	private import RandomFunctions::*;
	calc def Pair { in a : Real; in b : Real; return r : Real = a - b; }
	calc def Shift { in a : Real; in b : Real; in n : Integer; return r : Real = a - b + n; }
	part def Box {
		action bump { in a : Real; in b : Real; out d : Real = a - b; }
	}
	part box : Box;
	action def Diff { in a : Real; in b : Real; out d : Real = a - b; }
	analysis def Study { subject s : Box; in a : Real; in b : Real; return diff : Real = a - b; }
	analysis def Mc :> Simulation::MonteCarlo {
		subject s : Box;
		out :>> runs; out :>> mean; out :>> deviation; out :>> outOfSpec;
		in lo : Real;
		in hi : Real;
		attribute :>> observed : Real = lo - hi;
		return Mean : Real = mean;
	}
}`

const (
	twoDraws       = "uniform(0.0, 1.0), uniform(0.0, 1.0)"
	twoDrawsSpaced = "uniform(0.0, 1.0) uniform(0.0, 1.0)"
)

// twoDrawsSession seeds a session holding the model and an object, and reads
// what one expression drawing twice in one run evaluates to under that seed:
// the difference every argument list of two draws must reproduce.
func twoDrawsSession(t *testing.T) (*Session, float64) {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(twoDrawsModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	wants(t, run(t, s, "%instantiate Two::box"), "Created instance")
	wants(t, run(t, s, "%seed 7"), "seed: 7")
	want := number(t, run(t, s, "%eval uniform(0.0, 1.0) - uniform(0.0, 1.0)"), "=")
	if want == 0 {
		t.Fatalf("two draws of one expression drew the same value")
	}
	return s, want
}

// number reads the first "<label> <value>" line of out: a quantity's magnitude
// included, and the sole element of a one-element sequence ("[0.5]").
func number(t *testing.T, out, label string) float64 {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), label+" "); ok {
			rest, _, _ = strings.Cut(strings.TrimSpace(rest), " ")
			rest = strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]")
			v, err := strconv.ParseFloat(rest, 64)
			if err != nil {
				t.Fatalf("%s %q is not a number in\n%s", label, rest, out)
			}
			return v
		}
	}
	t.Fatalf("no %q line in\n%s", label, out)
	return 0
}

// drawsSuccessively checks that an argument list's two seeded draws came from
// one stream, the second continuing where the first left off.
func drawsSuccessively(t *testing.T, surface, out, label string, want float64) {
	t.Helper()
	if got := number(t, out, label); got != want {
		t.Errorf("%s: two seeded argument draws gave %v, want %v (0 means both drew the seed's first value):\n%s", surface, got, want, out)
	}
}

// A seeded argument list is one evaluation in one run: the second argument's
// draw continues the first's stream, on every surface that binds arguments.
func TestSeededArgumentListDrawsSuccessivelyOnEverySurface(t *testing.T) {
	s, want := twoDrawsSession(t)
	drawsSuccessively(t, "%calc", run(t, s, "%calc Two::Pair("+twoDraws+")"), "=", want)
	drawsSuccessively(t, "%invoke", run(t, s, "%invoke box bump "+twoDrawsSpaced), "d =", want)
	drawsSuccessively(t, "%invoke named", run(t, s, "%invoke box bump a=uniform(0.0, 1.0) b=uniform(0.0, 1.0)"), "d =", want)
	drawsSuccessively(t, "action inputs", actionOutput(t, s.RunActionWithInputs("Two::Diff", []string{"uniform(0.0, 1.0)", "uniform(0.0, 1.0)"})), "d =", want)
	drawsSuccessively(t, "action inputs named", actionOutput(t, s.RunActionWithInputs("Two::Diff", []string{"b=uniform(0.0, 1.0)", "a=uniform(0.0, 1.0)"})), "d =", -want)
	drawsSuccessively(t, "%analysis", run(t, s, "%analysis Two::Study("+twoDraws+") box"), "diff =", want)
	drawsSuccessively(t, "%analysis named", run(t, s, "%analysis Two::Study(a=uniform(0.0, 1.0), b=uniform(0.0, 1.0)) box"), "diff =", want)
	// A mixed list draws in the order written: here b draws first, so a - b is
	// the second draw less the first.
	drawsSuccessively(t, "%analysis mixed", run(t, s, "%analysis Two::Study(b=uniform(0.0, 1.0), uniform(0.0, 1.0)) box"), "diff =", -want)

	sweep := run(t, s, "%sweep Two::Shift("+twoDraws+") n=0..1")
	for i, row := range sweepRows(sweep) {
		if got := row[len(row)-1]; got != want+float64(i) {
			t.Errorf("%%sweep row %d: r = %v, want %v:\n%s", i, got, want+float64(i), sweep)
		}
	}
	// The case's own runs are seeded per run; its arguments are evaluated once,
	// at the prompt, so every run observes the one difference they drew.
	runs := run(t, s, "%runs 3 7 Two::Mc("+twoDraws+") box")
	drawsSuccessively(t, "%runs", runs, "Mean =", want)
	for i, row := range sweepRows(runs) {
		if got := row[len(row)-1]; got != want {
			t.Errorf("%%runs run %d observed %v, want %v:\n%s", i, got, want, runs)
		}
	}
	mixed := run(t, s, "%runs 3 7 Two::Mc(hi=uniform(0.0, 1.0), uniform(0.0, 1.0)) box")
	drawsSuccessively(t, "%runs mixed", mixed, "Mean =", -want)
}

// actionOutput renders a run's verdict as "<name> <value>" lines, one per output.
func actionOutput(t *testing.T, v Verdict) string {
	t.Helper()
	if v.Status != VerdictHolds {
		t.Fatalf("run of %s: %s %v", v.Subject, v.Status, v.Lines)
	}
	var lines []string
	for _, nv := range v.Values {
		lines = append(lines, nv.Name+" = "+nv.Value)
	}
	return strings.Join(lines, "\n")
}

// sweepRows reads the numeric cells of each table row of out.
func sweepRows(out string) [][]float64 {
	var rows [][]float64
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 2 {
			continue
		}
		var row []float64
		for _, f := range fields {
			if v, err := strconv.ParseFloat(strings.TrimSpace(f), 64); err == nil {
				row = append(row, v)
			}
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

// The same seed reproduces the whole invocation: the same argument list bound
// twice under one seed draws the same values both times.
func TestSeededArgumentListIsDeterministic(t *testing.T) {
	s, want := twoDrawsSession(t)
	first := run(t, s, "%calc Two::Pair("+twoDraws+")")
	again := run(t, s, "%calc Two::Pair("+twoDraws+")")
	if first != again {
		t.Errorf("the same seeded argument list evaluated differently:\n%s\nthen\n%s", first, again)
	}
	drawsSuccessively(t, "%calc", again, "=", want)
	if other := run(t, s, "%invoke box bump "+twoDrawsSpaced); number(t, other, "d =") != want {
		t.Errorf("%%invoke under the same seed drew differently from %%calc:\n%s", other)
	}
	inputs := []string{"uniform(0.0, 1.0)", "uniform(0.0, 1.0)"}
	if other := actionOutput(t, s.RunActionWithInputs("Two::Diff", inputs)); number(t, other, "d =") != want {
		t.Errorf("a run's bound inputs under the same seed drew differently from %%calc:\n%s", other)
	}
}

// An argument that fails to evaluate is reported by its text or parameter, as
// before the list was evaluated in one run.
func TestArgumentListReportsTheFailingArgument(t *testing.T) {
	s, _ := twoDrawsSession(t)
	wants(t, run(t, s, "%calc Two::Pair(1.0, nowhere)"), `evaluation of argument "nowhere" failed`)
	wants(t, run(t, s, "%invoke box bump a=1.0 b=nowhere"), "argument b:")
	if v := s.RunActionWithInputs("Two::Diff", []string{"1.0", "nowhere"}); v.Status != VerdictUnresolved || !strings.Contains(strings.Join(v.Lines, "\n"), `evaluation of argument "nowhere" failed`) {
		t.Errorf("a run's failing input: %s %v", v.Status, v.Lines)
	}
	wants(t, run(t, s, "%analysis Two::Study(nowhere, 1.0) box"), `evaluation of argument "nowhere" failed`)
}
