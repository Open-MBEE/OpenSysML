// A model's own calc of a library function's name is what a call runs, whether
// the library package is imported or not.
package test {
	private import ScalarValues::*;
	private import RealFunctions::*;

	calc def sqrt { in x : Real; return : Real = x * 10.0; }
	calc def abs { in x : Real; return : Real = 99.0; }

	part def Probe {
		attribute imported = sqrt(4.0);
		attribute unimported = abs(-2.5);
	}
}
