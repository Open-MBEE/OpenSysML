// A cast reads the value's own type from the library, so `x as T` decides the
// same way in a scope that writes every ScalarValues name out in full and
// imports none of them (KerML 1.1 §8.3.4.9).
package test {
	// Declarations wearing the ScalarValues names, which classify no scalar.
	attribute def Integer;
	attribute def String;

	calc def QualifiedCasts {
		in whole : ScalarValues::Real = 4.0;
		in fraction : ScalarValues::Real = 2.5;
		out wholeAsInteger : ScalarValues::Integer[0..1] = whole as ScalarValues::Integer;
		out wholeAsRational : ScalarValues::Rational[0..1] = whole as ScalarValues::Rational;
		out fractionAsInteger : ScalarValues::Integer[0..1] = fraction as ScalarValues::Integer;
		out fractionAsReal : ScalarValues::Real[0..1] = fraction as ScalarValues::Real;
		out stringAsString : ScalarValues::String[0..1] = "kept" as ScalarValues::String;
		out shadowedInteger : ScalarValues::Integer[0..1] = 3 as ScalarValues::Integer;
		out localInteger : Integer[0..1] = 3 as Integer;
	}

	calc c : QualifiedCasts;
}
