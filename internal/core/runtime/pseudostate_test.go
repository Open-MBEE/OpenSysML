package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const forkJoinMachine = `package test {
    state Machine {
        attribute leftRan : Integer = 0;
        attribute rightRan : Integer = 0;
        attribute merged : Integer = 0;

        entry; then init;
        state init;
        state idle;
        state running parallel {
            state left {
                entry; then lstart;
                state lstart;
                state working { entry { assign leftRan := 1; } }
                succession first lstart then working;
            }
            state right {
                entry; then rstart;
                state rstart;
                state watching { entry { assign rightRan := 1; } }
                succession first rstart then watching;
            }
        }
        fork split;
        join sync;

        succession first init then idle;
        transition first idle then split;
        transition first split then working;
        transition first split then watching;
        transition first working then sync;
        transition first watching then sync;
        transition first sync then done;
    }
}`

// A fork makes one state active per orthogonal region; the join only releases
// once every branch has arrived.
func TestForkAndJoinPseudostates(t *testing.T) {
	ctx, machine := loadState(t, forkJoinMachine, "Machine")

	data, visited, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}

	for _, name := range []string{"leftRan", "rightRan"} {
		if got := intValue(t, data, name); got != 1 {
			t.Errorf("%s = %d, want 1 (fork branch entered)", name, got)
		}
	}
	for _, want := range []string{"working", "watching", "done"} {
		if !containsState(visited, want) {
			t.Errorf("state %q not visited, visits: %v", want, visited)
		}
	}
}

func TestForkBranchesMustBeInDistinctRegions(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    state Machine {
        entry; then init;
        state init;
        state idle;
        state running parallel {
            state left {
                entry; then lstart;
                state lstart;
                state working;
                state alsoWorking;
                succession first lstart then working;
            }
            state right {
                entry; then rstart;
                state rstart;
                state watching;
                succession first rstart then watching;
            }
        }
        fork split;

        succession first init then idle;
        transition first idle then split;
        transition first split then working;
        transition first split then alsoWorking;
    }
}`, "Machine")

	_, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err == nil || !strings.Contains(err.Error(), "same region") {
		t.Fatalf("error = %v, want branches in the same region to be rejected", err)
	}
}

func TestJoinWaitsForEveryBranch(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    state Machine {
        entry; then init;
        state init;
        state idle;
        state running parallel {
            state left {
                entry; then lstart;
                state lstart;
                state working;
                succession first lstart then working;
            }
            state right {
                entry; then rstart;
                state rstart;
                state watching;
                state stillWatching;
                succession first rstart then watching;
                transition first watching then stillWatching;
            }
        }
        fork split;
        join sync;

        succession first init then idle;
        transition first idle then split;
        transition first split then working;
        transition first split then watching;
        transition first working then sync;
        transition first stillWatching then sync;
        transition first sync then done;
    }
}`, "Machine")

	_, visited, err := ctx.ExecuteStateWithEvents(machine, nil)
	if err != nil {
		t.Fatalf("ExecuteStateWithEvents: %v", err)
	}
	// The left branch reaches the join first and must wait for the right branch
	// to move through stillWatching before done is entered.
	for _, want := range []string{"working", "watching", "stillWatching", "done"} {
		if !containsState(visited, want) {
			t.Fatalf("state %q not visited, visits: %v", want, visited)
		}
	}
	if indexOfState(visited, "stillWatching") > indexOfState(visited, "done") {
		t.Errorf("join released before the right branch arrived, visits: %v", visited)
	}
}

// A transition into a join is not enabled until every branch has arrived: an
// event reaching one branch first fires nothing, Decide and LastDispatch say so,
// and the join fires on the event that completes the last branch.
func TestJoinBranchArrivingFirstFiresNothing(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    attribute def Go;
    state Machine {
        entry; then running;
        state running parallel {
            state left {
                entry; then a;
                state a;
            }
            state right {
                entry; then b0;
                state b0;
                state b;
                transition first b0 accept after 2 then b;
            }
        }
        join sync;
        transition first a accept Go then sync;
        transition first b then sync;
        transition first sync then done;
    }
}`, "Machine")
	exec, err := ctx.CreateStateExecutor(machine)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	goMsg := Message{SignalType: "Go"}
	if accepted, err := exec.AcceptsMessage(goMsg); err != nil || !accepted {
		t.Fatalf("AcceptsMessage(Go) = %v, %v; want a in the left region to accept it", accepted, err)
	}
	if d, err := exec.Decide(goMsg); err != nil || d.Enabled() {
		t.Errorf("Decide(Go) with the right branch in b0 = %+v, %v; want nothing enabled", d, err)
	}

	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || d.Fired || d.Deferred {
		t.Errorf("LastDispatch after Go = %+v, %v; want dispatched, neither fired nor deferred", d, ok)
	}
	if got := activeStateNames(exec); got != "a|b0" {
		t.Fatalf("configuration after Go = %s, want a|b0", got)
	}

	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(after 2): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after the timer = %+v, %v; want b0 -> b fired", d, ok)
	}
	if got := activeStateNames(exec); got != "a|b" {
		t.Fatalf("configuration after the timer = %s, want a|b", got)
	}

	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(completion of b): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after b completed = %+v, %v; want the join fired", d, ok)
	}
	if exec.State() != StateCompleted {
		t.Errorf("machine %v after the join, want completed; configuration %s", exec.State(), activeStateNames(exec))
	}
}

// entryStart designates the state a machine or region starts in, as the
// `entry; then <name>;` succession out of the body's entry action does.
func entryStart(name string) *ast.SuccessionEdge {
	return &ast.SuccessionEdge{
		SourceMember: &ast.EntryMember{},
		Target:       &ast.QualifiedName{Parts: []ast.NameSegment{{Text: name}}},
	}
}

func transitionMember(source, target string) *ast.TransitionMember {
	return &ast.TransitionMember{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: source}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: target}}},
	}
}

func stateExecutorFor(t *testing.T, machine *ast.Usage) *StateExecutor {
	t.Helper()
	idx := symbols.NewIndex()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 100000)

	exec, err := newStateExecutor(ctx, &symbols.Symbol{
		Kind: symbols.SymbolStateUsage,
		Name: machine.Ident.Name,
		Decl: machine,
	}, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	return exec
}

func containsState(visits []string, name string) bool {
	return indexOfState(visits, name) >= 0
}

func indexOfState(visits []string, name string) int {
	for i, visit := range visits {
		if visit == name {
			return i
		}
	}
	return -1
}
