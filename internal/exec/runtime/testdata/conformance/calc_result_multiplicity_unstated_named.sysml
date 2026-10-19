// A named result parameter stating no multiplicity holds its value as computed, as an anonymous one does.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return r : Real = x;
	}
}
