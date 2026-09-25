// Without a subject the same expression is evaluated lexically in the named
// scope, so it reads the declared default rather than any object's slot.
package demo {
	private import ScalarValues::*;

	part def Car {
		attribute mass : Real default = 1500.0;
	}

	part car : Car {
		attribute redefines mass = 1600.0;
	}
}
