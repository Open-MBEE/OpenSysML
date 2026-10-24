package runtime

import (
	"reflect"
	"testing"
	"time"
)

// TestRuntimeRobustnessDoBodyAccept covers an accept inside a state's do body and how it
// yields to the transitions, choices and regions dispatched around it.
func TestRuntimeRobustnessDoBodyAccept(t *testing.T) {
	t.Run("state_do_body_accept_waits_for_the_message", testStateDoBodyAcceptWaitsForTheMessage)
	t.Run("state_do_body_accept_is_decided_for_a_send", testStateDoBodyAcceptIsDecidedForASend)
	t.Run("state_do_body_accept_yields_to_a_transition", testStateDoBodyAcceptYieldsToATransition)
	t.Run("state_do_body_accept_yields_to_a_substate_transition", testStateDoBodyAcceptYieldsToASubstateTransition)
	t.Run("state_do_body_accept_yields_to_a_substate_transition_leaving_it", testStateDoBodyAcceptYieldsToASubstateTransitionLeavingIt)
	t.Run("state_do_body_accept_follows_the_transition_chosen", testStateDoBodyAcceptFollowsTheTransitionChosen)
	t.Run("state_do_body_accept_yields_to_an_open_choice", testStateDoBodyAcceptYieldsToAnOpenChoice)
	t.Run("state_do_body_accept_yields_to_a_transition_into_its_region", testStateDoBodyAcceptYieldsToATransitionIntoItsRegion)
	t.Run("state_do_body_accept_runs_before_the_choice_reads", testStateDoBodyAcceptRunsBeforeTheChoiceReads)
	t.Run("state_choice_route_reads_the_accepted_payload", testStateChoiceRouteReadsTheAcceptedPayload)
	t.Run("state_do_body_accept_shares_the_dispatch_with_a_region", testStateDoBodyAcceptSharesTheDispatchWithARegion)
	t.Run("state_do_body_accept_shares_the_dispatch_with_a_fork_in_a_region", testStateDoBodyAcceptSharesTheDispatchWithAForkInARegion)
	t.Run("state_do_body_nested_accept_cancelled_on_exit", testStateDoBodyNestedAcceptCancelledOnExit)
}

// testStateDoBodyAcceptWaitsForTheMessage: an accept in an inline do body's flow
// with no message in flight suspends the machine in its state, as a transition
// triggered by a signal does, rather than hanging or deadlocking; the message
// posted later lets the body finish and the state complete.
func testStateDoBodyAcceptWaitsForTheMessage(t *testing.T) {
	var exec *StateExecutor
	done := make(chan error, 1)
	go func() {
		exec = stateWithDoBody(t, `
			first start;
			then action reader accept n : Integer;
			then action count assign total := n;
			then done;
		`)
		done <- exec.RunToCompletion()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-watchdog(10 * time.Second):
		t.Fatal("a do body waiting for a message did not suspend")
	}
	if exec.State() != StateSuspended || StateVertexName(exec.CurrentState()) != "active" {
		t.Fatalf("state = %v in %s, want suspended in active", exec.State(), StateVertexName(exec.CurrentState()))
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Error("a do body parked at its accept must not be due with no message in flight")
	}
	nine := integerValue(9)
	exec.ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &nine})
	if exec.HasPendingDoWork() || !exec.HasPendingSignal() {
		t.Fatal("the message in flight must be one the machine dispatches, to the parked do body")
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after the message: %v", err)
	}
	if exec.State() != StateCompleted {
		t.Errorf("state = %v in %s, want completed", exec.State(), StateVertexName(exec.CurrentState()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(9)) {
		t.Errorf("total = %v, want 9: the accepted value", total)
	}
}

// testStateDoBodyAcceptIsDecidedForASend: a signal a do body is parked at an
// accept for is one the machine takes, though no transition fires on it: the
// previews say so, without moving the body, and the dispatch lets it go on.
func testStateDoBodyAcceptIsDecidedForASend(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	attribute def Other;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 1;
				then done;
			}
		}
		succession first active then finished;
		state finished;
	}
	part def Box { exhibit state w : Waiter; }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "w.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("w.sysml")
	box, err := ctx.Instantiate(resolveSymbol(t, root, "Box"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := box.ExhibitedState()
	if !ok {
		t.Fatal("the box exhibits no machine")
	}
	exec := behavior.State
	if activeLeaf(exec) != "active" || exec.HasPendingDoWork() {
		t.Fatalf("state %s with pending do work %v, want parked in active", activeLeaf(exec), exec.HasPendingDoWork())
	}
	other, err := ctx.SignalMessage(resolveSymbol(t, root, "Other"), nil, box)
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := exec.AcceptsMessage(other); err != nil || accepted {
		t.Errorf("AcceptsMessage(Other) = %v, %v; want false: the accept names Go", accepted, err)
	}
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, box)
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := exec.AcceptsMessage(goMsg); err != nil || !accepted {
		t.Errorf("AcceptsMessage(Go) = %v, %v; want true: the do body is parked at accept Go", accepted, err)
	}
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	if len(decision.Fires) != 0 || len(decision.Resumes) != 1 || decision.Resumes[0] != "do behavior of state active" {
		t.Errorf("Decide(Go) = %+v, want only the do behavior of active resumed", decision)
	}
	if activeLeaf(exec) != "active" || exec.HasPendingDoWork() || len(ctx.PendingMessages()) != 0 {
		t.Fatal("the previews must leave the machine, the body and the bus as they were")
	}
	ctx.PostMessage(goMsg)
	if exec.HasPendingDoWork() || !exec.HasPendingSignal() {
		t.Fatal("the message in flight must be one the machine dispatches, to the parked do body")
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || dispatch.Fired || len(dispatch.Resumed) != 1 || dispatch.Resumed[0] != decision.Resumes[0] {
		t.Errorf("dispatch = %+v, %v; want the do behavior of active resumed, as decided", dispatch, ok)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after the message: %v", err)
	}
	if activeLeaf(exec) != "finished" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want finished with the message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the body went on past its accept once", total)
	}
}

