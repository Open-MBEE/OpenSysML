// A String that is no Real notation is a typed error from ToReal, not zero.
package test {
	private import ScalarValues::*;

	calc def parse {
		return : Real = RealFunctions::ToReal("1.5 meters");
	}
}
