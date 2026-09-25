package Machines {
	private import ScalarValues::*;

	// Each body chooses where it starts by the guards of its entry transitions,
	// tried in declaration order: the machine's own body, a composite state's
	// body, and a region of a parallel state.
	state def Thermostat {
		attribute cold : Boolean = true;
		attribute wet : Boolean = false;
		attribute dark : Boolean = false;

		entry; if cold then heating; if not cold then idle;

		state heating;
		state idle {
			entry; if wet then drying; then resting;

			state drying;
			state resting;
		}
		state monitoring parallel {
			state lights {
				entry; if dark then lit; if not dark then unlit;

				state lit;
				state unlit;
			}
			state fans {
				entry; then off;

				state off;
				state on;

				transition first off then on;
			}
		}

		transition first heating then idle;
		transition first idle then monitoring;
	}
}

package MachineViews {
	private import StandardViewDefinitions::*;

	view thermostat : StateTransitionView {
		expose Machines::Thermostat;
	}
}
