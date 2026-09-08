// A cast reads the value's own type from the library, so `x as T` decides the
// same way in a scope that writes every ScalarValues name out in full and
// imports none of them (KerML 1.1 §8.3.4.9).
package test {
	calc def QualifiedCasts {
		in whole : ScalarValues::Real = 4.0;
		in fraction : ScalarValues::Real = 2.5;
		out wholeAsInteger : ScalarValues::Integer[0..1] = whole as ScalarValues::Integer;
		out fractionAsInteger : ScalarValues::Integer[0..1] = fraction as ScalarValues::Integer;
		out fractionAsReal : ScalarValues::Real[0..1] = fraction as ScalarValues::Real;
		out stringAsString : ScalarValues::String[0..1] = "kept" as ScalarValues::String;
	}

	calc c : QualifiedCasts;
}
