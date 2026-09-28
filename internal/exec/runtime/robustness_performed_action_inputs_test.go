package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessPerformedActionInputs exercises a performed action whose
// input nothing binds: materializing its performer records the failure on the
// performance rather than failing the object's creation, the failed performance
// is ended for good, an explicit execution of the same action is refused for
// the same input, and a failure the input does not cause still fails creation.
func TestRuntimeRobustnessPerformedActionInputs(t *testing.T) {
	classes := []struct {
		name     string
		mult     string
		required bool
	}{
		{"bare", "", true},
		{"one", "[1]", true},
		{"one_one", "[1..1]", true},
		{"one_many", "[1..*]", true},
		{"zero_many", "[0..*]", false},
		{"zero_one", "[0..1]", false},
	}
	for _, c := range classes {
		t.Run("nested_"+c.name, func(t *testing.T) {
			testPerformedActionUnboundInput(t, c.mult, c.required)
		})
		t.Run("toplevel_"+c.name, func(t *testing.T) {
			testPerformedUsageUnboundInput(t, c.mult, c.required)
		})
	}
	t.Run("declared_behavior_start_fails", testDeclaredBehaviorStartFailsUnbound)
	t.Run("other_failure_fails_creation", testPerformedActionOtherFailureFailsCreation)
}

// unboundInputModel is a Toaster performing toastBread, whose applyHeat step
// binds nothing to ApplyHeat's energy input of the given multiplicity, and a
// part slow of it whose cycleTime does not depend on the performance.
func unboundInputModel(mult string) string {
	return `package test {
	private import ISQ::*;
	private import SI::*;

	action def ApplyHeat { in energy` + mult + ` : ISQ::EnergyValue; }
	action def ToastBread { action applyHeat : ApplyHeat; }
	part def Toaster {
		attribute cycleTime : ISQ::DurationValue;
		perform action toastBread : ToastBread;
	}
	part slow : Toaster { attribute :>> cycleTime = 200.0 [SI::s]; }
}`
}

// unboundUsageModel performs the unbound action on the object directly, rather
// than a step of the action it performs.
func unboundUsageModel(mult string) string {
	return `package test {
	private import ISQ::*;
	private import SI::*;

	action def ApplyHeat { in energy` + mult + ` : ISQ::EnergyValue; }
	part def Toaster {
		attribute cycleTime : ISQ::DurationValue;
		perform action applyHeat : ApplyHeat;
	}
	part slow : Toaster { attribute :>> cycleTime = 200.0 [SI::s]; }
}`
}

// testPerformedActionUnboundInput: slow's creation records a required input's
// refusal on the toastBread performance it performs, ended for good, while the
// object itself and its other features stand; an optional input runs nothing.
func testPerformedActionUnboundInput(t *testing.T, mult string, required bool) {
	ctx, inst, err := instantiateWithLibraries(t, unboundInputModel(mult), "test::slow")
	if err != nil {
		t.Fatalf("instantiate slow: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "cycleTime")
	if err != nil {
		t.Fatalf("read cycleTime: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValQuantity || got.Quantity() == nil || got.Quantity().Num.Real != 200.0 {
		t.Fatalf("cycleTime = %v, want 200.0 [s]", FormatValue(got))
	}
	behavior, ok := inst.Behavior("toastBread")
	if !ok {
		t.Fatal("slow performs no toastBread")
	}
	if !required {
		if behavior.Err != nil {
			t.Fatalf("an optional input records %v, want no failure", behavior.Err)
		}
		return
	}
	if !errors.Is(behavior.Err, ErrUnboundParameter) {
		t.Fatalf("behavior.Err = %v, want ErrUnboundParameter", behavior.Err)
	}
	if !behavior.completed() || behavior.hasPendingWork() {
		t.Error("a failed performance must be ended and hold no pending work")
	}
	// The same action executed on its own is refused for the same input.
	idx := ctx.model.resolver.Index()
	if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::ToastBread")); !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("ExecuteAction(ToastBread) = %v, want ErrUnboundParameter", err)
	}
}

// testPerformedUsageUnboundInput: a usage the object performs directly records
// no failure for an unbound input — the input check is a step's, and a bodiless
// performed action takes no step — so the object and its features stand as ever.
func testPerformedUsageUnboundInput(t *testing.T, mult string, required bool) {
	ctx, inst, err := instantiateWithLibraries(t, unboundUsageModel(mult), "test::slow")
	if err != nil {
		t.Fatalf("instantiate slow: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "cycleTime")
	if err != nil {
		t.Fatalf("read cycleTime: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValQuantity || got.Quantity() == nil || got.Quantity().Num.Real != 200.0 {
		t.Fatalf("cycleTime = %v, want 200.0 [s]", FormatValue(got))
	}
	behavior, ok := inst.Behavior("applyHeat")
	if !ok {
		t.Fatal("slow performs no applyHeat")
	}
	if behavior.Err != nil {
		t.Fatalf("a bodiless performed usage records %v, want no failure", behavior.Err)
	}
}

// testDeclaredBehaviorStartFailsUnbound: an explicit `perform obj.beh.start` of
// a behavior the object's type only declares is no type-bound performance, so
// its unbound input fails the start as every start failure does.
func testDeclaredBehaviorStartFailsUnbound(t *testing.T) {
	src := `package test {
	private import ISQ::*;

	action def ApplyHeat { in energy : ISQ::EnergyValue; }
	action def ToastBread { action applyHeat : ApplyHeat; }
	part def Holder { action beh : ToastBread; }
	part slow : Holder;
	action def Kick {
		first start;
		action kick { in target : Holder; perform target.beh.start; }
		done;
		succession first start then kick;
		succession first kick then done;
	}
}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Kick")); !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("ExecuteAction(Kick) = %v, want ErrUnboundParameter", err)
	}
}

// testPerformedActionOtherFailureFailsCreation: a failure of a performed action
// that is no unbound parameter — a write its performer's feature type does not
// admit — still fails the object's creation.
func testPerformedActionOtherFailureFailsCreation(t *testing.T) {
	src := `package test {
	private import ISQ::*;
	private import SI::*;

	part def Host {
		attribute source = 3.0 [m / s];
		attribute t : ISQSpaceTime::TimeValue = 0.0 [s];
		perform action b {
			first start;
			action step { assign t := source; }
			done;
			succession first start then step;
			succession first step then done;
		}
	}
}`
	if _, _, err := instantiateWithLibraries(t, src, "test::Host"); !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("instantiate Host = %v, want ErrTypeMismatch", err)
	}
}
