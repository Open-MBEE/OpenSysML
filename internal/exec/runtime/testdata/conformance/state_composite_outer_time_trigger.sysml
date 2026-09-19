// A time trigger on a composite state counts from entering it and fires while a
// substate is active: the substate is exited first, then the composite.
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working {
			exit { assign log := log * 10 + 2; }

			state Step1 {
				exit { assign log := log * 10 + 1; }
			}

			accept after 5 [s] then Done;
		}
		state Done {
			entry { assign log := log * 10 + 3; }
		}

		succession first start then Step1;
	}
}
