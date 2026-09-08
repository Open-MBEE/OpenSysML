package passes

import (
	"slices"
	"testing"
)

// enumPrelude declares types outside the scalar lattice: an enumeration, a
// structural hierarchy, and an unrelated definition.
const valuePrelude = `package M {
	enum def Color { red; green; }
	enum def Size { small; large; }
	part def Vehicle;
	part def Truck specializes Vehicle;
	part def Boat;
}
`

func valueDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	return typeDiags(t, scalarPrelude+valuePrelude+src)
}

func wantOneValueDiag(t *testing.T, src, want string) {
	t.Helper()
	diags := valueDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if got := diags[0].Message; got != want {
		t.Fatalf("expected message %q, got %q", want, got)
	}
}

func wantNoValueDiags(t *testing.T, src string) {
	t.Helper()
	if diags := valueDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestValueEnumerationLiteralOfSameEnumOK(t *testing.T) {
	wantNoValueDiags(t, `package P { attribute c : M::Color = M::Color::red; }`)
}

func TestValueEnumerationLiteralOfOtherEnum(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute c : M::Color = M::Size::small; }`,
		"cannot bind a value of type Size to a feature typed by Color")
}

func TestValueScalarLiteralToEnumeration(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute c : M::Color = 5; }`,
		"cannot bind Natural value to a feature typed by Color")
}

func TestValueSubtypeInstanceConforms(t *testing.T) {
	wantNoValueDiags(t, `package P {
		part t : M::Truck;
		part v : M::Vehicle = t;
	}`)
}

func TestValueUnrelatedInstanceDoesNot(t *testing.T) {
	wantOneValueDiag(t, `package P {
		part b : M::Boat;
		part v : M::Vehicle = b;
	}`, "cannot bind a value of type Boat to a feature typed by Vehicle")
}

// A binding equates the two features, so a supertype-typed value may still
// hold an instance of the subtype; only unrelated types are rejected.
func TestValueSupertypeInstanceConforms(t *testing.T) {
	wantNoValueDiags(t, `package P {
		part v : M::Vehicle;
		part t : M::Truck = v;
	}`)
}

func TestValueTooManyValuesForUpperBound(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[2] = (1, 2, 3); }`,
		"3 value(s) bound to a feature with multiplicity upper bound 2")
}

func TestValueTooFewValuesForLowerBound(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[2] = 1; }`,
		"1 value(s) bound to a feature with multiplicity lower bound 2")
}

// An empty collection is a count of zero, which a nonzero lower bound rejects.
func TestValueEmptyCollectionForLowerBound(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[1..3] = (); }`,
		"0 value(s) bound to a feature with multiplicity lower bound 1")
}

// A redefining feature that states no multiplicity of its own is bound by the
// one it inherits (KerML 1.0 §7.3.4.5), so a default it adds is checked there.
func TestValueCountAgainstRedefinedMultiplicity(t *testing.T) {
	wantOneValueDiag(t, `package P {
		part def Base { attribute xs : ScalarValues::Integer[3]; }
		part def Derived :> Base { attribute :>> xs = (1, 2); }
	}`, "2 value(s) bound to a feature with multiplicity lower bound 3")
}

func TestValueSingleValueForRangeOK(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute opt : ScalarValues::Integer[0..1] = 7;
		attribute many : ScalarValues::Integer[1..*] = (1, 2, 3);
		attribute exact : ScalarValues::Integer[3] = (1, 2, 3);
	}`)
}

// A reference may itself be multi-valued, so its count is not known statically
// and the multiplicity check must stay silent rather than assume one value.
func TestValueReferenceCountUnknown(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute xs : ScalarValues::Integer[2] = (1, 2);
		attribute ys : ScalarValues::Integer[2] = xs;
	}`)
}

// Every element of a collection is checked against the feature's type, not just
// the first.
func TestValueCollectionElementTypes(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute names : ScalarValues::String[*] = ("a", 2); }`,
		"cannot bind Natural value to a feature typed by String")
}

