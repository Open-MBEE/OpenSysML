// A transition out of a composite state into one of its own substates is also
// external: the composite is exited and re-entered around the effect.
package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working {
			entry { assign log := log * 10 + 1; }
			exit { assign log := log * 10 + 2; }

			state Step1 {
				entry { assign log := log * 10 + 3; }
				exit { assign log := log * 10 + 4; }
			}

			state Step2 {
				entry { assign log := log * 10 + 5; }
			}
		}

		succession first start then Working::Step1;
		transition first Working when advance do assign log := log * 10 + 9 then Working::Step2;
	}
}
