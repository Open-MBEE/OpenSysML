package runtime

import (
	"strings"
	"testing"
)

// A machine whose only outgoing transition is a change trigger progresses under
// RunToCompletion: the condition is re-tested by the run itself, not by whatever
// external driver happens to poll.
func TestChangeTriggerRunsWithoutAnExternalPoll(t *testing.T) {
	outputs, visits, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			attribute log : Integer = 0;
			entry; then start;
			state start;
			state waiting;
			accept when ready then done;
			state done { entry { assign log := 1; } }
			succession first start then waiting;
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := []string{"start", "waiting", "done"}; !equalStrings(visits, want) {
		t.Errorf("visits = %v, want %v", visits, want)
	}
	if log := outputs["log"]; log.Kind != ValConst || log.Const.Int != 1 {
		t.Errorf("log = %v, want 1: the change transition did not fire", log)
	}
}

// A change condition risen by a do behavior is taken by the step after it, which
// is what re-testing per micro-step buys: the run neither waits for an event nor
// misses the rise.
func TestChangeTriggerFiresOnRiseFromDoBehavior(t *testing.T) {
	_, visits, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			attribute count : Integer = 0;
			entry; then start;
			state start;
			state counting {
				do action tick {
					assign count := count + 1;
					assign count := count + 1;
				}
			}
			accept when count >= 2 then done;
			state done;
			succession first start then counting;
		}
	}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := []string{"start", "counting", "done"}; !equalStrings(visits, want) {
		t.Errorf("visits = %v, want %v", visits, want)
	}
}

// A false change condition is not quiescence: the machine suspends, and says
// which condition it is waiting on rather than reporting silent completion.
func TestChangeTriggerFalseConditionIsReported(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			entry; then start;
			state start;
			state waiting;
			accept when ready then done;
			state done;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v, want suspended", exec.State())
	}
	reason := exec.SuspendReason()
	if !strings.Contains(reason, "waiting on change condition") || !strings.Contains(reason, "condition is false") {
		t.Errorf("reason = %q, want the false change condition it waits on", reason)
	}
	if !strings.Contains(reason, "when ready") {
		t.Errorf("reason = %q, want the trigger as written", reason)
	}

	// The condition rising makes the transition due; the same run takes it.
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if reason := exec.SuspendReason(); !strings.HasPrefix(reason, "quiesced") {
		t.Errorf("reason = %q, want quiescence once no condition is watched", reason)
	}
}

// A machine watching no change condition reports quiescence, so the two cases a
// stalled machine can be in stay distinguishable.
func TestQuiescedMachineReportsNoChangeCondition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then start;
			state start;
			state waiting;
			accept sig then done;
			state done;
			succession first start then waiting;
		}
		attribute def sig;
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	reason := exec.SuspendReason()
	if !strings.HasPrefix(reason, "quiesced") {
		t.Errorf("reason = %q, want quiescence", reason)
	}
}

// A condition that stays true fires its edge once: a self-transition on a
// lasting condition suspends instead of exhausting the event budget.
func TestChangeTriggerDoesNotRefireUnchangedCondition(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			attribute laps : Integer = 0;
			entry; then start;
			state start;
			state waiting;
			succession first start then waiting;
			transition first waiting accept when ready do assign laps := laps + 1 then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v, want suspended", exec.State())
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 1 {
		t.Errorf("laps = %v, want 1: the edge fired other than once on the rise", laps)
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "has not changed since it fired") {
		t.Errorf("reason = %q, want the already-fired condition", reason)
	}

	// Observing the condition false re-arms the edge, so the next rise fires.
	exec.stateData["ready"] = boolValue(false)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun false: %v", err)
	}
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun true: %v", err)
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 2 {
		t.Errorf("laps = %v, want 2: the re-armed edge did not fire on the next rise", laps)
	}
}

// A guard blocking a risen condition consumes nothing: the transition stays
// armed and the guard is re-tested, and the wait names the guard.
func TestChangeTriggerGuardBlocks(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			attribute allowed : Boolean = false;
			entry; then start;
			state start;
			state waiting;
			state done;
			succession first start then waiting;
			transition first waiting accept when ready if allowed then done;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "waiting")
	if reason := exec.SuspendReason(); !strings.Contains(reason, "guard is false") {
		t.Errorf("reason = %q, want the blocking guard", reason)
	}

	exec.stateData["allowed"] = boolValue(true)
	fired, err := exec.PollChangeEvents()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if !fired {
		t.Fatal("poll fired nothing once the guard passed")
	}
	assertCurrentState(t, exec, "done")
}

