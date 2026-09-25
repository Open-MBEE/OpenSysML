// A composite state that self-transitions inside an orthogonal region is exited
// and re-entered like any other external transition, and its sibling region is
// left untouched.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Outer parallel {
			state left {
				entry; then lstart;
				state lstart;
				state Grouping parallel {
					entry { assign log := log * 10 + 1; }
					exit { assign log := log * 10 + 2; }

					state inner {
						entry; then gstart;
						state gstart;
						state g1 {
							entry { assign log := log * 10 + 3; }
							exit { assign log := log * 10 + 4; }
						}

						succession first gstart then g1;
					}
				}

				succession first lstart then Grouping;
				transition first Grouping when restart do assign log := log * 10 + 9 then Grouping;
			}
			state right {
				entry; then rstart;
				state rstart;
				state Watching {
					entry { assign log := log * 10 + 7; }
				}

				succession first rstart then Watching;
			}
		}

		succession first start then Outer;
	}
}