// testStateDoBodyAcceptYieldsToATransition: a signal both a transition out of the
// active state and its do behavior, parked at an accept, would take goes to the
// transition alone — the behavior is abandoned when the state is left — and
// Decide reports just that, before.
func testStateDoBodyAcceptYieldsToATransition(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
		}
		transition leave first active accept Go then stopped;
		state stopped { entry assign total := total + 1; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition leave"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition takes it alone", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "stopped" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want stopped with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of stopped alone, the do behavior abandoned at its accept", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() || len(ctx.Clock().Waits()) != 0 {
		t.Error("nothing of the abandoned do behavior must remain due")
	}
}

// testStateDoBodyAcceptYieldsToASubstateTransition: a signal a transition between
// two substates of the active state accepts goes to the transition alone: the
// state's own do behavior, parked at an accept for it, stays parked — the state
// stays active, so the behavior takes the next occurrence — and Decide reports
// just the transition, before.
func testStateDoBodyAcceptYieldsToASubstateTransition(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
			entry; then left;
			state left;
			transition shift first left accept Go then right;
			state right { entry assign total := total + 1; }
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition shift"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition takes it alone, the do behavior of the state it stays in not too", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "right" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want right with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of right alone, the do behavior still parked at its accept", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Fatal("the do behavior must still be parked at its accept with nothing due")
	}
	decision, err = exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go) again: %v", err)
	}
	want = Decision{Resumes: []string{"do behavior of state active"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) again = %+v, want %+v: no transition accepts it in right, so the do behavior takes it", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the second message: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v, want 11: the do behavior's count on the second occurrence", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Error("the do behavior has ended; nothing of it must remain due")
	}
}

// testStateDoBodyAcceptYieldsToASubstateTransitionLeavingIt: a signal a transition
// out of a substate accepts goes to the transition alone when its target lies
// outside the state whose do behavior is parked for it — that state is left, its
// behavior abandoned — and Decide reports just that, before.
func testStateDoBodyAcceptYieldsToASubstateTransitionLeavingIt(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
			entry; then left;
			state left;
			transition leave first left accept Go then stopped;
		}
		state stopped { entry assign total := total + 1; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition leave"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition leaving the state takes it alone", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "stopped" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want stopped with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of stopped alone, the do behavior abandoned at its accept", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() || len(ctx.Clock().Waits()) != 0 {
		t.Error("nothing of the abandoned do behavior must remain due")
	}
}

