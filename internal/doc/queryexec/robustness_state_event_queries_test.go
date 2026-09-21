package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// endedBody gives the queries one machine that runs to `done`, one that a
// `terminate` ends, and a calc destroying the terminator's object.
const endedBody = `
private import OccurrenceFunctions::*;
attribute def Abort;
state def FinishingMachine {
	entry; then working;
	state working;
	transition first working accept Abort then done;
}
state def TerminatingMachine {
	entry; then busy;
	state busy;
	transition first busy accept Abort then stop;
	action stop terminate;
}
part def Finisher { exhibit state fm : FinishingMachine; }
part def Terminator { exhibit state tm : TerminatingMachine; }
part f : Finisher;
part t : Terminator;
calc def DestroyTerminator { in x : Terminator; return : Terminator = destroy(x); }
`

const endedQueries = `
calc def Where :> Query {
	in root : Element;
	Project(source = States(source = root), properties = ("machine", "statePath"))
}
calc def Busy :> Query {
	InState(name = "busy")
}
calc def Happenings :> Query {
	in root : Element;
	Project(source = Events(source = root), properties = ("kind", "state", "from", "to"))
}
`

// endedFixture holds the two machines' objects over a traced runtime.
type endedFixture struct {
	executionFixture
	ctx      *runtime.Context
	finisher *runtime.Instance
	termin   *runtime.Instance
}

func loadEndedFixture(t *testing.T) endedFixture {
	t.Helper()
	fixture := loadExecutionFixture(t, endedBody+endedQueries)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	ctx.SetTrace(runtime.NewTraceRecorder())
	f := endedFixture{executionFixture: fixture, ctx: ctx}
	f.finisher = f.instantiate(t, "f")
	f.termin = f.instantiate(t, "t")
	return f
}

func (f endedFixture) instantiate(t *testing.T, name string) *runtime.Instance {
	t.Helper()
	inst, err := f.ctx.Instantiate(f.symbol(t, name))
	if err != nil {
		t.Fatalf("Instantiate %s: %v", name, err)
	}
	return inst
}

func (f endedFixture) send(t *testing.T, inst *runtime.Instance, signal string) {
	t.Helper()
	msg, err := f.ctx.SignalMessage(f.symbol(t, signal), nil, inst)
	if err != nil {
		t.Fatalf("send %s: %v", signal, err)
	}
	f.ctx.PostMessage(msg)
}

func (f endedFixture) advance(t *testing.T, seconds float64) {
	t.Helper()
	if _, err := f.ctx.Advance(seconds); err != nil {
		t.Fatalf("advance %v: %v", seconds, err)
	}
}

// destroy runs `destroy(x)` on inst through the calc the fixture declares.
func (f endedFixture) destroy(t *testing.T, inst *runtime.Instance) {
	t.Helper()
	sym := f.symbol(t, "DestroyTerminator")
	arg := runtime.Value{Kind: runtime.ValInstance, Instance: inst.ID}
	if _, err := f.ctx.InvokeCalc(sym, []runtime.Value{arg}, sym.Scope); err != nil {
		t.Fatalf("destroy: %v", err)
	}
}

func (f endedFixture) context() Context {
	return Context{
		Index: f.index, Resolver: f.resolver, Model: f.model, Runtime: f.ctx,
		Roots: []Root{{Label: "f", Object: f.finisher}, {Label: "t", Object: f.termin}},
	}
}

func (f endedFixture) run(t *testing.T, name string, bindings Bindings) (*RowSet, error) {
	t.Helper()
	return Execute(f.program(t, name), f.context(), bindings, Options{})
}

func (f endedFixture) on(inst *runtime.Instance, label string) Bindings {
	return Bindings{"root": {ObjectValue(inst, label)}}
}

