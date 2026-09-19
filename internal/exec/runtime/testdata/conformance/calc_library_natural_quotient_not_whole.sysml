// NaturalFunctions::'/' returns the Natural it declares, so a quotient no
// Natural equals is reported rather than truncated or answered as a Rational.
package test {
	private import ScalarValues::*;

	calc def halve {
		return : Natural = NaturalFunctions::'/'(7, 2);
	}
}
