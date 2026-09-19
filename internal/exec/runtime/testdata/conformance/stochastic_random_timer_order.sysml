// Two timers in orthogonal regions, one fixed at 30 [s] and one drawn from
// uniform(1, 80) [s] when its state is entered: the drawn duration orders
// them, and a fixed model seed fixes the draw.
package test {
	private import SI::*;
	private import RandomFunctions::*;

	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working parallel {
			state random {
				entry; then rstart;
				state rstart;
				state rWaiting;
				accept after uniform(1, 80) [s] then rDone;
				state rDone {
					entry { assign log := log * 10 + 2; }
				}

				transition first rstart then rWaiting;
			}

			state fixed {
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
