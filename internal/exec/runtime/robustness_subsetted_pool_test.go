package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessSubsettedPool covers a subsetting feature drawing its values from what it
// subsets (KerML 1.0 §7.3.4.4): too few there is a multiplicity violation; a write there redraws it.
func TestRuntimeRobustnessSubsettedPool(t *testing.T) {
	t.Run("under-populated subsetted collection is a multiplicity violation", testSubsettedPoolTooSmall)
	t.Run("a write to the subsetted collection redraws the subsetter", testSubsettedPoolRewritten)
}

const subsettedPoolTooSmall = `package P {
	item def Node;
	part def Holder {
		item x : Node;
		item coll : Node [1] = x;
		item two : Node [2] :> coll;
	}
}`

func testSubsettedPoolTooSmall(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<subsetted-pool-too-small>", parseAndBuild(t, subsettedPoolTooSmall))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Holder"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "two")
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("two = %v, %v; want %v", fv, err, ErrMultiplicityViolation)
	}
	coll, err := inst.GetFeatureValue(ctx, "coll")
	if err != nil || len(elementsOf(coll.HeldValue())) != 1 {
		t.Errorf("coll = %v, %v; want the one object it declares", coll, err)
	}
}

const subsettedPoolRewritten = `package P {
	item def Node;
	part def Holder {
		item x : Node;
		item y : Node;
		item z : Node;
		item coll : Node [0..*] = (x, y);
		item pick : Node [1] :> coll;
	}
}`

func testSubsettedPoolRewritten(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<subsetted-pool-rewritten>", parseAndBuild(t, subsettedPoolRewritten))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Holder"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	object := func(name string) Value {
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return fv.HeldValue()
	}
	x, y, z := object("x"), object("y"), object("z")
	if pick := object("pick"); !ctx.equalValues(pick, x) {
		t.Fatalf("pick = %s, want x, the first value coll holds", FormatValue(pick))
	}
	if err := inst.SetFeatureValue(ctx, "coll", sequenceOf([]Value{z, y})); err != nil {
		t.Fatalf("write coll: %v", err)
	}
	pick := object("pick")
	if !ctx.equalValues(pick, z) {
		t.Errorf("pick = %s after coll := (z, y), want z, the first value coll now holds", FormatValue(pick))
	}
	if ctx.equalValues(pick, x) {
		t.Errorf("pick still holds x, which coll no longer holds")
	}
}
