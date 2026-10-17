package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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

// A join segment fires on its own occurrence and waits for the other segment.
func TestJoinSegmentFiresOnOwnOccurrence(t *testing.T) {
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
	if d, err := exec.Decide(goMsg); err != nil || !d.Enabled() {
		t.Fatalf("Decide(Go) with the right branch in b0 = %+v, %v; want the left segment enabled to arrive", d, err)
	}

	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Fatalf("LastDispatch after Go = %+v, %v; want the left segment fired", d, ok)
	}
	if got := activeStateNames(exec); got != "b0" || len(exec.joinArrived) != 1 {
		t.Fatalf("after Go: configuration = %s, arrivals = %v; want b0 and the left segment waiting", got, exec.joinArrived)
	}

	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(after 2): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after the timer = %+v, %v; want b0 -> b fired", d, ok)
	}
	if got := activeStateNames(exec); got != "b" {
		t.Fatalf("configuration after the timer = %s, want b", got)
	}

	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(completion of b): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Fatalf("LastDispatch after b completed = %+v, %v; want b's completion to finish the join", d, ok)
	}
	if got := activeStateNames(exec); got != "done" || len(exec.joinArrived) != 0 {
		t.Fatalf("after b completed: configuration = %s, arrivals = %v; want done with arrivals cleared", got, exec.joinArrived)
	}
}

// A sibling's effect can disarm one segment while another still arrives.
func TestJoinDisarmedSegmentCanArriveAlone(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    attribute def Go;
    state Machine {
        attribute armed : Boolean = true;
        entry; then running;
        state running parallel {
            state c {
                entry; then c1;
                state c1;
                state c2;
                transition first c1 accept Go do assign armed := false then c2;
            }
            state a {
                entry; then a1;
                state a1;
                state a2;
                transition first a1 accept Go then sync;
                transition first a1 accept Go then a2;
            }
            state b {
                entry; then b1;
                state b1;
                transition first b1 accept Go if armed then sync;
            }
        }
        join sync;
        transition first sync then done;
    }
}`, "Machine")
	// `declared` fires the regions in declaration order, c before a.
	policy, err := ParseSchedulePolicy("declared")
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.SetSchedule(policy); err != nil {
		t.Fatal(err)
	}
	exec, err := ctx.CreateStateExecutor(machine)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if got := activeStateNames(exec); got != "c2|b1" || len(exec.joinArrived) != 1 {
		t.Fatalf("after Go: configuration = %s, arrivals = %v; want a's arrival while b is disarmed", got, exec.joinArrived)
	}
	transitionChoice := false
	for _, c := range ctx.Choices() {
		if c.Kind == ChoiceTransition {
			transitionChoice = true
			if !strings.Contains(c.String(), "took 1->sync") {
				t.Errorf("transition choice = %s, want declared policy to take the join segment", c)
			}
		}
	}
	if !transitionChoice {
		t.Fatal("choices have no transition draw between the join segment and a2")
	}
}

// A segment into a join may leave a composite state whose substate is active:
// the occurrence reaches that state from within, so Decide reports the join
// enabled and dispatching the occurrence fires it, the substate exited too.
func TestJoinFromActiveCompositeSourcesFires(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    attribute def Go;
    state Machine {
        entry; then running;
        state running parallel {
            state left {
                entry; then ia;
                state ia {
                    entry; then a1;
                    state a1;
                }
                transition first ia accept Go then sync;
            }
            state right {
                entry; then b;
                state b;
                transition first b accept Go then sync;
            }
        }
        join sync;
        transition first sync then done;
    }
}`, "Machine")
	exec, err := ctx.CreateStateExecutor(machine)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if got := activeStateNames(exec); got != "a1|b" {
		t.Fatalf("initial configuration = %s, want a1|b", got)
	}
	goMsg := Message{SignalType: "Go"}
	if d, err := exec.Decide(goMsg); err != nil || !d.Enabled() {
		t.Errorf("Decide(Go) = %+v, %v; want the join enabled with ia active through a1", d, err)
	}
	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after Go = %+v, %v; want the join fired", d, ok)
	}
	if exec.State() != StateCompleted {
		t.Errorf("machine %v after the join, want completed; configuration %s", exec.State(), activeStateNames(exec))
	}
}

