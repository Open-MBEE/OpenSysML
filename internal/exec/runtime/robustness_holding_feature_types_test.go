package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessHoldingFeatureTypes: an object classified by its holding feature's
// types is still not of a type none of those conform to; an unresolved type is the typed error.
func TestRuntimeRobustnessHoldingFeatureTypes(t *testing.T) {
	t.Run("a_type_the_object_cannot_be_is_false", testHoldingFeatureTypesImpossibleType)
	t.Run("an_unresolved_type_operand_is_the_typed_error", testHoldingFeatureTypesUnresolvedOperand)
}

func testHoldingFeatureTypesImpossibleType(t *testing.T) {
	ctx, rig, err := instantiateWithLibraries(t, `package test {
		item def Panel;
		item def Rig :> Objects::StructuredSpaceObject {
			item front : Panel :> faces;
			attribute isCurve = front istype Objects::StructuredSpaceObject::StructuredCurve;
			attribute atCurve = front @ Objects::StructuredSpaceObject::StructuredCurve;
			attribute hasCurve = front hastype Objects::StructuredSpaceObject::StructuredCurve;
			attribute isInteger = front istype ScalarValues::Integer;
			item asCurve [0..1] = front as Objects::StructuredSpaceObject::StructuredCurve;
		}
		item rig : Rig;
	}`, "test::rig")
	if err != nil {
		t.Fatalf("instantiate rig: %v", err)
	}
	for _, name := range []string{"isCurve", "atCurve", "hasCurve", "isInteger"} {
		if readBool(t, ctx, rig, name) {
			t.Errorf("%s = true for a Panel that is a StructuredSurface, want false", name)
		}
	}
	fv, err := featureValueAtPath(t, ctx, rig, "asCurve")
	if err != nil {
		t.Fatalf("asCurve: %v", err)
	}
	if v := fv.HeldValue(); v.Kind != ValSequence || v.Sequence().Size() != 0 {
		t.Errorf("front as StructuredCurve = %s, want ()", FormatValue(v))
	}
	if errs, _ := ctx.MaterializationErrors(rig); len(errs) != 0 {
		t.Errorf("materialization = %v, want no finding", errs)
	}
}

func testHoldingFeatureTypesUnresolvedOperand(t *testing.T) {
	ctx, rig, err := instantiateWithLibraries(t, `package test {
		item def Panel;
		item def Rig :> Objects::StructuredSpaceObject {
			item front : Panel :> faces;
			attribute isMissing = front istype Objects::NoSuchSurface;
		}
		item rig : Rig;
	}`, "test::rig")
	if err != nil {
		t.Fatalf("instantiate rig: %v", err)
	}
	_, err = featureValueAtPath(t, ctx, rig, "isMissing")
	if !errors.Is(err, errUnresolvedType) {
		t.Fatalf("front istype Objects::NoSuchSurface: err = %v, want errUnresolvedType", err)
	}
	if !strings.Contains(err.Error(), "NoSuchSurface") {
		t.Errorf("error = %v, want it to name the unresolved type", err)
	}
}
