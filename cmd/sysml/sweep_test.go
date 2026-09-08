package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// sweepCLIModel declares what -sweep and -samples run: a calc of one
// parameter, one that fails on a value in range, and an analysis case with an
// objective run against an object.
const sweepCLIModel = `package Sw {
    private import ScalarValues::*;
    calc def Twice { in n : Integer; return : Integer = n * 2; }
    calc def Ratio { in a : Real; in b : Real; return : Real = a / b; }
    calc def Lift { in 'launch mass' : Integer; return : Integer = 'launch mass' * 2; }
    part def Ship { attribute cost : Real = 5.0; }
    analysis def Priced {
        subject s : Ship;
        in tax : Real;
        objective { require constraint { total <= 8.0 } }
        out total : Real = s.cost * (1.0 + tax);
    }
    part ship : Ship;
}
`

// elapsedCell matches the time one run took and dashRun the rule under a
// header, whose width follows that time; both differ between runs.
var (
	elapsedCell = regexp.MustCompile(`[0-9]+\.[0-9]{3}ms`)
	dashRun     = regexp.MustCompile(`-{2,}`)
)

// sweepTable renders a report with the run times masked, so a comparison covers
// the rows, their order and their values.
func sweepTable(out string) string {
	return dashRun.ReplaceAllString(elapsedCell.ReplaceAllString(out, "<time>"), "-")
}

// TestSweepThroughCLI checks the table -sweep prints: one row per value of the
// range, in order, with the run's result and its time.
func TestSweepThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, sweepCLIModel, "-calc", "Sw::Twice", "-sweep", "n=1..4")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	want := strings.Join([]string{
		"sweep Sw::Twice — 4 run(s)",
		"n | result | time",
		"-+-+-",
		"1 | 2      | <time>",
		"2 | 4      | <time>",
		"3 | 6      | <time>",
		"4 | 8      | <time>",
	}, "\n")
	if table := sweepTable(got.output()); !strings.Contains(table, want) {
		t.Errorf("report is\n%s\nwant it to carry\n%s", table, want)
	}
}

// TestSweepProductThroughCLI checks that several -sweep flags run their
// cartesian product, the first flag given varying slowest.
func TestSweepProductThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, sweepCLIModel,
		"-calc", "Sw::Ratio", "-sweep", "a=1.0..2.0:1.0", "-sweep", "b=2.0..4.0:2.0")
	want := strings.Join([]string{
		"a   | b   | result | time",
		"-+-+-+-",
		"1.0 | 2.0 | 0.5    | <time>",
		"1.0 | 4.0 | 0.25   | <time>",
		"2.0 | 2.0 | 1.0    | <time>",
		"2.0 | 4.0 | 0.5    | <time>",
	}, "\n")
	if table := sweepTable(got.output()); !strings.Contains(table, want) {
		t.Errorf("report is\n%s\nwant it to carry\n%s", table, want)
	}
}

// TestSweepAnalysisThroughCLI checks that an analysis case is swept on the
// object -instantiate materialized, each row reporting what its objective
// decided, and that an unsatisfied objective fails the check.
func TestSweepAnalysisThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, sweepCLIModel,
		"-instantiate", "Sw::ship", "-analysis", "Sw::Priced Sw::ship", "-sweep", "tax=0.0..1.0:0.5")
	if got.status != 1 {
		t.Errorf("exit status = %d, want 1 for the run whose objective failed\n%s", got.status, got.output())
	}
	want := strings.Join([]string{
		"tax | total | verdict            | time",
		"-+-+-+-",
		"0.0 | 5.0   | obj: satisfied     | <time>",
		"0.5 | 7.5   | obj: satisfied     | <time>",
		"1.0 | 10.0  | obj: not satisfied | <time>",
	}, "\n")
	if table := sweepTable(got.output()); !strings.Contains(table, want) {
		t.Errorf("report is\n%s\nwant it to carry\n%s", table, want)
	}
}

// TestSamplesThroughCLI checks that -samples draws one row per draw from the
// seed given, that the seed is echoed, and that the same seed draws the same
// table while another seed draws another.
func TestSamplesThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	drawn := func(seed string) string {
		got := check(t, binary, sweepCLIModel,
			"-calc", "Sw::Twice", "-sweep", "n=0..999", "-samples", "4", "-seed", seed)
		if got.status != 0 {
			t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
		}
		return sweepTable(got.output())
	}
	first, again, other := drawn("42"), drawn("42"), drawn("43")
	want := strings.Join([]string{
		"samples Sw::Twice — 4 run(s), seed 42",
		"n   | result | time",
		"-+-+-",
		"454 | 908    | <time>",
		"972 | 1944   | <time>",
		"719 | 1438   | <time>",
		"345 | 690    | <time>",
	}, "\n")
	if !strings.Contains(first, want) {
		t.Errorf("report is\n%s\nwant it to carry\n%s", first, want)
	}
	if first != again {
		t.Errorf("seed 42 drew\n%s\nthen\n%s", first, again)
	}
	if first == other {
		t.Errorf("seed 43 drew what seed 42 did:\n%s", first)
	}
}

