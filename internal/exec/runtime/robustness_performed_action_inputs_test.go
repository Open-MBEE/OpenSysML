package runtime

import (
	"errors"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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
		{"bare", "", false},
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
	t.Run("nested_action_parameter_multiplicity", testNestedActionParameterMultiplicity)
	t.Run("other_failure_fails_creation", testPerformedActionOtherFailureFailsCreation)
	t.Run("snapshot_restore_undoes_recorded_failure", testSnapshotRestoreUndoesRecordedFailure)
	t.Run("held_image_carries_recorded_failure", testHeldImageCarriesRecordedFailure)
	t.Run("advance_records_clock_driven_failure", testAdvanceRecordsClockDrivenFailure)
	t.Run("shared_clock_records_clock_driven_failure", testSharedClockRecordsClockDrivenFailure)
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
	if l, ok := ctx.OccurrenceLife(behavior.Action.occurrence.ID); !ok || l.Ended == 0 {
		t.Errorf("performance occurrence life = %v, %v; want ended with the recorded failure", l, ok)
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

	action def ApplyHeat { in energy : ISQ::EnergyValue[1]; }
	action def ToastBread { action applyHeat : ApplyHeat; }
	part def Holder { action beh : ToastBread; }
	part slow : Holder;
	action def Kick {
		first start;
		action kick { in target : Holder[1]; perform target.beh.start; }
		done;
		succession first start then kick;
		succession first kick then done;
	}
}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	waiters := len(ctx.clock.waiters)
	if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Kick")); !errors.Is(err, ErrUnboundParameter) {
		t.Fatalf("ExecuteAction(Kick) = %v, want ErrUnboundParameter", err)
	}
	if got := len(ctx.clock.waiters); got != waiters {
		t.Errorf("clock waiters = %d after the failed start, want %d: the failed executor leaked", got, waiters)
	}
}

