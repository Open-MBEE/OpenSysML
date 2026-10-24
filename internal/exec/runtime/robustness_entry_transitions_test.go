package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessEntryTransitions covers entry transitions: guards, triggers, targets
// that are not states and the descent into nested regions.
func TestRuntimeRobustnessEntryTransitions(t *testing.T) {
	t.Run("no_entry_transition_guard_holds", testNoEntryTransitionGuardHolds)
	t.Run("entry_transition_target_is_not_a_state", testEntryTransitionTargetIsNotAState)
	t.Run("entry_transition_carries_a_trigger", testEntryTransitionCarriesATrigger)
	t.Run("entry_transition_into_done_completes_at_initialize", testEntryTransitionIntoDoneCompletesAtInitialize)
	t.Run("named_entry_action_transition_into_done_completes_at_initialize", testNamedEntryActionTransitionIntoDoneCompletesAtInitialize)
	t.Run("own_entry_transitions_replace_inherited_ones", testOwnEntryTransitionsReplaceInheritedOnes)
	t.Run("region_entry_transitions_into_done_complete_at_initialize", testRegionEntryTransitionsIntoDoneCompleteAtInitialize)
	t.Run("nested_regions_into_done_complete_at_initialize", testNestedRegionsIntoDoneCompleteAtInitialize)
	t.Run("nested_regions_into_done_without_completion_transition_stay_active", testNestedRegionsIntoDoneWithoutCompletionTransitionStayActive)
	t.Run("transition_into_nested_regions_in_done_completes", testTransitionIntoNestedRegionsInDoneCompletes)
	t.Run("region_start_descends_through_entry_transitions", testRegionStartDescendsThroughEntryTransitions)
	t.Run("region_entry_guards_read_the_region_state_attributes", testRegionEntryGuardsReadTheRegionStateAttributes)
	t.Run("leaving_regions_descends_through_entry_transitions", testLeavingRegionsDescendsThroughEntryTransitions)
}

// testNoEntryTransitionGuardHolds: `entry; if c then s;` chooses the starting
// state by guard at initialize (SysML v2 7.18.3); when no guard holds the
// machine has nowhere to start, a typed error rather than a silent stall.
func testNoEntryTransitionGuardHolds(t *testing.T) {
	src := `
		package test {
			part def Heater {
				attribute cold : Boolean = true;
				exhibit state control {
					entry;
					if not cold then ready;
					if cold and not cold then warming;
					state warming;
					state ready;
				}
			}
		}
	`
	_, _, err := instantiateWithLibraries(t, src, "test::Heater")
	if !errors.Is(err, errNoEntryTransitionHolds) {
		t.Fatalf("expected errNoEntryTransitionHolds, got %v", err)
	}
	want := "no entry transition holds: state machine control declares 2 transitions out of its entry action and the guard of none holds"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("message:\n got %q\nwant it to contain %q", err.Error(), want)
	}
}

