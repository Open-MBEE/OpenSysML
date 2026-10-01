// An entry behavior's output returns to the same-named attribute of the machine
// once it completes, a write after the machine's occurrence is initialized, so
// returning into a constant attribute (KerML 1.0 §8.3.3.3) is refused.
package test {
	private import ScalarValues::*;

	action def Raise {
		out limit : Integer;

		first start;
		then action raising { assign limit := 9; }
		then done;
	}

	state def Counter {
		constant attribute limit : Integer = 5;

		entry; then idle;

		state idle {
			entry action raised : Raise;
		}
	}
}