// A's signal segment arrives while b's do behavior runs; b's completion then
// finishes the join without abandoning that behavior early.
func TestJoinCompletionSegmentWaitsForItsSourcesDoBehavior(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    attribute def Go;
    attribute def Tick;
    state Machine {
        attribute ticks : Integer = 0;
        entry; then running;
        state running parallel {
            state left {
                entry; then a;
                state a;
            }
            state right {
                entry; then b;
                state b {
                    do action busy {
                        first start;
                        then action wait accept Tick;
                        then action count assign ticks := ticks + 1;
                        then done;
                    }
                }
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
	if d, err := exec.Decide(goMsg); err != nil || !d.Enabled() {
		t.Fatalf("Decide(Go) with b's do behavior running = %+v, %v; want a's segment enabled to arrive", d, err)
	}
	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Fatalf("LastDispatch after Go = %+v, %v; want a's segment fired", d, ok)
	}
	if got := activeStateNames(exec); got != "b" || len(exec.joinArrived) != 1 {
		t.Fatalf("after Go: configuration = %s, arrivals = %v; want b and a's waiting arrival", got, exec.joinArrived)
	}

	exec.SendSignal("Tick", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Tick): %v", err)
	}
	if got := ticksAfter(t, exec); got != 1 {
		t.Fatalf("ticks after Tick = %d, want 1: the do behavior ran on rather than being abandoned", got)
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(completion of b): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Fatalf("LastDispatch after b completed = %+v, %v; want b's segment to finish the join", d, ok)
	}
	if exec.State() != StateCompleted || len(exec.joinArrived) != 0 {
		t.Errorf("machine %v after the join with arrivals %v, want completed with arrivals cleared", exec.State(), exec.joinArrived)
	}
}

// A completion segment waits until its composite source's body is done, even if
// the other segment has already arrived and the body is re-entered.
func TestJoinCompletionSegmentWaitsForItsSourcesBody(t *testing.T) {
	ctx, machine := loadState(t, `package test {
    attribute def Go;
    attribute def Tick;
    attribute def Again;
    state Machine {
        entry; then running;
        state running parallel {
            state left {
                entry; then a;
                state a;
            }
            state right {
                entry; then b;
                state b {
                    entry; then b1;
                    state b1;
                    transition first b1 accept Tick then done;
                }
                transition first b accept Again then b;
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
	dispatch := func(signal string) {
		t.Helper()
		exec.SendSignal(signal, nil)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent(%s): %v", signal, err)
		}
	}
	if d, err := exec.Decide(goMsg); err != nil || !d.Enabled() {
		t.Fatalf("Decide(Go) with b's body at b1 = %+v, %v; want a's segment enabled to arrive", d, err)
	}
	dispatch("Go")
	if got := activeStateNames(exec); got != "b1" || len(exec.joinArrived) != 1 {
		t.Fatalf("after Go: configuration = %s, arrivals = %v; want b1 and a's waiting arrival", got, exec.joinArrived)
	}

	dispatch("Again")
	if got := activeStateNames(exec); got != "b1" || len(exec.joinArrived) != 1 {
		t.Fatalf("configuration after Again = %s, want b1 with a's arrival still held", got)
	}
	if d, err := exec.Decide(goMsg); err != nil || d.Enabled() {
		t.Errorf("Decide(Go) after a's segment arrived = %+v, %v; want no transition from the inactive source", d, err)
	}

	dispatch("Tick")
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(completion of b): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after b completed = %+v, %v; want the join fired", d, ok)
	}
	if exec.State() != StateCompleted || len(exec.joinArrived) != 0 {
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
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 100000)

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