// TestSweepJSONThroughCLI checks the rows -json carries inside the check
// document it already prints: the values bound for each run, its outputs, its
// time and its error.
func TestSweepJSONThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, sweepCLIModel, "-json", "-calc", "Sw::Ratio(a = 4.0)", "-sweep", "b=-1..1")
	var report struct {
		Checks []struct {
			Kind    string `json:"kind"`
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Rows    []struct {
				Inputs []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"inputs"`
				Outputs []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"outputs"`
				Milliseconds float64 `json:"milliseconds"`
				Error        string  `json:"error"`
			} `json:"rows"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("report is not the check document: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 {
		t.Fatalf("report carries %d check(s), want one: %s", len(report.Checks), got.stdout)
	}
	rows := report.Checks[0].Rows
	if len(rows) != 3 {
		t.Fatalf("check carries %d row(s), want 3: %s", len(rows), got.stdout)
	}
	if len(rows[0].Inputs) != 1 || rows[0].Inputs[0].Name != "b" || rows[0].Inputs[0].Value != "-1" {
		t.Errorf("first row binds %+v, want b = -1", rows[0].Inputs)
	}
	if len(rows[0].Outputs) != 1 || rows[0].Outputs[0].Value != "-4.0" {
		t.Errorf("first row returned %+v, want -4.0", rows[0].Outputs)
	}
	if !strings.Contains(rows[1].Error, "division by zero") || len(rows[1].Outputs) != 0 {
		t.Errorf("second row = %+v, want the typed failure and no output", rows[1])
	}
	if rows[2].Error != "" || rows[2].Outputs[0].Value != "4.0" {
		t.Errorf("third row = %+v, want the run after the failed one", rows[2])
	}
	for i, row := range rows {
		if row.Milliseconds < 0 {
			t.Errorf("row %d took %v ms", i, row.Milliseconds)
		}
	}
	if report.Checks[0].Status != "fails" {
		t.Errorf("status = %q, want a table with a failed run to fail", report.Checks[0].Status)
	}
}

// TestSweepMisuseThroughCLI checks that flags no sweep follows from are refused
// before anything runs, each refusal naming what to write instead.
func TestSweepMisuseThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name  string
		args  []string
		wants string
	}{
		{"no target", []string{"-sweep", "n=1..4"}, "name one, as -analysis <name> or -calc <name>"},
		{"two targets", []string{"-calc", "Sw::Twice", "-analysis", "Sw::Priced", "-sweep", "n=1..4"},
			"name a single -analysis or -calc"},
		{"samples without a range", []string{"-calc", "Sw::Twice", "-samples", "4", "-seed", "1"},
			"-samples draws from a range"},
		{"samples without a seed", []string{"-calc", "Sw::Twice", "-sweep", "n=1..9", "-samples", "4"},
			"-samples draws from a seed"},
		{"a seed without samples", []string{"-calc", "Sw::Twice", "-sweep", "n=1..9", "-seed", "1"},
			"name how many to draw"},
		{"samples that are no number", []string{"-calc", "Sw::Twice", "-sweep", "n=1..9", "-samples", "none"},
			"-samples takes the number of values to draw"},
		{"a seed that is no number", []string{"-calc", "Sw::Twice", "-sweep", "n=1..9", "-samples", "2", "-seed", "x"},
			"-seed takes a whole number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := check(t, binary, sweepCLIModel, tc.args...)
			if got.status != 2 {
				t.Errorf("exit status = %d, want 2\n%s", got.status, got.output())
			}
			if !strings.Contains(got.output(), tc.wants) {
				t.Errorf("report is\n%s\nwant it to name %q", got.output(), tc.wants)
			}
		})
	}
}

// TestSweepRefusalsThroughCLI checks that a range no runs follow from is a
// typed refusal that decides nothing rather than a table.
func TestSweepRefusalsThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name  string
		args  []string
		wants string
	}{
		{"a real range stating no step", []string{"-calc", "Sw::Ratio(b = 1.0)", "-sweep", "a=0.0..1.0"},
			"needs `:<step>`"},
		{"a step of zero", []string{"-calc", "Sw::Twice", "-sweep", "n=1..4:0"}, "zero"},
		{"a parameter the calc does not declare", []string{"-calc", "Sw::Twice", "-sweep", "nope=1..4"}, "nope"},
		{"a parameter the arguments bind", []string{"-calc", "Sw::Twice(n = 1)", "-sweep", "n=1..4"},
			"both an argument"},
		{"more runs than the budget allows", []string{"-calc", "Sw::Twice", "-sweep", "n=1..2000000"},
			"OPENSYSML_MAX_SWEEP_RUNS"},
		{"a named distribution", []string{"-calc", "Sw::Twice", "-sweep", "n=normal(1.0, 0.2)",
			"-samples", "4", "-seed", "1"}, "no library in this build states a probability distribution"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := check(t, binary, sweepCLIModel, tc.args...)
			if got.status != 2 {
				t.Errorf("exit status = %d, want 2\n%s", got.status, got.output())
			}
			if !strings.Contains(got.output(), tc.wants) {
				t.Errorf("report is\n%s\nwant it to name %q", got.output(), tc.wants)
			}
		})
	}
}

// TestSweepUnrestrictedParameterThroughCLI checks that a parameter whose name
// needs the quotes of an unrestricted name is swept under that name.
func TestSweepUnrestrictedParameterThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, sweepCLIModel, "-calc", "Sw::Lift", "-sweep", "'launch mass'=1..3")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	want := strings.Join([]string{
		"launch mass | result | time",
		"-+-+-",
		"1           | 2      | <time>",
		"2           | 4      | <time>",
		"3           | 6      | <time>",
	}, "\n")
	if table := sweepTable(got.output()); !strings.Contains(table, want) {
		t.Errorf("report is\n%s\nwant it to carry\n%s", table, want)
	}
}
