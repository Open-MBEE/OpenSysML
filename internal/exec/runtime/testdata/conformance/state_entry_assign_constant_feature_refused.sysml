// An entry behavior's assignment is a write by the state's performance after
// the machine's occurrence is initialized, so assigning a constant feature of
// the machine there (KerML 1.0 §8.3.3.3 Feature::isConstant) is refused.
package test {
	private import ScalarValues::*;

	state def Counter {
		constant attribute limit : Integer = 5;

		entry; then idle;

		state idle {
			entry assign limit := 9;
		}
	}
}
