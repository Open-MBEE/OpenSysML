// An action written on its own does not name the features of the object
// performing it: its body resolves in its own namespace. The object passes what
// the action reads through the parameters the binding binds, and reads what the
// action computed back from the performance.
package test {
	private import ScalarValues::*;

	action def Mark {
		in start : Integer;
		out counted : Integer;

		action step {
			assign counted := start + 1;
		}
		first step;
	}

	part def Host {
		attribute seen : Integer = 4;

		perform action marking : Mark {
			in start = seen;
		}

		attribute total : Integer = marking.counted;
	}
}
