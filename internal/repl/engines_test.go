package repl

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
)

// engineCalcSource is a calc the prompt can put to the engines.
const engineCalcSource = `
package Engines {
	private import ScalarValues::*;
	calc def Twice { in n : Integer; return : Integer = n * 2; }
}
`

// %engines tables every registered engine in name order with its authority, the
// questions it answers and the state of its process.
func TestEnginesListsEveryRegisteredEngine(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	out := run(t, s, "%engines")
	wantsInOrder(t, out, "engine", "authority", "answers", "status",
		"explore", "run", "solve", "sweep")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected a header and four engines, got:\n%s", out)
	}
	wants(t, lines[1], "explore", "proved", "outcomes", "ready")
	wants(t, lines[2], "run", "observed", "evaluate", "ready")
	wants(t, lines[3], "solve", "proved", "satisfiable")
	wants(t, lines[4], "sweep", "observed", "sweep", "ready")
}

// %engine shows the selection, sets it by name, to auto or to all, and reports a
// name no engine carries as a typed error that leaves the selection in force.
func TestEngineShowsAndSetsTheSelection(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	wants(t, run(t, s, "%engine"), "engine: auto")
	wants(t, run(t, s, "%engine run"), "engine: run")
	if got := s.Engine(); got != analysis.Only("run") {
		t.Fatalf("Engine() = %v, want run", got)
	}
	out := run(t, s, "%engine nope")
	wants(t, out, `error: analysis: no engine named "nope"`, "explore, run, solve, sweep, or auto, or all")
	rejects(t, out, "engine: ")
	wants(t, run(t, s, "%engine"), "engine: run")
	wants(t, run(t, s, "%engine all"), "engine: all")
	wants(t, run(t, s, "%engine auto"), "engine: auto")

	var unknown *analysis.UnknownEngineError
	if err := s.SetEngine("nope"); !errors.As(err, &unknown) || unknown.Name != "nope" {
		t.Fatalf("SetEngine(nope) = %v, want UnknownEngineError", err)
	}
}

// %engine explore is a synonym of %schedule explore, which the prompt refuses:
// its debuggers step one run. The selection and the schedule stay as they were.
func TestEngineExploreAtThePromptIsRefused(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	run(t, s, "%schedule reverse")
	out := run(t, s, "%engine explore")
	wants(t, out, "error: explore replays a behavior from the start once per linearization",
		"run `sysml -schedule explore -action <name>`")
	rejects(t, out, "engine: ")
	wants(t, run(t, s, "%engine"), "engine: auto")
	wants(t, run(t, s, "%schedule"), "schedule: reverse")
}

// Selecting an engine leaves the schedule alone, and setting a schedule leaves
// the engine alone: the two are independent settings of the session.
func TestEngineSelectionIsIndependentOfTheSchedule(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	run(t, s, "%engine run")
	run(t, s, "%schedule declared")
	wants(t, run(t, s, "%engine"), "engine: run")
	wants(t, run(t, s, "%schedule"), "schedule: declared")
	v := s.RunCalc("Engines::Twice(3)")
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	wants(t, strings.Join(v.Lines, "\n"), standingPrefix+"value (observed: 1 run under declared)")
}

// Every verdict an engine answered ends with its standing line and carries the
// plan that answered it, under auto and under a named engine alike.
func TestVerdictsCarryTheirPlanAndStanding(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	v := s.RunCalc("Engines::Twice(3)")
	if v.Plan == nil {
		t.Fatalf("verdict carries no plan:\n%s", strings.Join(v.Lines, "\n"))
	}
	if v.Plan.Selection != analysis.Auto() || len(v.Plan.Steps) != 1 || v.Plan.Steps[0].Engine != "run" {
		t.Fatalf("plan = %+v, want one run step under auto", v.Plan)
	}
	last := v.Lines[len(v.Lines)-1]
	if last != standingPrefix+"value (observed: 1 run under reverse)" {
		t.Fatalf("last line = %q, want the standing", last)
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "= 6", "standing: value (observed: 1 run under reverse)")

	run(t, s, "%engine run")
	v = s.RunCalc("Engines::Twice(4)")
	if v.Plan == nil || v.Plan.Selection != analysis.Only("run") {
		t.Fatalf("plan = %+v, want the named run engine", v.Plan)
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "= 8", standingPrefix+"value (observed: 1 run under reverse)")
}

// A named engine that refuses the question is final: the verdict is its refusal
// with the plan recording it, and no other engine is tried.
func TestNamedEngineRefusalIsFinal(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	run(t, s, "%engine sweep")
	v := s.RunCalc("Engines::Twice(3)")
	if v.Status != VerdictUnresolved {
		t.Fatalf("status = %v, want unresolved:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	report := strings.Join(v.Lines, "\n")
	wants(t, report, "sweep does not answer evaluate questions", standingPrefix+"not covered")
	rejects(t, report, "= 6")
	if v.Plan == nil || len(v.Plan.Steps) != 1 || v.Plan.Steps[0].Engine != "sweep" || v.Plan.Steps[0].Refusal == nil {
		t.Fatalf("plan = %+v, want one refused sweep step", v.Plan)
	}
}

// all puts the question to every covering engine, in name order, and the
// standing lists each one's part.
func TestAllRunsEveryCoveringEngine(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	run(t, s, "%engine all")
	v := s.RunCalc("Engines::Twice(3)")
	if v.Status != VerdictHolds {
		t.Fatalf("status = %v:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	if v.Plan == nil || v.Plan.Selection != analysis.All() {
		t.Fatalf("plan = %+v, want all", v.Plan)
	}
	var ran []string
	for _, step := range v.Plan.Steps {
		if step.Result != nil {
			ran = append(ran, step.Engine)
		}
	}
	if len(ran) != 1 || ran[0] != "run" {
		t.Fatalf("engines that ran = %v, want run alone to cover an evaluate question", ran)
	}
	wantsInOrder(t, strings.Join(v.Lines, "\n"), "= 6", standingPrefix+"value (observed: 1 run under reverse); all: run value (observed)")
}

// A debugger stepping a run survives an engine selection, which decides how
// questions asked from here on are answered, not what is running.
func TestEngineSelectionKeepsTheDebuggerSession(t *testing.T) {
	s := loadSource(t, choiceForkSource)
	run(t, s, "%action tally")
	if s.actionExec == nil {
		t.Fatal("no debugger session started")
	}
	wants(t, run(t, s, "%engine run"), "engine: run")
	if s.actionExec == nil {
		t.Fatal("selecting an engine ended the debugger session")
	}
	wants(t, run(t, s, "%engine auto"), "engine: auto")
	if s.actionExec == nil {
		t.Fatal("selecting auto ended the debugger session")
	}
}
