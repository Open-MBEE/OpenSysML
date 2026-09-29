package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessAssignChain exercises a chained write through a step
// holding a collection: one element names that object and the write reaches it,
// none names an uninitialized step, and several names no one object to write on.
func TestRuntimeRobustnessAssignChain(t *testing.T) {
	t.Run("chain_step_holding_one_object_writes_it", testAssignChainStepHoldingOneObject)
	t.Run("chain_step_holding_several_objects_is_refused", testAssignChainStepHoldingSeveralObjects)
	t.Run("chain_step_holding_nothing_is_uninitialized", testAssignChainStepHoldingNothing)
}

// chainMarkFixture is a car with a two-place mate and an empty missing part,
// and action usages writing through a bare parameter bound to the performer and
// through each part, which store their objects as collections.
const chainMarkFixture = `package test {
	private import ScalarValues::*;
	part def Vehicle { attribute seen : Integer default 0; }
	part def Car :> Vehicle {
		part mate : Vehicle[2];
		part missing : Vehicle[0];
		action def Mark { in ref vehicle : Vehicle; first write; action write { assign vehicle.seen := 1; } }
		action def WriteMany { first write; action write { assign mate.seen := 1; } }
		action def WriteNone { first write; action write { assign missing.seen := 1; } }
		action mark : Mark { in ref :>> vehicle = this; }
		action markMany : WriteMany;
		action markNone : WriteNone;
	}
	part car : Car;
}`

// testAssignChainStepHoldingOneObject: a bare `in ref` parameter stores its
// object as a collection of one, and a chained write reaches that one object.
func testAssignChainStepHoldingOneObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, chainMarkFixture))
	self, err := ctx.Instantiate(oneSymbol(t, idx, "test::car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	if _, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "test::Car::mark"), self, nil); err != nil {
		t.Fatalf("mark on car: %v", err)
	}
	fv, err := self.GetFeatureValue(ctx, "seen")
	if err != nil {
		t.Fatalf("read seen of car: %v", err)
	}
	if got := intOutput(t, map[string]Value{"seen": fv.HeldValue()}, "seen"); got != 1 {
		t.Errorf("mark left seen = %d, want 1", got)
	}
}

// testAssignChainStepHoldingSeveralObjects: a step holding two objects names no
// one object, so the write is still refused rather than reaching either. A
// two-place part is that collection step.
func testAssignChainStepHoldingSeveralObjects(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, chainMarkFixture))
	self, err := ctx.Instantiate(oneSymbol(t, idx, "test::car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	_, err = ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "test::Car::markMany"), self, nil)
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "a write reaches one object") {
		t.Fatalf("markMany on car: err = %v, want the holds error", err)
	}
}

// testAssignChainStepHoldingNothing: a step holding no object is an
// uninitialized feature value, not a write to nothing.
func testAssignChainStepHoldingNothing(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, chainMarkFixture))
	self, err := ctx.Instantiate(oneSymbol(t, idx, "test::car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	_, err = ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "test::Car::markNone"), self, nil)
	if !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Fatalf("markNone on car: err = %v, want %v", err, ErrUninitializedFeatureValue)
	}
}
