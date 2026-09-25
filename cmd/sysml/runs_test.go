package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// runsCLIModel declares what -runs runs: an action whose decision is weighted
// and whose wait is a random duration, on a random attribute of its own.
const runsCLIModel = `package MC {
    private import ScalarValues::*;
    private import ISQ::*;
    private import SI::*;
    private import Stochastic::*;
    private import RandomFunctions::*;
    action def Route {
        attribute taken : Integer = 0;
        attribute d : Real = uniform(0.0, 10.0);
        first start;
        then decide select;
        first select then fast { @Probability { p = 0.7; } }
        first select then slow { @Probability { p = 0.3; } }
        action fast { assign taken := 1; }
        then wait;
        action slow { assign taken := 2; }
        then wait;
        action wait accept after uniform(1, 80) [s];
        then done;
    }
    action route : Route;
}
`

// TestRunsThroughCLI checks the table -runs prints: one numbered row per run
// with the observables named, the same for the same seed and another for
// another, and each observable's distribution below it.
func TestRunsThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	made := func(seed string) string {
		got := check(t, binary, runsCLIModel, "-action", "MC::route", "-runs", "5", "-seed", seed, "-observe", "taken", "-observe", "clock")
		if got.status != 0 {
			t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
		}
		return sweepTable(got.output())
	}
	first, again, other := made("7"), made("7"), made("8")
	want := strings.Join([]string{
		"runs MC::route — 5 run(s), seed 7",
		"run | taken | clock                  | time",
		"-+-+-+-",
		"1   | 1     | 45.771104597451966 [s] | <time>",
		"2   | 1     | 18.029229676573745 [s] | <time>",
		"3   | 1     | 36.432448124734556 [s] | <time>",
		"4   | 2     | 35.38279454977086 [s]  | <time>",
		"5   | 1     | 33.085779305138026 [s] | <time>",
		"taken: 5 run(s), min 1, mean 1.2, max 2, p50 1, p90 2",
	}, "\n")
	if !strings.Contains(first, want) {
		t.Errorf("report is\n%s\nwant it to carry\n%s", first, want)
	}
	if !strings.Contains(first, "clock: 5 run(s), min 18.029229676573745 [s], mean 33.74027125073383 [s], max 45.771104597451966 [s], p50 35.38279454977086 [s], p90 45.771104597451966 [s]") {
		t.Errorf("report is\n%s\nwant the clock's distribution", first)
	}
	if first != again {
		t.Errorf("seed 7 made\n%s\nthen\n%s", first, again)
	}
	if first == other {
		t.Errorf("seed 8 made what seed 7 did:\n%s", first)
	}
}

// TestRunsDefaultObservablesThroughCLI checks that without -observe every
// feature the action holds is reported, in name order, then the clock.
func TestRunsDefaultObservablesThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, runsCLIModel, "-action", "MC::route", "-runs", "3", "-seed", "7")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	if !strings.Contains(got.output(), "run | d                  | taken | clock") {
		t.Errorf("report is\n%s\nwant every feature and the clock as columns", got.output())
	}
}

// TestRunsJSONThroughCLI checks the rows -json carries: the run number bound,
// the observables and the seed.
func TestRunsJSONThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, runsCLIModel, "-json", "-action", "MC::route", "-runs", "2", "-seed", "7", "-observe", "taken")
	if got.status != 0 {
		t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
	}
	var doc struct {
		Checks []struct {
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Values  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"values"`
			Rows []struct {
				Inputs []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"inputs"`
				Outputs []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"outputs"`
			} `json:"rows"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &doc); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, got.output())
	}
	if len(doc.Checks) != 1 || doc.Checks[0].Subject != "runs MC::route" || doc.Checks[0].Status != "holds" {
		t.Fatalf("checks are %+v; want one holding runs MC::route", doc.Checks)
	}
	rows := doc.Checks[0].Rows
	if len(rows) != 2 {
		t.Fatalf("rows are %+v; want two", rows)
	}
	for i, row := range rows {
		if len(row.Inputs) != 1 || row.Inputs[0].Name != "run" || row.Inputs[0].Value != strconv.Itoa(i+1) {
			t.Errorf("row %d binds %+v; want run=%d", i+1, row.Inputs, i+1)
		}
		if len(row.Outputs) != 1 || row.Outputs[0].Name != "taken" {
			t.Errorf("row %d reports %+v; want taken alone", i+1, row.Outputs)
		}
	}
	seeded := false
	for _, v := range doc.Checks[0].Values {
		seeded = seeded || v.Name == "seed" && v.Value == "7"
	}
	if !seeded {
		t.Errorf("values are %+v; want the seed among them", doc.Checks[0].Values)
	}
}

