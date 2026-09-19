// rat(n, 0) is the division by zero `/` reports, never a silent infinity.
package test {
	private import ScalarValues::*;

	calc def undefined {
		return : Rational = RationalFunctions::rat(1, 0);
	}
}
