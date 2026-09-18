// An expression evaluated against an object reads that object's slots: `car`
// redefines the definition's default, so `mass * 2` is about 1600, not 1500.
package demo {
	private import ScalarValues::*;

	part def Car {
		attribute mass : Real default = 1500.0;
	}

	part car : Car {
		attribute redefines mass = 1600.0;
	}
}
