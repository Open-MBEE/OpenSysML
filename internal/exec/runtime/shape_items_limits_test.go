package runtime

import (
	"errors"
	"strings"
	"testing"
)

// ShapeItems expressions the runtime cannot honestly compute fail with a typed error naming
// what stopped them: a missing Kernel-frame feature, a witnessless partial binding, a contradicted bound.
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
		{"box.matingOccurrences", ErrNoSuchFeature, "member matingOccurrences not found in instance"},
		{"rect.vertices#(1).matingOccurrences", ErrNoSuchFeature, "member matingOccurrences not found in instance"},
		{"box.spaceBoundary", ErrNoSuchFeature, "member spaceBoundary not found in instance"},
		{"box.tfe", ErrBindingEnd, "which makes some value of tfe a value of tf.edges without saying which value of either; the model does not state what tfe holds"},
		{"box.tfe.length", ErrBindingEnd, "binding [1] bind [0..1] tf.edges = [0..1] tfe"},
		{"box.tflv", ErrBindingEnd, "binding [1] bind [0..1] tf.edges = [0..1] tfe"},
		{"box.vertices", ErrBindingEnd, "subsetting feature tflv of vertices: binding end cannot be resolved: box.tfe is bound by `binding [1] bind [0..1] tf.edges = [0..1] tfe`"},
		{"cyl.edges", ErrNoSuchFeature, "member edges not found in instance"},
		{"cyl.faces.edges", ErrNoSuchFeature, "member edges not found in instance"},
		{"cyl.vertices", ErrNoSuchFeature, "member vertices not found in instance"},
		{"cyl.be", ErrBindingEnd, `"cf.edges": feature edges not found`},
		{"cyl.ae", ErrBindingEnd, `"cf.edges": feature edges not found`},
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

	// A curve the library declares without vertices holds none: that is a value, not a gap.
	val, err := evalIn(t, ctx, pkg.Scope, "ell.vertices")
	if err != nil || FormatValue(val) != "[]" {
		t.Errorf("ell.vertices = %s, %v; want [] with no error", FormatValue(val), err)
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
