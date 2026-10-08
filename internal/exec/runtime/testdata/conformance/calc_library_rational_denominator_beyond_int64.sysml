// The Rational 10^-22 has denominator 10^22, an Integer beyond int64, so denom
// answers it exactly rather than a truncated or overflowing value.
package test {
	private import ScalarValues::*;

	calc def tooFine {
		return : Integer = RationalFunctions::denom(0.0000000000000000000001);
	}
}
