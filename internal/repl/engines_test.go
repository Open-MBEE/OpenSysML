package repl

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/enginewire"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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
		"check", "explore", "run", "smt", "solve", "sweep")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 7 {
		t.Fatalf("expected a header and six engines, got:\n%s", out)
	}
	wants(t, lines[1], "check", "bounded", "outcomes, holds", "ready")
	wants(t, lines[2], "explore", "proved", "outcomes", "ready")
	wants(t, lines[3], "run", "observed", "evaluate", "ready")
	wants(t, lines[4], "smt", "proved", "holds")
	wants(t, lines[5], "solve", "proved", "satisfiable")
	wants(t, lines[6], "sweep", "observed", "sweep", "ready")
}

// manifestEngine stands in for an engine registered from a manifest: it counts the probes
// made of it and reports its origin as -engines prints it.
type manifestEngine struct{ probed *int }

func (manifestEngine) Name() string { return "standin" }

func (manifestEngine) Describe() analysis.Description {
	return analysis.Description{Questions: []analysis.Kind{analysis.Holds}, Authority: analysis.Bounded, Process: "/opt/standin/bin/standin"}
}

func (manifestEngine) Covers(*analysis.Model, analysis.Question) analysis.Coverage {
	return analysis.Coverage{Covered: true}
}

func (manifestEngine) Run(context.Context, *analysis.Model, analysis.Question, analysis.Budget) (analysis.Result, error) {
	return analysis.Result{}, errors.New("not run by this test")
}

func (manifestEngine) Process() (string, error) {
	return "standin 1.0.0 at /opt/standin/bin/standin", nil
}

func (e manifestEngine) Probe() (string, error) {
	*e.probed++
	return "standin 1.0.0 at /opt/standin/bin/standin; describe agrees", nil
}

func (manifestEngine) Origin() analysis.Origin {
	return analysis.Origin{Kind: analysis.KindEngine, Version: "1.0.0", File: "/etc/opensysml/engines/standin.json",
		Command: "/opt/standin/bin/standin", Transport: analysis.TransportStdio, Protocol: 1}
}

// %engines lists a manifest engine with its kind, protocol and origin without probing it;
// %engines probe probes each external engine once; any other argument is refused.
func TestEnginesListsManifestEnginesAndProbesOnRequest(t *testing.T) {
	s := loadSource(t, engineCalcSource)
	probed := 0
	engines := analysis.Default()
	if err := engines.Register(manifestEngine{probed: &probed}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.SetEngines(engines); err != nil {
		t.Fatalf("SetEngines: %v", err)
	}

	out := run(t, s, "%engines")
	wantsInOrder(t, out, "engine", "kind", "protocol", "authority", "answers", "status",
		"check    built-in  -", "standin  engine    stdio/1   bounded    holds", "ready (standin 1.0.0 at /opt/standin/bin/standin)",
		"standin 1.0.0: engine from /etc/opensysml/engines/standin.json, runs /opt/standin/bin/standin, not admitted")
	rejects(t, out, "describe agrees")
	if probed != 0 {
		t.Fatalf("%%engines probed %d times", probed)
	}

	out = run(t, s, "%engines probe")
	wants(t, out, "ready (standin 1.0.0 at /opt/standin/bin/standin; describe agrees)")
	if probed != 1 {
		t.Fatalf("%%engines probe probed %d times, want once", probed)
	}

	out = run(t, s, "%engines all")
	wants(t, out, "error: %engines takes `probe` or nothing, not \"all\"")
	if !errors.Is(&EnginesArgumentError{Args: []string{"all"}}, ErrEnginesArgument) {
		t.Fatal("EnginesArgumentError does not match ErrEnginesArgument")
	}
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
	wants(t, out, `error: analysis: no engine named "nope"`, "check, explore, run, smt, solve, sweep, or auto, or all")
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

// toolActionSource is an action a tool computes, with a default for its one input so the
// debugger can start it bare.
const toolActionSource = `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	action def Doubling {
		metadata ToolExecution { toolName = "MC"; uri = "u"; }
		in k : Real = 2.0 { @ToolVariable { name = "k"; } }
		out y : Real     { @ToolVariable { name = "y"; } }
	}
}
`

// computeEngine stands in for a tool's engine: it answers every Compute for its tool with
// y = 2k, observed.
type computeEngine struct{ ran *int }

func (computeEngine) Name() string { return analysis.ToolEngineName("MC") }

func (computeEngine) Describe() analysis.Description {
	return analysis.Description{Questions: []analysis.Kind{analysis.Compute}, Authority: analysis.Observed}
}

func (computeEngine) Covers(*analysis.Model, analysis.Question) analysis.Coverage {
	return analysis.Coverage{Covered: true}
}

func (e computeEngine) Run(_ context.Context, _ *analysis.Model, q analysis.Question, _ analysis.Budget) (analysis.Result, error) {
	*e.ran++
	k := q.Compute.Call.Inputs[0].Value.Value.Real
	value := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 2 * k}}
	return analysis.Result{Claim: analysis.ClaimValue, Strength: analysis.Observed,
		Values: []analysis.Evaluation{{Name: "y", Value: value}}}, nil
}

