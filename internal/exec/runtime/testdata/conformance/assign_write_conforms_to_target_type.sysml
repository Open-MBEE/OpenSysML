// A write is legal when its value is one the target feature may hold: a Rover
// written to a reference typed by Vehicle is a value of that feature by
// specialization, and an Integer written to a Real is one by the scalar lattice
// the analyzer already applies to an initial value, as is a decimal written to
// a Rational, which a decimal denotes exactly.
package test {
	private import ScalarValues::*;

	part def Vehicle;
	part def Rover :> Vehicle;

	part def Fleet {
		part scout : Rover;
		ref part lead : Vehicle;
		attribute reading : Real = 0.0;
		attribute ratio : Rational = 0.25;

		exhibit state run {
			entry; then assigning;

			state assigning {
				entry action set {
					assign lead := scout;
					assign reading := 4;
					assign ratio := 0.5;
				}
			}
		}
	}
}
