package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// A transition out of a composite state is enabled while any of its substates is
// active, so the composite handles an event its active substate does not.
func TestCompositeStateHandlesEventItsSubstateDoesNot(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state Working {
				state Step1;
				state Step2;
				transition first Step1 accept next then Step2;
			}
			state Done;
			succession first start then Step1;
			transition first Working accept abort then Done;
		}
	}`)

	exec.SendSignal("abort", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Done")
}

// A guard that is false does not consume the event: matching continues outward,
// so the composite state still handles it.
func TestFalseGuardInsideACompositeStateFallsOutward(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute ready : Boolean = false;

			entry; then start;
			state start;
			state Working {
				state Step1;
				state Step2;
				transition first Step1 accept abort if ready then Step2;
			}
			state Done;
			succession first start then Step1;
			transition first Working accept abort then Done;
		}
	}`)

	exec.SendSignal("abort", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Done")
	if containsState(exec.stateVisits, "Step2") {
		t.Errorf("the blocked transition fired anyway, visits: %v", exec.stateVisits)
	}
}

// A transition out of a state between the active substate and the machine's root
// leaves only the states below it: the composite state enclosing its source stays
// active.
func TestTransitionOutOfAnIntermediateCompositeStateKeepsItsOwnerActive(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state Outer {
				state Middle {
					state Inner;
				}
				state Recovered;
				transition first Middle accept abort then Recovered;
			}
			succession first start then Inner;
			transition first Outer accept shutdown then Done;
			state Done;
		}
	}`)

	exec.SendSignal("abort", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Recovered")
	if !exec.inActiveConfiguration(stateNamed(t, exec, "Outer")) {
		t.Error("Outer was left although the transition's source was Middle")
	}
}

// Outer still handles the event only it accepts once its own substate moved: a
// composite state keeps reacting for whichever substate is active.
func TestOuterCompositeStateStillHandlesItsEventAfterASubstateMoved(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state Outer {
				state Middle {
					state Inner;
				}
				state Recovered;
				transition first Middle accept abort then Recovered;
			}
			succession first start then Inner;
			transition first Outer accept shutdown then Done;
			state Done;
		}
	}`)

	exec.SendSignal("abort", nil)
	exec.SendSignal("shutdown", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertVisits(t, exec.stateVisits, "start", "Outer", "Middle", "Inner", "Recovered", "Done")
}

// An event no state accepts at any level is dropped: the machine stays where it
// was, and the event does not reach an enclosing state that never declared it.
func TestEventNoLevelAcceptsLeavesTheConfigurationAlone(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state Working {
				state Step1;
				state Step2;
				transition first Step1 accept next then Step2;
			}
			state Done;
			succession first start then Step1;
			transition first Working accept abort then Done;
		}
	}`)

	exec.SendSignal("unknown", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertCurrentState(t, exec, "Step1")
	if len(exec.deferred) != 0 {
		t.Errorf("expected the event dropped rather than deferred, got %d held", len(exec.deferred))
	}
}

// Run-to-completion: a state entered while an event is dispatched does not react
// to that same event, even when it accepts it and lies outside the state that
// just moved.
func TestOneEventTakesOneTransitionPerActiveLeaf(t *testing.T) {
	source := `package test {
		state sm {
			entry; then start;
			state start;
			state Working {
				state Step1;
			}
			state Idle;
			state Done;
			succession first start then Working::Step1;
			transition first Step1 accept e then Idle;
			transition first Idle accept e then Done;
		}
	}`

	exec := stateExecutorForSource(t, "sm", source)
	exec.SendSignal("e", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "Idle")
	if containsState(exec.stateVisits, "Done") {
		t.Errorf("one event took two transitions, visits: %v", exec.stateVisits)
	}

	exec = stateExecutorForSource(t, "sm", source)
	exec.SendSignal("e", nil)
	exec.SendSignal("e", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "Done")
}

// Every leaf inside a composite state selects the same transition out of it, and
// one event still takes that transition once, re-entering both regions.
func TestOneEventTakesACompositesTransitionOnce(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute log : Integer = 0;

			entry; then start;
			state start;
			state Working parallel {
				state left {
					entry; then lstart;
					state lstart;
					state l1;
					succession first lstart then l1;
				}
				state right {
					entry; then rstart;
					state rstart;
					state r1;
					succession first rstart then r1;
				}
			}
			succession first start then Working;
			transition first Working accept restart do assign log := log + 1 then Working;
		}
	}`)
	exec.SendSignal("restart", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := exec.StateData()["log"]; got.Const.Int != 1 {
		t.Errorf("one event ran the composite's effect %v times, want 1", got.Const.Int)
	}
	for _, state := range []string{"l1", "r1"} {
		if visits := countVisits(exec.stateVisits, state); visits != 2 {
			t.Errorf("state %s entered %d times, want 2 (initial entry and re-entry), visits: %v", state, visits, exec.stateVisits)
		}
	}
}

