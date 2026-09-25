// Leaving a composite state whose region holds another composite with its own
// region exits each level exactly once, innermost first.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Outer parallel {
			exit { assign log := log * 10 + 1; }

			state left {
				entry; then lstart;
				state lstart;
				state Grouping parallel {
					exit { assign log := log * 10 + 2; }

					state inner {
						entry; then gstart;
						state gstart;
						state g1 {
							exit { assign log := log * 10 + 3; }
						}

						succession first gstart then g1;
					}
				}

				succession first lstart then Grouping;
			}
			state right {
				entry; then rstart;
				state rstart;
				state Watching {
					exit { assign log := log * 10 + 4; }
				}

				succession first rstart then Watching;
			}
		}
		state Done;

		succession first start then Outer;
		transition first Outer when stop do assign log := log * 10 + 9 then Done;
	}
}
