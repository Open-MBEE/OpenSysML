package runtime

import (
	"fmt"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
)

// A transition reaching `done` completes the machine: the state it leaves runs
// its exit action and the executor comes to rest completed.
func TestTransitionToDoneCompletesTheMachineAndRunsExitActions(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		private import ScalarValues::*;
		state sm {
			attribute left : Integer = 0;

			entry; then start;
			state start;
			state working {
				exit action { assign left := 1; }
			}
			succession first start then working;
			transition first working accept stop then done;
		}
	}`)

	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted, got %s", exec.State())
	}
	if got := exec.stateData["left"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the exit action of the completing state did not run, left = %v", got)
	}
}

// A machine whose states have no transition to `done` stays active, even where
// the state it rests in has no outgoing transition at all: a sink is not a
// completion, because an ancestor or cross-region transition may still leave it.
func TestMachineWithoutATransitionToDoneStaysActive(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state resting;
			succession first start then resting;
		}
	}`)

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "resting")
	if exec.State() == StateCompleted {
		t.Errorf("the machine completed without reaching `done`")
	}
}

// An orthogonal machine completes only once every top-level region has reached
// `done`; one region completing leaves the machine running.
func TestOrthogonalMachineCompletesOnlyWhenEveryRegionDoes(t *testing.T) {
	const src = `package test {
		state sm parallel {
			state left {
				entry; then lstart;
				state lstart;
				transition first lstart accept first then done;
			}
			state right {
				entry; then rstart;
				state rstart;
				transition first rstart accept second then done;
			}
		}
	}`

	exec := stateExecutorForSource(t, "sm", src)
	exec.SendSignal("first", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Fatalf("the machine completed with one region still running")
	}

	exec.SendSignal("second", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("expected StateCompleted once both regions reached `done`, got %s", exec.State())
	}
}

// A composite state completes once every one of its regions reached `done`,
// and its completion transition then fires: one region reaching `done` leaves
// the composite running, both reaching it takes the composite's completion
// transition, here into the machine's own `done`.
func TestNestedOrthogonalRegionsCompleteOnlyWhenEveryRegionDoes(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then busy;
			state busy parallel {
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart accept first then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					transition first rstart accept second then done;
				}
			}
			transition first busy then done;
		}
	}`)

	exec.SendSignal("first", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Fatalf("the machine completed with one nested region still running")
	}

	exec.SendSignal("second", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("expected StateCompleted once busy's completion transition fired, got %s", exec.State())
	}
}

// A composite state whose regions all reached `done` but which has no
// completion transition stays completed and active: nothing propagates its
// completion to the machine, which keeps running and still reacts through it.
func TestCompletedCompositeWithoutCompletionTransitionStaysActive(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then busy;
			state busy parallel {
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart accept first then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					transition first rstart accept second then done;
				}
			}
			state idle;
			transition first busy accept reset then idle;
		}
	}`)

	exec.SendSignal("first", nil)
	exec.SendSignal("second", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() == StateCompleted {
		t.Fatalf("the machine completed although busy has no completion transition")
	}
	if got := exec.FinalStateName(); got != "done+done" {
		t.Fatalf("active configuration = %q, want busy completed at done+done", got)
	}

	exec.SendSignal("reset", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "idle")
}