// TestRunsMisuseThroughCLI checks that flags no Monte Carlo follows from are
// refused before anything runs, each refusal naming what to write instead.
func TestRunsMisuseThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name  string
		args  []string
		wants string
	}{
		{"no action", []string{"-runs", "3", "-seed", "1"}, "-runs runs an action or a Simulation::MonteCarlo analysis case; name one, as -action <name> or -analysis <name>"},
		{"two actions", []string{"-action", "MC::route", "-action", "MC::route", "-runs", "3", "-seed", "1"},
			"name a single -action"},
		{"a state machine", []string{"-action", "MC::route", "-state", "MC::route", "-runs", "3", "-seed", "1"},
			"a state machine is run once"},
		{"no seed", []string{"-action", "MC::route", "-runs", "3"}, "-runs draws each run's randomness from a seed"},
		{"a sweep too", []string{"-action", "MC::route", "-runs", "3", "-seed", "1", "-calc", "MC::route", "-sweep", "n=1..2"},
			"-sweep and -samples run an analysis case or calc"},
		{"an advance too", []string{"-action", "MC::route", "-runs", "3", "-seed", "1", "-advance", "10"},
			"-advance runs it for a time"},
		{"observe without runs", []string{"-action", "MC::route", "-observe", "taken"}, "ask for the runs, as -runs <number>"},
		{"runs that are no number", []string{"-action", "MC::route", "-runs", "many", "-seed", "1"},
			"-runs takes the number of runs to make"},
		{"zero runs", []string{"-action", "MC::route", "-runs", "0", "-seed", "1"}, "-runs takes the number of runs to make"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := check(t, binary, runsCLIModel, tc.args...)
			if got.status != 2 {
				t.Errorf("exit status = %d, want 2\n%s", got.status, got.output())
			}
			if !strings.Contains(got.output(), tc.wants) {
				t.Errorf("report is\n%s\nwant it to name %q", got.output(), tc.wants)
			}
		})
	}
}

// TestSeedAloneSeedsOneRunThroughCLI checks that -seed without -runs seeds the
// model's draws of the one run made — its attribute, its weighted decision —
// whatever -schedule shuffles the tokens with, and that without any seed the run
// is refused naming the draw and the flag.
func TestSeedAloneSeedsOneRunThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	seeded := func(args ...string) string {
		got := check(t, binary, runsCLIModel, append([]string{"-action", "MC::route"}, args...)...)
		if got.status != 0 {
			t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
		}
		return got.output()
	}
	for _, tc := range []struct {
		args     []string
		d, taken string
	}{
		{[]string{"-seed", "7"}, "d = 7.74817894359002", "taken = 1"},
		{[]string{"-seed", "7", "-schedule", "seed:3"}, "d = 7.74817894359002", "taken = 1"},
		{[]string{"-seed", "7", "-schedule", "declared"}, "d = 7.74817894359002", "taken = 1"},
		{[]string{"-seed", "6"}, "d = 0.0035874273979208393", "taken = 2"},
	} {
		if out := seeded(tc.args...); !strings.Contains(out, tc.d) || !strings.Contains(out, tc.taken) {
			t.Errorf("%s ran\n%s\nwant %s and %s", strings.Join(tc.args, " "), out, tc.d, tc.taken)
		}
	}
	unseeded := check(t, binary, runsCLIModel, "-action", "MC::route")
	if unseeded.status != 2 {
		t.Errorf("exit status = %d, want 2\n%s", unseeded.status, unseeded.output())
	}
	if !strings.Contains(unseeded.output(), "modeled randomness needs a seed") || !strings.Contains(unseeded.output(), "-seed <n>") {
		t.Errorf("report is\n%s\nwant the draw refused for want of a seed, naming -seed", unseeded.output())
	}
}

// TestRunsRefusesAnUnheldObservableThroughCLI checks that an observable the
// action holds no feature by is a typed refusal that decides nothing.
func TestRunsRefusesAnUnheldObservableThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, runsCLIModel, "-action", "MC::route", "-runs", "2", "-seed", "7", "-observe", "nope")
	if got.status != 2 {
		t.Errorf("exit status = %d, want 2\n%s", got.status, got.output())
	}
	if !strings.Contains(got.output(), "no completed run of MC::route produced a value named nope") {
		t.Errorf("report is\n%s\nwant the observable refused", got.output())
	}
}

// TestClockStepThroughCLI checks -clock-step ticks the clock of every run: the
// waits of a -runs table come due on the ticks, 0 leaves it continuous, and a
// step that is no finite, non-negative number is refused at parse time.
func TestClockStepThroughCLI(t *testing.T) {
	binary := buildCLI(t)

	made := func(args ...string) string {
		got := check(t, binary, runsCLIModel, append([]string{"-action", "MC::route", "-runs", "2", "-seed", "7", "-observe", "clock"}, args...)...)
		if got.status != 0 {
			t.Fatalf("exit status = %d, want 0\n%s", got.status, got.output())
		}
		return sweepTable(got.output())
	}
	stepped := made("-clock-step", "1")
	for _, want := range []string{"| 46.0 [s]", "| 19.0 [s]"} {
		if !strings.Contains(stepped, want) {
			t.Errorf("under -clock-step 1 the report is\n%s\nwant it to carry %q", stepped, want)
		}
	}
	if continuous, plain := made("-clock-step", "0"), made(); continuous != plain {
		t.Errorf("-clock-step 0 made\n%s\nwant the continuous clock's\n%s", continuous, plain)
	}

	for _, tc := range []struct{ step, wants string }{
		{"-1", "invalid value \"-1\" for flag -clock-step: -clock-step: invalid clock step: a clock steps by a finite, non-negative number of seconds, not -1.0"},
		{"soon", "-clock-step: invalid clock step: \"soon\" is not a number of seconds"},
	} {
		got := check(t, binary, runsCLIModel, "-action", "MC::route", "-runs", "2", "-seed", "7", "-clock-step", tc.step)
		if got.status != 2 {
			t.Errorf("-clock-step %s: exit status = %d, want 2\n%s", tc.step, got.status, got.output())
		}
		if !strings.Contains(got.output(), tc.wants) {
			t.Errorf("-clock-step %s: report is\n%s\nwant it to name %q", tc.step, got.output(), tc.wants)
		}
	}
}
