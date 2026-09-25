package passes

import "testing"

// A KerML `bool`/`expr` member declares an Expression, which is a feature, so
// naming one in an expression is valid.
func TestW10BBoolExpressionIsAFeature(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	behavior B {
		bool g;
		expr e : Boolean;
		feature x : Boolean = g;
		feature y : Boolean = e;
	}
	feature b : B;
	feature chained : Boolean = b.g;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if got := w8cCount(msgs, msgReferentIsFeature); got != 0 {
		t.Errorf("want no %q, got %d of %v", msgReferentIsFeature, got, msgs)
	}
}

// A referent that is not a feature at all still reports.
func TestW10BNonFeatureReferentStillReports(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	struct S;
	feature x : Boolean = S;
}`
	msgs := w8cLibraryMessagesIn(t, "<t>.kerml", src)
	if got := w8cCount(msgs, msgReferentIsFeature); got != 1 {
		t.Errorf("want one %q, got %d of %v", msgReferentIsFeature, got, msgs)
	}
}