// testEntryTransitionTargetIsNotAState: a fork remains an unsupported entry target.
func testEntryTransitionTargetIsNotAState(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state Machine {
				entry; then pick;
				fork pick;
				state idle;
			}
		}
	`, "Machine")
	var targetErr *lower.EntryTransitionTargetError
	if !errors.As(err, &targetErr) {
		t.Fatalf("expected EntryTransitionTargetError, got %v", err)
	}
	want := "create state executor: lower state machine: " + fmt.Sprintf(lower.EntryTransitionTargetFormat, "the fork pick")
	if err.Error() != want {
		t.Fatalf("message:\n got %q\nwant %q", err.Error(), want)
	}
}

// testEntryTransitionCarriesATrigger: an entry transition chooses the start by
// its guard alone; a trigger on it is a typed lowering error.
func testEntryTransitionCarriesATrigger(t *testing.T) {
	err := stateExecutorError(t, `
		package test {
			state Machine {
				entry; accept go then idle;
				state idle;
			}
		}
	`, "Machine")
	var shapeErr *lower.EntryTransitionShapeError
	if !errors.As(err, &shapeErr) {
		t.Fatalf("expected EntryTransitionShapeError, got %v", err)
	}
	want := "create state executor: lower state machine: " + lower.EntryTransitionAccepterSourceMessage
	if err.Error() != want {
		t.Fatalf("message:\n got %q\nwant %q", err.Error(), want)
	}
}

// testEntryTransitionIntoDoneCompletesAtInitialize: an entry transition whose
// guard chooses `done` completes the machine as it starts — its exit behavior
// runs and no event is left waiting — rather than leaving it running in `done`.
func testEntryTransitionIntoDoneCompletesAtInitialize(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute skip : Boolean = true;
			attribute left : Boolean = false;
			entry; if skip then done;
			then busy;
			exit action { assign left := true; }
			state busy;
			transition first busy accept after 1 [SI::s] then done;
		}
	}`)
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted right after initialize, got %s", exec.State())
	}
	assertCurrentState(t, exec, ast.DoneFeature)
	if got := exec.StateData()["left"]; got.Kind != ValConst || !got.Const.Bool {
		t.Errorf("the machine's exit action did not run, left = %v", got)
	}
	if exec.EventQueue().Len() != 0 {
		t.Errorf("a completed machine keeps %d events waiting", exec.EventQueue().Len())
	}
}

// testNamedEntryActionTransitionIntoDoneCompletesAtInitialize: a transition
// out of a named entry action into an undeclared `done` completes the machine as
// it starts, in each syntax the succession can be written in; a declared state
// named `done` is entered instead.
func testNamedEntryActionTransitionIntoDoneCompletesAtInitialize(t *testing.T) {
	for name, successions := range map[string]string{
		"guarded transition": `transition begin if skip then done;
			transition begin then busy;`,
		"transition": `transition begin then done;`,
		"succession": `succession first begin then done;`,
	} {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;
			state Machine {
				attribute skip : Boolean = true;
				attribute left : Boolean = false;
				entry action begin { }
				`+successions+`
				exit action { assign left := true; }
				state busy;
			}
		}`)
		if exec.State() != StateCompleted {
			t.Errorf("%s: expected StateCompleted right after initialize, got %s", name, exec.State())
		}
		if got := exec.StateData()["left"]; got.Kind != ValConst || !got.Const.Bool {
			t.Errorf("%s: the machine's exit action did not run, left = %v", name, got)
		}
	}
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry action begin { }
			transition begin then done;
			state done;
		}
	}`)
	if exec.State() != StateRunning {
		t.Errorf("expected the machine to be running in its declared state done, got %s", exec.State())
	}
	assertCurrentState(t, exec, "done")
}

// testOwnEntryTransitionsReplaceInheritedOnes: the entry transitions a state
// writes itself replace the ones it inherits, guarded or not, at the machine's
// top level, in a nested typed usage and in a typed orthogonal region; a state
// writing none keeps the inherited start.
func testOwnEntryTransitionsReplaceInheritedOnes(t *testing.T) {
	const base = `
		state def Base {
			attribute c : Boolean = true;
			entry; if c then old;
			then older;
			state old;
			state older;
		}`
	for name, tc := range map[string]struct {
		machine string
		want    []string
	}{
		"specializing machine": {machine: `
			state def Machine :> Base {
				entry; then fresh;
				state fresh;
			}`, want: []string{"fresh"}},
		"typed usage": {machine: `
			state def Machine {
				entry; then u;
				state u : Base {
					entry; then fresh;
					state fresh;
				}
			}`, want: []string{"fresh"}},
		"guarded typed usage": {machine: `
			state def Machine {
				entry; then u;
				state u : Base {
					entry; if not c then fresh;
					then fresher;
					state fresh;
					state fresher;
				}
			}`, want: []string{"fresher"}},
		"redeclared entry behavior only": {machine: `
			state def Machine {
				entry; then u;
				state u : Base {
					entry assign c := true;
				}
			}`, want: []string{"old"}},
		"typed region": {machine: `
			state def Machine parallel {
				state left : Base {
					entry; then fresh;
					state fresh;
				}
				state right : Base;
			}`, want: []string{"fresh", "old"}},
	} {
		exec := stateExecutorForSource(t, "Machine", `package test {
			private import ScalarValues::*;`+base+tc.machine+`
		}`)
		var got []string
		for _, state := range exec.ActiveStates() {
			got = append(got, state.Name)
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s: started in %v, want %v", name, got, tc.want)
		}
	}
}