// A change condition on a composite state is watched while a substate is active.
// Polling is the driver of change triggers, so the test polls directly.
func TestChangeConditionOnACompositeStateFiresWhileASubstateIsActive(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute ready : Boolean = false;

			entry; then start;
			state start;
			state Working {
				state Step1;
				accept when ready then Done;
			}
			state Done;
			succession first start then Step1;
		}
	}`)

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := exec.pollChangeEvents(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	assertCurrentState(t, exec, "Step1")

	exec.stateData["ready"] = boolValue(true)
	if _, err := exec.pollChangeEvents(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	assertCurrentState(t, exec, "Done")
}

// A watched condition whose guard blocks its transition does not consume the poll:
// the other regions still move on the conditions they watch, one transition each.
func TestGuardBlockedChangeConditionDoesNotSilenceTheOtherRegions(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm parallel {
			attribute ready : Boolean = false;
			attribute allowed : Boolean = false;

			state left {
				entry; then lstart;
				state lstart;
				state l1;
				state lmoved;
				transition first lstart then l1;
				transition first l1 accept when ready if allowed then lmoved;
			}
			state right {
				entry; then rstart;
				state rstart;
				state r1;
				state rmoved;
				transition first rstart then r1;
				transition first r1 accept when ready then rmoved;
			}
		}
	}`)

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	exec.stateData["ready"] = boolValue(true)
	if _, err := exec.pollChangeEvents(); err != nil {
		t.Fatalf("poll: %v", err)
	}

	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["l1"] || !active["rmoved"] {
		t.Errorf("expected the blocked left region to stay in l1 and the right one to reach rmoved, got %v", active)
	}

	exec.stateData["allowed"] = boolValue(true)
	if _, err := exec.pollChangeEvents(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	active = make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if !active["lmoved"] {
		t.Errorf("expected the left region to move once its guard holds, got %v", active)
	}
}

// A condition satisfying an inner and an enclosing transition at once resolves the
// way the equivalent signal does: the inner one wins and the composite stays.
func TestChangeConditionTakesTheInnermostTransitionOnly(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute ready : Boolean = false;

			entry; then start;
			state start;
			state Working parallel {
				state left {
					entry; then lstart;
					state lstart;
					state l1;
					state l2;
					transition first lstart then l1;
					transition first l1 accept when ready then l2;
				}
				state right {
					entry; then rstart;
					state rstart;
					state r1;
					transition first rstart then r1;
				}
			}
			accept when ready then Done;
			state Done;
			succession first start then Working;
		}
	}`)

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	exec.stateData["ready"] = boolValue(true)
	if _, err := exec.pollChangeEvents(); err != nil {
		t.Fatalf("poll: %v", err)
	}

	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	if active["Done"] || !active["l2"] || !active["r1"] {
		t.Errorf("expected the inner transition to win and Working to stay active in l2 | r1, got %v", active)
	}
}

// An enabled transition inside one region does not suppress the transition an
// enclosing state offers a concurrent region: only the state a fired transition
// left is disabled for this event.
func TestOneRegionsInnerTransitionLeavesAConcurrentRegionsOwnTransition(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			entry; then start;
			state start;
			state Working parallel {
				state left {
					entry; then lstart;
					state lstart;
					state l1;
					state l2;

					succession first lstart then l1;
					transition first l1 accept e then l2;
				}

				state right {
					entry; then rstart;
					state rstart;
					state r1;
					state r2;

					succession first rstart then r1;
					transition first r1 accept e then r2;
				}
			}
			succession first start then Working;
		}
	}`)

	exec.SendSignal("e", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertVisits(t, exec.stateVisits, "start", "Working", "lstart", "rstart", "l1", "r1", "l2", "r2")
}