// Polling costs no event budget of its own: a machine whose conditions stay
// false runs to suspension under a budget of one event.
func TestChangeTriggerPollingKeepsEventBudget(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			entry; then start;
			state start;
			state waiting {
				do action tick { assign ready := ready; }
			}
			accept when ready then done;
			state done;
			succession first start then waiting;
		}
	}`)
	exec.ctx.maxStateEvents = 1
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v, want suspended", exec.State())
	}
}

// A self-transition on a composite state leaves and re-enters it, and that exit
// must not re-arm the edge that caused it: the condition stays true, so the edge
// is taken once rather than spinning until the budget aborts. It fires again once
// the condition has fallen and risen.
func TestChangeTriggerOnACompositeSelfTransitionFiresOnce(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			attribute laps : Integer = 0;
			entry; then start;
			state start;
			state Working {
				entry; then w0;
				state w0;
				state inner;
				transition first w0 then inner;
			}
			succession first start then Working;
			transition first Working accept when ready do assign laps := laps + 1 then Working;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 1 {
		t.Fatalf("laps = %v, want 1: the re-entry re-armed the edge that caused it", laps)
	}

	// The condition falling and rising again is a new edge, so it fires once more.
	exec.stateData["ready"] = boolValue(false)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("lower condition: %v", err)
	}
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("raise condition: %v", err)
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 2 {
		t.Errorf("laps = %v, want 2: the risen condition did not fire again", laps)
	}
}

// A change watch belongs to an activation of its state: leaving and re-entering
// the state arms it again, even though its condition never went false.
func TestChangeTriggerRearmsOnStateExit(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		attribute def Back;
		state Machine {
			attribute ready : Boolean = true;
			entry; then start;
			state start;
			state waiting;
			accept when ready then working;
			state working;
			accept Back then waiting;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "working")

	exec.SendSignal("Back", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	assertCurrentState(t, exec, "working")
	if got := countVisits(exec.stateVisits, "working"); got != 2 {
		t.Errorf("working visited %d time(s), want 2: the re-entered watch stayed latched", got)
	}
}

// One rise is one occurrence: a transition that lost conflict resolution to a
// nested one is consumed by that rise too, so the enclosing state it would leave
// stays active until the condition falls and rises again.
func TestChangeTriggerConsumesTheRiseForALosingTransition(t *testing.T) {
	source := `package test {
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
	}`
	exec := stateExecutorForSource(t, "sm", source)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("raise condition: %v", err)
	}
	if active := activeStates(exec); active["Done"] || !active["l2"] || !active["r1"] {
		t.Errorf("active = %v, want l2 | r1: the losing transition fired on a later poll", active)
	}

	// The condition falling and rising again is a new occurrence, which the
	// enclosing transition is now the only one left to take.
	exec.stateData["ready"] = boolValue(false)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("lower condition: %v", err)
	}
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("raise condition again: %v", err)
	}
	if active := activeStates(exec); !active["Done"] {
		t.Errorf("active = %v, want Done: the new rise did not reach the enclosing transition", active)
	}
}

// A state a firing re-enters mid-poll gets a fresh watch: the rise the poll
// observed before that entry belongs to the activation that is gone, so it must
// not latch the watch of the new one.
func TestChangeTriggerArmsAWatchReenteredDuringThePoll(t *testing.T) {
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
				}
				state right {
					entry; then rstart;
					state rstart;
					state C parallel {
						state inner {
							entry; then cstart;
							state cstart;
							state c2;
							transition first cstart then c2;
						}
					}
					state r2;
					transition first rstart then C;
					transition first C accept when ready then r2;
				}
			}
			fork split;
			transition first l1 accept when ready then split;
			transition first split then l2;
			transition first split then C;
			succession first start then Working;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	// The fork re-enters C, so the watch out of C is armed again; its own
	// candidate is skipped because the leaf it was selected for is no longer active.
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("raise condition: %v", err)
	}
	if active := activeStates(exec); !active["r2"] || !active["l2"] {
		t.Errorf("active = %v (%s), want l2 | r2: the re-entered watch stayed latched",
			active, exec.SuspendReason())
	}
}

// activeStates is the executor's active configuration, by state name.
func activeStates(exec *StateExecutor) map[string]bool {
	active := make(map[string]bool)
	for _, state := range exec.ActiveStates() {
		active[state.Name] = true
	}
	return active
}

// A machine that has not been stepped yet makes no claim about progress: the
// initial transition it has queued is pending work.
func TestSuspendReasonMakesNoClaimBeforeTheMachineIsStepped(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = true;
			entry; then start;
			state start;
			state waiting;
			accept when ready then done;
			state done;
			succession first start then waiting;
		}
	}`)
	if reason := exec.SuspendReason(); reason != "" {
		t.Errorf("reason = %q, want none before the machine is stepped", reason)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertCurrentState(t, exec, "done")
}

