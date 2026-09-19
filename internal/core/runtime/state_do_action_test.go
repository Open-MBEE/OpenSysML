package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// zeroTicks seeds the counter the do actions below add to.
func zeroTicks() Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}}
}

// stepMachine advances the machine by one do round and, if one is queued, one
// event — the grain RunToCompletion runs at.
func stepMachine(t *testing.T, exec *StateExecutor) {
	t.Helper()
	if _, err := exec.runDoRound(); err != nil {
		t.Fatalf("do round: %v", err)
	}
	if exec.eventQueue.Len() > 0 {
		if err := exec.processNextEvent(); err != nil {
			t.Fatalf("process event: %v", err)
		}
	}
}

// bump is a do action that adds one to `ticks`.
func bump() ast.Node {
	return &ast.AssignmentActionNode{
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ticks"}}},
		Value: &ast.OperatorExpr{
			Operator: ast.OpAdd,
			Operands: []ast.Node{
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ticks"}}}},
				&ast.LiteralInteger{Value: "1"},
			},
		},
	}
}

func ticksAfter(t *testing.T, exec *StateExecutor) int64 {
	t.Helper()
	value, ok := exec.stateData["ticks"]
	if !ok {
		t.Fatal("the do behavior never ran")
	}
	if value.Kind != ValConst || value.Const.Kind != semantics.ValInt {
		t.Fatalf("ticks is %v, want an integer", value.Kind)
	}
	return value.Const.Int
}

// A do behavior runs while its state is active rather than at entry, so a
// transition out of the state cancels the actions it has not reached yet.
func TestDoBehaviorIsCancelledWhenItsStateIsExited(t *testing.T) {
	work := &ast.StateNode{Name: "work", Do: []ast.Node{bump(), bump(), bump(), bump()}}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			work,
			&ast.StateNode{Name: "done"},
			transitionMember("init", "work"),
			triggeredTransition("work", "done", "Stop"),
		},
	}

	exec := stateExecutorFor(t, machine)
	exec.stateData["ticks"] = zeroTicks()
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.SendSignal("Stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "init", "work", "done")
	if ticks := ticksAfter(t, exec); ticks >= 4 {
		t.Errorf("the do behavior ran to its end (%d actions) although its state was exited", ticks)
	}
	if exec.hasRunningDoAction(work) {
		t.Error("the do behavior of an exited state is still running")
	}
}

// A do behavior that ends without being interrupted runs every one of its
// actions, exactly once.
func TestUninterruptedDoBehaviorRunsEveryAction(t *testing.T) {
	work := &ast.StateNode{Name: "work", Do: []ast.Node{bump(), bump(), bump()}}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			work,
			&ast.StateNode{Name: "done"},
			transitionMember("init", "work"),
			transitionMember("work", "done"),
		},
	}

	exec := stateExecutorFor(t, machine)
	exec.stateData["ticks"] = zeroTicks()
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "init", "work", "done")
	if ticks := ticksAfter(t, exec); ticks != 3 {
		t.Errorf("do behavior ran %d actions, want 3", ticks)
	}
}

// The completion transition of a state with a do behavior only becomes enabled
// once that behavior has ended.
func TestCompletionWaitsForTheDoBehavior(t *testing.T) {
	work := &ast.StateNode{Name: "work", Do: []ast.Node{bump(), bump(), bump()}}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			work,
			&ast.StateNode{Name: "done"},
			transitionMember("init", "work"),
			transitionMember("work", "done"),
		},
	}

	exec := stateExecutorFor(t, machine)
	exec.stateData["ticks"] = zeroTicks()
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	for i := 0; i < 100 && !containsState(exec.stateVisits, "done"); i++ {
		stepMachine(t, exec)
	}
	if !containsState(exec.stateVisits, "done") {
		t.Fatal("the machine never completed")
	}
	if ticks := ticksAfter(t, exec); ticks != 3 {
		t.Errorf("completion fired after %d do actions, want 3", ticks)
	}
}

// Stepping the machine one event at a time drives a do behavior to its end:
// advancing it is progress even while no event is queued, which is the only way
// the completion transition it gates ever gets queued.
func TestSteppingDrivesADoBehaviorToItsEnd(t *testing.T) {
	work := &ast.StateNode{Name: "work", Do: []ast.Node{bump(), bump(), bump()}}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			entryStart("init"),
			&ast.StateNode{Name: "init"},
			work,
			&ast.StateNode{Name: "done"},
			transitionMember("init", "work"),
			transitionMember("work", "done"),
		},
	}

	exec := stateExecutorFor(t, machine)
	exec.stateData["ticks"] = zeroTicks()
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	for i := 0; i < 100 && exec.HasPendingWork(); i++ {
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	assertVisits(t, exec.stateVisits, "init", "work", "done")
	if ticks := ticksAfter(t, exec); ticks != 3 {
		t.Errorf("do behavior ran %d actions, want 3", ticks)
	}
}

// Do behaviors in orthogonal regions share the machine: neither region's
// behavior runs to its end before the other one starts.
func TestDoBehaviorsOfOrthogonalRegionsInterleave(t *testing.T) {
	lwork := &ast.StateNode{Name: "lwork", Do: []ast.Node{bump(), bump(), bump()}}
	rwork := &ast.StateNode{Name: "rwork", Do: []ast.Node{bump(), bump(), bump()}}
	machine := &ast.Usage{
		Kind:  ast.UsageState,
		Ident: ast.Identification{Name: "Machine"},
		Members: []ast.Node{
			&ast.StateRegion{Name: "left", States: []ast.Node{
				entryStart("lstart"),
				&ast.StateNode{Name: "lstart"},
				lwork,
				&ast.StateNode{Name: "ldone"},
				transitionMember("lstart", "lwork"),
				transitionMember("lwork", "ldone"),
			}},
			&ast.StateRegion{Name: "right", States: []ast.Node{
				entryStart("rstart"),
				&ast.StateNode{Name: "rstart"},
				rwork,
				&ast.StateNode{Name: "rdone"},
				transitionMember("rstart", "rwork"),
				transitionMember("rwork", "rdone"),
			}},
		},
	}

	exec := stateExecutorFor(t, machine)
	exec.stateData["ticks"] = zeroTicks()
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	var bothRan bool
	for i := 0; i < 100 && !containsState(exec.stateVisits, "rdone"); i++ {
		stepMachine(t, exec)
		if exec.hasRunningDoAction(lwork) && exec.hasRunningDoAction(rwork) {
			bothRan = true
		}
	}
	if !containsState(exec.stateVisits, "ldone") || !containsState(exec.stateVisits, "rdone") {
		t.Fatalf("both regions should have completed, visits: %v", exec.stateVisits)
	}
	if !bothRan {
		t.Error("the two do behaviors never ran concurrently")
	}
	if ticks := ticksAfter(t, exec); ticks != 6 {
		t.Errorf("do behaviors ran %d actions, want 6", ticks)
	}
}
