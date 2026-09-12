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

// A constant equal to an enumerated value is admitted and any other refused;
// a value of another kind is the scalar lattice's to report, once.
func TestValueScalarConstantToScalarValuedEnumeration(t *testing.T) {
	const level = `package L {
		enum def Level :> ScalarValues::Integer { low = 1; high = 3; }
		enum def Grade :> ScalarValues::Real { a = 4.0; b = 3.0; }
	}
	`
	wantNoValueDiags(t, level+`package P {
		attribute l : L::Level = 3;
		attribute h : L::Level = L::Level::high;
		attribute g : L::Grade = 4;
		attribute many : L::Level[*] = (1, 3);
		attribute n : ScalarValues::Integer = 2;
		attribute fromFeature : L::Level = n;
		attribute folded : L::Level = 1 + 2;
		attribute computed : L::Level = n + 1;
	}`)
	wantOneValueDiag(t, level+`package P { attribute l : L::Level = 1 + 1; }`,
		"cannot bind 2 (an Integer) to a feature typed by Level, whose values are Level::low = 1, Level::high = 3")
	wantOneValueDiag(t, level+`package P { attribute l : L::Level = 2; }`,
		"cannot bind 2 (an Integer) to a feature typed by Level, whose values are Level::low = 1, Level::high = 3")
	wantOneValueDiag(t, level+`package P { attribute g : L::Grade = 2.5; }`,
		"cannot bind 2.5 (a Real) to a feature typed by Grade, whose values are Grade::a = 4.0, Grade::b = 3.0")
	wantOneValueDiag(t, level+`package P { attribute many : L::Level[*] = (1, 2); }`,
		"cannot bind 2 (an Integer) to a feature typed by Level, whose values are Level::low = 1, Level::high = 3")
	wantOneValueDiag(t, level+`package P { attribute l : L::Level = "x"; }`,
		"cannot bind String value to a feature typed by Integer")
	wantOneValueDiag(t, `package L { enum def Flag :> ScalarValues::Boolean { off = false; on = true; } }
		package P { attribute f : L::Flag = 1; }`,
		"cannot bind Natural value to a feature typed by Boolean")
	wantOneValueDiag(t, `package L { enum def Empty :> ScalarValues::Integer {} }
		package P { attribute e : L::Empty = 1; }`,
		"cannot bind 1 (an Integer) to a feature typed by Empty, which enumerates no values")
	const size = `package L { enum def Size :> ScalarValues::Real { = 60.0; = 70.0; } }`
	wantNoValueDiags(t, size+`package P { attribute s : L::Size = 60.0; }`)
	wantOneValueDiag(t, size+`package P { attribute s : L::Size = 65.0; }`,
		"cannot bind 65.0 (a Real) to a feature typed by Size, whose values are 60.0, 70.0")
	const wide = size + `package W { enum def Wide :> L::Size { = 80.0; } }`
	wantNoValueDiags(t, wide+`package P { attribute s : W::Wide = 60.0; attribute w : W::Wide = 80.0; }`)
	wantOneValueDiag(t, wide+`package P { attribute s : W::Wide = 65.0; }`,
		"cannot bind 65.0 (a Real) to a feature typed by Wide, whose values are 80.0, 60.0, 70.0")
	const mixed = `package L { enum def Level :> ScalarValues::Integer { unknown; high = 3; } }`
	wantNoValueDiags(t, mixed+`package P { attribute l : L::Level = 3; }`)
	wantOneValueDiag(t, mixed+`package P { attribute l : L::Level = 2; }`,
		"cannot bind 2 (an Integer) to a feature typed by Level, whose values are Level::high = 3")
	wantOneValueDiag(t, `package L { enum def Level :> ScalarValues::Integer { unknown; } }
		package P { attribute l : L::Level = 2; }`,
		"cannot bind 2 (an Integer) to a feature typed by Level, whose values are only its literals")
	wantOneValueDiag(t, `package L { enum def Color { red; green; } }
		package P { attribute c : L::Color = 2; }`,
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
		attribute cs : M::Color[*] nonunique = (M::Color::red, (M::Color::red, M::Color::red));
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

// collectionValueDiags is the name-resolution and type diagnostics of a model of vehicles and
// boats whose collection values, in members, are judged element by element against the library.
func collectionValueDiags(t *testing.T, members string) []string {
	t.Helper()
	diags := libraryDiags(t, `package P {
		private import ScalarValues::*;
		private import ControlFunctions::*;
		part def Vehicle; part def Truck :> Vehicle; part def Car :> Vehicle; part def Boat;
		part vs : Vehicle[*];
		part truck : Truck;
		part car : Car;
		part one : Vehicle[1];
		part two : Vehicle[2..*];
		part none : Vehicle[0];
		part boat : Boat;
		function Boats { in v : Vehicle; return r : Boat; }
		function Sail { in b : Boat; return r : Boat; }
		function Drive { in v : Vehicle; return r : Vehicle; }
		function Half { in v : Vehicle; return r : Real; }
		function Name { in v : Vehicle; return r : String; }
		function Nobody { in v : Vehicle; return r : Vehicle[0]; }
		function Nobody2 :> Nobody { in v : Vehicle; return r :>> r; }
		function Nobody3 :> Nobody { in v : Vehicle; return r : Vehicle; }
		alias noone for none;
		attribute nothing : Integer[0];
		part def Pair { part items : Vehicle[2]; part item : Vehicle[1]; }
		part def Pairs :> Pair { part :>> items; part :>> item; }
		part pair : Pairs[1];
		part couple : Pairs[2];
		part nobody : Pairs[0];
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
// selectOne by the collection's elements, a sequence-valued body element by element. An
// untyped body parameter is of the elements' type; over an untypable collection it is open.
func TestValueCollectionResultIsJudged(t *testing.T) {
	wantCollectionValueDiags(t, `part b : Boat = vs.{ in v : Vehicle; v };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs.{ in v; v };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs->collect { in v; (v, boat) };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `part b : Boat = vs->reduce { in a; in b; a };`,
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
		attribute b4 : Boolean = vs->forAll { in v; true };
		part b5 : Boat = vs.{ in v; boat };
		attribute anys;
		part open : Boat = anys.{ in v; v };
		part open2 : Boat = anys.{ in v; (v, boat) };`)
}

// A scalar element a collection value spells out is exact, as a literal bound directly is:
// a decimal does not bind to an Integer feature because Integer values are Real, nor does a
// quotient, a Rational whatever it divides. An element a feature or function result types
// only bounds its values, so it binds either way.
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
		attribute i2 : Integer = vs->collect { in v : Vehicle; r / 2 };`,
		"cannot bind Real value to a feature typed by Integer")
	wantCollectionValueDiags(t, `
		attribute r : Real;
		attribute i : Integer = vs.{ in v : Vehicle; r };
		attribute i3 : Integer = vs->collect Half;
		attribute i4 : Integer = vs.{ in v : Vehicle; 2 };
		attribute r2 : Real = vs.{ in v : Vehicle; 2 };
		attribute i5 : Integer = (1, 2)->select { in a : Integer; true };
		attribute i6 : Integer = (1, 2).?{ in a : Integer; true };`)
}

// An element that is itself a collection value binds by the elements it holds: a scalar
// literal nested in an inner collect, or kept by a selection, is judged as if written out.
func TestValueNestedCollectionElementsAreJudged(t *testing.T) {
	wantCollectionValueDiags(t, `attribute i : Integer = vs.{ in v : Vehicle; vs.{ in w : Vehicle; 1.5 } };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->collect { in v : Vehicle; vs->collect { in w : Vehicle; 1.5 } };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = vs->select { in v : Vehicle; true }.{ in w : Vehicle; (1, 2.5) };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = (vs.{ in v : Vehicle; 1.5 }).?{ in r : Real; true };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `attribute i : Integer = (vs.{ in v : Vehicle; 1.5 })->select { in r : Real; true };`,
		"cannot bind Rational value to a feature typed by Integer")
	wantCollectionValueDiags(t, `part b : Boat = vs.{ in v : Vehicle; vs->select { in w : Vehicle; true } };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
	wantCollectionValueDiags(t, `
		attribute i : Integer = vs.{ in v : Vehicle; vs.{ in w : Vehicle; 2 } };
		attribute r : Real = vs.{ in v : Vehicle; vs.{ in w : Vehicle; 1.5 } };
		attribute i2 : Integer = vs.{ in v : Vehicle; ().{ in w : Integer; 1.5 } };`)
}

// A feature valued by `xs.?{…}` over a sequence written out takes the type its elements share,
// as one valued by `xs->select {…}` does, so a result, a subject or a cast it is bound to is
// judged by that type rather than by the Anything the sequence is.
func TestValueSelectShorthandOfSequenceTypesFeature(t *testing.T) {
	for _, keep := range []string{`.?{ in v : Vehicle; true }`, `->select { in v : Vehicle; true }`} {
		wantCollectionValueDiags(t, `function F { return r : Boat; (one, one)`+keep+` }`,
			"Bound features should have conforming types")
		wantCollectionValueDiags(t, `
			part picked = (one, one)`+keep+`;
			function F { return r : Boat; picked }`,
			"Bound features should have conforming types")
		wantCollectionValueDiags(t, `
			part picked = (one, one)`+keep+`;
			requirement def R { subject s : Boat; }
			requirement r : R { subject s = picked; }`,
			"Bound features should have conforming types")
		wantCollectionValueDiags(t, `
			part picked = (one, one)`+keep+`;
			function F { return r : Vehicle; picked }
			requirement def R { subject s : Vehicle; }
			requirement r : R { subject s = picked; }`)
	}
	wantCollectionValueDiags(t, `
		attribute picked = (1, 2).?{ in v : Integer; true };
		part b = picked as Boat;`,
		"cast argument is typed by Integer, unrelated to the target Boat: neither type specializes the other, so the cast selects no value")
	wantCollectionValueDiags(t, `
		part any = (one, boat).?{ in v; true };
		function F { return r : Boat; any }`)
}

// Sibling elements kept or mapped to share their nearest supertype: a feature valued by a
// selection of a Truck and a Car is a Vehicle, so it is judged as one where it is bound or cast.
func TestValueSiblingElementsShareSupertype(t *testing.T) {
	for _, keep := range []string{`.?{ in v : Vehicle; true }`, `->select { in v : Vehicle; true }`} {
		wantCollectionValueDiags(t, `
			part kin = (truck, car)`+keep+`;
			function F { return r : Boat; kin }`,
			"Bound features should have conforming types")
		wantCollectionValueDiags(t, `
			part kin = (truck, car)`+keep+`;
			part b = kin as Boat;`,
			"cast argument is typed by Vehicle, unrelated to the target Boat: neither type specializes the other, so the cast selects no value")
		wantCollectionValueDiags(t, `
			part kin = (truck, car)`+keep+`;
			function F { return r : Vehicle; kin }
			part t = kin as Truck;`)
	}
	wantCollectionValueDiags(t, `
		part kin = vs.{ in v : Vehicle; (truck, car) };
		function F { return r : Boat; kin }`,
		"Bound features should have conforming types")
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
		attribute s2 : String = (one, boat)->reduce { in a : Vehicle; in b : Vehicle; "s" };
		attribute i : Integer = (two->collect Name)->reduce { in a : String; in b : String; 3 };`)
}

// A collection value binds as many values as it is known to hold: none over a collection
// holding none, one per element a collect maps over a known count, from a reduce the one element
// unreduced or as many as the reducer yields over two or more; one only bounded is reported where
// even its fewest or its most cannot fit, else left to evaluation.
func TestValueCollectionCountIsJudged(t *testing.T) {
	wantCollectionValueDiags(t, `part b : Boat[1] = none.{ in v : Vehicle; boat };`,
		"0 value(s) bound to a feature with multiplicity lower bound 1")
	wantCollectionValueDiags(t, `part b : Boat[1..2] = ()->collect { in v : Vehicle; boat };`,
		"0 value(s) bound to a feature with multiplicity lower bound 1")
	wantCollectionValueDiags(t, `part b : Boat[2] = none->select { in v : Vehicle; true };`,
		"0 value(s) bound to a feature with multiplicity lower bound 2")
	wantCollectionValueDiags(t, `part b : Boat[1] = ()->reduce { in a : Boat; in b : Boat; a };`,
		"0 value(s) bound to a feature with multiplicity lower bound 1")
	wantCollectionValueDiags(t, `part b : Boat[1] = pair.items.{ in v : Vehicle; boat };`,
		"2 value(s) bound to a feature with multiplicity upper bound 1")
	wantCollectionValueDiags(t, `part b : Boat[0..1] = pair.items->collect { in v : Vehicle; (boat, boat) };`,
		"4 value(s) bound to a feature with multiplicity upper bound 1")
	wantCollectionValueDiags(t, `attribute s : String[3] = ("a", one.{ in v : Vehicle; "b" });`,
		"2 value(s) bound to a feature with multiplicity lower bound 3")
	wantCollectionValueDiags(t, `part b : Boat[3] = pair.items->collect Boats;`,
		"2 value(s) bound to a feature with multiplicity lower bound 3")
	wantCollectionValueDiags(t, `
		part huge : Vehicle[9223372036854775807];
		attribute s : String[9223372036854775807] = (huge.{ in v : Vehicle; "a" }, "b");`,
		"more than 9223372036854775807 value(s) bound to a feature with multiplicity upper bound 9223372036854775807")
	wantCollectionValueDiags(t, `part b : Boat[1] = two.{ in v : Vehicle; boat };`,
		"at least 2 value(s) bound to a feature with multiplicity upper bound 1")
	wantCollectionValueDiags(t, `part v : Vehicle[3] = one->select { in v : Vehicle; true };`,
		"at most 1 value(s) bound to a feature with multiplicity lower bound 3")
	wantCollectionValueDiags(t, `part v : Vehicle[2] = pair.items->selectOne { in v : Vehicle; true };`,
		"at most 1 value(s) bound to a feature with multiplicity lower bound 2")
	wantCollectionValueDiags(t, `part b : Boat[1] = pair.items->reduce { in a : Vehicle; in b : Vehicle; (boat, boat) };`,
		"2 value(s) bound to a feature with multiplicity upper bound 1")
	wantCollectionValueDiags(t, `part b : Boat[1] = pair.items->reduce { in a : Vehicle; in b : Vehicle; () };`,
		"0 value(s) bound to a feature with multiplicity lower bound 1")
	wantCollectionValueDiags(t, `part b : Boat[1] = two->reduce { in a : Vehicle; in b : Vehicle; (boat, boat) };`,
		"2 value(s) bound to a feature with multiplicity upper bound 1")
	wantCollectionValueDiags(t, `part v : Vehicle[3] = vs->reduce { in a : Vehicle; in b : Vehicle; (a, b) };`,
		"at most 2 value(s) bound to a feature with multiplicity lower bound 3")
	wantCollectionValueDiags(t, `
		part b : Boat[2] = pair.items.{ in v : Vehicle; boat };
		part b2 : Boat[1] = pair.items->reduce { in a : Vehicle; in b : Vehicle; boat };
		part b5 : Boat[2] = pair.items->reduce { in a : Vehicle; in b : Vehicle; (boat, boat) };
		part b6 : Boat[0..1] = pair.items->reduce { in a : Vehicle; in b : Vehicle; () };
		part v3 : Vehicle[1..2] = vs->reduce { in a : Vehicle; in b : Vehicle; (a, b) };
		part v4 : Vehicle[1] = one->reduce { in a : Vehicle; in b : Vehicle; (a, b) };
		part v5 : Vehicle[0..1] = pair.items->reduce { in a : Vehicle; in b : Vehicle; a };
		part v : Vehicle[0..1] = pair.items->selectOne { in v : Vehicle; true };
		part b3 : Boat[1] = vs.{ in v : Vehicle; boat };
		part v2 : Vehicle[0..2] = pair.items->select { in v : Vehicle; true };
		part b4 : Boat[2] = pair.items->collect Boats;`)
}

// A collection operation over a collection known to hold nothing returns nothing, and never
// applies the reducer or body: no element is bound, though the value is still typed by them.
func TestValueReduceOfNothingIsJudgedByNeither(t *testing.T) {
	wantCollectionValueDiags(t, `
		attribute s : String = ()->reduce { in a : Integer; in b : Integer; 3 };
		attribute s2 : String = none->reduce { in a : Vehicle; in b : Vehicle; 3 };
		part b : Boat = none->reduce { in a : Vehicle; in b : Vehicle; a };
		part b2 : Boat = nobody.item->reduce { in a : Vehicle; in b : Vehicle; a };
		attribute s3 : String = ()->collect { in a : Integer; 3 };
		attribute s4 : String = none.{ in a : Vehicle; 3 };
		part b3 = Sail(none->reduce { in a : Vehicle; in b : Vehicle; a });`)
}

// A body mapping every element to nothing — a `[0]` feature or function result — holds nothing
// either, as does any operation over such a collection: no element is judged.
func TestValueMappingToNothingIsJudgedByNeither(t *testing.T) {
	wantCollectionValueDiags(t, `
		attribute s : String = vs.{ in v : Vehicle; nothing };
		attribute s2 : String = vs->collect { in v : Vehicle; (nothing, nothing) };
		part b : Boat = vs->collect Nobody;
		part b2 : Boat = vs.{ in v : Vehicle; Nobody(v) };
		attribute s3 : String = (vs.{ in v : Vehicle; nothing }).{ in n : Integer; 3 };
		attribute s4 : String = (().{ in a : Integer; a }).{ in b : Integer; 3 };
		part b3 : Boat = (none->select { in v : Vehicle; true })->reduce { in a : Vehicle; in b : Vehicle; a };
		part b4 : Boat = (vs->collect Nobody).?{ in v : Vehicle; true };
		part b5 : Boat = (vs->selectOne { in v : Vehicle; true }).{ in v : Vehicle; Nobody(v) }->collect { in v : Vehicle; v };
		part b6 = Sail(vs.{ in v : Vehicle; Nobody(v) });
		part b7 = Sail((vs->collect Nobody)->select { in v : Vehicle; true });
		part b8 : Boat = vs->collect Nobody2;
		part b11 : Boat = vs->collect Nobody3;
		part b9 : Boat = noone.{ in v : Vehicle; v };
		part b10 : Boat = noone->reduce { in a : Vehicle; in b : Vehicle; a };`)
	wantCollectionValueDiags(t, `attribute s : String = vs.{ in v : Vehicle; (nothing, 3) };`,
		"cannot bind Natural value to a feature typed by String")
	wantCollectionValueDiags(t, `part b : Boat = (vs.{ in v : Vehicle; v }).{ in v : Vehicle; v };`,
		"cannot bind a value of type Vehicle to a feature typed by Boat")
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
	wantCollectionValueDiags(t, `part v = Drive(vs->reduce { in a : Vehicle; in b : Vehicle; boat });`,
		"argument 1 of Drive expects Vehicle, found Boat")
	wantCollectionValueDiags(t, `part b = Sail(one->reduce { in a : Vehicle; in b : Vehicle; boat });`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `
		part b = Sail(vs->collect { in v : Vehicle; boat });
		part v = Drive(vs->select { in v : Vehicle; true });
		part v4 = Drive(vs.?{ in v : Vehicle; true });
		part v2 = Drive(two->reduce { in a : Vehicle; in b : Vehicle; one });
		part v3 = Drive(vs->reduce { in a : Vehicle; in b : Vehicle; a });
		part v5 = Drive(one->reduce { in a : Vehicle; in b : Vehicle; boat });
		part v6 = Drive((none, one)->reduce { in a : Vehicle; in b : Vehicle; boat });`)
}

// Each element a collection value holds is judged on its own, so a collection whose elements
// share no type does not pass unjudged, and a literal an element spells binds exactly.
func TestArgumentCollectionElementsAreJudgedSeverally(t *testing.T) {
	wantCollectionValueDiags(t, `part b = Sail(vs.{ in v : Vehicle; (v, boat) });`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `part b = Sail((boat, vs.{ in v : Vehicle; v }));`,
		"argument 1 of Sail expects Boat, found Vehicle")
	wantCollectionValueDiags(t, `part v = Drive(vs->collect { in v : Vehicle; (boat, v) });`,
		"argument 1 of Drive expects Vehicle, found Boat")
	wantCollectionValueDiags(t, `
		function Count { in n : Integer; return r : Integer; }
		part n = Count(vs.{ in v : Vehicle; 1.5 });`,
		"argument 1 of Count expects Integer, found Rational")
	wantCollectionValueDiags(t, `
		function Count { in n : Integer; return r : Integer; }
		part n = Count(n = vs->collect { in v : Vehicle; (1, 1.5) });`,
		"argument n of Count expects Integer, found Rational")
	wantCollectionValueDiags(t, `
		function Count { in n : Integer; return r : Integer; }
		attribute h : Real;
		part n = Count(vs.{ in v : Vehicle; 1 });
		part n2 = Count(vs.{ in v : Vehicle; h });
		part b = Sail((boat, vs.{ in v : Vehicle; boat }));`)
}

// A constructor argument binds each element a collection value holds to the feature: a mixed
// collection is refused by the element that does not conform, a collected literal by its exact type.
func TestConstructorCollectionElementsAreJudgedSeverally(t *testing.T) {
	wantCollectionValueDiags(t, `
		part def Fleet { part b : Boat; }
		part f = new Fleet(vs.{ in v : Vehicle; (v, boat) });`,
		"b of Fleet is typed by Boat; cannot bind a value of type Vehicle")
	wantCollectionValueDiags(t, `
		part def Fleet { part b : Boat; }
		part f = new Fleet(b = (boat, vs.{ in v : Vehicle; v }));`,
		"b of Fleet is typed by Boat; cannot bind a value of type Vehicle")
	wantCollectionValueDiags(t, `
		part def Fleet { attribute n : Integer; }
		part f = new Fleet(vs.{ in v : Vehicle; 1.5 });`,
		"n of Fleet expects Integer, found Rational")
	wantCollectionValueDiags(t, `
		part def Fleet { attribute n : Integer; }
		part f = new Fleet(n = vs->collect { in v : Vehicle; (1, 1.5) });`,
		"n of Fleet expects Integer, found Rational")
	wantCollectionValueDiags(t, `
		part def Fleet { part b : Boat; attribute n : Integer; }
		attribute h : Real;
		part f = new Fleet(vs.{ in v : Vehicle; boat }, vs.{ in v : Vehicle; 1 });
		part f2 = new Fleet(n = vs.{ in v : Vehicle; h });
		part f3 = new Fleet((boat, vs.{ in v : Vehicle; boat }));`)
}

// A multi-valued feature is unique unless declared nonunique (KerML 7.3.4.4), so a
// literal repeating a constant is refused where it is written; nonunique accepts it.
func TestValueUniqueFeatureRefusesRepeatedConstant(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[*] = (1, 1); }`,
		"1 (an Integer) is written at positions 1 and 2 of a unique feature")
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[*] ordered = (3, 1, 2, 1 + 0); }`,
		"1 (an Integer) is written at positions 2 and 4 of a unique feature")
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Real[*] = (1.5, 2.0, 1.5); }`,
		"1.5 (a Real) is written at positions 1 and 3 of a unique feature")
	wantOneValueDiag(t,
		`package P { attribute bs : ScalarValues::Boolean[*] = (true, false, true); }`,
		"true (a Boolean) is written at positions 1 and 3 of a unique feature")
	wantOneValueDiag(t,
		`package P { attribute ss : ScalarValues::String[*] = ("a", "b", "a"); }`,
		"\"a\" (string) is written at positions 1 and 3 of a unique feature")
	wantOneValueDiag(t,
		`package P { attribute cs : M::Color[*] = (M::Color::red, M::Color::green, M::Color::red); }`,
		"Color::red (enumeration literal) is written at positions 1 and 3 of a unique feature")
	wantNoValueDiags(t, `package P { attribute xs : ScalarValues::Integer[*] nonunique = (1, 1); }`)
	wantNoValueDiags(t, `package P { attribute xs : ScalarValues::Integer[*] ordered nonunique = (1, 1); }`)
	wantNoValueDiags(t, `package P { attribute xs : ScalarValues::Integer[*] = (1, 2, 3); }`)
}