// A signal sent to a suspended machine is a step it can take, so the report
// stops claiming the machine is stuck on its false condition.
func TestSuspendReasonMakesNoClaimOnceASignalIsQueued(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		attribute def Go;
		state Machine {
			attribute ready : Boolean = false;
			entry; then start;
			state start;
			state waiting;
			accept when ready then done;
			accept Go then done;
			state done;
			succession first start then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "condition is false") {
		t.Errorf("reason = %q, want the false condition it waits on", reason)
	}

	exec.SendSignal("Go", nil)
	if reason := exec.SuspendReason(); reason != "" {
		t.Errorf("reason = %q, want none while a queued signal can still be dispatched", reason)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	assertCurrentState(t, exec, "done")
}

// A guard blocked by an earlier candidate's effect in the same poll stays armed:
// the fire path re-tests the guard, so latching such an edge would lose it for as
// long as its condition stays true.
func TestChangeTriggerKeepsAnEdgeBlockedDuringThePoll(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			attribute allowed : Boolean = true;
			attribute laps : Integer = 0;
			entry; then start;
			state start;
			state Working parallel {
				state alpha {
					entry; then f0;
					state f0;
					state fWait;
					state fDone;
					transition first f0 then fWait;
					transition first fWait accept when ready do assign allowed := false then fDone;
				}
				state beta {
					entry; then s0;
					state s0;
					state sWait;
					state sDone { entry { assign laps := 1; } }
					transition first s0 then sWait;
					transition first sWait accept when ready if allowed then sDone;
				}
			}
			succession first start then Working;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}

	// Both regions watch the condition, so raising it puts both transitions in
	// one poll and the first one's effect blocks the second one's guard.
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("raise condition: %v", err)
	}
	if !containsState(exec.stateVisits, "fDone") {
		t.Fatalf("the first region did not fire, visits: %v", exec.stateVisits)
	}
	if containsState(exec.stateVisits, "sDone") {
		t.Fatalf("the second region fired against a blocked guard, visits: %v", exec.stateVisits)
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "guard is false") {
		t.Errorf("reason = %q, want the guard blocked during the poll", reason)
	}

	// The blocked edge is still armed, so it fires once its guard passes even
	// though its condition never went false.
	exec.stateData["allowed"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if laps := exec.stateData["laps"]; laps.Kind != ValConst || laps.Const.Int != 1 {
		t.Errorf("laps = %v, want 1: the blocked edge was lost", laps)
	}
}

// A change condition moving the machine out of a deferring state recalls the
// events that state held back: the run dispatches the deferred signal rather
// than declaring itself settled with the signal still held.
func TestChangeTriggerRecallsDeferredEvents(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		attribute def Ping;
		state Machine {
			attribute ready : Boolean = false;
			entry; then start;
			state start;
			state busy {
				defer Ping;
			}
			accept when ready then waiting;
			state waiting;
			accept Ping then done;
			state done;
			succession first start then busy;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	exec.SendSignal("Ping", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("deliver Ping: %v", err)
	}
	assertCurrentState(t, exec, "busy")
	if len(exec.deferred) != 1 {
		t.Fatalf("deferred = %d, want the Ping held by busy", len(exec.deferred))
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "still deferred") {
		t.Errorf("reason = %q, want the deferred event it holds", reason)
	}

	// The condition rising leaves busy, so the Ping it deferred is dispatched by
	// the same run rather than stranded.
	exec.stateData["ready"] = boolValue(true)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	assertCurrentState(t, exec, "done")
	if len(exec.deferred) != 0 {
		t.Errorf("deferred = %d, want the recalled Ping delivered", len(exec.deferred))
	}
}

// equalStrings compares two ordered lists of names.
func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
