// A time trigger on a composite state inside an orthogonal region is
// region-local: the deeper substate and the composite are exited, and the
// sibling region keeps its own active state.
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working parallel {
			state left {
				entry; then lstart;
				state lstart;
				state Grouping {
					exit { assign log := log * 10 + 2; }

					state Step1 {
						exit { assign log := log * 10 + 1; }
					}

					accept after 5 [s] then Recovered;
				}
				state Recovered {
					entry { assign log := log * 10 + 3; }
				}

				transition first lstart then Step1;
			}

			state right {
				entry; then rstart;
				state rstart;
				state Watching;

				succession first rstart then Watching;
			}
		}

		succession first start then Working;
	}
}
