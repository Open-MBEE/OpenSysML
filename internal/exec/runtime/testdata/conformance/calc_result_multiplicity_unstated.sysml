// A result parameter stating no multiplicity holds its value as computed: one value stays bare, the rule for a body local whose multiplicity is omitted.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return : Real = x;
	}
}
