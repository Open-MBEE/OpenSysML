// Two timers in orthogonal regions are ordered by the duration their units
// measure, not by the number written: 30 [s] is due before 1 [min].
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working parallel {
			state slow {
				entry; then sstart;
				state sstart;
				state sWaiting;
				accept after 1 [min] then sDone;
				state sDone {
					entry { assign log := log * 10 + 2; }
				}

				transition first sstart then sWaiting;
			}

			state fast {
				entry; then fstart;
				state fstart;
				state fWaiting;
				accept after 30 [s] then fDone;
				state fDone {
					entry { assign log := log * 10 + 1; }
				}

				transition first fstart then fWaiting;
			}
		}

		succession first start then Working;
	}
}
