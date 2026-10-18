package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessSubsettingTypes covers objects held by a feature whose
// types come together through subsetting: a member inherited from the subsetted
// feature's type that the model leaves without a value fails with a typed error
// naming what is missing, never with a missing-feature error or a panic.
func TestRuntimeRobustnessSubsettingTypes(t *testing.T) {
	t.Run("inherited_member_bound_without_a_witness", testInheritedMemberBoundWithoutAWitness)
	t.Run("member_outside_the_conjunction_is_still_missing", testMemberOutsideTheConjunctionIsStillMissing)
}

const subsettingTypesModel = `package test {
	private import ScalarValues::*;
	item def Panel { attribute area : Real = 2.0; }
	item def Framed { item trim : Strip [1]; attribute border : Real; }
	item def Strip { attribute length : Real = 9.0; }
	part def Holder {
		item frames : Framed [0..*];
		item front : Panel [1] :> frames;
		item spares : Strip [0..*];
		binding [1] bind [0..*] front.trim = [0..*] spares;
	}
	part holder : Holder;
}`

// testInheritedMemberBoundWithoutAWitness: the binding reaches `trim` through
// Framed, the type `front` inherits by subsetting, but binds one of many
// spares without saying which; the read reports the undetermined binding.
func testInheritedMemberBoundWithoutAWitness(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, subsettingTypesModel))
	holder := instantiateNamed(t, ctx, idx, "test::holder")
	front, err := holder.GetFeatureValue(ctx, "front")
	if err != nil {
		t.Fatalf("holder.front: %v", err)
	}
	obj, ok := ctx.Instance(front.HeldValue().Instance)
	if !ok {
		t.Fatalf("holder.front = %s, want an object", FormatValue(front.HeldValue()))
	}
	_, err = obj.GetFeatureValue(ctx, "trim")
	if !errors.Is(err, errBindingEnd) {
		t.Fatalf("front.trim error = %v, want the undetermined binding", err)
	}
	if !strings.Contains(err.Error(), "binding [1] bind [0..*] front.trim = [0..*] spares") || strings.Contains(err.Error(), "no such feature") {
		t.Errorf("front.trim error %q, want it to name the binding and not a missing feature", err)
	}
}

// testMemberOutsideTheConjunctionIsStillMissing: the conjunction adds Framed's
// members to what `front` holds and nothing else; a member of neither type is
// still reported missing by name, while Panel's own members keep their values.
func testMemberOutsideTheConjunctionIsStillMissing(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, subsettingTypesModel))
	holder := instantiateNamed(t, ctx, idx, "test::holder")
	front, err := holder.GetFeatureValue(ctx, "front")
	if err != nil {
		t.Fatalf("holder.front: %v", err)
	}
	obj, ok := ctx.Instance(front.HeldValue().Instance)
	if !ok {
		t.Fatalf("holder.front = %s, want an object", FormatValue(front.HeldValue()))
	}
	if _, err := obj.GetFeatureValue(ctx, "length"); !errors.Is(err, ErrNoSuchFeature) || !strings.Contains(err.Error(), "length") {
		t.Errorf("front.length error = %v, want ErrNoSuchFeature naming length", err)
	}
	if area, err := obj.GetFeatureValue(ctx, "area"); err != nil || FormatTraceValue(area.HeldValue()) != "2.0" {
		t.Errorf("front.area = %v, %v; want Panel's 2.0", area, err)
	}
	if _, err := obj.GetFeatureValue(ctx, "border"); err != nil {
		t.Errorf("front.border: %v, want Framed's member read without error", err)
	}
}