// A nested collection binds flat, so its elements are checked against the
// feature's type like the outer ones.
func TestValueNestedCollectionElementTypes(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[1..*] = (1, (2, "s")); }`,
		"cannot bind String value to a feature typed by Integer")
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[1..*] = (1, ((), (2, 3.5))); }`,
		"cannot bind Rational value to a feature typed by Integer")
	wantOneValueDiag(t, `package P {
		attribute cs : M::Color[*] = (M::Color::red, (M::Color::red, M::Size::large));
	}`, "cannot bind a value of type Size to a feature typed by Color")
	wantNoValueDiags(t, `package P {
		attribute xs : ScalarValues::Integer[1..*] = (1, ((), (2, 3)));
		attribute cs : M::Color[*] = (M::Color::red, (M::Color::red, M::Color::red));
	}`)
}

func TestValueCollectionOfEnumerationLiterals(t *testing.T) {
	wantOneValueDiag(t, `package P {
		attribute cs : M::Color[*] = (M::Color::red, M::Size::large);
	}`, "cannot bind a value of type Size to a feature typed by Color")
}

// An unresolved or untyped feature must not produce a diagnostic: the checker
// only reports when both sides are known.
func TestValueUntypedFeatureNotReported(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute unknownType : Missing = 5;
		part p = 5;
	}`)
}

// A part constructor is not an invocation target because parts are not
// behaviors.
func TestValuePartConstructorInvocationReported(t *testing.T) {
	diags := valueDiags(t, `package P {
		part def Wheel;
		part w : Wheel = Wheel();
	}`)
	if len(diags) != 1 || diags[0].Code != "invocation-not-behavior" {
		t.Fatalf("expected invocation-not-behavior, got %v", diags)
	}
}

// A referenced value's type name must resolve where the value was declared: the
// scope the declaration owns also exposes its own members, which would shadow
// the type it names and make a well-formed model report a mismatch.
func TestValueTypeNameNotShadowedByOwnMembers(t *testing.T) {
	wantNoValueDiags(t, `package P {
		private import M::*;
		part t : Truck {
			attribute Truck : ScalarValues::Integer = 1;
		}
		part v : Vehicle = t;
	}`)
}

// Binding flattens a nested collection literal, so its elements are counted the
// way the runtime materializes them.
func TestValueNestedCollectionCountsItsElements(t *testing.T) {
	wantNoValueDiags(t, `package P { attribute xs : ScalarValues::Integer[4] = ((1, 2), (3, 4)); }`)
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[3] = ((1, 2), (3, 4)); }`,
		"4 value(s) bound to a feature with multiplicity upper bound 3")
}

