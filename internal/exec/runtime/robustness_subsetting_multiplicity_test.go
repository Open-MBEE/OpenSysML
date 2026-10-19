package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessSubsettingMultiplicity covers a declared value whose count the feature's
// multiplicity refuses: declared, the usage default of one (SysML v2 §7.6.3), or a subsetted bound.
func TestRuntimeRobustnessSubsettingMultiplicity(t *testing.T) {
	t.Run("two_values_bound_to_a_declared_one", testTwoValuesBoundToADeclaredOne)
	t.Run("two_values_bound_to_a_defaulted_one", testTwoValuesBoundToADefaultedOne)
	t.Run("three_values_bound_through_a_scoped_two", testThreeValuesBoundThroughAScopedTwo)
}

// A subsetting feature restating `[1]` is bound by it: two distinct corners bound to it
// are a multiplicity violation, however many its subsetted feature admits.
func testTwoValuesBoundToADeclaredOne(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<subsetting-declared-one>", parseAndBuild(t, `package test {
		item def Point;
		item def Face { item corners : Point [2]; }
		item def Wedge {
			item faces : Face [*];
			item left [1] :> faces;
			item right [1] :> faces;
			item vertices : Point [*] = faces.corners;
			item ridge [1] :> vertices = (left.corners#(1), right.corners#(1));
		}
	}`))
	wedge, err := ctx.Instantiate(oneSymbol(t, idx, "test::Wedge"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = wedge.GetFeatureValue(ctx, "ridge")
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("wedge.ridge: err = %v, want ErrMultiplicityViolation", err)
	}
	if want := "feature value Wedge.ridge: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1"; err.Error() != want {
		t.Errorf("wedge.ridge: error = %q, want %q", err, want)
	}
}

// An item usage subsetting nothing takes the usage default of one, so two values
// bound to it are a multiplicity violation though it declares no multiplicity.
func testTwoValuesBoundToADefaultedOne(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<subsetting-defaulted-one>", parseAndBuild(t, `package test {
		item def Point;
		item def Wedge {
			item corners : Point [2];
			item tip : Point = corners;
		}
	}`))
	wedge, err := ctx.Instantiate(oneSymbol(t, idx, "test::Wedge"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = wedge.GetFeatureValue(ctx, "tip")
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("wedge.tip: err = %v, want ErrMultiplicityViolation", err)
	}
	if want := "feature value Wedge.tip: multiplicity violation: 2 value(s) bound to a feature with multiplicity upper bound 1"; err.Error() != want {
		t.Errorf("wedge.tip: error = %q, want %q", err, want)
	}
}

// A subsetted bound naming a value of the owner binds the subsetter: three corners bound to
// `ridge :> vertices` under `vertices [limit]`, `limit = 2`, fail on a read of `ridge` alone.
func testThreeValuesBoundThroughAScopedTwo(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<subsetting-scoped-two>", parseAndBuild(t, `package test {
		item def Point;
		item def Wedge {
			attribute limit = 2;
			item a : Point;
			item b : Point;
			item c : Point;
			item vertices : Point [limit];
			item ridge :> vertices = (a, b, c);
		}
	}`))
	wedge, err := ctx.Instantiate(oneSymbol(t, idx, "test::Wedge"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	_, err = wedge.GetFeatureValue(ctx, "ridge")
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("wedge.ridge: err = %v, want ErrMultiplicityViolation", err)
	}
	if want := "feature value Wedge.ridge: multiplicity violation: 3 value(s) bound to a feature with multiplicity upper bound 2"; err.Error() != want {
		t.Errorf("wedge.ridge: error = %q, want %q", err, want)
	}
}
