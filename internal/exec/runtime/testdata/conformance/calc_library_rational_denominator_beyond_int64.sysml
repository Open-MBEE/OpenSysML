// The binary64 nearest 0.0001 has denominator 2^66, an Integer beyond int64,
// so denom answers it exactly rather than a truncated or overflowing value.
package test {
	private import ScalarValues::*;

	calc def tooFine {
		return : Integer = RationalFunctions::denom(0.0001);
	}
}