// A collection element that is a reference may itself hold several values, so
// the count is left to the runtime rather than guessed at one per element.
func TestValueCollectionOfReferencesIsNotCountedStatically(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute src : ScalarValues::Integer[3] = (1, 2, 3);
		attribute xs : ScalarValues::Integer[2] = (src, src);
	}`)
}

// A feature typed by several types, or a value so typed, binds when any one
// pairing of their types conforms one way or the other (KerML 8.3.4.3); only
// wholly unrelated types are rejected. The pinned pilot agrees.
func TestValueMultiTypedFeaturesConformByAnyPairing(t *testing.T) {
	wantNoValueDiags(t, `package P {
		part vb : M::Vehicle, M::Boat;
		part v : M::Vehicle = vb;
		part b : M::Boat = vb;
		part t : M::Truck = vb;
		part w : M::Vehicle, M::Boat = t;
		calc def GivesVB { return : M::Vehicle, M::Boat; }
		part b2 : M::Boat = GivesVB();
		part t2 : M::Truck = GivesVB();
	}`)
	wantOneValueDiag(t, `package P {
		part def Plane;
		part vb : M::Vehicle, M::Boat;
		part p : Plane = vb;
	}`, "cannot bind a value of type Vehicle, Boat to a feature typed by Plane")
	wantOneValueDiag(t, `package P {
		part def Plane;
		part p : Plane;
		part vb : M::Vehicle, M::Boat = p;
	}`, "cannot bind a value of type Plane to a feature typed by Vehicle, Boat")
	wantOneValueDiag(t, `package P {
		part def Plane;
		calc def GivesVB { return : M::Vehicle, M::Boat; }
		part p : Plane = GivesVB();
	}`, "cannot bind a value of type Vehicle, Boat to a feature typed by Plane")
}

// A feature typed by a scalar and a structural type keeps the lattice rules
// for scalar values, while a non-scalar value is judged against every declared
// type as it is for any other feature. The pinned pilot warns on each rejection.
func TestValueMixedScalarAndStructuralTypes(t *testing.T) {
	wantNoValueDiags(t, `package P {
		part v : M::Vehicle;
		part t : M::Truck;
		attribute n : ScalarValues::Integer;
		ref a : M::Vehicle, ScalarValues::Integer = t;
		ref b : M::Vehicle, ScalarValues::Integer = 1;
		ref c : M::Vehicle, ScalarValues::Integer = n;
		ref d : ScalarValues::Integer, M::Vehicle = v;
	}`)
	wantOneValueDiag(t, `package P {
		part b : M::Boat;
		ref a : M::Vehicle, ScalarValues::Integer = b;
	}`, "cannot bind a value of type Boat to a feature typed by Vehicle, Integer")
	wantOneValueDiag(t, `package P {
		part b : M::Boat;
		ref a : ScalarValues::Integer, M::Vehicle = b;
	}`, "cannot bind a value of type Boat to a feature typed by Integer, Vehicle")
	wantOneValueDiag(t, `package P {
		part b : M::Boat;
		attribute a : ScalarValues::Integer = b;
	}`, "cannot bind a value of type Boat to a feature typed by Integer")
	wantOneValueDiag(t, `package P {
		calc def GivesBoat { return : M::Boat; }
		ref a : M::Vehicle, ScalarValues::Integer = GivesBoat();
	}`, "cannot bind a value of type Boat to a feature typed by Vehicle, Integer")
	wantOneValueDiag(t, `package P {
		ref a : M::Vehicle, ScalarValues::Integer = "s";
	}`, "cannot bind String value to a feature typed by Integer")
}

// Indexing selects an element: one of the feature's type for a sequence, and
// one of unknown type for a Collection, which no feature type rejects.
func TestValueIndexedCollectionElementIsUnknown(t *testing.T) {
	if diags := libraryTypeDiags(t, `package P {
		private import Collections::*;
		part def Vehicle;
		attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
		attribute i : ScalarValues::Integer = a#(3, 1);
		part v : Vehicle = a#(1);
	}`); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
	wantNoValueDiags(t, `package P {
		part vs : M::Vehicle[2];
		part v : M::Vehicle = vs#(1);
	}`)
	wantOneValueDiag(t, `package P {
		part bs : M::Boat[2];
		part v : M::Vehicle = bs#(1);
	}`, "cannot bind a value of type Boat to a feature typed by Vehicle")
}

// collectionValueDiags is the type diagnostics of a model of vehicles and boats whose
// collection values, in members, are judged element by element against the library.
func collectionValueDiags(t *testing.T, members string) []string {
	t.Helper()
	diags := libraryTypeDiags(t, `package P {
		private import ScalarValues::*;
		private import ControlFunctions::*;
		part def Vehicle; part def Truck :> Vehicle; part def Boat;
		part vs : Vehicle[*];
		part one : Vehicle[1];
		part two : Vehicle[2..*];
		part none : Vehicle[0];
		part boat : Boat;
		function Boats { in v : Vehicle; return r : Boat; }
		function Sail { in b : Boat; return r : Boat; }
		function Drive { in v : Vehicle; return r : Vehicle; }
		function Half { in v : Vehicle; return r : Real; }
		function Name { in v : Vehicle; return r : String; }
		part def Pair { part items : Vehicle[2]; part item : Vehicle[1]; }
		part def Pairs :> Pair { part :>> items; part :>> item; }
		part pair : Pairs[1];
		part couple : Pairs[2];
		`+members+`
	}`)
	var got []string
	for _, d := range diags {
		got = append(got, d.Message)
	}
	return got
}

func wantCollectionValueDiags(t *testing.T, members string, want ...string) {
	t.Helper()
	if got := collectionValueDiags(t, members); !slices.Equal(got, want) {
		t.Errorf("%s:\n got %q\nwant %q", members, got, want)
	}
}

// A collection value binds by the elements it maps to or keeps, not by the Anything the
// library declares its result: collect and `xs.{…}` by the body's result, select and
// selectOne by the collection's elements, a sequence-valued body element by element.
func TestValueCollectionResultIsJudged(t *testing.T) {
	wantCollectionValueDiags(t, `part b : Boat = vs.{ in v : Vehicle; v };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs->collect { in v : Vehicle; v };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs->collect Drive;`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs->select { in v : Vehicle; true };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs->selectOne { in v : Vehicle; true };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs.?{ in v : Vehicle; true };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `attribute i : Integer = vs.{ in v : Vehicle; true };`,
		"cannot bind Boolean value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->collect { in v : Vehicle; (true, 1) };`,
		"cannot bind Boolean value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->collect Name;`,
		"cannot bind a value of type String to a feature typed by Integer")
	wantCollectionValueDiags(t, `part b : Boat = vs.{ in v : Vehicle; (v, boat) };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `
		part t : Truck = vs->collect { in v : Vehicle; v };
		part v : Vehicle = vs->select { in v : Vehicle; true };
		part v2 : Vehicle = vs.?{ in v : Vehicle; true };
		part b : Boat = vs->collect { in v : Vehicle; boat };
		part b2 : Boat = vs->collect Boats;
		attribute i : Integer = vs->collect { in v : Vehicle; (1, 2) };
		attribute b3 : Boolean = vs->forAll { in v : Vehicle; true };
		part open : Boat = vs.{ in v; v };
		part open2 : Boat = vs.{ in v; (v, boat) };`)
}

// A scalar element a collection value spells out is exact, as a literal bound directly is:
// a decimal does not bind to an Integer feature because Integer values are Real. An
// element a feature or function result types only bounds its values, so it binds either way.
func TestValueCollectionElementLiteralIsExact(t *testing.T) {
	wantCollectionValueDiags(t, `attribute i : Integer = vs.{ in v : Vehicle; 1.5 };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->collect { in v : Vehicle; 1.5 };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->collect { in v : Vehicle; (1, 2.5) };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute n : Natural = vs.{ in v : Vehicle; -1 };`,
		"cannot bind Integer value to a feature typed by Natural")
	wantCollectionValueDiags(t, `attribute i : Integer = (1, 2)->reduce { in a : Integer; in b : Integer; 1.5 };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = (1.5, 2.5)->select { in a : Real; true };`,
		"cannot bind Rational value to a feature typed by Integer",
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = (1.5, 2.5).?{ in a : Real; true };`,
		"cannot bind Rational value to a feature typed by Integer",
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `
		attribute r : Real;
		attribute i : Integer = vs.{ in v : Vehicle; r };
		attribute i2 : Integer = vs->collect { in v : Vehicle; r / 2 };
		attribute i3 : Integer = vs->collect Half;
		attribute i4 : Integer = vs.{ in v : Vehicle; 2 };
		attribute r2 : Real = vs.{ in v : Vehicle; 2 };
		attribute i5 : Integer = (1, 2)->select { in a : Integer; true };
		attribute i6 : Integer = (1, 2).?{ in a : Integer; true };`)
}