// testStateDoBodyAcceptFollowsTheTransitionChosen: a substate with two transitions
// enabled for the signal, one between the enclosing state's substates and one out
// of it. The one the policy chooses (the first declared) stays inside, and takes
// the signal alone all the same, the enclosing do behavior staying parked;
// Decide names the same transition and no resume.
func testStateDoBodyAcceptFollowsTheTransitionChosen(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 10;
				then done;
			}
			entry; then left;
			state left;
			transition shift first left accept Go then right;
			transition leave first left accept Go then stopped;
			state right { entry assign total := total + 1; }
		}
		state stopped { entry assign total := total + 100; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition shift"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition chosen takes it alone", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed, as decided", dispatch, ok)
	}
	if activeLeaf(exec) != "right" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want right with the one message consumed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of right alone, the do behavior still parked", total)
	}
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Error("the do behavior must still be parked at its accept with nothing due")
	}
}

// testStateDoBodyAcceptYieldsToAnOpenChoice: the transition chosen targets a choice
// with a guarded branch to another substate and a default branch out of the
// enclosing state. The choice is read only once the transition is under way, so
// whichever branch it then takes, the enclosing do behavior — parked in a state a
// branch may leave — does not take the signal: the transition takes it alone.
func testStateDoBodyAcceptYieldsToAnOpenChoice(t *testing.T) {
	model := func(stay string) string {
		return `
		private import ScalarValues::*;
		attribute def Go;
		state def Waiter {
			attribute total : Integer = 0;
			attribute stay : Boolean = ` + stay + `;
			entry; then active;
			state active {
				do action work {
					first start;
					then action reader accept Go;
					then action count assign total := total + 10;
					then done;
				}
				entry; then left;
				state left;
				choice pick;
				transition route first left accept Go then pick;
				transition first pick if stay then right;
				transition first pick then stopped;
				state right { entry assign total := total + 1; }
			}
			state stopped { entry assign total := total + 100; }
		}
		part def Box { exhibit state w : Waiter; }
		`
	}
	cases := []struct {
		stay  string
		want  Decision
		leaf  string
		total int64
	}{
		{"true", Decision{Fires: []string{"transition route"}}, "right", 1},
		{"false", Decision{Fires: []string{"transition route"}}, "stopped", 100},
	}
	for _, tc := range cases {
		exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, model(tc.stay))
		decision, err := exec.Decide(goMsg)
		if err != nil {
			t.Fatalf("stay = %s: Decide(Go): %v", tc.stay, err)
		}
		if !reflect.DeepEqual(decision, tc.want) {
			t.Errorf("stay = %s: Decide(Go) = %+v, want %+v: a branch of the open choice may leave the do behavior's state", tc.stay, decision, tc.want)
		}
		ctx.PostMessage(goMsg)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("stay = %s: dispatch the message: %v", tc.stay, err)
		}
		dispatch, ok := exec.LastDispatch()
		if !ok || !dispatch.Fired || !reflect.DeepEqual(dispatch.Resumed, tc.want.Resumes) {
			t.Errorf("stay = %s: dispatch = %+v, %v; want the transition fired and the do behavior resumed as decided", tc.stay, dispatch, ok)
		}
		if activeLeaf(exec) != tc.leaf || len(ctx.PendingMessages()) != 0 {
			t.Errorf("stay = %s: state %s with %d messages in flight, want %s with the one message consumed", tc.stay, activeLeaf(exec), len(ctx.PendingMessages()), tc.leaf)
		}
		if total := exec.StateData()["total"]; !valueEqual(total, integerValue(tc.total)) {
			t.Errorf("stay = %s: total = %v, want %d", tc.stay, total, tc.total)
		}
	}
}

