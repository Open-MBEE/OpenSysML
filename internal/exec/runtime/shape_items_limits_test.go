package runtime

import (
	"errors"
	"strings"
	"testing"
)

// ShapeItems expressions the runtime cannot honestly compute fail with a typed error naming
// what stopped them: a member the type lacks, a witnessless partial binding, a contradicted bound.
func TestShapeItemsUnsupportedExpressionsAreTypedErrors(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package Geo {
		private import ShapeItems::*; private import SI::*;
		part box : Box      { :>> length = 2 [m]; :>> width = 1 [m]; :>> height = 1 [m]; }
		part rect : Rectangle { :>> length = 4 [m]; :>> width = 3 [m]; }
		part ell : Ellipse  { :>> semiMajorAxis = 4 [m]; :>> semiMinorAxis = 3 [m]; }
		part cyl : Cylinder { :>> semiMajorAxis = 4 [m]; :>> semiMinorAxis = 3 [m]; :>> height = 2 [m]; }
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("Geo")
	if !ok {
		t.Fatal("package Geo not found")
	}
	cases := []struct {
		expr string
		want error
		text string
	}{
		{"box.spaceBoundary", ErrNoSuchFeature, "box has no member spaceBoundary: Items::Item::shape redefines it"},
		// `tfe [2] :> edges` holds objects of the subsetted feature's type, StructuredCurve, which
		// declares no length; Line, the type of the faces' edges, does.
		{"box.tfe.length", ErrNoSuchFeature, "StructuredCurve has no member length"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			_, err := evalIn(t, ctx, pkg.Scope, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s = %v, want %v", tc.expr, err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("%s error %q does not name %q", tc.expr, err, tc.text)
			}
		})
	}

	// `binding [1] bind [0..1] tf.edges = [0..1] tfe` identifies one value of each end
	// without determining either, so a bound subsetter holds what its own declaration
	// states, and the collections it subsets read through it (KerML 1.0 §7.4.6).
	for expr, want := range map[string]int{
		"box.tfe": 2, "box.tflv": 3, "box.vertices": 18, "box.edges": 24,
		"cyl.edges": 2, "cyl.faces.edges": 2, "cyl.be": 2, "cyl.ae": 0,
	} {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := len(elementsOf(val)); got != want {
			t.Errorf("%s = %s, want %d objects", expr, FormatValue(val), want)
		}
	}

	// A Kernel Semantic Library member the shape leaves out is a value, not a gap: a
	// required collection the runtime cannot enumerate is undetermined.
	for expr, want := range map[string]ValueKind{"box.matingOccurrences": ValUndetermined, "rect.vertices#(1).matingOccurrences": ValUndetermined} {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil || val.Kind != want {
			t.Errorf("%s = %s, %v; want a %v", expr, FormatValue(val), err, want)
		}
	}

	// A shape the library declares without vertices holds none: that is a value, not a gap.
	for _, expr := range []string{"ell.vertices", "cyl.vertices"} {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil || FormatValue(val) != "[]" {
			t.Errorf("%s = %s, %v; want [] with no error", expr, FormatValue(val), err)
		}
	}

	// `binding [1] bind [0..*] base.edges = [0..*] be` links one value of each end, so a face's
	// `edges` keeps the edge its own `= shape` gives it instead of being bound to `be` whole.
	for _, expr := range []string{"cyl.base.edges", "cyl.af.edges"} {
		val, err := evalIn(t, ctx, pkg.Scope, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if val.Kind != ValInstance {
			t.Errorf("%s = %s, want one Instance", expr, FormatValue(val))
		}
	}
}
