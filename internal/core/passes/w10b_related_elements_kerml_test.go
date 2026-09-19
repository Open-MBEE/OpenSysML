package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// relatedElementsLines returns the 1-based source lines on which src draws a
// related-elements diagnostic, in order.
func relatedElementsLines(t *testing.T, src string, kerml bool) []int {
	t.Helper()
	var diags []diag.Diagnostic
	if kerml {
		diags = constraintDiagsKerML(t, src)
	} else {
		diags = constraintDiags(t, src)
	}
	var lines []int
	for _, d := range only(diags, "related-elements") {
		if d.Message != msgRelatedElements {
			t.Errorf("message = %q, want %q", d.Message, msgRelatedElements)
		}
		lines = append(lines, strings.Count(src[:d.Span.Offset], "\n")+1)
	}
	return lines
}

func sameLines(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// The corpus case semantic/k25-assoc-one-end.kerml, and the other KerML
// association shapes the rule reaches: an `assoc` or `assoc struct` with fewer
// than two ends is reported unless abstract, and an inherited pair is enough.
func TestW10BKerMLAssociationEnds(t *testing.T) {
	const src = `package P {
	class T;
	assoc A { end feature a : T; end feature b : T; }
	assoc E;
	assoc F { end feature a : T; }
	assoc struct S { end feature a : T; }
	abstract assoc G;
	assoc H :> A;
	assoc I :> A { end feature c : T; }
	assoc struct J :> A;
}`
	if got, want := relatedElementsLines(t, src, true), []int{4, 5, 6}; !sameLines(got, want) {
		t.Fatalf("related-elements on lines %v, want %v", got, want)
	}
}

// An interaction is an association (KerML 1.1 §8.3.5.4): it needs two ends of
// its own or inherited, and does not take them from Transfers::Transfer.
func TestW10BKerMLInteractionEnds(t *testing.T) {
	const src = `package P {
	class T;
	interaction I1 { end a : T; }
	interaction I2 { end a : T; end b : T; }
	interaction I3;
	abstract interaction I4 { end a : T; }
	interaction I5 :> I2;
	interaction I6 :> Transfers::Transfer;
}`
	if got, want := relatedElementsLines(t, src, true), []int{3, 5}; !sameLines(got, want) {
		t.Fatalf("related-elements on lines %v, want %v", got, want)
	}
}

// A KerML connector relates the features its ends reference: a `from`/`to`
// clause, a `then` succession, a binding and `end` features with a reference
// clause all count; a typed connector with no ends of its own, or an end that
// only declares a type, does not (pilot validateConnectorRelatedFeatures).
func TestW10BKerMLConnectorRelatedFeatures(t *testing.T) {
	const src = `package P {
	class T { feature v; }
	assoc A { end feature a : T; end feature b : T; }
	classifier C {
		feature x : T; feature y : T;
		flow f1 of T;
		flow f2;
		connector c1 : A;
		connector c2;
		connector c5 : A { end feature redefines a references x; end feature redefines b references y; }
		connector c6 : A { end feature redefines a ::> x; }
		connector c7 { end feature ea ::> x; end feature eb ::> y; }
		connector c8 { end feature ea : T; end feature eb : T; }
		abstract connector c9;
		succession s1 first x then y;
		binding b1 of x = y;
		binding b2;
		connector x to y;
		connector [0..1] x to [1..*] y;
		connector e ::> x to y;
		connector link from x to y;
		connector n (x, y, x);
		flow f3 from x.v to y.v;
		flow f4 :> f3;
	}
}`
	if got, want := relatedElementsLines(t, src, true), []int{6, 7, 8, 9, 11, 13, 17}; !sameLines(got, want) {
		t.Fatalf("related-elements on lines %v, want %v", got, want)
	}
}

// One referenced end inherited through two abstract intermediates relates one
// feature, not one per path; a diamond over a two-ended base stays silent.
func TestW10BKerMLDiamondInheritance(t *testing.T) {
	const src = `package P {
	classifier K {
		feature x; feature y;
		abstract connector one { end feature e references x; }
		abstract connector mid1 :> one;
		abstract connector mid2 :> one;
		connector leaf :> mid1, mid2;
		connector two from x to y;
		connector mid3 :> two;
		connector mid4 :> two;
		connector leaf2 :> mid3, mid4;
	}
}`
	if got, want := relatedElementsLines(t, src, true), []int{7}; !sameLines(got, want) {
		t.Fatalf("related-elements on lines %v, want %v", got, want)
	}
}

// SysML: a definition or usage that relates fewer than two features is
// reported; a message or flow with no ends is abstract in the reference, and a
// usage typed by a definition with two ends inherits them.
func TestW10BSysMLRelatedFeatures(t *testing.T) {
	const src = `package P {
	part def T { attribute v; }
	connection def CD { end a : T; end b : T; }
	connection def E;
	interface def IE;
	allocation def AE;
	abstract connection def AC;
	part v {
		part x : T; part y : T;
		message m1 of T;
		flow f1 of T;
		flow f2 from x.v to y.v;
		connection c1 : CD;
		connection c2 : CD connect x to y;
		connection c3 : CD { end ::> x; end ::> y; }
		connection c4 : CD { end ::> x; }
		connection c5 : CD { end a : T; end b : T; }
		binding bind x = y;
		succession first x then y;
		allocation al allocate x to y;
		port px; port py;
		interface i connect px to py;
		abstract connection c6 : CD;
	}
}`
	if got, want := relatedElementsLines(t, src, false), []int{4, 5, 13, 16, 17}; !sameLines(got, want) {
		t.Fatalf("related-elements on lines %v, want %v", got, want)
	}
}