// Region-internal transitions complete an orthogonal machine only when every
// top-level region reaches a final state, regardless of the source spelling.
func TestOrthogonalMachineCompletesAfterAllRegionFinalStates(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "standard parallel",
			body: `state Machine parallel {
				attribute exits : Integer = 0;
				exit action bye { assign exits := exits + 1; }
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart when First then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					transition first rstart when Second then done;
				}
			}`,
		},
		{
			name: "explicit regions",
			body: `state Machine parallel {
				attribute exits : Integer = 0;
				exit action bye { assign exits := exits + 1; }
				state left {
					entry; then lstart;
					state lstart;
					transition first lstart when First then done;
				}
				state right {
					entry; then rstart;
					state rstart;
					transition first rstart when Second then done;
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := stateExecutorForSource(t, "Machine", "package test {\n"+tt.body+"\n}")

			exec.SendSignal("First", nil)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("run after First: %v", err)
			}
			if got := exec.State(); got != StateSuspended {
				t.Fatalf("state after First = %v, want StateSuspended", got)
			}
			if got := exec.StateData()["exits"].Const.Int; got != 0 {
				t.Fatalf("exit behavior after First ran %d times, want 0", got)
			}

			exec.SendSignal("Second", nil)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("run after Second: %v", err)
			}
			if got := exec.State(); got != StateCompleted {
				t.Fatalf("state after Second = %v, want StateCompleted", got)
			}
			if got := exec.StateData()["exits"].Const.Int; got != 1 {
				t.Fatalf("exit behavior ran %d times, want 1", got)
			}

			exec.SendSignal("Second", nil)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("rerun after completion: %v", err)
			}
			if got := exec.StateData()["exits"].Const.Int; got != 1 {
				t.Fatalf("exit behavior ran %d times after completion, want 1", got)
			}
		})
	}
}

// A timed transition looping back to its own simple state re-arms its timer, so
// it fires once per period rather than only the first time.
func TestTimedSelfTransitionFiresEveryPeriod(t *testing.T) {
	exec := stateExecutorForSource(t, "sm", `package test {
		state sm {
			attribute ticks : Integer = 0;

			entry; then start;
			state start;
			state s;
			succession first start then s;
			transition first s accept after 1 do assign ticks := ticks + 1 then s;
		}
	}`)

	for step := 0; step < 3; step++ {
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
	}
	if got := exec.StateData()["ticks"]; got.Const.Int != 2 {
		t.Errorf("timed self-transition fired %v times over three steps, want 2", got.Const.Int)
	}
	if exec.EventQueue().Len() != 1 {
		t.Errorf("expected the timer re-armed for the next period, queue holds %d events", exec.EventQueue().Len())
	}
}

func assertCurrentState(t *testing.T, exec *StateExecutor, want string) {
	t.Helper()
	current, _ := exec.CurrentState().(*ast.StateNode)
	if current == nil || current.Name != want {
		t.Errorf("expected the machine in %s, got %v", want, exec.CurrentState())
	}
}

func stateNamed(t *testing.T, exec *StateExecutor, name string) *ast.StateNode {
	t.Helper()
	for _, state := range exec.graph.States {
		if state.Name == name {
			return state
		}
	}
	t.Fatalf("state %s not found in the lowered graph", name)
	return nil
}
