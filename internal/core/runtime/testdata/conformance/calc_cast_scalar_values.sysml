// `x as T` selects, it does not convert (KerML 1.1 §8.3.4.9): a scalar is kept
// where T classifies the value it is by its own type — an integer is an Integer
// and so a Rational, Real and Number; a real is a Rational and never an Integer,
// whole or not — and is dropped where T excludes it. Natural and Positive are
// integers no evaluation marks, so an integer their bounds do not refuse
// (KerML 1.1 §9.3.2.2.4, §9.3.2.2.7) cannot be decided by the value.
package test {
	private import ScalarValues::*;

	calc def ScalarCasts {
		in whole : Real = 4.0;
		in fraction : Real = 2.5;
		out wholeAsInteger : Integer[0..1] = whole as Integer;
		out wholeAsRational : Rational[0..1] = whole as Rational;
		out fractionAsInteger : Integer[0..1] = fraction as Integer;
		out fractionAsRational : Rational[0..1] = fraction as Rational;
		out integerAsReal : Real[0..1] = 7 as Real;
		out negativeAsNatural : Natural[0..1] = -7 as Natural;
		out zeroAsPositive : Positive[0..1] = 0 as Positive;
		out negativeAsPositive : Positive[0..1] = -1 as Positive;
		out integerAsNumber : Number[0..1] = 7 as Number;
		out realAsScalarValue : ScalarValue[0..1] = fraction as ScalarValue;
		out booleanAsBoolean : Boolean[0..1] = true as Boolean;
		out stringAsString : String[0..1] = "kept" as String;
	}

	calc c : ScalarCasts;

	attribute integerAsNatural : Natural[0..1] = 7 as Natural;
	attribute zeroAsNatural : Natural[0..1] = 0 as Natural;
	attribute oneAsPositive : Positive[0..1] = 1 as Positive;
	attribute positivesOfMixed : Positive[0..*] = (2, 0, -1, 3) as Positive;
	attribute declaredNatural : Natural = 7;
	attribute declaredAsNatural : Natural[0..1] = declaredNatural as Natural;
	attribute declaredAsPositive : Positive[0..1] = declaredNatural as Positive;
}
