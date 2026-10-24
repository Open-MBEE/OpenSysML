// A named result parameter stating no multiplicity is held in its effective `[0..*]`, as an anonymous one is.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return r : Real = x;
	}
}
