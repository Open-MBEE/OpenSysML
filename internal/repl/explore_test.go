package repl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// exploreRaceSource forks three writers of one feature, which the library leaves
// unordered: six linearizations, three values of x.
const exploreRaceSource = `
package Race {
	private import ScalarValues::*;
	action race {
		attribute x : Integer = 0;
		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		action c { assign x := 3; }
		join sync;
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}
	action steady {
		attribute y : Integer = 0;
		first start;
		action only { assign y := 7; }
		done;
		succession first start then only;
		succession first only then done;
	}
	part def Holder {
		attribute x : Integer = 0;
	}
}
`

// RunAction under explore tables every distinct outcome, sorted, with how many
// linearizations reached it and one witness, then reports the exploration
// complete; the run holds.
func TestRunActionExploresEveryLinearization(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::race")
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v, want holds:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"),
		"✓ explored Race::race: 3 outcomes",
		"outcome | linearizations | witness",
		"x = 1   | 2              | step 3: 3@b first of 2@a, 3@b, 4@c; step 3: 4@c first of 2@a, 4@c",
		"x = 2   | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 3: 4@c first of 3@b, 4@c",
		"x = 3   | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 3: 3@b first of 3@b, 4@c",
		"complete (6 runs)")
	if len(v.Outcomes) != 3 || v.Exploration == nil || !v.Exploration.Complete || v.Exploration.Runs != 6 {
		t.Errorf("verdict outcomes = %+v, exploration = %+v", v.Outcomes, v.Exploration)
	}
	if got := v.Outcomes[0]; got.Linearizations != 2 || got.Error != "" || len(got.Values) != 1 ||
		got.Values[0].Name != "x" || got.Values[0].Value != "1" || len(got.Witness) != 2 {
		t.Errorf("first outcome = %+v", got)
	}
	// Exploring is deterministic: the same model tables the same rows.
	if again := s.RunAction("Race::race"); strings.Join(again.Lines, "\n") != strings.Join(v.Lines, "\n") {
		t.Errorf("exploration rendered\n%s\nthen\n%s", strings.Join(v.Lines, "\n"), strings.Join(again.Lines, "\n"))
	}
}

// A behavior with no choice point explores in a single run.
func TestRunActionExploresNoChoiceInOneRun(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::steady")
	wants(t, strings.Join(v.Lines, "\n"), "✓ explored Race::steady: 1 outcome", "y = 7   | 1              | no choice points", "complete (1 runs)")
}

