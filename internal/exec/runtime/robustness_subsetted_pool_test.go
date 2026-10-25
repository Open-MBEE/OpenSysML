package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessSubsettedPool covers a subsetting feature that states no value of its
// own and so holds values of the feature it subsets (KerML 1.0 §7.3.4.4): where that feature
// holds fewer values than the subsetter's lower bound asks, the read is a multiplicity
// violation, not a feature filled with objects the subsetted feature does not hold.
func TestRuntimeRobustnessSubsettedPool(t *testing.T) {
	t.Run("under-populated subsetted collection is a multiplicity violation", testSubsettedPoolTooSmall)
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
