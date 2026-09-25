package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessExitParameters exercises the failure modes of an exit
// action reading the leaving transition's payload, `T.d`: a payload of the wrong
// type, a transition not taken read by a parameter needing a value, a signal
// carrying no payload, a call missing a declared argument, and a transition not
// taken read with a fallback. Each is a typed error or an empty reading, never a
// panic, and a refused firing leaves the machine where it was.
func TestRuntimeRobustnessExitParameters(t *testing.T) {
	t.Run("payload_of_the_wrong_type", testExitParametersTypeMismatch)
	t.Run("transition_not_taken_binds_nothing", testExitParametersNotTakenNeedsOne)
	t.Run("signal_carrying_no_payload", testExitParametersPayloadAbsent)
	t.Run("call_missing_an_argument_is_not_taken", testExitParametersCallArity)
	t.Run("transition_not_taken_reads_the_fallback", testExitParametersNotTakenFallback)
}

// exitParametersModel: `idle` is left by `go` or `halt`, its exit reading whichever
// is taken; `typed` is left by `again`, whose Integer payload its exit binds to a
// Boolean; `strict` is left by `pass`, its exit reading `go`, which is not taken.
const exitParametersModel = `package test {
	private import ScalarValues::*;
	attribute def Go :> Integer;
	attribute def Halt :> Integer;
	attribute def Pass;
	state def Machine {
		attribute exited : Integer = -1;
		attribute flagged : Boolean = false;
		attribute strictly : Integer = -1;
		entry; then idle;
		state idle {
			exit action {
				in level : Integer = go.l ?? halt.h ?? 0;
				assign exited := level;
			}
		}
		state typed {
			exit action {
				in flag : Boolean = again.l;
				assign flagged := flag;
			}
		}
		state strict {
			exit action {
				in level : Integer = go.l;
				assign strictly := level;
			}
		}
		state passed;
		transition go first idle accept l : Go then typed;
		transition halt first idle accept h : Halt then strict;
		transition again first typed accept l : Go then done;
		transition pass first strict accept Pass then passed;
		transition first passed then done;
	}
}`

// exitParametersCallModel: `set` takes a `setSpeed` call carrying both arguments;
// the exit of `idle` reads one of them.
const exitParametersCallModel = `package test {
	private import ScalarValues::*;
	state def Machine {
		attribute exited : Integer = -1;
		entry; then idle;
		state idle {
			exit action {
				in v : Integer = set.speed;
				assign exited := v;
			}
		}
		transition set first idle accept setSpeed(speed, gear) then done;
	}
}`

// exitParametersMachine starts the model's machine and drives it to `idle`.
func exitParametersMachine(t *testing.T, model string) *StateExecutor {
	t.Helper()
	exec := callMachineOf(t, model)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatal(err)
	}
	return exec
}

// exitParametersSend queues a signal carrying one value and runs the machine.
func exitParametersSend(t *testing.T, exec *StateExecutor, signal string, value Value) error {
	t.Helper()
	if err := exec.Enqueue(QueuedEvent{Signal: signal, Value: &value}); err != nil {
		t.Fatal(err)
	}
	return exec.RunToCompletion()
}

// testExitParametersTypeMismatch: the Integer payload `again` carries cannot bind
// the exit's Boolean parameter; the firing is refused and `typed` stays active.
func testExitParametersTypeMismatch(t *testing.T) {
	exec := exitParametersMachine(t, exitParametersModel)
	if err := exitParametersSend(t, exec, "Go", constInt(3)); err != nil {
		t.Fatal(err)
	}
	err := exitParametersSend(t, exec, "Go", constInt(4))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("exit binding an Integer to a Boolean = %v; want ErrTypeMismatch", err)
	}
	if got := activeLeafName(exec); got != "typed" {
		t.Errorf("active state after the refusal = %v, want typed", got)
	}
	if flagged := exec.stateData["flagged"]; flagged.Const.Bool {
		t.Errorf("flagged = true after the refusal, want the exit's write undone")
	}
}

// testExitParametersNotTakenNeedsOne: `strict` is left by `pass`, so `go.l` reads
// empty, which its [1] parameter refuses as a multiplicity violation.
func testExitParametersNotTakenNeedsOne(t *testing.T) {
	exec := exitParametersMachine(t, exitParametersModel)
	if err := exitParametersSend(t, exec, "Halt", constInt(8)); err != nil {
		t.Fatal(err)
	}
	if exited := exec.stateData["exited"]; exited.Const.Int != 8 {
		t.Errorf("exited = %v after Halt, want 8 from halt.h", exited)
	}
	exec.SendSignal("Pass", nil)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("exit reading a transition not taken = %v; want ErrMultiplicityViolation", err)
	}
	if got := activeLeafName(exec); got != "strict" {
		t.Errorf("active state after the refusal = %v, want strict", got)
	}
}

// testExitParametersPayloadAbsent: a `Go` carrying no value cannot bind the
// trigger's payload, so the transition is refused before the exit is performed.
func testExitParametersPayloadAbsent(t *testing.T) {
	exec := exitParametersMachine(t, exitParametersModel)
	exec.SendSignal("Go", nil)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("Go without a payload = %v; want ErrNoValue", err)
	}
	if exited := exec.stateData["exited"]; exited.Const.Int != -1 {
		t.Errorf("exited = %v, want the exit not performed", exited)
	}
}

// testExitParametersCallArity: a `setSpeed` call missing `gear` does not fire
// `set`, so the exit is not performed and `idle` stays active; a call carrying
// both fires it and the exit reads `speed`.
func testExitParametersCallArity(t *testing.T) {
	exec := exitParametersMachine(t, exitParametersCallModel)
	exec.InvokeOperation("setSpeed", map[string]Value{"speed": constInt(5)})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatal(err)
	}
	if got := activeLeafName(exec); got != "idle" {
		t.Errorf("active state after the partial call = %v, want idle", got)
	}
	if exited := exec.stateData["exited"]; exited.Const.Int != -1 {
		t.Errorf("exited = %v after the partial call, want the exit not performed", exited)
	}
	exec.InvokeOperation("setSpeed", map[string]Value{"speed": constInt(5), "gear": constInt(2)})
	if err := exec.RunToCompletion(); err != nil {
		t.Fatal(err)
	}
	if exited := exec.stateData["exited"]; exited.Const.Int != 5 {
		t.Errorf("exited = %v after the full call, want 5 from set.speed", exited)
	}
}

// testExitParametersNotTakenFallback: `idle` left by `halt` reads `go.l` as empty,
// so the exit's `??` chain falls through to `halt.h`.
func testExitParametersNotTakenFallback(t *testing.T) {
	exec := exitParametersMachine(t, exitParametersModel)
	if err := exitParametersSend(t, exec, "Go", constInt(3)); err != nil {
		t.Fatal(err)
	}
	if exited := exec.stateData["exited"]; exited.Const.Int != 3 {
		t.Errorf("exited = %v after Go, want 3 from go.l", exited)
	}
	exec = exitParametersMachine(t, exitParametersModel)
	if err := exitParametersSend(t, exec, "Halt", constInt(8)); err != nil {
		t.Fatal(err)
	}
	if exited := exec.stateData["exited"]; exited.Const.Int != 8 {
		t.Errorf("exited = %v after Halt, want 8 from halt.h", exited)
	}
}

// activeLeafName is the machine's single active leaf, "" when none is.
func activeLeafName(exec *StateExecutor) string {
	leaves := exec.ActiveLeaves()
	if len(leaves) != 1 {
		return ""
	}
	return leaves[0].Name
}
