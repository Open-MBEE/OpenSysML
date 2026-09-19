// A constraint whose condition calls a library function and exponentiates.
package test {
	private import ScalarValues::*;
	private import RealFunctions::*;

	constraint marginHolds {
		sqrt(16.0) * 2.0 >= 2.0 ** 3
	}
}
