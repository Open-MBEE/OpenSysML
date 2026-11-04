package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessSubsettedPool covers a subsetting feature drawing its values from what it
// subsets (KerML 1.1 §7.3.4.4), objects or values: too few there is a multiplicity violation; a write redraws it.
func TestRuntimeRobustnessSubsettedPool(t *testing.T) {
	t.Run("under-populated subsetted collection is a multiplicity violation", testSubsettedPoolTooSmall)
	t.Run("a write to the subsetted collection redraws the subsetter", testSubsettedPoolRewritten)
	t.Run("too few subsetted values is a multiplicity violation", testSubsettedValuesTooFew)
	t.Run("a value the subsetter's type does not admit is not drawn", testSubsettedValueOfOtherType)
	t.Run("a write to the subsetted values redraws the subsetter", testSubsettedValuesRewritten)
	t.Run("a subsetted feature holding a default lends no value", testSubsettedDefaultNoPool)
}

const subsettedValues = `package P {
	private import ScalarValues::*;
	part def Holder {
		attribute one : Integer [1] = 1;
		attribute two : Integer [2] :> one;
		attribute rl : Real = 4.5;
		attribute ri : Integer [1] :> rl;
		attribute coll : Integer [0..*] = (1, 2);
		attribute pick : Integer [1] :> coll;
		attribute d : Integer default 5;
		attribute e : Integer [0..1] :> d;
	}
}`

func subsettedValuesHolder(t *testing.T) (*Context, *Instance) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<subsetted-values>", parseAndBuild(t, subsettedValues))
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "P::Holder"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return ctx, inst
}

func testSubsettedValuesTooFew(t *testing.T) {
	ctx, inst := subsettedValuesHolder(t)
	if fv, err := inst.GetFeatureValue(ctx, "two"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("two = %v, %v; want %v", fv, err, ErrMultiplicityViolation)
	}
}

func testSubsettedValueOfOtherType(t *testing.T) {
	ctx, inst := subsettedValuesHolder(t)
	if fv, err := inst.GetFeatureValue(ctx, "ri"); !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("ri = %v, %v; want %v, 4.5 being no Integer", fv, err, ErrMultiplicityViolation)
	}
}

func testSubsettedValuesRewritten(t *testing.T) {
	ctx, inst := subsettedValuesHolder(t)
	read := func() Value {
		fv, err := inst.GetFeatureValue(ctx, "pick")
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		return fv.HeldValue()
	}
	if pick := read(); !ctx.equalValues(pick, integerValue(1)) {
		t.Fatalf("pick = %s, want 1, the first value coll holds", FormatValue(pick))
	}
	if err := inst.SetFeatureValue(ctx, "coll", sequenceOf([]Value{integerValue(3), integerValue(2)})); err != nil {
		t.Fatalf("write coll: %v", err)
	}
	if pick := read(); !ctx.equalValues(pick, integerValue(3)) {
		t.Errorf("pick = %s after coll := (3, 2), want 3, the first value coll now holds", FormatValue(pick))
	}
}

func testSubsettedDefaultNoPool(t *testing.T) {
	ctx, inst := subsettedValuesHolder(t)
	fv, err := inst.GetFeatureValue(ctx, "e")
	if err != nil {
		t.Fatalf("e: %v", err)
	}
	if got := elementsOf(fv.HeldValue()); len(got) != 0 && got[0].Kind != ValUndetermined {
		t.Errorf("e = %s, want no value: d holds only a default", FormatValue(fv.HeldValue()))
	}
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