// An Integer and a Real that compare equal are one value, as they are at run time.
func TestValueUniqueFeatureComparesAcrossNumericKinds(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Real[*] = (1, 2.5, 1.0); }`,
		"1.0 (a Real) is written at positions 1 and 3 of a unique feature")
}

// Only an equality the literal decides is reported: a computed element is the run time's.
func TestValueUniqueFeatureLeavesDynamicElementsToRuntime(t *testing.T) {
	wantNoValueDiags(t, `package P {
		attribute n : ScalarValues::Integer;
		attribute xs : ScalarValues::Integer[*] = (1, n, 1 + n);
	}`)
	wantOneValueDiag(t, `package P {
		attribute n : ScalarValues::Integer;
		attribute xs : ScalarValues::Integer[*] = (1, n, 1);
	}`, "1 (an Integer) is written at positions 1 and 3 of a unique feature")
}

// A redefinition stating neither keyword takes the uniqueness of what it redefines
// (KerML 7.3.4.5), so Bag's elements stay nonunique and OrderedSet's unique.
func TestValueUniquenessFollowsRedefinition(t *testing.T) {
	wantNoValueDiags(t, `package P {
		part def Base { attribute xs : ScalarValues::Integer[*] nonunique; }
		part def Derived :> Base { attribute :>> xs = (1, 1); }
	}`)
	wantOneValueDiag(t, `package P {
		part def Base { attribute xs : ScalarValues::Integer[*]; }
		part def Derived :> Base { attribute :>> xs = (1, 1); }
	}`, "1 (an Integer) is written at positions 1 and 2 of a unique feature")
}

// A metadata body declaration redefines the metadata type's feature of its name
// (KerML 7.4.7), so it repeats a value only where that feature is nonunique.
func TestValueUniquenessFollowsMetadataBodyRedefinition(t *testing.T) {
	wantNoValueDiags(t, `package P {
		metadata def M { attribute xs : ScalarValues::Integer[*] ordered nonunique; }
		part def A { @M { xs = (1, 1); } }
		metadata M about A { xs = (2, 2); }
	}`)
	wantOneValueDiag(t, `package P {
		metadata def M { attribute xs : ScalarValues::Integer[*] ordered; }
		metadata M about P { xs = (1, 1); }
	}`, "1 (an Integer) is written at positions 1 and 2 of a unique feature")
}

// The count check precedes the uniqueness check, so a literal both too long and
// repeating a value is reported for its count alone.
func TestValueCountViolationPrecedesUniqueness(t *testing.T) {
	wantOneValueDiag(t,
		`package P { attribute xs : ScalarValues::Integer[2] = (1, 1, 1); }`,
		"3 value(s) bound to a feature with multiplicity upper bound 2")
}

// Only an error on the value withholds the uniqueness check; a warning raised
// while typing an element (an always-false comparison) leaves the repeat reported.
func TestValueUniquenessReportedBesideWarning(t *testing.T) {
	diags := valueDiags(t, `package P { attribute bs : ScalarValues::Boolean[*] = (true, true, 1 == "a"); }`)
	got := map[Severity]string{}
	for _, d := range diags {
		got[d.Severity] = d.Message
	}
	if len(diags) != 2 || got[SeverityWarning] != "comparing Natural with String is always false" ||
		got[SeverityError] != "true (a Boolean) is written at positions 1 and 2 of a unique feature" {
		t.Fatalf("expected the comparison warning beside the uniqueness error, got %v", diags)
	}
}