// %action on a tool-computed action puts the computation to the session's engines under its
// selection, as a run does: the executor starts completed with the tool's outputs; without
// an engine for the tool, or under a named engine that does not compute, it does not start.
func TestActionDebuggerPutsToolComputationsToTheEngines(t *testing.T) {
	s := loadSource(t, toolActionSource)
	out := run(t, s, "%action Tools::Doubling")
	wants(t, out, "error: failed to create executor:", "tool 'MC' is not registered; set OPENSYSML_TOOLS")
	if s.actionExec != nil {
		t.Fatal("a refused %action left a debugging session")
	}

	ran := 0
	engines := analysis.Default()
	if err := engines.Register(computeEngine{ran: &ran}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.SetEngines(engines); err != nil {
		t.Fatalf("SetEngines: %v", err)
	}
	wants(t, run(t, s, "%action Tools::Doubling"), "✓ Started action executor", "State: Completed", "Tokens: 0")
	wants(t, run(t, s, "%continue"), "✓ Action already completed")
	if ran != 1 || s.actionExec == nil || s.actionExec.executor.State() != runtime.StateCompleted {
		t.Fatalf("tool ran %d times; want once, leaving a completed executor", ran)
	}
	if got := runtime.FormatValue(s.actionExec.executor.Results()["y"]); got != "4.0" {
		t.Fatalf("y = %s, want 4.0", got)
	}

	run(t, s, "%engine run")
	out = run(t, s, "%action Tools::Doubling")
	wants(t, out, "error: failed to create executor:", "run does not answer compute questions")
	if ran != 1 {
		t.Fatalf("the tool ran %d times under the run engine alone, want once in all", ran)
	}
}

// reportingEngine stands in for an external engine: it reports progress through the plan's
// reporter before answering, and records whether the plan installed one.
type reportingEngine struct{ reported *bool }

func (reportingEngine) Name() string { return "reporting" }

func (reportingEngine) Describe() analysis.Description {
	return analysis.Description{Questions: []analysis.Kind{analysis.Compute}, Authority: analysis.Observed}
}

func (reportingEngine) Covers(*analysis.Model, analysis.Question) analysis.Coverage {
	return analysis.Coverage{Covered: true}
}

func (e reportingEngine) Run(ctx context.Context, _ *analysis.Model, q analysis.Question, _ analysis.Budget) (analysis.Result, error) {
	if report := analysis.ReporterFrom(ctx); report != nil {
		*e.reported = true
		report(analysis.ProgressReport{Engine: "reporting", Progress: enginewire.ProgressParams{Runs: 3, Depth: 2, Text: "unrolling"}})
	}
	k := q.Compute.Call.Inputs[0].Value.Value.Real
	value := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 2 * k}}
	return analysis.Result{Claim: analysis.ClaimValue, Strength: analysis.Observed,
		Values: []analysis.Evaluation{{Name: "y", Value: value}}}, nil
}

// A session given a progress writer runs its plans under a reporter that prints what an
// engine reports, one line naming the engine; without one the plans carry no reporter.
func TestProgressIsPrintedWhereTheSessionSays(t *testing.T) {
	s := loadSource(t, toolActionSource)
	reported := false
	engines := analysis.Default()
	if err := engines.Register(reportingEngine{reported: &reported}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.SetEngines(engines); err != nil {
		t.Fatalf("SetEngines: %v", err)
	}
	run(t, s, "%engine reporting")

	wants(t, run(t, s, "%action Tools::Doubling"), "✓ Started action executor")
	if reported {
		t.Fatal("a session printing no progress installed a reporter")
	}

	var progress strings.Builder
	s.SetProgress(&progress)
	wants(t, run(t, s, "%action Tools::Doubling"), "✓ Started action executor")
	if !reported {
		t.Fatal("the plan ran without the session's reporter")
	}
	if got := progress.String(); got != "engine reporting: runs 3, depth 2: unrolling\n" {
		t.Fatalf("progress printed %q", got)
	}

	s.SetProgress(nil)
	reported = false
	wants(t, run(t, s, "%action Tools::Doubling"), "✓ Started action executor")
	if reported {
		t.Fatal("SetProgress(nil) left the reporter installed")
	}
}
