// A calc usage declared in an action's body binds its inputs from the values the
// body has reached, not from what the attribute declares: `k` binds `v` after the
// body assigned 2.0, so `doubled` is 4.0 rather than the 2.0 of `v = 1.0`. The
// usage in the loop binds anew each iteration, summing 2+4+6.
package test {
	private import ScalarValues::*;

	calc def Twice {
		in k : Real;
		out d = k * 2.0;
	}

	action run {
		attribute v : Real = 1.0;
		attribute doubled : Real = 0.0;
		attribute i : Integer = 0;
		attribute acc : Real = 0.0;

		first start;

		action compute {
			assign v := 2.0;
			calc t : Twice { in k = v; }
			assign doubled := t.d;
			while i < 3 {
				assign i := i + 1;
				calc step : Twice { in k = i; }
				assign acc := acc + step.d;
			}
		}

		done;

		succession first start then compute;
		succession first compute then done;
	}
}