// A budget hit leaves the verdict unresolved and names the budget; the outcomes
// reached within it are still tabled.
func TestRunActionExploreReportsTheBudgetHit(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	if err := s.SetSchedule(mustSchedule(t, "explore:runs=2")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::race")
	if v.Status != VerdictUnresolved {
		t.Errorf("status = %v, want unresolved", v.Status)
	}
	wants(t, strings.Join(v.Lines, "\n"), "? explored Race::race: 2 outcomes", "incomplete: runs budget 2 hit after 2 runs")
	if v.Exploration == nil || v.Exploration.Complete || strings.Join(v.Exploration.BudgetsHit, ",") != "runs" {
		t.Errorf("exploration = %+v", v.Exploration)
	}

	if err := s.SetSchedule(mustSchedule(t, "explore:depth=1,runs=100")); err != nil {
		t.Fatal(err)
	}
	v = s.RunAction("Race::race")
	wants(t, strings.Join(v.Lines, "\n"), "incomplete: depth budget 1 hit after 3 runs")
}

// With tracing on, the trace shown per outcome is its witness run's.
func TestRunActionExploreTracesTheWitnessOfEachOutcome(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	run(t, s, "%trace on")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	out := strings.Join(s.RunAction("Race::race").Lines, "\n")
	wantsInOrder(t, out,
		"complete (6 runs)",
		"trace of outcome 1's witness (run 4):",
		"x := 1 by token 2 stood",
		"took 3@b first",
		"trace of outcome 2's witness (run 2):",
		"x := 2 by token 3 stood",
		"trace of outcome 3's witness (run 1):",
		"x := 3 by token 4 stood")
	if strings.Count(out, "trace of outcome") != 3 {
		t.Errorf("want one trace per outcome:\n%s", out)
	}
}

// An action performed by an object explores on an object each run makes for
// itself; an object of the session cannot be named, since no run has it.
func TestRunActionExploreInstantiatesThePerformer(t *testing.T) {
	s := loadSource(t, exploreRaceSource)
	run(t, s, "%instantiate Race::Holder")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	v := s.RunAction("Race::race", "Race::Holder")
	wants(t, strings.Join(v.Lines, "\n"), "✓ explored Race::race: 3 outcomes", "complete (6 runs)")

	v = s.RunAction("Race::race", "#1")
	var explored *ExploredObjectError
	if v.Status != VerdictUnresolved || !strings.Contains(strings.Join(v.Lines, "\n"), (&ExploredObjectError{Ref: "#1"}).Error()) {
		t.Errorf("an object of the session was explored on:\n%s", strings.Join(v.Lines, "\n"))
	}
	if !errors.As(&ExploredObjectError{}, &explored) {
		t.Fatal("ExploredObjectError is not an error")
	}
}

// %schedule explore is a typed error at the prompt, leaving the policy in force,
// and a session set to explore programmatically refuses to start a debugger
// while still answering %schedule.
func TestSchedulingExploreAtThePromptIsRefused(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%schedule declared")
	out := run(t, s, "%schedule explore:runs=8")
	wants(t, out, "error: explore:runs=8 replays a behavior from the start once per linearization",
		"run `sysml -schedule explore:runs=8 -action <name>`")
	rejects(t, out, "schedule: ")
	wants(t, run(t, s, "%schedule"), "schedule: declared")

	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	wants(t, run(t, s, "%schedule"), "schedule: explore")
	wants(t, run(t, s, "%action tally"), "error: explore replays a behavior from the start")
	wants(t, run(t, s, "%state tally"), "error: explore replays a behavior from the start")
	if s.actionExec != nil || s.stateExec != nil {
		t.Error("a debugger started under explore")
	}
	var typed *ExploreAtPromptError
	if _, err := s.startAction("tally", nil); !errors.As(err, &typed) {
		t.Errorf("startAction under explore = %v, want *ExploreAtPromptError", err)
	}
}

// A session set to explore keeps its own context on the default policy, and a
// debugger running when explore is set is left alone.
func TestSetScheduleExploreLeavesTheSessionContextDriven(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%trace on")
	run(t, s, "%action tally")
	run(t, s, "%step")
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatal(err)
	}
	if got := s.rtCtx.Schedule(); got != runtime.DefaultSchedulePolicy {
		t.Errorf("session context schedule = %s, want the default", got)
	}
	run(t, s, "%step")
	wants(t, run(t, s, "%step"), "took 3@right first")
	wants(t, run(t, s, "%continue"), "✓ Action completed")
}

// Malformed explore spellings are refused with a typed error naming the option.
func TestSetScheduleRejectsMalformedExplore(t *testing.T) {
	for spelling, reason := range map[string]string{
		"explore:":                "explore: needs runs=<n> and/or depth=<d> after the colon, or no colon",
		"explore:bogus":           `explore option "bogus" is not runs=<n> or depth=<d>`,
		"explore:runs=0":          `explore runs "0" is not a decimal integer of at least 1`,
		"explore:depth=-1":        `explore depth "-1" is not a decimal integer of at least 0`,
		"explore:runs=2,runs=3":   "explore option runs is given twice",
		"explore:runs=x":          `explore runs "x" is not a decimal integer of at least 1`,
		"explores":                "want one of declared, reverse, seed:<n>, explore[:runs=<n>,depth=<d>]",
		"explore:runs=1,depth=1,": `explore option "" is not runs=<n> or depth=<d>`,
	} {
		_, err := runtime.ParseSchedulePolicy(spelling)
		var typed *runtime.SchedulePolicyError
		if !errors.As(err, &typed) {
			t.Errorf("%s: %v, want *SchedulePolicyError", spelling, err)
			continue
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("%s: %v, want %q", spelling, err, reason)
		}
		s := loadSource(t, choiceForkSource)
		wants(t, run(t, s, "%schedule "+spelling), "error: invalid scheduling policy \""+spelling+"\"")
	}
}
