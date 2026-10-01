package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessGuardPayload exercises the failure modes of a guard that
// reads a payload through its transition's name: a payload of the wrong type, a
// member the transition binds nothing under, a transition whose trigger binds
// no payload at all, and a read that leaves the guard non-Boolean. Each is a typed
// error; none panics or hangs.
func TestRuntimeRobustnessGuardPayload(t *testing.T) {
	t.Run("payload_of_the_wrong_type", testGuardPayloadWrongType)
	t.Run("unknown_payload_name_on_the_transition", testGuardPayloadUnknownName)
	t.Run("transition_without_a_trigger", testGuardPayloadTransitionWithoutTrigger)
	t.Run("guard_non_boolean_after_the_read", testGuardPayloadNonBoolean)
}

// guardPayloadMachine is a machine whose transition `raise` out of idle carries
// the given trigger and guard, driven by one Level signal.
func guardPayloadMachine(trigger, guard string) string {
	return `package test {
		private import ScalarValues::*;
		attribute def Level :> Integer;
		state Machine {
			entry; then idle;
			state idle;
			state high;
			transition raise first idle ` + trigger + ` if ` + guard + ` then high;
		}
	}`
}

// guardPayloadError sends one Level carrying value to the machine and returns
// the error its run ends with, failing the test if the run hangs or succeeds.
func guardPayloadError(t *testing.T, model string, value Value) error {
	t.Helper()
	exec := stateExecutorForSource(t, "Machine", model)
	exec.enqueueSignal(Message{SignalType: "Level", Value: &value})
	done := make(chan error, 1)
	go func() { done <- exec.RunToCompletion() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected the guard's payload read to fail the run")
		}
		return err
	case <-watchdog(10 * time.Second):
		t.Fatal("RunToCompletion hung on a guard reading its transition's payload")
	}
	return nil
}

// testGuardPayloadWrongType: the payload arrives as a string, so comparing
// `raise.l` with an Integer is the operator's type error, not a silent false.
func testGuardPayloadWrongType(t *testing.T) {
	err := guardPayloadError(t, guardPayloadMachine("accept l : Level", "raise.l > 5"), NewStringValue("eight"))
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "eval guard of transition raise: type mismatch: operator '>' is not defined for a string and an Integer") {
		t.Fatalf("error = %v, want ErrTypeMismatch naming the guard's operands", err)
	}
}

// testGuardPayloadUnknownName: `raise.level` names nothing the transition's
// trigger binds, so the read is the unresolved-reference error naming the member.
func testGuardPayloadUnknownName(t *testing.T) {
	err := guardPayloadError(t, guardPayloadMachine("accept l : Level", "raise.level > 5"), intConst(8))
	if !errors.Is(err, ErrUnresolvedReference) || !strings.Contains(err.Error(), "eval guard of transition raise: unresolved reference: raise has no member level") {
		t.Fatalf("error = %v, want ErrUnresolvedReference naming the member", err)
	}
}

// testGuardPayloadTransitionWithoutTrigger: a bare `accept Level` binds no
// payload name, so `raise.l` names no member of the transition.
func testGuardPayloadTransitionWithoutTrigger(t *testing.T) {
	err := guardPayloadError(t, guardPayloadMachine("accept Level", "raise.l > 5"), intConst(8))
	if !errors.Is(err, ErrUnresolvedReference) || !strings.Contains(err.Error(), "eval guard of transition raise: unresolved reference: raise has no member l") {
		t.Fatalf("error = %v, want ErrUnresolvedReference over a transition binding no payload", err)
	}
}

// testGuardPayloadNonBoolean: the read succeeds, but an Integer payload is no
// guard verdict; the guard's type error names what it got.
func testGuardPayloadNonBoolean(t *testing.T) {
	err := guardPayloadError(t, guardPayloadMachine("accept l : Level", "raise.l"), intConst(8))
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "guard of transition raise must be boolean, got an Integer") {
		t.Fatalf("error = %v, want ErrTypeMismatch over a non-Boolean guard", err)
	}
}