// testRegionEntryTransitionsIntoDoneCompleteAtInitialize: every orthogonal
// region starting in `done` completes the machine as it starts, exactly once.
func testRegionEntryTransitionsIntoDoneCompleteAtInitialize(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine parallel {
			attribute exits : Integer = 0;
			exit action { assign exits := exits + 1; }
			state left {
				entry; then done;
			}
			state right {
				entry; then done;
			}
		}
	}`)
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted right after initialize, got %s", exec.State())
	}
	if got := exec.StateData()["exits"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the machine's exit action ran %v times, want once", got)
	}
}

// testNestedRegionsIntoDoneCompleteAtInitialize: a machine starting in a
// parallel state whose every region starts in `done` completes that state as
// it starts; its completion transition then completes the machine.
func testNestedRegionsIntoDoneCompleteAtInitialize(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute exits : Integer = 0;
			exit action { assign exits := exits + 1; }
			entry; then outer;
			state outer parallel {
				state left {
					entry; then done;
				}
				state right {
					entry; then done;
				}
			}
			transition first outer then done;
		}
	}`)
	if exec.State() != StateRunning {
		t.Fatalf("expected the completion of outer pending right after initialize, got %s", exec.State())
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("completion of outer: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted once outer's completion transition fired, got %s", exec.State())
	}
	if got := exec.StateData()["exits"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the machine's exit action ran %v times, want once", got)
	}
}

// testNestedRegionsIntoDoneWithoutCompletionTransitionStayActive: a parallel
// state whose every region starts in `done` and which has no completion
// transition stays active and completed; the machine keeps running.
func testNestedRegionsIntoDoneWithoutCompletionTransitionStayActive(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute exits : Integer = 0;
			exit action { assign exits := exits + 1; }
			entry; then outer;
			state outer parallel {
				state left {
					entry; then done;
				}
				state right {
					entry; then done;
				}
			}
			transition first outer accept again then outer;
		}
	}`)
	if exec.State() != StateRunning {
		t.Fatalf("expected the machine running with outer completed, got %s", exec.State())
	}
	if exec.EventQueue().Len() != 0 {
		t.Fatalf("%d events pending, want none: outer has no completion transition", exec.EventQueue().Len())
	}
	if got := exec.StateData()["exits"]; got.Kind != ValConst || got.Const.Int != 0 {
		t.Errorf("the machine's exit action ran %v times, want never", got)
	}
	exec.SendSignal("again", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("again: %v", err)
	}
	dispatch, _ := exec.LastDispatch()
	if !dispatch.Fired {
		t.Error("the completed outer state no longer reacts to an event")
	}
	if exec.State() != StateRunning {
		t.Errorf("expected the machine still running after re-entering outer, got %s", exec.State())
	}
}

// testTransitionIntoNestedRegionsInDoneCompletes: a transition into a parallel
// state whose every region starts in `done` completes that state, and its
// completion transition the machine.
func testTransitionIntoNestedRegionsInDoneCompletes(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine {
			attribute exits : Integer = 0;
			exit action { assign exits := exits + 1; }
			entry; then idle;
			state idle;
			transition first idle accept go then outer;
			state outer parallel {
				state left {
					entry; then done;
				}
				state right {
					entry; then done;
				}
			}
			transition first outer then done;
		}
	}`)
	assertCurrentState(t, exec, "idle")
	exec.SendSignal("go", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("go: %v", err)
	}
	if exec.State() != StateRunning {
		t.Fatalf("expected the completion of outer pending after entering it, got %s", exec.State())
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("completion of outer: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Fatalf("expected StateCompleted once outer's completion transition fired, got %s", exec.State())
	}
	if got := exec.StateData()["exits"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("the machine's exit action ran %v times, want once", got)
	}
}

