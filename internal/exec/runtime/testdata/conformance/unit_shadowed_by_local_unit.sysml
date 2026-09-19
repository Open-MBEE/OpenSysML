// A sibling named like an imported unit that is itself a measurement unit keeps
// working: the nearest declaration conforms to what the position expects.
package test {
	public import SI::*;

	constraint def Tall {
		attribute m : ISQBase::LengthUnit;
		1.0 [m] > 500.0 [m]
	}
}
