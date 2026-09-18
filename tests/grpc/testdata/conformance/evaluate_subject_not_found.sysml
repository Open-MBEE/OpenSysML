// A subject naming nothing is a wrong request, reported in band rather than
// evaluated against declared defaults.
package demo {
	private import ScalarValues::*;

	part def Car {
		attribute mass : Real = 1500.0;
	}
}
