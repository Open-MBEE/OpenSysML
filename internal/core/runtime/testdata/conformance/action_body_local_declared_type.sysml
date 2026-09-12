// A body-local declaration in an action body answers to its declared type at
// run time (KerML 1.0 §7.3.4: the values of a feature are instances of its
// types): Level's extent is {1, 3}, so holding 2 in a local typed by Level is
// a type mismatch refused where the local is declared, not a value the body
// goes on computing with.
package test {
	private import ScalarValues::*;

	enum def Level :> Integer { low = 1; high = 3; }

	action run {
		attribute n : Integer = 2;
		out r : Integer = 0;

		first start;

		action hold {
			attribute l : Level = n;
			assign r := l + 0;
		}

		done;

		succession first start then hold;
		succession first hold then done;
	}
}