// TestQueryRobustnessStateEventQueries exercises state and event queries at a
// machine that is over: terminated reads as no active state, completed keeps
// its final state, and a destroyed object is a typed refusal, never a panic.
func TestQueryRobustnessStateEventQueries(t *testing.T) {
	t.Run("states_of_a_terminated_machine_are_empty", testStatesOfTerminatedAreEmpty)
	t.Run("states_of_a_completed_machine_report_its_final_state", testStatesOfCompletedReportFinal)
	t.Run("states_of_a_destroyed_object", testStatesOfDestroyed)
	t.Run("events_of_a_destroyed_object", testEventsOfDestroyed)
	t.Run("in_state_over_a_terminated_machine", testInStateOverTerminated)
	t.Run("events_after_termination_still_read", testEventsAfterTermination)
}

// A terminated machine holds no active configuration: States answers no row
// for its object rather than an error or a stale leaf.
func testStatesOfTerminatedAreEmpty(t *testing.T) {
	f := loadEndedFixture(t)
	f.send(t, f.termin, "Abort")
	f.advance(t, 1)
	result, err := f.run(t, "Where", f.on(f.termin, "t"))
	if err != nil {
		t.Fatalf("States over a terminated machine: %v", err)
	}
	if got := len(result.Rows()); got != 0 {
		t.Fatalf("rows = %d, want 0: %s", got, joinLines(rowTexts(t, result)))
	}
}

// A completed machine reports its final state: `done` answers as a leaf row.
func testStatesOfCompletedReportFinal(t *testing.T) {
	f := loadEndedFixture(t)
	f.send(t, f.finisher, "Abort")
	f.advance(t, 1)
	result, err := f.run(t, "Where", f.on(f.finisher, "f"))
	if err != nil {
		t.Fatalf("States over a completed machine: %v", err)
	}
	got := rowTexts(t, result)
	if len(got) != 1 || !strings.Contains(got[0], "statePath=done") {
		t.Fatalf("rows = %v, want one row in done", got)
	}
}

// States over an object the run destroyed is a typed refusal naming the object
// and the instant it ended, not a stale row.
func testStatesOfDestroyed(t *testing.T) {
	f := loadEndedFixture(t)
	f.destroy(t, f.termin)
	_, err := f.run(t, "Where", f.on(f.termin, "t"))
	got := executionError(t, err, ErrorObjectDestroyed)
	if got.Target != "t" || !strings.Contains(got.Error(), "t, an object destroyed at") {
		t.Fatalf("object-destroyed error = %v", got)
	}
}

// Events refuses a destroyed source the same way, through the same argument.
func testEventsOfDestroyed(t *testing.T) {
	f := loadEndedFixture(t)
	f.send(t, f.termin, "Abort")
	f.advance(t, 1)
	f.destroy(t, f.termin)
	_, err := f.run(t, "Happenings", f.on(f.termin, "t"))
	if got := executionError(t, err, ErrorObjectDestroyed); got.Target != "t" {
		t.Fatalf("object-destroyed error = %v", got)
	}
}

// InState over a terminated machine answers nothing: `busy` is still declared,
// so there is no unknown-state error and no matching row.
func testInStateOverTerminated(t *testing.T) {
	f := loadEndedFixture(t)
	f.send(t, f.termin, "Abort")
	f.advance(t, 1)
	result, err := f.run(t, "Busy", nil)
	if err != nil {
		t.Fatalf("InState over a terminated machine: %v", err)
	}
	if got := len(result.Rows()); got != 0 {
		t.Fatalf("rows = %d, want 0", got)
	}
}

// Termination leaves the recorded run readable: the accept, the exit of its
// source and the transition are still event rows.
func testEventsAfterTermination(t *testing.T) {
	f := loadEndedFixture(t)
	f.send(t, f.termin, "Abort")
	f.advance(t, 1)
	result, err := f.run(t, "Happenings", f.on(f.termin, "t"))
	if err != nil {
		t.Fatalf("Events after termination: %v", err)
	}
	got := joinLines(rowTexts(t, result))
	for _, want := range []string{"kind=accept", "kind=exit state=busy", "kind=transition state= from=busy to=stop"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rows after termination =\n%s\nwant a row containing %q", got, want)
		}
	}
}
