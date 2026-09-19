// A transition from a composite state back to itself is external: it exits the
// active substates innermost-first and the composite, runs the effect, then
// re-enters the composite and its regions' initial states.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working parallel {
			entry { assign log := log * 10 + 1; }
			exit { assign log := log * 10 + 2; }

			state left {
				entry; then lstart;
				state lstart;
				state l1 {
					entry { assign log := log * 10 + 3; }
					exit { assign log := log * 10 + 4; }
				}

				succession first lstart then l1;
			}
		}

		succession first start then Working;
		transition first Working when restart do assign log := log * 10 + 9 then Working;
	}
}
