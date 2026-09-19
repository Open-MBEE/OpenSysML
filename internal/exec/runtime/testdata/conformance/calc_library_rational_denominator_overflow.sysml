// The binary64 nearest 0.0001 has denominator 2^66, past the Integer range,
// so denom reports the overflow rather than a truncated Integer.
package test {
	private import ScalarValues::*;

	calc def tooFine {
		return : Integer = RationalFunctions::denom(0.0001);
	}
}