// A collection value's body is checked once, as inferring the value: reading the types of
// the elements it produces to judge their binding reports nothing again.
func TestValueCollectionBodyIsCheckedOnce(t *testing.T) {
	wantCollectionValueDiags(t, `attribute i : Integer = vs.{ in v : Vehicle; 1 + true };`,
		"operator '+' is not defined for Natural and Boolean")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->collect { in v : Vehicle; 1 + true };`,
		"operator '+' is not defined for Natural and Boolean")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->collect { in v : Vehicle; (1 + true, 2.5) };`,
		"operator '+' is not defined for Natural and Boolean",
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = one->reduce { in a : Vehicle; in b : Vehicle; 1 + true };`,
		"cannot bind a value of type Vehicle to a feature typed by Integer",
		"operator '+' is not defined for Natural and Boolean")
}

// reduce returns the reducer's result, or the collection's one element unreduced:
// both bind unless the collection is known to hold two or more.
func TestValueReduceResultIsJudged(t *testing.T) {
	wantCollectionValueDiags(t, `part b : Boat = vs->reduce { in a : Vehicle; in b : Vehicle; boat };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = one->reduce { in a : Vehicle; in b : Vehicle; boat };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part v : Vehicle = vs->reduce { in a : Vehicle; in b : Vehicle; boat };`,
		"cannot bind a value of type Boat to a feature typed by Vehicle")
	wantCollectionValueDiags(t, `attribute s : String = (1, 2)->reduce { in a : Integer; in b : Integer; 3 };`,
		"cannot bind Natural value to a feature typed by String")
	wantCollectionValueDiags(t, `part b : Boat = pair.item->reduce { in a : Vehicle; in b : Vehicle; boat };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `
		part b : Boat = two->reduce { in a : Vehicle; in b : Vehicle; boat };
		part b2 : Boat = pair.items->reduce { in a : Vehicle; in b : Vehicle; boat };
		part b3 : Boat = couple.item->reduce { in a : Vehicle; in b : Vehicle; boat };
		part v : Vehicle = vs->reduce { in a : Vehicle; in b : Vehicle; a };
		attribute s : String = (1, 2)->reduce { in a : Integer; in b : Integer; "s" };
		attribute s2 : String = (one, boat)->reduce { in a : Vehicle; in b : Vehicle; "s" };`)
}

