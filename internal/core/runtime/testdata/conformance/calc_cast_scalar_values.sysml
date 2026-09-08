// `x as T` selects, it does not convert (KerML 1.1 §8.3.4.9): a scalar is kept
// where T classifies the value it is — the ScalarValues types are nested value
// sets, so a whole Real is an Integer and 2.5 is not — and is dropped otherwise.
package test {
	private import ScalarValues::*;

	calc def ScalarCasts {
		in whole : Real = 4.0;
		in fraction : Real = 2.5;
		out wholeAsInteger : Integer[0..1] = whole as Integer;
		out fractionAsInteger : Integer[0..1] = fraction as Integer;
		out fractionAsRational : Rational[0..1] = fraction as Rational;
		out integerAsReal : Real[0..1] = 7 as Real;
		out integerAsNatural : Natural[0..1] = 7 as Natural;
		out negativeAsNatural : Natural[0..1] = -7 as Natural;
		out zeroAsNatural : Natural[0..1] = 0 as Natural;
		out oneAsPositive : Positive[0..1] = 1 as Positive;
		out zeroAsPositive : Positive[0..1] = 0 as Positive;
		out negativeAsPositive : Positive[0..1] = -1 as Positive;
		out positivesOfMixed : Positive[0..*] = (2, 0, -1, 3) as Positive;
		out integerAsNumber : Number[0..1] = 7 as Number;
		out realAsScalarValue : ScalarValue[0..1] = fraction as ScalarValue;
		out booleanAsBoolean : Boolean[0..1] = true as Boolean;
		out stringAsString : String[0..1] = "kept" as String;
	}

	calc c : ScalarCasts;
}
