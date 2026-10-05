// An action usage with no declared type but an invocation value is typed by
// the invoked definition (KerML checkFeatureValuationSpecialization), so
// `run.r` reads Base's output and the invocation's arguments bind its inputs.
package test {
	private import ScalarValues::*;

	action def Base {
		in x : Integer default 3;
		in z : Integer default 7;
		out r : Integer;

		first start;
		then assign r := z;
		then done;
	}

	action caller {
		out attribute result : Integer;

		first start;
		then action run = Base(x = 3, z = 7);
		then action keep { assign result := run.r; }
		then done;
	}
}