// testRegionStartDescendsThroughEntryTransitions: a region whose starting state
// is composite starts that state where its own entry transitions choose, and the
// nested state is the region's active state, so its transitions are armed.
func testRegionStartDescendsThroughEntryTransitions(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		private import ScalarValues::*;
		state Machine parallel {
			attribute cold : Boolean = true;
			state control {
				entry; then running;
				state running {
					entry; if cold then heating;
					if not cold then idle;
					state heating;
					transition first heating accept warm then idle;
					state idle;
				}
			}
			state monitor {
				entry; then watching;
				state watching;
			}
		}
	}`)
	activeNames := func() map[string]bool {
		active := make(map[string]bool)
		for _, state := range exec.ActiveStates() {
			active[state.Name] = true
		}
		return active
	}
	if active := activeNames(); !active["heating"] || !active["watching"] {
		t.Fatalf("expected heating and watching active after initialize, got %v", active)
	}
	exec.SendSignal("warm", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if active := activeNames(); !active["idle"] || active["heating"] {
		t.Errorf("warm did not move heating to idle, active states %v", active)
	}
}

// testRegionEntryGuardsReadTheRegionStateAttributes: the entry transitions of a
// region of a parallel state read the attributes that region's own state
// declares, after its entry behavior has run, whether the parallel state is the
// machine itself or a composite state entered below it.
func testRegionEntryGuardsReadTheRegionStateAttributes(t *testing.T) {
	regions := `
			state left {
				attribute cold : Boolean = true;
				entry assign cold := false;
				if cold then on;
				then off;
				state on;
				state off;
			}
			state right {
				attribute cold : Boolean = false;
				entry assign cold := true;
				if cold then on;
				then off;
				state on;
				state off;
			}`
	machines := map[string]string{
		"parallel machine": `package test {
		private import ScalarValues::*;
		state Machine parallel {` + regions + `
		}
	}`,
		"parallel state": `package test {
		private import ScalarValues::*;
		state Machine {
			entry; then outer;
			state outer parallel {` + regions + `
			}
		}
	}`,
	}
	for name, src := range machines {
		exec := stateExecutorForSource(t, "Machine", src)
		active := make(map[string]string)
		for _, state := range exec.ActiveStates() {
			active[exec.graph.ParentState[state].Name] = state.Name
		}
		if active["left"] != "off" || active["right"] != "on" {
			t.Errorf("%s: expected left in off and right in on after their entry behaviors, got %v", name, active)
		}
	}
}

// testLeavingRegionsDescendsThroughEntryTransitions: a transition out of an
// orthogonal region into a composite state outside it starts that state where
// its own entry transitions choose, and the nested state's timer is armed.
func testLeavingRegionsDescendsThroughEntryTransitions(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test {
		private import ScalarValues::*;
		state Machine {
			entry; then both;
			state both parallel {
				state left {
					entry; then l1;
					state l1;
				}
				state right {
					entry; then r1;
					state r1;
				}
			}
			transition first both.left.l1 accept leave then running;
			state running {
				entry; then waiting;
				state waiting;
				transition first waiting accept after 1 [SI::s] then finished;
				state finished;
			}
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("Machine not found")
	}
	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	exec.SendSignal("leave", nil)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("leave: %v", err)
	}
	assertCurrentState(t, exec, "waiting")
	if got := len(ctx.Clock().Waits()); got != 1 {
		t.Fatalf("%d wait(s) on the clock after entering waiting; want its timer armed", got)
	}
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance: %v", err)
	}
	assertCurrentState(t, exec, "finished")
}
