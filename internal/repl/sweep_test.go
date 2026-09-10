package repl

import (
	"regexp"
	"strings"
	"testing"
)

// sweepModel declares what %sweep and %samples run: calcs of one and two
// parameters, one that fails on a value in range, and an analysis case with an
// objective and a quantity parameter.
const sweepModel = `package Sw {
	private import ScalarValues::*;
	private import SI::*;
	calc def Twice { in n : Integer; return : Integer = n * 2; }
	calc def Plus { in a : Integer; in b : Integer; return : Integer = a + b; }
	calc def Ratio { in a : Real; in b : Real; return : Real = a / b; }
	calc def Reach { in v : Real; in t : Real; return : Real = v * t; }
	calc def Lift { in 'launch mass' : Integer; return : Integer = 'launch mass' * 2; }
	calc def Toggle { in on : Boolean; return : Boolean = not on; }
	calc def Echo { in v; return r = v; }
	part def Ship { attribute cost : Real = 5.0; }
	analysis def Priced {
		subject s : Ship;
		in tax : Real;
		objective { require constraint { total <= 8.0 } }
		out total : Real = s.cost * (1.0 + tax);
	}
	part ship : Ship;
}`

func sweepSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	if errs := errorDiagnostics(s.Submit(sweepModel).Diagnostics); len(errs) > 0 {
		t.Fatalf("model has errors: %v", errs)
	}
	return s
}

// elapsedColumn matches the time one run took, which differs between runs, and
// dashRun the rule under a header, whose width follows that time.
var (
	elapsedColumn = regexp.MustCompile(`[0-9]+\.[0-9]{3}ms`)
	dashRun       = regexp.MustCompile(`-{2,}`)
)

// sweepTable renders what a command answered with the run times masked, so a
// comparison covers the rows, their order and their values.
func sweepTable(out string) string {
	return dashRun.ReplaceAllString(elapsedColumn.ReplaceAllString(out, "<time>"), "-")
}

