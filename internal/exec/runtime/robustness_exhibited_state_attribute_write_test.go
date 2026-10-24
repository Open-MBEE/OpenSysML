package runtime

import (
	"strings"
	"testing"
)

// TestRuntimeRobustnessExhibitedStateAttributeWrite exercises writes to an
// exhibited machine's attributes made through the exhibit usage by another
// performance of the exhibiting part: a write the feature does not admit is a
// typed error that leaves the machine's value standing, a like-named attribute
// of the part stays the part's own, and each write re-polls the machine's
// change triggers on the value it carried. Never a panic or a hang.
func TestRuntimeRobustnessExhibitedStateAttributeWrite(t *testing.T) {
	t.Run("inadmissible_write_through_the_exhibit_usage_is_refused", testInadmissibleWriteThroughTheExhibitUsageIsRefused)
	t.Run("like_named_part_attribute_stays_the_parts_own", testLikeNamedPartAttributeStaysThePartsOwn)
	t.Run("each_write_re_polls_the_change_trigger", testEachWriteRePollsTheChangeTrigger)
	t.Run("same_value_assignment_ends_the_binding", testSameValueAssignmentEndsTheBinding)
}

// exhibitedCountOf reads the exhibited machine's `count` through the part's `g`.
func exhibitedCountOf(t *testing.T, ctx *Context, inst *Instance) int64 {
	t.Helper()
	g, ok, err := ctx.fvObject(inst, "g")
	if err != nil || !ok {
		t.Fatalf("g: ok=%v err=%v", ok, err)
	}
	fv, err := g.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatal(err)
	}
	held := fv.HeldValue()
	if held.Kind != ValConst {
		t.Fatalf("g.count = %v, want a constant", held)
	}
	return int64(held.Const.AsReal())
}

// testInadmissibleWriteThroughTheExhibitUsageIsRefused: a string assigned to the
// machine's Integer attribute through `g.count` is refused as a typed error
// naming the feature, and the machine keeps what it held.
func testInadmissibleWriteThroughTheExhibitUsageIsRefused(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		state def Counting {
			attribute count : Integer = 0;
			entry; then active;
			state active;
		}
		part def Counter {
			exhibit state g : Counting;
			perform action spoil {
				first start;
				then action bad assign g.count := "many";
				then done;
			}
		}
	}`
	_, _, err := instantiateWithLibraries(t, src, "test::Counter")
	if err == nil {
		t.Fatal("expected a typed error for a string written to an Integer attribute")
	}
	if !strings.Contains(err.Error(), "count") {
		t.Errorf("error = %v, want it to name the feature", err)
	}
}

// testLikeNamedPartAttributeStaysThePartsOwn: the part's own `count` and the
// machine's `count` are two features; a write to the part's reaches no machine.
func testLikeNamedPartAttributeStaysThePartsOwn(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		state def Counting {
			attribute count : Integer = 0;
			entry; then active;
			state active;
		}
		part def Counter {
			attribute count : Integer = 10;
			exhibit state g : Counting;
			perform action bump {
				first start;
				then action up assign count := count + 1;
				then done;
			}
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Counter")
	if err != nil {
		t.Fatal(err)
	}
	if got := exhibitedCountOf(t, ctx, inst); got != 0 {
		t.Errorf("g.count = %d, want 0 (the part's own count is not the machine's)", got)
	}
	fv, err := inst.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatal(err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || int64(got.Const.AsReal()) != 11 {
		t.Errorf("count = %v, want 11", got)
	}
}

// testEachWriteRePollsTheChangeTrigger: a write short of the guard leaves the
// machine where it is; the next one past it fires the transition.
func testEachWriteRePollsTheChangeTrigger(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		state def Counting {
			attribute count : Integer = 0;
			attribute tripped : Boolean = false;
			entry; then low;
			state low;
			state high { entry assign tripped := true; }
			transition first low accept when count > 5 then high;
		}
		part def Counter {
			exhibit state g : Counting;
			perform action bump {
				first start;
				then action some assign g.count := 3;
				then action more assign g.count := 7;
				then done;
			}
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Counter")
	if err != nil {
		t.Fatal(err)
	}
	if got := exhibitedCountOf(t, ctx, inst); got != 7 {
		t.Errorf("g.count = %d, want 7", got)
	}
	g, _, _ := ctx.fvObject(inst, "g")
	fv, err := g.GetFeatureValue(ctx, "tripped")
	if err != nil {
		t.Fatal(err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || !got.Const.Bool {
		t.Errorf("g.tripped = %v, want true", got)
	}
}

// testSameValueAssignmentEndsTheBinding: a machine attribute bound to the
// part's, assigned through the exhibit usage the value the binding derived, is
// the assignment's from then on; a later change of the part's attribute no
// longer reaches it, so the guard on it does not fire.
func testSameValueAssignmentEndsTheBinding(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		state def Guard {
			attribute distance : Integer = 0;
			attribute tripped : Boolean = false;
			entry; then cruise;
			state cruise;
			state aborting { entry assign tripped := true; }
			transition first cruise accept when distance < 10 then aborting;
		}
		part def Flight {
			attribute distance : Integer = 20;
			exhibit state g : Guard { :>> distance = Flight::distance; }
			perform action nav {
				first start;
				then action hold assign g.distance := 20;
				then action close assign distance := 5;
				then done;
			}
		}
	}`
	ctx, inst, err := instantiateWithLibraries(t, src, "test::Flight")
	if err != nil {
		t.Fatal(err)
	}
	g, ok, err := ctx.fvObject(inst, "g")
	if err != nil || !ok {
		t.Fatalf("g: ok=%v err=%v", ok, err)
	}
	fv, err := g.GetFeatureValue(ctx, "distance")
	if err != nil {
		t.Fatal(err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || int64(got.Const.AsReal()) != 20 {
		t.Errorf("g.distance = %v, want 20 (the assignment ended the binding)", got)
	}
	fv, err = g.GetFeatureValue(ctx, "tripped")
	if err != nil {
		t.Fatal(err)
	}
	if got := fv.HeldValue(); got.Kind != ValConst || got.Const.Bool {
		t.Errorf("g.tripped = %v, want false", got)
	}
}
