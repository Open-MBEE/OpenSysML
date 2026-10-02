package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	checkpasses "github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestRuntimeRobustnessNamespaceSuccession(t *testing.T) {
	t.Run("cycle", testNamespaceSuccessionCycle)
	t.Run("explicit_start_violation", testNamespaceSuccessionExplicitStartViolation)
	t.Run("action_performance_violation", testNamespaceSuccessionActionPerformanceViolation)
	t.Run("release_during_clock_advance", testNamespaceSuccessionClockRelease)
	t.Run("anything_uses_each_package_head", testNamespaceSuccessionAnythingPaths)
	t.Run("missing_predecessor", testNamespaceSuccessionMissingPredecessor)
	t.Run("multiple_predecessors", testNamespaceSuccessionMultiplePredecessors)
	t.Run("redefined_behavior", testNamespaceSuccessionRedefinedBehavior)
	t.Run("destroyed_predecessor", testNamespaceSuccessionDestroyedPredecessor)
	t.Run("snapshot_and_held_image", testNamespaceSuccessionSnapshotAndImage)
	t.Run("state_speller_dependencies", testNamespaceSuccessionStateSpeller)
	t.Run("explore_unrelated_interleavings", testNamespaceSuccessionExploreInterleavings)
	t.Run("no_common_feature_type_has_no_note", testNamespaceSuccessionNoCommonFeatureTypeHasNoNote)
	t.Run("behavior_type_message_has_no_note", testNamespaceSuccessionMessageHasNoNote)
	t.Run("event_and_performed_action_note", testNamespaceSuccessionEventAndPerformedActionNote)
	t.Run("check_runtime_refusal_agreement", testNamespaceSuccessionRefusalAgreement)
}

