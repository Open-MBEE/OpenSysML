// A result parameter declaring [2] holds its two values as a sequence, counted against the declared multiplicity.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return : Real[2] = (x, x + 1.0);
	}
}
