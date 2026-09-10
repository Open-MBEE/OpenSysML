package passes

import (
	"fmt"
	"strings"
	"testing"
)

// An index is a position, so a value that is not a whole number is reported
// where it is written rather than only when the expression is evaluated.
func TestIndexNonIntegerIndexReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#(1.5); }`,
		"sequence index must be an Integer")
}

func TestIndexStringIndexReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#("a"); }`,
		"sequence index must be an Integer")
}

// The library declares `in index: Positive[1]`, so 0 is not a position.
func TestIndexZeroReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#(0); }`,
		"sequence index counts from 1")
}

// The length is known only where the sequence itself is written out; there the
// index is checked against it.
func TestIndexPastWrittenSequenceReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, 2, 3)#(4); }`,
		"sequence index 4 is outside 1..3")
}

// `()` and `null` are the same expression and hold nothing, so no position is
// inside them.
func TestIndexIntoNullReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = ()#(1); }`,
		"sequence index 1 is outside 1..0")
	wantOneDiag(t,
		`package P { attribute x = null#(1); }`,
		"sequence index 1 is outside 1..0")
}

// A sequence is flat: a `null` element adds no position, a nested sequence adds
// its own, and a lone scalar is a sequence of one.
func TestIndexCountsWrittenElementsFlat(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1, null)#(2); }`,
		"sequence index 2 is outside 1..1")
	wantOneDiag(t,
		`package P { attribute x = (null, null)#(1); }`,
		"sequence index 1 is outside 1..0")
	wantOneDiag(t,
		`package P { attribute x = (1, (2, 3))#(4); }`,
		"sequence index 4 is outside 1..3")
	wantOneDiag(t,
		`package P { attribute x = 5#(2); }`,
		"sequence index 2 is outside 1..1")
	wantNoDiags(t, `package P { attribute x = (1, null, 2)#(2); }`)
	wantNoDiags(t, `package P { attribute x = (1, (2, 3))#(3); }`)
	wantNoDiags(t, `package P { attribute x = 5#(1); }`)
}

func TestIndexInRangeOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute x = (1, 2, 3)#(3); }`)
}

// A model counting positions holds them in an `Integer`, which the library's
// `Positive` parameter accepts a value of: whether that value is a position the
// operand has is known from the value, so it is checked at evaluation and not
// reported here.
func TestIndexTypedIntegerNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute xs = (1, 2, 3);
		attribute i : ScalarValues::Integer = 2;
		attribute x = xs#(i);
	}`)
}

// An element of the indexed sequence is typed once, so a mistake inside it is
// reported once rather than for each pass over the elements.
func TestIndexReportsAnErrorInAnElementOnce(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x = (1 + true, 2)#(1); }`,
		"operator '+' is not defined for Natural and Boolean")
}

// Indexing per iteration is the idiomatic spelling, so the loop variable of a
// `for` over a sequence of numbers is an index and is not reported.
func TestIndexByLoopVariableNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		private import ScalarValues::*;
		calc total {
			attribute xs : Integer[*] = (1, 2, 3);
			attribute sum : Integer = 0;
			for i in (1, 2, 3) {
				sum = sum + xs#(i);
			}
			return : Integer = sum;
		}
	}`)
}

// An index of a value whose length the checker does not know is not reported:
// the runtime checks the position it turns out to be.
func TestIndexOfReferenceNotReported(t *testing.T) {
	wantNoDiags(t, `package P { attribute xs = (1, 2, 3); attribute x = xs#(4); }`)
}

// The element type of a written sequence is the type of the indexing, so
// binding it to a feature of another type is reported.
func TestIndexElementTypeIsCheckedAgainstTheBinding(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute x : ScalarValues::Boolean = (1, 2, 3)#(1); }`,
		"cannot bind Natural value to a feature typed by Boolean")
}

// A sequence of no one scalar type has no element type, so indexing it is
// checked no further here; the runtime answers the element it turns out to be.
func TestIndexOfMixedSequenceHasNoElementType(t *testing.T) {
	wantNoDiags(t, `package P { attribute x : ScalarValues::Boolean = (1, "a")#(1); }`)
}

