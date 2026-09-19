// A transition out of an orthogonal composite state exits every active region,
// each innermost-first and in declaration order, before the composite's own exit
// behavior, the transition effect and the target's entry.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working parallel {
			exit { assign log := log * 10 + 3; }

			state left {
				entry; then lstart;
				state lstart;
				state lwork {
					exit { assign log := log * 10 + 1; }
				}

				succession first lstart then lwork;
			}

			state right {
				entry; then rstart;
				state rstart;
				state rwork {
					exit { assign log := log * 10 + 2; }
				}

				succession first rstart then rwork;
			}
		}
		state Done {
			entry { assign log := log * 10 + 5; }
		}

		succession first start then Working;
		transition first Working when Abort do assign log := log * 10 + 4 then Done;
	}
}