// A composite state running a do behavior completes only once both the
// behavior has finished and its body has reached `done`, in whichever order
// the two happen.
func TestCompositeCompletesOnceItsDoBehaviorAndItsBodyHaveBothEnded(t *testing.T) {
	const src = `package test {
		private import ScalarValues::*;
		attribute def Finish;
		state sm {
			attribute worked : Integer = 0;
			entry; then s1;
			state s1 {
				do action work {
					first start;
					then action wait accept Finish;
					then action count assign worked := worked + 1;
					then done;
				}
				entry; then wait;
				state wait;
				transition first wait accept go then done;
			}
			state s2;
			transition first s1 then s2;
		}
	}`

	t.Run("body_done_before_the_do_behavior_ends", func(t *testing.T) {
		exec := stateExecutorForSource(t, "sm", src)
		exec.SendSignal("go", nil)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run: %v", err)
		}
		if got := exec.FinalStateName(); got != "done" {
			t.Fatalf("configuration = %q after go, want s1 at done with its do behavior running", got)
		}
		exec.SendSignal("Finish", nil)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run: %v", err)
		}
		assertCurrentState(t, exec, "s2")
		if got := exec.stateData["worked"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("the do behavior did not run to its end before s1 completed, worked = %v", got)
		}
	})

	t.Run("do_behavior_ends_before_the_body_is_done", func(t *testing.T) {
		exec := stateExecutorForSource(t, "sm", src)
		exec.SendSignal("Finish", nil)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run: %v", err)
		}
		if got := exec.FinalStateName(); got != "wait" {
			t.Fatalf("configuration = %q after finish, want s1 still at wait", got)
		}
		exec.SendSignal("go", nil)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run: %v", err)
		}
		assertCurrentState(t, exec, "s2")
	})
}

// A completed composite state queues its completion transitions exactly as a
// leaf does: one event per transition whose guard holds, stamped with the
// clock instant of the completion, in declaration order.
func TestCompositeCompletionQueuesItsTransitionsLikeALeaf(t *testing.T) {
	const machine = `package test {
		private import ScalarValues::*;
		private import SI::*;
		state sm {
			attribute flag : Boolean = true;
			%s
			state a;
			state b;
			state c;
			transition first s1 if flag then a;
			transition first s1 if not flag then c;
			transition first s1 then b;
		}
	}`
	const composite = `entry; then s1;
	state s1 {
		entry; then wait;
		state wait;
		transition first wait accept after 3 [s] then done;
	}`
	const leaf = `entry; then s0;
	state s0;
	state s1;
	transition first s0 accept after 3 [s] then s1;`

	start := func(t *testing.T, body string) *StateExecutor {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(machine, body)))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "sm", ast.DefState)
		exec, err := newStateExecutor(ctx, sym, nil)
		if err != nil {
			t.Fatalf("newStateExecutor: %v", err)
		}
		if err := exec.initialize(); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		return exec
	}
	queued := func(t *testing.T, body string) []Event {
		t.Helper()
		exec := start(t, body)
		if err := exec.processNextEvent(); err != nil {
			t.Fatalf("dispatch the timed transition: %v", err)
		}
		var events []Event
		for exec.eventQueue.Len() > 0 {
			events = append(events, exec.eventQueue.Pop())
		}
		return events
	}
	describe := func(events []Event) []string {
		var out []string
		for _, event := range events {
			trans, ok := event.Payload.(*lower.Transition)
			if !ok || trans.Trigger != nil {
				out = append(out, "not a completion event")
				continue
			}
			out = append(out, fmt.Sprintf("%s->%s@%g", getNodeName(trans.Source), getNodeName(trans.Target), event.Timestamp))
		}
		return out
	}

	want := []string{"s1->a@3", "s1->b@3"}
	got := describe(queued(t, composite))
	if !slices.Equal(got, want) {
		t.Fatalf("composite completion queued %v, want %v", got, want)
	}
	if fromLeaf := describe(queued(t, leaf)); !slices.Equal(fromLeaf, want) {
		t.Fatalf("leaf completion queued %v, want %v", fromLeaf, want)
	}

	exec := start(t, composite)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "a")
}

// Completion is reached through a pseudostate as through any other path: the
// junction routes into `done` and the machine completes.
func TestCompletionReachedThroughAPseudostate(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state working;
			junction meet;
			succession first start then working;
			transition first working accept stop then meet;
			transition first meet then done;
		}
	}`)

	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("expected StateCompleted through the junction, got %s", exec.State())
	}
}

// A state the machine declares itself as `done` is an ordinary state and wins:
// completion is the library feature the name reaches when nothing nearer
// declares it, so a machine naming its own state `done` keeps running.
func TestADeclaredDoneStateIsAnOrdinaryState(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state done;
			transition first start accept stop then done;
		}
	}`)

	exec.SendSignal("stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if exec.State() == StateCompleted {
		t.Errorf("the declared state completed the machine")
	}
}
