// A library feature value is readable as a name: TrigFunctions::pi is the pi
// the feature-value seam supplies, and it computes like any other value.
package test {
	private import ScalarValues::*;

	calc def circumference {
		in r : Real;
		return : Real = 2 * TrigFunctions::pi * r;
	}
}