// reduce over a collection known to hold nothing returns nothing, and never applies the
// reducer: neither the reducer's result nor the collection's element is bound.
func TestValueReduceOfNothingIsJudgedByNeither(t *testing.T) {
	wantCollectionValueDiags(t, `
		attribute s : String = ()->reduce { in a : Integer; in b : Integer; 3 };
		attribute s2 : String = none->reduce { in a : Vehicle; in b : Vehicle; 3 };
		part b : Boat = none->reduce { in a : Vehicle; in b : Vehicle; a };
		part b2 : Boat = none.item->reduce { in a : Vehicle; in b : Vehicle; a };
		attribute s3 : String = ()->collect { in a : Integer; 3 };
		part b3 = Sail(none->reduce { in a : Vehicle; in b : Vehicle; a });`)
}

// A collection value passed as an argument is typed by its elements, so the parameter
// it binds is judged and the overload selected by them.
func TestArgumentCollectionResultIsJudged(t *testing.T) {
	wantCollectionValueDiags(t, `part b = Sail(vs->collect { in v : Vehicle; v });`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `part b = Sail(vs->select { in v : Vehicle; true });`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `part b = Sail(vs->selectOne { in v : Vehicle; true });`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `part b = Sail(vs.{ in v : Vehicle; v });`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `part b = Sail(vs.?{ in v : Vehicle; true });`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `part v = Drive(two->reduce { in a : Vehicle; in b : Vehicle; boat });`,
		"argument 1 of Drive expects Vehicle, found Boat")
	wantCollectionValueDiags(t, `
		part b = Sail(vs->collect { in v : Vehicle; boat });
		part v = Drive(vs->select { in v : Vehicle; true });
		part v4 = Drive(vs.?{ in v : Vehicle; true });
		part v2 = Drive(vs->reduce { in a : Vehicle; in b : Vehicle; boat });
		part v3 = Drive(vs->reduce { in a : Vehicle; in b : Vehicle; a });`)
}
