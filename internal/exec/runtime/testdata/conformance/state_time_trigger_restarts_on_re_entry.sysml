// A state left before its timer expires withdraws the time event it queued, so
// re-entering it times the whole interval again: the sibling region's clock,
// which the exit does not touch, reaches c1 first.
package Test {
	private import SI::*;

	state Machine parallel {
		attribute log : Integer = 0;

		state work {
			entry; then wstart;
			state wstart;
			state waiting;
			accept after 10 [s] then late;
			state away;
			accept after 4 [s] then waiting;
			state late {
				entry { assign log := log * 10 + 1; }
			}

			transition first wstart then waiting;
			transition first waiting accept pause then away;
		}
		state clock {
			entry; then cstart;
			state cstart;
			state c0;
			accept after 12 [s] then c1;
			state c1 {
				entry { assign log := log * 10 + 2; }
			}

			transition first cstart then c0;
		}
	}
}