// testStateDoBodyAcceptYieldsToATransitionIntoItsRegion: a transition out of one
// orthogonal region into a sibling region exits the state that region is in on the
// way, so the do behavior parked there does not take the signal the transition
// accepts, in Decide and in the dispatch alike; the signal is consumed once.
func testStateDoBodyAcceptYieldsToATransitionIntoItsRegion(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter parallel {
		attribute total : Integer = 0;
		state left {
			entry; then l1;
			state l1;
			transition swap first l1 accept Go then r2;
		}
		state right {
			entry; then r1;
			state r1 {
				do action work {
					first start;
					then action reader accept Go;
					then action count assign total := total + 10;
					then done;
				}
			}
			state r2 { entry assign total := total + 1; }
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition swap"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition replaces the state the do behavior runs in", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || len(dispatch.Resumed) != 0 {
		t.Errorf("dispatch = %+v, %v; want the transition fired and no do behavior resumed", dispatch, ok)
	}
	assertRegionConfig(t, exec, map[string]string{"right": "r2"})
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("%d messages in flight, want the one message consumed", len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of r2 only, the do behavior of r1 cancelled at its accept", total)
	}
}

// testStateDoBodyAcceptRunsBeforeTheChoiceReads: the do behaviors go on with the
// signal (one node, then yield) before the chosen transition fires, and a choice
// on its route reads its guards only then, so a do behavior in a sibling region
// that rewrites the guard on its way sends the transition down the branch the
// rewritten data selects.
func testStateDoBodyAcceptRunsBeforeTheChoiceReads(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter parallel {
		attribute total : Integer = 0;
		attribute stay : Boolean = true;
		state watcher {
			entry; then work;
			state work {
				do action work {
					first start;
					then action reader accept Go;
					then action flip assign stay := false;
					then action count assign total := total + 10;
					then done;
				}
			}
		}
		state active {
			entry; then left;
			state left;
			choice pick;
			transition route first left accept Go then pick;
			transition first pick if stay then right;
			transition first pick then other;
			state right { entry assign total := total + 1; }
			state other { entry assign total := total + 100; }
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition route"}, Resumes: []string{"do behavior of state work"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || !reflect.DeepEqual(dispatch.Resumed, want.Resumes) {
		t.Errorf("dispatch = %+v, %v; want the transition fired and the do behavior resumed as decided", dispatch, ok)
	}
	assertRegionConfig(t, exec, map[string]string{"watcher": "work", "active": "other"})
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("%d messages in flight, want the one message consumed: the choice read stay after the do behavior cleared it", len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(100)) {
		t.Errorf("total = %v, want 100: the entry of other, the do behavior yielded after flip with count still to run", total)
	}
	if !exec.HasPendingDoWork() {
		t.Fatal("the do behavior must be due to go on with count")
	}
	if _, err := exec.RunDoRound(); err != nil {
		t.Fatalf("run the do round: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(110)) {
		t.Errorf("total = %v, want 110: the do behavior's count in the round after the dispatch", total)
	}
}

// testStateDoBodyAcceptSharesTheDispatchWithARegion: a signal a do behavior in one
// orthogonal region is parked at an accept for and a transition in a sibling
// region accepts is dispatched once to both, the behavior going on from its accept
// before the transition fires, and Decide reports both, before.
func testStateDoBodyAcceptSharesTheDispatchWithARegion(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter parallel {
		attribute total : Integer = 0;
		state left {
			entry; then lwork;
			state lwork {
				do action work {
					first start;
					then action reader accept Go;
					then action count assign total := total + 10;
					then done;
				}
			}
		}
		state right {
			entry; then rwait;
			state rwait;
			transition leave first rwait accept Go then rdone;
			state rdone { entry assign total := total + 1; }
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition leave"}, Resumes: []string{"do behavior of state lwork"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the transition and the do behavior of the sibling region both take it", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || !reflect.DeepEqual(dispatch.Resumed, want.Resumes) {
		t.Errorf("dispatch = %+v, %v; want the transition fired and the do behavior resumed, as decided", dispatch, ok)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("%d messages in flight, want the one message consumed", len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v, want 11: the do behavior's count, then the entry of rdone", total)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run on: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(11)) {
		t.Errorf("total = %v after running on, want 11 still", total)
	}
}

// testStateDoBodyAcceptSharesTheDispatchWithAForkInARegion: the transition the
// sibling region takes leads into a fork whose branches stay within that region,
// so the do behavior's region is kept and the behavior goes on with the signal.
func testStateDoBodyAcceptSharesTheDispatchWithAForkInARegion(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go;
	state def Waiter parallel {
		attribute total : Integer = 0;
		state left {
			entry; then lwork;
			state lwork {
				do action work {
					first start;
					then action reader accept Go;
					then action count assign total := total + 10;
					then done;
				}
			}
		}
		state right {
			entry; then rwait;
			state rwait;
			fork split;
			transition leave first rwait accept Go then split;
			state rwork parallel {
				state ra { state a { entry assign total := total + 1; } }
				state rb { state b { entry assign total := total + 2; } }
			}
			transition first split then a;
			transition first split then b;
		}
	}
	part def Box { exhibit state w : Waiter; }
	`
	exec, ctx, goMsg := boxDoBehaviorParkedAtGo(t, src)
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition leave"}, Resumes: []string{"do behavior of state lwork"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v: the fork leaves only its own region, so the sibling's do behavior takes the signal too", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	dispatch, ok := exec.LastDispatch()
	if !ok || !dispatch.Fired || !reflect.DeepEqual(dispatch.Resumed, want.Resumes) {
		t.Errorf("dispatch = %+v, %v; want the transition fired and the do behavior resumed, as decided", dispatch, ok)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("%d messages in flight, want the one message consumed", len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(13)) {
		t.Errorf("total = %v, want 13: the do behavior's count, then the entries of a and b", total)
	}
}

// testStateChoiceRouteReadsTheAcceptedPayload: a choice guard along the chosen
// transition's route reads the payload the accept binds, so the route is settled
// with the payload bound and the branch the payload selects is the one entered.
func testStateChoiceRouteReadsTheAcceptedPayload(t *testing.T) {
	src := `
	private import ScalarValues::*;
	attribute def Go { attribute level : Integer; }
	state def Waiter {
		attribute total : Integer = 0;
		entry; then left;
		state left;
		choice pick;
		transition route first left accept g : Go then pick;
		transition first pick if g.level > 0 then right;
		transition first pick then stopped;
		state right { entry assign total := total + 1; }
		state stopped { entry assign total := total + 100; }
	}
	part def Box { exhibit state w : Waiter; }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "w.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("w.sysml")
	box, err := ctx.Instantiate(resolveSymbol(t, root, "Box"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := box.ExhibitedState()
	if !ok {
		t.Fatal("the box exhibits no machine")
	}
	exec := behavior.State
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), map[string]Value{"level": integerValue(1)}, box)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := exec.Decide(goMsg)
	if err != nil {
		t.Fatalf("Decide(Go): %v", err)
	}
	want := Decision{Fires: []string{"transition route"}}
	if !reflect.DeepEqual(decision, want) {
		t.Errorf("Decide(Go) = %+v, want %+v", decision, want)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("dispatch the message: %v", err)
	}
	if activeLeaf(exec) != "right" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("state %s with %d messages in flight, want right with the one message consumed: g.level was 1 when the choice was routed", activeLeaf(exec), len(ctx.PendingMessages()))
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Errorf("total = %v, want 1: the entry of right", total)
	}
}

// boxDoBehaviorParkedAtGo instantiates Box from the source, whose exhibited machine
// starts a do behavior that parks at an accept of Go, and builds a Go for it.
func boxDoBehaviorParkedAtGo(t *testing.T, src string) (*StateExecutor, *Context, Message) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "w.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("w.sysml")
	box, err := ctx.Instantiate(resolveSymbol(t, root, "Box"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := box.ExhibitedState()
	if !ok {
		t.Fatal("the box exhibits no machine")
	}
	exec := behavior.State
	if exec.HasPendingDoWork() || exec.HasPendingSignal() {
		t.Fatal("the do behavior must be parked at its accept with nothing due")
	}
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, box)
	if err != nil {
		t.Fatal(err)
	}
	return exec, ctx, goMsg
}

// testStateDoBodyNestedAcceptCancelledOnExit: an accept nested one node deep in
// a do body, for a signal nothing sends, parks the body while the machine waits
// for its timed exit; leaving the state cancels the body, leaving no waiter
// behind and running nothing after the accept.
func testStateDoBodyNestedAcceptCancelledOnExit(t *testing.T) {
	exec := stateMachineWithLibraries(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		attribute def Go;
		state Machine {
			attribute total : Integer = 0;
			entry; then active;
			state active {
				do action ops {
					first start;
					then action inner {
						first start;
						then action w accept g : Go;
						then action c assign total := 1;
						then done;
					}
					then done;
				}
			}
			transition first active accept after 10 [s] then finished;
			state finished;
		}
	}`)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "finished")
	if now := exec.ctx.Clock().Now(); now != 10 {
		t.Errorf("clock at %v, want 10: the exit transition's instant", now)
	}
	if exec.HasPendingDoWork() {
		t.Error("leaving the state must end its parked do body")
	}
	if got := len(exec.ctx.Clock().Waits()); got != 0 {
		t.Errorf("%d wait(s) left on the clock after the state exited", got)
	}
	exec.ctx.PostMessage(Message{SignalType: "Go", Target: "w"})
	if exec.HasPendingDoWork() {
		t.Error("a message after the exit must not revive the cancelled body")
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(0)) {
		t.Errorf("total = %v, want 0: the node after the cancelled accept must not run", total)
	}
}
