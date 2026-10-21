// A result parameter declaring [0..*] bound by the body's trailing expression holds the value as a one-element sequence, as a declared result binding does.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return : Real[0..*];
		attribute y : Real = x + 1.0;
		y
	}
}
