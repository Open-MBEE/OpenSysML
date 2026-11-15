package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// coverageModel is a machine whose two orders reach one outcome through viaA
// and through viaB: the outcome's witness enters viaA, so only the coverage
// over every run tells that viaB was entered.
const coverageModel = `package Cov {
	private import ScalarValues::*;
	part def M {
		exhibit state run {
			entry; then s0;
			state s0;
			transition first s0 then viaA;
			transition first s0 then viaB;
			state viaA;
			transition first viaA then fin;
			state viaB;
			transition first viaB then fin;
			state fin;
		}
	}
	part def Sys {
		part m : M;
		exhibit state judge {
			entry; then watching;
			state watching;
		}
	}
	part sys : Sys;
}`

func TestExploreReportsCoverageOverEveryRun(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, coverageModel, "-schedule", "explore:runs=10,depth=50", "-state", "Cov::Sys::judge Cov::sys", "-advance", "5")
	if got.status != 0 {
		t.Fatalf("status = %d\n%s", got.status, got.output())
	}
	for _, want := range []string{
		"complete (2 runs)\ncoverage over all runs: 5 states entered, 4 transitions taken\n",
		"  state Cov::sys#1.m.run: viaA (Cov::M::run::viaA): 1 run\n",
		"  state Cov::sys#1.m.run: viaB (Cov::M::run::viaB): 1 run\n",
		"  state Cov::sys#1.m.run: s0 (Cov::M::run::s0): 2 runs\n",
		"  state Cov::sys#1.judge: watching (Cov::Sys::judge::watching): 2 runs\n",
		"  transition Cov::sys#1.m.run: s0 -> viaB (Cov::M::run::s0 -> viaB): 1 run\n",
		"  transition Cov::sys#1.m.run: viaB -> fin (Cov::M::run::viaB -> fin): 1 run\n",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout lacks %q\n%s", want, got.output())
		}
	}
}

type coverageJSON struct {
	States      []coverageEntryJSON `json:"states"`
	Transitions []coverageEntryJSON `json:"transitions"`
}

type coverageEntryJSON struct {
	Path        string `json:"path"`
	Declaration string `json:"declaration"`
	Runs        int    `json:"runs"`
}

func (c coverageJSON) entry(path string) (coverageEntryJSON, bool) {
	for _, e := range append(c.States, c.Transitions...) {
		if e.Path == path {
			return e, true
		}
	}
	return coverageEntryJSON{}, false
}

func TestJSONReportsExplorationCoverage(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, coverageModel, "-json", "-schedule", "explore:runs=10,depth=50", "-state", "Cov::Sys::judge Cov::sys", "-advance", "5")
	var report struct {
		Checks []struct {
			Exploration struct {
				Runs     int          `json:"runs"`
				Coverage coverageJSON `json:"coverage"`
			} `json:"exploration"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if got.status != 0 || len(report.Checks) != 1 || report.Checks[0].Exploration.Runs != 2 {
		t.Fatalf("status = %d, checks = %d\n%s", got.status, len(report.Checks), got.output())
	}
	assertCoverage(t, report.Checks[0].Exploration.Coverage, got.output())
}

func TestJSONReportsCheckCoverage(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, coverageModel, "-json", "-engine", "check", "-state", "Cov::Sys::judge Cov::sys", "-advance", "5")
	var report struct {
		Checks []struct {
			Results []struct {
				Check struct {
					Coverage coverageJSON `json:"coverage"`
				} `json:"check"`
			} `json:"results"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if got.status != 0 || len(report.Checks) != 1 || len(report.Checks[0].Results) != 1 {
		t.Fatalf("status = %d, checks = %d\n%s", got.status, len(report.Checks), got.output())
	}
	assertCoverage(t, report.Checks[0].Results[0].Check.Coverage, got.output())

	text := check(t, binary, coverageModel, "-engine", "check", "-state", "Cov::Sys::judge Cov::sys", "-advance", "5")
	for _, want := range []string{
		"\n  coverage over all schedules: 5 states entered, 4 transitions taken\n",
		"\n    state Cov::sys#1.m.run: viaB (Cov::M::run::viaB): 1 schedule\n",
		"\n    transition Cov::sys#1.m.run: s0 -> viaA (Cov::M::run::s0 -> viaA): 1 schedule\n",
	} {
		if !strings.Contains(text.stdout, want) {
			t.Errorf("stdout lacks %q\n%s", want, text.output())
		}
	}
}

// assertCoverage checks the coverage of the model's two runs: viaB entered by
// the run that is not the witness, every shared state by both.
func assertCoverage(t *testing.T, c coverageJSON, output string) {
	t.Helper()
	if len(c.States) != 5 || len(c.Transitions) != 4 {
		t.Fatalf("coverage has %d states and %d transitions, want 5 and 4\n%s", len(c.States), len(c.Transitions), output)
	}
	for path, want := range map[string]coverageEntryJSON{
		"Cov::sys#1.m.run: viaB":        {Path: "Cov::sys#1.m.run: viaB", Declaration: "Cov::M::run::viaB", Runs: 1},
		"Cov::sys#1.m.run: viaA":        {Path: "Cov::sys#1.m.run: viaA", Declaration: "Cov::M::run::viaA", Runs: 1},
		"Cov::sys#1.m.run: fin":         {Path: "Cov::sys#1.m.run: fin", Declaration: "Cov::M::run::fin", Runs: 2},
		"Cov::sys#1.judge: watching":    {Path: "Cov::sys#1.judge: watching", Declaration: "Cov::Sys::judge::watching", Runs: 2},
		"Cov::sys#1.m.run: s0 -> viaB":  {Path: "Cov::sys#1.m.run: s0 -> viaB", Declaration: "Cov::M::run::s0 -> viaB", Runs: 1},
		"Cov::sys#1.m.run: viaA -> fin": {Path: "Cov::sys#1.m.run: viaA -> fin", Declaration: "Cov::M::run::viaA -> fin", Runs: 1},
	} {
		got, ok := c.entry(path)
		if !ok || got != want {
			t.Errorf("coverage of %s = %+v, want %+v\n%s", path, got, want, output)
		}
	}
}