// An element whose type is not known leaves the element type of the sequence
// unknowable, so nothing is reported about what an index of it holds: `flag` is
// a Boolean here, and typing the sequence from the elements that happen to be
// known would reject the model for it.
func TestIndexOfSequenceWithAnUntypedElementIsNotReported(t *testing.T) {
	wantNoDiags(t, `package P {
		attribute flag = true;
		attribute y : ScalarValues::Boolean = (1, flag)#(2);
	}`)
}

// The bracket form is a quantity, not an index, so the unit name in it is not
// checked as a position.
func TestQuantityBracketFormIsNotIndexed(t *testing.T) {
	wantNoDiags(t, `package P { attribute def m; attribute x = 5 [m]; }`)
}

// A selector states a condition, so a body whose result is of a known type that
// is not Boolean is reported where it is written.
func TestSelectNonBooleanBodyReported(t *testing.T) {
	wantOneDiag(t,
		`package P { attribute xs = (1, 2, 3); attribute x = xs.?{in e; 1}; }`,
		"select predicate must be Boolean")
}

func TestSelectBooleanBodyOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute xs = (1, 2, 3); attribute x = xs.?{in e; e > 1}; }`)
}

// An untyped body parameter is of the element type of the collection the body is applied
// to (KerML 8.3.4.8), so a member it names is resolved and checked: `x.nosuch` is reported
// as unresolved in every notation, `x.mass` binds as a MassValue does, and over a collection
// whose elements cannot be typed the parameter has no members to read, as an untyped
// feature has none.
func TestBodyParameterTakesElementType(t *testing.T) {
	const model = `package P {
		private import ScalarValues::*;
		private import ISQ::*;
		private import ControlFunctions::*;
		part def C { attribute mass :> ISQ::mass; }
		part cs : C[*];
		attribute anys;
		%s
	}`
	for _, member := range []string{
		`attribute bad = cs.{in x; x.nosuch};`,
		`attribute bad = cs.?{in x; x.nosuch};`,
		`attribute bad = cs->collect {in x; x.nosuch};`,
		`attribute bad = cs->forAll {in x; x.nosuch};`,
		`attribute bad = collect(cs, {in x; x.nosuch});`,
		`attribute bad = collect(mapper = {in x; x.nosuch}, collection = cs);`,
		`attribute bad = cs->reduce {in a; in b; b.nosuch};`,
		`attribute bad = cs.{in x; cs.{in y; y.nosuch}};`,
	} {
		diags := libraryDiags(t, fmt.Sprintf(model, member))
		if len(diags) != 1 || diags[0].Source != "name-resolution" || diags[0].Message != "unresolved member: nosuch" {
			t.Errorf("%s: got %v, want one unresolved-member diagnostic naming nosuch", member, diags)
		}
	}
	diags := libraryDiags(t, fmt.Sprintf(model, `attribute s : String = cs.{in x; x.mass};`))
	if len(diags) != 1 || diags[0].Source != "type" || !strings.Contains(diags[0].Message, "MassValue") {
		t.Errorf("x.mass as String: got %v, want one type diagnostic naming MassValue", diags)
	}
	wantLibraryClean(t, fmt.Sprintf(model, `attribute m : MassValue = cs.{in x; x.mass}; part c : C = cs->selectOne {in x; x.mass > 1 [SI::kg]};`))
	for _, member := range []string{
		`attribute s : String = anys.{in x; x.mass};`,
		`attribute open = anys.{in x; x.nosuch};`,
	} {
		diags := libraryDiags(t, fmt.Sprintf(model, member))
		if len(diags) != 1 || diags[0].Source != "name-resolution" || !strings.HasPrefix(diags[0].Message, "no scope for member lookup in x") {
			t.Errorf("%s: got %v, want one diagnostic that x has no members to read", member, diags)
		}
	}
}

func TestCollectBodyOK(t *testing.T) {
	wantNoDiags(t, `package P { attribute xs = (1, 2, 3); attribute x = xs.{in e; e * 2}; }`)
}

// A body parameter is visible in the body and nowhere else.
func TestBodyParameterIsNotVisibleOutsideTheBody(t *testing.T) {
	diags := exprDiags(t,
		`package P { attribute xs = (1, 2, 3); attribute x = xs.{in e; e * 2}; attribute y = e; }`)
	if len(diags) != 0 {
		t.Fatalf("expected the type tier to leave the outside reference to the name tier, got %v", diags)
	}
}
