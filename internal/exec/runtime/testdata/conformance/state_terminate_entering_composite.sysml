// A transition from outside a composite state into a terminate action the
// composite declares leaves the source, performs its effect, enters the
// composite on its way (its entry behavior runs), then ends the machine there:
// the composite's own entry transition never fires and nothing is exited.
package Test {
	private import ScalarValues::*;

	attribute def Start;

	state def Machine {
		attribute log : String = "";

		entry; then wait;
		state wait {
			exit { assign log := log + "wait(exit);"; }
		}

		state s1 {
			entry { assign log := log + "s1(entry);"; }
			exit { assign log := log + "s1(exit);"; }

			entry; then inner;
			state inner {
				entry { assign log := log + "inner(entry);"; }
			}
			action stop terminate;
		}

		transition first wait accept Start do { assign log := log + "start(effect);"; } then s1.stop;
	}
}
