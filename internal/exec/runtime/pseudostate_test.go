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

// A transition into a join is not enabled until every branch has arrived: an
// event reaching one branch first fires nothing, Decide and LastDispatch say so;
// the completion of the last branch fires nothing either while the other
// segment's trigger is not that occurrence, and the join fires on the event
// that enables every segment at once.
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
	if d, ok := exec.LastDispatch(); !ok || d.Fired || d.Deferred {
		t.Errorf("LastDispatch after b completed = %+v, %v; want dispatched, neither fired nor deferred: a's segment waits for Go", d, ok)
	}
	if got := activeStateNames(exec); got != "a|b" {
		t.Fatalf("configuration after b completed = %s, want a|b", got)
	}

	if d, err := exec.Decide(goMsg); err != nil || !d.Enabled() {
		t.Errorf("Decide(Go) with both branches arrived = %+v, %v; want the join enabled", d, err)
	}
	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(second Go): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after the second Go = %+v, %v; want the join fired", d, ok)
	}
	if exec.State() != StateCompleted {
		t.Errorf("machine %v after the join, want completed; configuration %s", exec.State(), activeStateNames(exec))
	}
}

// A segment into a join drawn among several transitions out of its source, whose
// join another region's effect then disarms before its turn: the join fires
// nothing, and the draw that never fired is not among the run's choices.
func TestJoinDisarmedBeforeItsTurnRecordsNoChoice(t *testing.T) {
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
	// `declared` fires the regions in declaration order, c before a, and takes
	// a1's first transition, the segment into the join.
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
	if got := activeStateNames(exec); got != "c2|a1|b1" {
		t.Fatalf("configuration after Go = %s, want c2|a1|b1: c's effect disarmed b's segment, so a's fires nothing", got)
	}
	for _, c := range ctx.Choices() {
		if c.Kind == ChoiceTransition {
			t.Errorf("choices include %s: a's segment fired nothing, so its draw is no choice the run made", c)
		}
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

// A completion segment into a join is enabled once its source has completed,
// not while it is merely active: a Go dispatched while b's do behavior still runs
// fires nothing and the behavior goes on; b's completion, once the behavior ends,
// fires nothing while a's segment waits for Go; the Go that follows fires the join.
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
	if d, err := exec.Decide(goMsg); err != nil || d.Enabled() {
		t.Errorf("Decide(Go) with b's do behavior running = %+v, %v; want nothing enabled", d, err)
	}
	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || d.Fired || d.Deferred {
		t.Errorf("LastDispatch after Go = %+v, %v; want dispatched, neither fired nor deferred", d, ok)
	}
	if got := activeStateNames(exec); got != "a|b" {
		t.Fatalf("configuration after Go = %s, want a|b", got)
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
	if d, ok := exec.LastDispatch(); !ok || d.Fired || d.Deferred {
		t.Errorf("LastDispatch after b completed = %+v, %v; want dispatched, neither fired nor deferred: a's segment waits for Go", d, ok)
	}
	if got := activeStateNames(exec); got != "a|b" {
		t.Fatalf("configuration after b completed = %s, want a|b", got)
	}

	if d, err := exec.Decide(goMsg); err != nil || !d.Enabled() {
		t.Errorf("Decide(Go) with b completed = %+v, %v; want the join enabled", d, err)
	}
	exec.SendSignal("Go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(second Go): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after the second Go = %+v, %v; want the join fired", d, ok)
	}
	if exec.State() != StateCompleted {
		t.Errorf("machine %v after the join, want completed; configuration %s", exec.State(), activeStateNames(exec))
	}
}

// A completion segment out of a composite state is enabled once the state's body
// is at `done`, and re-entering the state starts its body over: a Go dispatched
// while b's body is at b1 fires nothing; a Go once it is done fires the join.
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
	if d, err := exec.Decide(goMsg); err != nil || d.Enabled() {
		t.Errorf("Decide(Go) with b's body at b1 = %+v, %v; want nothing enabled", d, err)
	}
	dispatch("Go")
	if got := activeStateNames(exec); got != "a|b1" {
		t.Fatalf("configuration after Go = %s, want a|b1", got)
	}

	dispatch("Tick")
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(completion of b): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || d.Fired {
		t.Errorf("LastDispatch after b completed = %+v, %v; want nothing fired: a's segment waits for Go", d, ok)
	}
	if d, err := exec.Decide(goMsg); err != nil || !d.Enabled() {
		t.Errorf("Decide(Go) with b's body done = %+v, %v; want the join enabled", d, err)
	}

	dispatch("Again")
	if got := activeStateNames(exec); got != "a|b1" {
		t.Fatalf("configuration after Again = %s, want a|b1: b re-entered starts its body over", got)
	}
	if d, err := exec.Decide(goMsg); err != nil || d.Enabled() {
		t.Errorf("Decide(Go) after b re-entered = %+v, %v; want nothing enabled", d, err)
	}
	dispatch("Go")
	if got := activeStateNames(exec); got != "a|b1" {
		t.Fatalf("configuration after Go = %s, want a|b1", got)
	}

	dispatch("Tick")
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(completion of b): %v", err)
	}
	dispatch("Go")
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after the last Go = %+v, %v; want the join fired", d, ok)
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
