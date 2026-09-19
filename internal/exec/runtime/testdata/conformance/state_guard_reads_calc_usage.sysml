// A transition guard reading a calc usage reads it over the values that step
// sees: each visit to "step" evaluates the usage again over the counter its entry
// actions have reached, so the machine leaves once the doubled counter reaches 4
// rather than looping on the first evaluation's answer.
package test {
	private import ScalarValues::*;

	calc def Twice {
		in k : Real;
		out d = k * 2.0;
	}

	state machine {
		attribute i : Integer = 0;
		calc t : Twice { in k = i; }

		entry; then start;
		state start;
		state step {
			entry { assign i := i + 1; }
		}
		state again {
			entry { assign i := i + 1; }
		}

		succession first start then step;

		transition first step if t.d < 4.0 then again;
		transition first step if t.d >= 4.0 then done;
		transition first again then step;
	}
}
