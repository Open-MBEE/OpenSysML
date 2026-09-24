// TrigFunctions::deg and ::rad convert with the pi the library feature-value
// seam supplies, which their library bodies read.
package test {
	private import ScalarValues::*;
	private import TrigFunctions::*;

	calc def halfTurn {
		in theta : Real;
		return : Real = deg(rad(theta));
	}
}