func testNestedActionParameterMultiplicity(t *testing.T) {
	for _, tc := range []struct {
		name, outerMult, innerMult string
		wantErr                    error
	}{
		{name: "bare parameters admit no input"},
		{name: "required inner rejects no input", innerMult: "[1]", wantErr: ErrMultiplicityViolation},
		{name: "required outer remains unbound", outerMult: "[1]", wantErr: ErrUnboundParameter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package test {
				item def Bread;
				action def Inner { in bread` + tc.innerMult + ` : Bread; }
				action def Outer {
					in bread` + tc.outerMult + ` : Bread;
				first start;
				then action applyHeat : Inner {
					in bread = Outer::bread;
				}
				then done;
			}
			action def Runner {
				first start;
				then action outer : Outer;
				then done;
			}
			}`
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			actionName := "Outer"
			if tc.wantErr == ErrUnboundParameter {
				actionName = "Runner"
			}
			_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::"+actionName))
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("ExecuteAction(%s) = %v, want success", actionName, err)
				}
				if tc.name == "bare parameters admit no input" {
					found := false
					for key, target := range ctx.model.writeTargets {
						if key.name != "bread" || target == nil {
							continue
						}
						sym, ok := ctx.lookupName(key.scope, key.name)
						if !ok || !semantics.IsParameter(sym) {
							continue
						}
						found = true
						if want := ctx.model.semantics.EffectiveParameterRange(sym); target.mult != want {
							t.Errorf("write target multiplicity = %s, want effective parameter range %s", target.mult.Text(), want.Text())
						}
						if !target.countJudged {
							t.Error("write target does not judge the effective parameter range")
						}
					}
					if !found {
						t.Fatal("execution did not check a parameter write target for bread")
					}
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ExecuteAction(%s) = %v, want %v", actionName, err, tc.wantErr)
			}
		})
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
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	waiters := len(ctx.clock.waiters)
	if _, err := ctx.Instantiate(oneSymbol(t, idx, "test::Host")); !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("instantiate Host = %v, want ErrTypeMismatch", err)
	}
	if got := len(ctx.clock.waiters); got != waiters {
		t.Errorf("clock waiters = %d after the failed creation, want %d: the failed executor leaked", got, waiters)
	}
}

// awaitingInputModel is a Toaster performing toastBread, which parks at an
// accept of a signal and then reaches applyHeat, whose energy input nothing
// binds: the failure is the message's, not the creation's, and so lands only
// after the object stands.
func awaitingInputModel() string {
	return `package test {
	private import ISQ::*;

	action def ApplyHeat { in energy : ISQ::EnergyValue[1]; }
	action def ToastBread {
		first start;
		action heard accept g : Integer;
		action applyHeat : ApplyHeat;
		done;
		succession first start then heard;
		succession first heard then applyHeat;
		succession first applyHeat then done;
	}
	part def Toaster { perform action toastBread : ToastBread; }
	part slow : Toaster;
}`
}

// testSnapshotRestoreUndoesRecordedFailure: the failure a woken performed
// action records is journaled, so restoring a snapshot taken while it was
// parked undoes the record, and the parked action hears the message again.
func testSnapshotRestoreUndoesRecordedFailure(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, awaitingInputModel(), "test::slow")
	if err != nil {
		t.Fatalf("instantiate slow: %v", err)
	}
	behavior, ok := inst.Behavior("toastBread")
	if !ok || behavior.Action == nil {
		t.Fatalf("slow performs no toastBread, behaviors: %v", inst.Behaviors())
	}
	if behavior.Err != nil || behavior.completed() {
		t.Fatalf("toastBread parked at its accept records %v, want still running", behavior.Err)
	}
	snap, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	defer snap.Release()

	post := func() {
		one := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
		ctx.PostMessage(Message{SignalType: "Integer", Object: inst.ID, Value: &one})
		if !behavior.hasPendingWork() {
			t.Fatal("the message in flight is work of the parked toastBread")
		}
		if err := ctx.drainObjectBehaviors(); err != nil {
			t.Fatalf("drainObjectBehaviors: %v", err)
		}
	}

	post()
	if !errors.Is(behavior.Err, ErrUnboundParameter) {
		t.Fatalf("behavior.Err = %v, want ErrUnboundParameter", behavior.Err)
	}
	occID := behavior.Action.occurrence.ID
	if l, _ := ctx.OccurrenceLife(occID); l.Ended == 0 {
		t.Error("performance occurrence life un-ended after the recorded failure")
	}

	snap.Restore()
	if behavior.Err != nil {
		t.Errorf("restored behavior.Err = %v, want the record undone", behavior.Err)
	}
	if l, _ := ctx.OccurrenceLife(occID); l.Ended != 0 {
		t.Errorf("restored performance occurrence life = %v, want un-ended", l)
	}
	if behavior.completed() {
		t.Error("restored toastBread is completed, want parked at its accept again")
	}

	post()
	if !errors.Is(behavior.Err, ErrUnboundParameter) {
		t.Fatalf("re-woken behavior.Err = %v, want ErrUnboundParameter", behavior.Err)
	}
}

// testHeldImageCarriesRecordedFailure: a held image carries a performance's
// recorded failure and its type-bound marking, so the materialized copy is
// ended the same way rather than starting over.
func testHeldImageCarriesRecordedFailure(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, unboundInputModel("[1]"), "test::slow")
	if err != nil {
		t.Fatalf("instantiate slow: %v", err)
	}
	behavior, ok := inst.Behavior("toastBread")
	if !ok || !errors.Is(behavior.Err, ErrUnboundParameter) {
		t.Fatalf("slow performs no failed toastBread: %v", inst.Behaviors())
	}

	dst := imageInto(t, ctx, inst)
	copied, _ := dst.Instance(inst.ID)
	copy, ok := copied.Behavior("toastBread")
	if !ok {
		t.Fatalf("the copy performs no toastBread, behaviors: %v", copied.Behaviors())
	}
	if !errors.Is(copy.Err, ErrUnboundParameter) {
		t.Errorf("copy.Err = %v, want ErrUnboundParameter", copy.Err)
	}
	if !copy.typeBound {
		t.Error("the copy is not marked type-bound, want carried from the image")
	}
	if !copy.completed() || copy.hasPendingWork() {
		t.Error("the copied performance must be ended and hold no pending work")
	}
}

// timedInputModel is a Toaster performing toastBread, which waits on the clock
// before reaching applyHeat, whose energy input nothing binds: the failure is
// the wait's wake, not the creation's, and lands when the clock drives it.
func timedInputModel() string {
	return `package test {
	private import ISQ::*;
	private import SI::*;

	action def ApplyHeat { in energy : ISQ::EnergyValue[1]; }
	action def ToastBread {
		first start;
		then action pause accept after 5 [s];
		then action applyHeat : ApplyHeat;
		then done;
	}
	part def Toaster { perform action toastBread : ToastBread; }
	part slow : Toaster;
}`
}

// testAdvanceRecordsClockDrivenFailure: a performed action reaching its unbound
// input after a timed wait fails on the clock's advance, where the same record
// a message wake takes applies: the failure ends the performance, its life and
// its clock work, and the advance itself stands.
func testAdvanceRecordsClockDrivenFailure(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, timedInputModel(), "test::slow")
	if err != nil {
		t.Fatalf("instantiate slow: %v", err)
	}
	behavior, ok := inst.Behavior("toastBread")
	if !ok || behavior.Action == nil {
		t.Fatalf("slow performs no toastBread, behaviors: %v", inst.Behaviors())
	}
	if behavior.Err != nil {
		t.Fatalf("toastBread waiting on the clock records %v, want no failure yet", behavior.Err)
	}
	if !slices.Contains(ctx.clock.waiters, clockWaiter(behavior.Action)) {
		t.Fatal("toastBread parked at its timed wait is no clock waiter")
	}
	if _, err := ctx.Advance(6); err != nil {
		t.Fatalf("Advance(6) = %v, want nil: the failure is the performance's", err)
	}
	if !errors.Is(behavior.Err, ErrUnboundParameter) {
		t.Fatalf("behavior.Err = %v, want ErrUnboundParameter", behavior.Err)
	}
	if l, _ := ctx.OccurrenceLife(behavior.Action.occurrence.ID); l.Ended == 0 {
		t.Error("performance occurrence life un-ended after the clock-driven failure")
	}
	if slices.Contains(ctx.clock.waiters, clockWaiter(behavior.Action)) {
		t.Error("the failed performance is still a clock waiter")
	}
}

// testSharedClockRecordsClockDrivenFailure: the same failure lands when another
// executor drives the shared clock past the wait — a run that advances time for
// its own waits runs the object's performance due on the way, and records its
// failure instead of failing its own.
func testSharedClockRecordsClockDrivenFailure(t *testing.T) {
	src := `package test {
	private import ISQ::*;
	private import SI::*;

	action def ApplyHeat { in energy : ISQ::EnergyValue[1]; }
	action def ToastBread {
		first start;
		then action pause accept after 5 [s];
		then action applyHeat : ApplyHeat;
		then done;
	}
	part def Toaster { perform action toastBread : ToastBread; }
	part slow : Toaster;
	action def Drive {
		first start;
		then action pause accept after 10 [s];
		then done;
	}
}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::slow"))
	if err != nil {
		t.Fatalf("instantiate slow: %v", err)
	}
	behavior, ok := inst.Behavior("toastBread")
	if !ok || behavior.Action == nil {
		t.Fatalf("slow performs no toastBread, behaviors: %v", inst.Behaviors())
	}
	if _, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Drive")); err != nil {
		t.Fatalf("ExecuteAction(Drive) = %v, want nil: the failure is the object's performance's", err)
	}
	if !errors.Is(behavior.Err, ErrUnboundParameter) {
		t.Fatalf("behavior.Err = %v, want ErrUnboundParameter", behavior.Err)
	}
	if l, _ := ctx.OccurrenceLife(behavior.Action.occurrence.ID); l.Ended == 0 {
		t.Error("performance occurrence life un-ended after the clock-driven failure")
	}
}