func testNamespaceSuccessionCycle(t *testing.T) {
	src := `package test {
		part def Ring {
			perform action a { action stepA; }
			perform action b { action stepB; }
			first a then b;
			first b then a;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.Instantiate(oneSymbol(t, idx, "test::Ring"))
	requireSuccessionError(t, err, ErrSuccessionOrderCycle, successionOrderCycleCode)
}

func testNamespaceSuccessionExplicitStartViolation(t *testing.T) {
	src := `package test {
		private import SI::*;
		part def Holder {
			perform action earlier {
				first start;
				then action wait accept after 10 [s];
				then done;
			}
			action later;
			first earlier then later;
		}
		action def Kick {
			first start;
			action make { out result : Holder = new Holder(); }
			action kick { in target : Holder; perform target.later.start; }
			flow make.result to kick.target;
			first start then make;
			first make then kick;
			first kick then done;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.ExecuteAction(oneSymbol(t, idx, "test::Kick"))
	requireSuccessionError(t, err, ErrSuccessionOrderViolated, successionOrderViolatedCode)
}

func testNamespaceSuccessionActionPerformanceViolation(t *testing.T) {
	src := `package test {
		private import SI::*;
		part def Holder {
			perform action earlier { action wait accept after 10 [s]; }
			action later { action step; }
			first earlier then later;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	holder, err := ctx.Instantiate(oneSymbol(t, idx, "test::Holder"))
	if err != nil {
		t.Fatalf("instantiate Holder: %v", err)
	}
	later := oneSymbol(t, idx, "test::Holder::later")
	_, err = ctx.CreateActionExecutorFor(later, holder)
	requireSuccessionError(t, err, ErrSuccessionOrderViolated, successionOrderViolatedCode)
}

func testNamespaceSuccessionClockRelease(t *testing.T) {
	ctx, inst := instantiateTimedSuccession(t)
	late := behaviorNamed(t, inst, "later")
	if late.deferred == nil {
		t.Fatal("later started before its predecessor ended")
	}
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance clock: %v", err)
	}
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("run released behaviors: %v", err)
	}
	if late.deferred != nil || late.Action == nil || !late.Action.State().Ended() {
		t.Errorf("later = %+v, want a released, completed action", late)
	}
	value, err := inst.GetFeatureValue(ctx, "n")
	if err != nil {
		t.Fatalf("read n: %v", err)
	}
	if got := FormatValue(value.HeldValue()); got != "10" {
		t.Errorf("n = %s, want 10 after earlier then later", got)
	}
}

func testNamespaceSuccessionAnythingPaths(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		private import SI::*;
		part def Bot {
			attribute n : Integer = 0;
			perform action earlier {
				first start;
				then action wait accept after 1 [s];
				then action bump { assign n := n + 1; }
				then done;
			}
			perform action later {
				first start;
				then action multiply { assign n := n * 10; }
				then done;
			}
		}
		part b : Bot;
		part c : Bot;
		first b.earlier then c.later;
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	if _, err := ctx.Instantiate(oneSymbol(t, idx, "test::b")); err != nil {
		t.Fatalf("instantiate b: %v", err)
	}
	c, err := ctx.Instantiate(oneSymbol(t, idx, "test::c"))
	if err != nil {
		t.Fatalf("instantiate c: %v", err)
	}
	later := behaviorNamed(t, c, "later")
	if later.deferred == nil {
		t.Fatal("c.later started before the predecessor on package-level b")
	}
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance clock: %v", err)
	}
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("run released behaviors: %v", err)
	}
	if later.deferred != nil {
		t.Error("c.later remains held after b.earlier ended")
	}
}

func testNamespaceSuccessionMissingPredecessor(t *testing.T) {
	src := `package test {
		part def Holder {
			action earlier;
			perform action later { action step; }
			first earlier then later;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::Holder"))
	if err != nil {
		t.Fatalf("instantiate Holder: %v", err)
	}
	if later := behaviorNamed(t, inst, "later"); later.deferred != nil {
		t.Error("later remains held without an earlier performance")
	}
	if !hasSuccessionNothingNote(ctx.Notes(), "no earlier-end performance") {
		t.Errorf("notes = %v, want a succession-orders-nothing note for the missing predecessor", ctx.Notes())
	}
}

func testNamespaceSuccessionMultiplePredecessors(t *testing.T) {
	src := `package test {
		private import SI::*;
		part def Arm {
			perform action earlier { action wait accept after 10 [s]; }
			perform action later { action step; }
		}
		part def Holder {
			part left[2] : Arm;
			part right : Arm;
			first left.earlier then right.later;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::Holder"))
	if err != nil {
		t.Fatalf("instantiate Holder: %v", err)
	}
	right := childByFeature(t, ctx, inst, "right")
	later := behaviorNamed(t, right, "later")
	if !ctx.deferredBehaviorReady(later) {
		t.Fatal("later remains held when multiple predecessor performances make pairing ambiguous")
	}
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("run released behaviors: %v", err)
	}
	if !hasSuccessionNothingNote(ctx.Notes(), "2 earlier-end performances") {
		t.Errorf("notes = %v, want a succession-orders-nothing note for multiple predecessors", ctx.Notes())
	}
}

func testNamespaceSuccessionRedefinedBehavior(t *testing.T) {
	src := `package test {
		private import SI::*;
		part def Base {
			perform action earlier { action wait accept after 10 [s]; }
			perform action later { action original; }
			first earlier then later;
		}
		part def Derived :> Base {
			perform action :>> later { action replacement; }
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::Derived"))
	if err != nil {
		t.Fatalf("instantiate Derived: %v", err)
	}
	later := behaviorNamed(t, inst, "later")
	if later.deferred == nil {
		t.Fatal("redefined later behavior was not held")
	}
	if orders := ctx.behaviorOrdersFor(later.member, inst, true); len(orders) != 1 {
		t.Errorf("orders for redefined later behavior = %d, want 1", len(orders))
	}
}

func testNamespaceSuccessionDestroyedPredecessor(t *testing.T) {
	src := `package test {
		private import SI::*;
		part def Arm {
			perform action earlier { action wait accept after 100 [s]; }
			perform action later { action step; }
		}
		part def Robot {
			part left : Arm;
			part right : Arm;
			first left.earlier then right.later;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	robot, err := ctx.Instantiate(oneSymbol(t, idx, "test::Robot"))
	if err != nil {
		t.Fatalf("instantiate Robot: %v", err)
	}
	left := childByFeature(t, ctx, robot, "left")
	right := childByFeature(t, ctx, robot, "right")
	if late := behaviorNamed(t, right, "later"); late.deferred == nil {
		t.Fatal("later started before the predecessor object ended")
	}
	if err := ctx.destroy(left); err != nil {
		t.Fatalf("destroy predecessor object: %v", err)
	}
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("run released behaviors: %v", err)
	}
	if late := behaviorNamed(t, right, "later"); late.deferred != nil {
		t.Error("later remains held after its predecessor object was destroyed")
	}
}

func testNamespaceSuccessionSnapshotAndImage(t *testing.T) {
	ctx, inst := instantiateTimedSuccession(t)
	late := behaviorNamed(t, inst, "later")
	if late.deferred == nil {
		t.Fatal("later started before the snapshot")
	}
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance clock: %v", err)
	}
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("run released behaviors: %v", err)
	}
	snapshot.Restore()
	if late = behaviorNamed(t, inst, "later"); late.deferred == nil {
		t.Fatal("snapshot restore did not return later to its held state")
	}
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance restored clock: %v", err)
	}
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("run restored behaviors: %v", err)
	}
	if late = behaviorNamed(t, inst, "later"); late.deferred != nil {
		t.Error("later remains held after the restored predecessor ended")
	}

	releasedCtx, releasedInst := instantiateTimedSuccession(t)
	if _, err := releasedCtx.Advance(1); err != nil {
		t.Fatalf("advance for released snapshot: %v", err)
	}
	if err := releasedCtx.runAttachedBehaviors(); err != nil {
		t.Fatalf("release before released snapshot: %v", err)
	}
	releasedSnapshot, err := releasedCtx.Snapshot()
	if err != nil {
		t.Fatalf("snapshot released behavior: %v", err)
	}
	releasedSnapshot.Restore()
	if released := behaviorNamed(t, releasedInst, "later"); released.deferred != nil {
		t.Error("released snapshot restore returned later to its held state")
	}

	source, original := instantiateTimedSuccession(t)
	img, err := source.Image(original)
	if err != nil {
		t.Fatalf("image held object: %v", err)
	}
	destination := NewContext(source.Model(), 10000)
	if err := img.Materialize(destination); err != nil {
		t.Fatalf("materialize held image: %v", err)
	}
	copy, ok := destination.Instance(original.ID)
	if !ok {
		t.Fatalf("held image omitted root object #%d", original.ID)
	}
	if held := behaviorNamed(t, copy, "later"); held.deferred == nil {
		t.Fatal("held image round trip did not preserve the deferred behavior")
	}
	if _, err := destination.Advance(1); err != nil {
		t.Fatalf("advance imaged clock: %v", err)
	}
	if err := destination.runAttachedBehaviors(); err != nil {
		t.Fatalf("run imaged behaviors: %v", err)
	}
	if held := behaviorNamed(t, copy, "later"); held.deferred != nil {
		t.Error("imaged later behavior remains held after its predecessor ended")
	}
}

func testNamespaceSuccessionStateSpeller(t *testing.T) {
	ctx, inst := instantiateTimedSuccession(t)
	earlier := behaviorNamed(t, inst, "earlier")
	before := (&Invocation{Actions: []*ActionExecutor{earlier.Action}}).canonicalState(nil).text
	if !strings.Contains(before, "held behavior") || !strings.Contains(before, "waits for") {
		t.Fatalf("held state spelling omits its dependency: %s", before)
	}
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("advance clock: %v", err)
	}
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("run released behaviors: %v", err)
	}
	after := (&Invocation{Actions: []*ActionExecutor{earlier.Action}}).canonicalState(nil).text
	if before == after {
		t.Error("state spelling is unchanged after the dependency was released")
	}
}

func testNamespaceSuccessionExploreInterleavings(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		private import SI::*;
		part def Trio {
			attribute log : String = "";
			perform action a {
				first start;
				then action wait accept after 1 [s];
				then action write { assign log := log + "a"; }
				then done;
			}
			perform action b {
				first start;
				then action wait accept after 1 [s];
				then action write { assign log := log + "b"; }
				then done;
			}
			perform action c {
				first start;
				then action write { assign log := log + "c"; }
				then done;
			}
			first a then c;
		}
	}`
	m := parseLibraryModel(t, src)
	trio := oneSymbol(t, m.idx, "test::Trio")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		inst, err := ctx.Instantiate(trio)
		if err != nil {
			return Outcome{}, err
		}
		if _, err := ctx.Advance(1); err != nil {
			return Outcome{}, err
		}
		if err := ctx.runAttachedBehaviors(); err != nil {
			return Outcome{}, err
		}
		value, err := inst.GetFeatureValue(ctx, "log")
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(map[string]Value{"log": value.HeldValue()}), nil
	})
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !exploration.Complete() || exploration.Runs < 2 {
		t.Fatalf("exploration = %s with %d runs, want all due-order interleavings", exploration.Status(), exploration.Runs)
	}
	got := make(map[string]bool)
	for _, outcome := range exploration.Outcomes {
		got[FormatValue(outcome.Outcome.Outputs["log"])] = true
	}
	if !got[`"abc"`] || !got[`"bac"`] {
		t.Errorf("explored logs = %v, want both a/b orders with c after a", got)
	}
}

func testNamespaceSuccessionNoCommonFeatureTypeHasNoNote(t *testing.T) {
	src := `package test {
		part p1 { action a; }
		part p2 { action b; }
		first p1::a then p2::b;
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	if _, err := ctx.Instantiate(oneSymbol(t, idx, "test::p1")); err != nil {
		t.Fatalf("instantiate p1: %v", err)
	}
	if hasSuccessionNothingNote(ctx.Notes(), "no common featuring type") {
		t.Fatalf("runtime notes = %v, want no note without a featuring object", ctx.Notes())
	}
}

func testNamespaceSuccessionMessageHasNoNote(t *testing.T) {
	src := `package test {
		part def Holder {
			message message;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	if _, err := ctx.Instantiate(oneSymbol(t, idx, "test::Holder")); err != nil {
		t.Fatalf("instantiate Holder: %v", err)
	}
	if got := ctx.Notes(); hasSuccessionNothingNote(got, "does not bind an object behavior performance") {
		t.Fatalf("runtime notes = %v, want no succession-orders-nothing note", got)
	}
}

func testNamespaceSuccessionEventAndPerformedActionNote(t *testing.T) {
	src := `package test {
		part def Sequenced {
			event occurrence event;
			perform action grip;
			first event then grip;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "test::Sequenced"))
	if err != nil {
		t.Fatalf("instantiate Sequenced: %v", err)
	}
	ctx.noteRefusedBehaviorOrders(inst)
	notes := ctx.Notes()
	count := 0
	for _, note := range notes {
		if finding, ok := note.(SuccessionOrdersNothing); ok &&
			strings.Contains(finding.Reason, "event is a occurrence usage") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("runtime notes = %v, want a succession-orders-nothing note", ctx.Notes())
	}
}

func testNamespaceSuccessionRefusalAgreement(t *testing.T) {
	src := `package Sequenced {
		part part1 {
			attribute n : ScalarValues::Integer := 0;
			perform action action1 {
				first start;
				then action increment { assign n := n + 1; }
				then done;
			}
		}
		requirement requirement1;
		first part1::action1 then requirement1;
	}`
	root := parseAndBuild(t, src)
	idx, _, ctx := buildRuntimeWithLibraries(t, "namespace-succession.sysml", root)
	diagnostics := checkpasses.Analyze("namespace-succession.sysml", root, nil, idx)
	var checked *diag.Diagnostic
	for i := range diagnostics {
		if diagnostics[i].Code == successionOrdersNothingCode {
			checked = &diagnostics[i]
			break
		}
	}
	if checked == nil {
		t.Fatalf("check diagnostics = %v, want %s", diagnostics, successionOrdersNothingCode)
	}
	if _, err := ctx.Instantiate(oneSymbol(t, idx, "Sequenced::part1")); err != nil {
		t.Fatalf("instantiate part1: %v", err)
	}
	for _, note := range ctx.Notes() {
		if runtimeNote, ok := note.(SuccessionOrdersNothing); ok {
			if runtimeNote.Reason != checked.Message {
				t.Errorf("runtime reason %q differs from check reason %q", runtimeNote.Reason, checked.Message)
			}
			return
		}
	}
	t.Fatalf("runtime notes = %v, want %s", ctx.Notes(), successionOrdersNothingCode)
}

func instantiateTimedSuccession(t *testing.T) (*Context, *Instance) {
	t.Helper()
	src := `package test {
		private import ScalarValues::*;
		private import SI::*;
		part def Timed {
			attribute n : Integer = 0;
			perform action earlier {
				first start;
				then action wait accept after 1 [s];
				then action bump { assign n := n + 1; }
				then done;
			}
			perform action later {
				first start;
				then action multiply { assign n := n * 10; }
				then done;
			}
			first earlier then later;
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Timed")
	if err != nil {
		t.Fatalf("instantiate Timed: %v", err)
	}
	return ctx, inst
}

func behaviorNamed(t *testing.T, inst *Instance, name string) *ObjectBehavior {
	t.Helper()
	behavior, ok := inst.Behavior(name)
	if !ok {
		t.Fatalf("object #%d has no %s behavior", inst.ID, name)
	}
	return behavior
}

func childByFeature(t *testing.T, ctx *Context, parent *Instance, name string) *Instance {
	t.Helper()
	for _, candidate := range ctx.instances {
		owner, feature := candidate.Owner()
		if owner == parent && feature == name {
			return candidate
		}
	}
	t.Fatalf("object #%d has no child feature %s", parent.ID, name)
	return nil
}

func requireSuccessionError(t *testing.T, err, sentinel error, code string) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v", err, sentinel)
	}
	var orderErr *SuccessionOrderError
	if !errors.As(err, &orderErr) || orderErr.Code != code {
		t.Fatalf("error = %v, want SuccessionOrderError code %s", err, code)
	}
}

func hasSuccessionNothingNote(notes []RunNote, fragment string) bool {
	for _, note := range notes {
		if finding, ok := note.(SuccessionOrdersNothing); ok && strings.Contains(finding.Reason, fragment) {
			return true
		}
	}
	return false
}
