// sum0 asserts isZero(zero) of its identity argument, so another value is
// reported rather than folded in.
package test {
	private import ScalarValues::*;

	calc def total {
		return : Integer = NumericalFunctions::sum0((1, 2), 1);
	}
}