// A sweep over a range between Integers runs the calc once per value, from the
// range's start through its end, and reports one row per run.
func TestSweepOverIntegersRunsEveryValue(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%sweep Sw::Twice n=1..4"))
	want := strings.Join([]string{
		"sweep Sw::Twice — 4 run(s)",
		"n | result | time",
		"-+-+-",
		"1 | 2      | <time>",
		"2 | 4      | <time>",
		"3 | 6      | <time>",
		"4 | 8      | <time>",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// Several ranges run their cartesian product, the first one given varying
// slowest, and a stated step advances the range by it.
func TestSweepSeveralRangesRunTheirProduct(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%sweep Sw::Plus a=1..2 b=10..12:2"))
	want := strings.Join([]string{
		"sweep Sw::Plus — 4 run(s)",
		"a | b  | result | time",
		"-+-+-+-",
		"1 | 10 | 11     | <time>",
		"1 | 12 | 13     | <time>",
		"2 | 10 | 12     | <time>",
		"2 | 12 | 14     | <time>",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// An analysis case is swept on the object named as its subject, each row
// reporting the outputs of that run and what its objective decided.
func TestSweepAnalysisCaseOnItsSubject(t *testing.T) {
	s := sweepSession(t)
	run(t, s, "%instantiate Sw::ship")
	got := sweepTable(run(t, s, "%sweep Sw::Priced Sw::ship tax=0.0..1.0:0.5"))
	want := strings.Join([]string{
		"sweep Sw::Priced — 3 run(s)",
		"tax | total | verdict            | time",
		"-+-+-+-",
		"0.0 | 5.0   | obj: satisfied     | <time>",
		"0.5 | 7.5   | obj: satisfied     | <time>",
		"1.0 | 10.0  | obj: not satisfied | <time>",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// A run that failed is that row's error, and the rows after it are run all the
// same.
func TestSweepFailedRunIsARowOfTheTable(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%sweep Sw::Ratio(a = 4.0) b=-1..1"))
	if !strings.Contains(got, "3 run(s)") {
		t.Fatalf("table is\n%s\nwant 3 runs", got)
	}
	if !strings.Contains(got, "division by zero") {
		t.Errorf("table is\n%s\nwant the failed run's error on its row", got)
	}
	for _, want := range []string{"| -4.0", "| 4.0"} {
		if !strings.Contains(got, want) {
			t.Errorf("table is\n%s\nwant a row reporting %s", got, want)
		}
	}
}

// Endpoints carry the same literals and units an argument does, and a swept
// quantity is bound in the unit its range's start carries.
func TestSweepEndpointsCarryUnits(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s,
		"%sweep Sw::Reach(t = 2.0) v=0.0..10.0:5.0"))
	want := strings.Join([]string{
		"sweep Sw::Reach — 3 run(s)",
		"v    | result | time",
		"-+-+-",
		"0.0  | 0.0    | <time>",
		"5.0  | 10.0   | <time>",
		"10.0 | 20.0   | <time>",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// Samples draw one row per draw, the same seed drawing the same table and
// another seed drawing another; the header echoes the seed drawn from.
func TestSamplesAreReproducibleFromTheirSeed(t *testing.T) {
	s := sweepSession(t)
	first := sweepTable(run(t, s, "%samples 5 42 Sw::Twice n=0..1000"))
	again := sweepTable(run(t, s, "%samples 5 42 Sw::Twice n=0..1000"))
	other := sweepTable(run(t, s, "%samples 5 43 Sw::Twice n=0..1000"))
	if first != again {
		t.Errorf("seed 42 drew\n%s\nthen\n%s", first, again)
	}
	if first == other {
		t.Errorf("seed 43 drew what seed 42 did:\n%s", first)
	}
	if !strings.HasPrefix(first, "samples Sw::Twice — 5 run(s), seed 42") {
		t.Errorf("header is %q; want it to echo the seed", strings.SplitN(first, "\n", 2)[0])
	}
}

// The drawn table is pinned, so the generator answers for the same seed on
// every platform.
func TestSamplesTableIsPinned(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%samples 4 42 Sw::Twice n=0..999"))
	want := strings.Join([]string{
		"samples Sw::Twice — 4 run(s), seed 42",
		"n   | result | time",
		"-+-+-",
		"454 | 908    | <time>",
		"972 | 1944   | <time>",
		"719 | 1438   | <time>",
		"345 | 690    | <time>",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// A range is typed by the parameter it sweeps, not by its literals: over a Real
// parameter Integer literals bind and print as Reals, over an Integer parameter
// real literals bind and print as Integers, and a Real parameter samples reals.
func TestSweepBindsInTheParameterType(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%sweep Sw::Ratio(b = 2.0) a=1..4:1"))
	want := strings.Join([]string{
		"sweep Sw::Ratio — 4 run(s)",
		"a   | result | time",
		"-+-+-",
		"1.0 | 0.5    | <time>",
		"2.0 | 1.0    | <time>",
		"3.0 | 1.5    | <time>",
		"4.0 | 2.0    | <time>",
	}, "\n")
	if !strings.Contains(got, want) {
		t.Errorf("table is\n%s\nwant it to carry\n%s", got, want)
	}
	got = sweepTable(run(t, s, "%sweep Sw::Twice n=1.0..3.0:1.0"))
	want = strings.Join([]string{
		"sweep Sw::Twice — 3 run(s)",
		"n | result | time",
		"-+-+-",
		"1 | 2      | <time>",
		"2 | 4      | <time>",
		"3 | 6      | <time>",
	}, "\n")
	if !strings.Contains(got, want) {
		t.Errorf("table is\n%s\nwant it to carry\n%s", got, want)
	}
	got = sweepTable(run(t, s, "%samples 4 7 Sw::Ratio(b = 1.0) a=1..4"))
	if !strings.Contains(got, "4 run(s), seed 7") || strings.Contains(got, "error") {
		t.Fatalf("table is\n%s\nwant four drawn runs", got)
	}
	for _, line := range strings.Split(got, "\n")[3:7] {
		if cell := strings.TrimSpace(strings.SplitN(line, "|", 2)[0]); !strings.Contains(cell, ".") {
			t.Errorf("row %q drew %s over a Real parameter; want a real", line, cell)
		}
	}
}

// A parameter declaring no type has its range read as written, and the table
// says so.
func TestSweepOverAnUntypedParameterIsReadAsWritten(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%sweep Sw::Echo v=1..2"))
	want := strings.Join([]string{
		"sweep Sw::Echo — 2 run(s)",
		"v | result | time",
		"-+-+-",
		"1 | 1      | <time>",
		"2 | 2      | <time>",
		"note: v declares no type; its range is read as written",
	}, "\n")
	if !strings.Contains(got, want) {
		t.Errorf("table is\n%s\nwant it to carry\n%s", got, want)
	}
	got = sweepTable(run(t, s, "%sweep Sw::Echo v=1.0..2.0:1.0"))
	if !strings.Contains(got, "1.0 | 1.0    | <time>") || !strings.Contains(got, "note: v declares no type") {
		t.Errorf("table is\n%s\nwant reals read as written and the note", got)
	}
}

// A sampled range needs no step, since it draws over its endpoints rather than
// stepping through them.
func TestSamplesOverARealRangeNeedNoStep(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%samples 3 7 Sw::Ratio(b = 2.0) a=1.0..2.0"))
	if !strings.Contains(got, "3 run(s), seed 7") || strings.Contains(got, "error") {
		t.Errorf("table is\n%s\nwant three drawn runs", got)
	}
}

// What stops a sweep is reported as an error naming what is wrong, never as a
// table: the refusals are typed and the usage is shown.
func TestSweepErrors(t *testing.T) {
	s := sweepSession(t)
	cases := []struct {
		command string
		wants   []string
	}{
		{"%sweep", []string{"usage: %sweep"}},
		{"%sweep Sw::Twice", []string{"no sweep range"}},
		{"%sweep Sw::Twice n=1", []string{"states no range"}},
		{"%sweep Sw::Ratio(b = 1.0) a=0.5..1.0", []string{"needs `:<step>`"}},
		{"%sweep Sw::Twice n=1.0..3.0:0.5", []string{"n : Integer", "0.5"}},
		{"%sweep Sw::Twice n=1.5..3", []string{"n : Integer", "1.5"}},
		{"%samples 2 1 Sw::Twice n=1.5..3", []string{"n : Integer", "1.5"}},
		{"%sweep Sw::Toggle on=0..1", []string{"on", "typed by Boolean"}},
		{"%samples 2 1 Sw::Toggle on=0..1", []string{"on", "typed by Boolean"}},
		{"%sweep Sw::Twice n=1..4:0", []string{"zero"}},
		{"%sweep Sw::Twice n=4..1:1", []string{"away from"}},
		{"%sweep Sw::Twice nope=1..4", []string{"nope"}},
		{"%sweep Sw::Twice(n = 1) n=1..4", []string{"both an argument"}},
		{"%sweep Sw::Priced(0.0) Sw::ship tax=0.0..1.0:0.5", []string{"both an argument"}},
		{"%sweep Sw::Priced Sw::ship s=1..4", []string{"subject"}},
		{"%sweep Sw::Twice n=1..4 n=5..6", []string{"swept twice"}},
		{"%sweep Sw::Twice n=1..2000000", []string{"OPENSYSML_MAX_SWEEP_RUNS"}},
		{"%sweep Sw::ship n=1..4", []string{"Sw::ship"}},
		{"%sweep Sw::Twice Sw::ship n=1..4", []string{"calc, which has no subject"}},
		{"%sweep Sw::Twice n=\"a\"..\"b\"", []string{"not a number"}},
		{"%samples", []string{"usage: %samples"}},
		{"%samples 4 Sw::Twice n=1..4", []string{"not a seed"}},
		{"%samples 0 1 Sw::Twice n=1..4", []string{"not a number of samples"}},
		{"%samples -1 1 Sw::Twice n=1..4", []string{"not a number of samples"}},
		{"%samples 4 1 Sw::Twice", []string{"no sweep range"}},
		{"%samples 4 1 Sw::Twice n=1..4:1", []string{"states a step"}},
		{"%samples 4 1 Sw::Twice n=normal(1.0, 0.2)", []string{
			"no library in this build states a probability distribution", "normal"}},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			wants(t, run(t, s, tc.command), tc.wants...)
		})
	}
}

// A sweep answers a verdict a report reads: how many runs it made, how many
// failed, and one entry per run.
func TestRunSweepVerdict(t *testing.T) {
	s := sweepSession(t)

	v := s.RunSweep("Sw::Twice", []string{"n=1..3"})
	if v.Status != VerdictHolds || v.Subject != "sweep Sw::Twice" {
		t.Fatalf("verdict = %+v", v)
	}
	if len(v.Rows) != 3 || len(v.Rows[0].Inputs) != 1 || v.Rows[0].Inputs[0].Value != "1" {
		t.Fatalf("rows = %+v", v.Rows)
	}
	if v.Rows[0].Outputs[0] != (NamedValue{Name: "result", Value: "2"}) {
		t.Errorf("first row's outputs = %+v", v.Rows[0].Outputs)
	}
	if len(v.Values) != 2 || v.Values[0] != (NamedValue{Name: "runs", Value: "3"}) ||
		v.Values[1] != (NamedValue{Name: "failed", Value: "0"}) {
		t.Errorf("values = %+v", v.Values)
	}

	failed := s.RunSweep("Sw::Ratio(a = 1.0)", []string{"b=-1..1"})
	if failed.Status != VerdictFails || failed.Values[1] != (NamedValue{Name: "failed", Value: "1"}) {
		t.Errorf("a table with a failed run = %+v", failed)
	}

	drawn := s.RunSamples("Sw::Twice", []string{"n=0..10"}, 3, 11)
	if drawn.Subject != "samples Sw::Twice" || len(drawn.Rows) != 3 ||
		drawn.Values[2] != (NamedValue{Name: "seed", Value: "11"}) {
		t.Errorf("verdict = %+v", drawn)
	}

	refused := s.RunSweep("Sw::Twice", []string{"n=1"})
	if refused.Status != VerdictUnresolved || len(refused.Rows) != 0 {
		t.Errorf("a refused plan = %+v", refused)
	}
}

// %help names both commands, so a reader at the prompt finds them.
func TestSweepCommandsAreInHelp(t *testing.T) {
	s := sweepSession(t)
	wants(t, run(t, s, "%help"), "%sweep", "%samples")
}

// A parameter whose name needs the quotes of an unrestricted name is swept
// under that name, as it is written in the model.
func TestSweepOverAnUnrestrictedParameterName(t *testing.T) {
	s := sweepSession(t)
	got := sweepTable(run(t, s, "%sweep Sw::Lift 'launch mass'=1..3"))
	want := strings.Join([]string{
		"sweep Sw::Lift — 3 run(s)",
		"launch mass | result | time",
		"-+-+-",
		"1           | 2      | <time>",
		"2           | 4      | <time>",
		"3           | 6      | <time>",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
