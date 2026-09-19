package Machines {
	attribute def Signal;

	part def Vehicle {
		attribute temperature;
	}

	state def VehicleStates {
		attribute temperature;

		entry; then off;

		state off;
		state operating {
			entry; then idle;

			state idle;
			state moving;

			transition idle_to_moving
				first idle
				accept Signal
				if temperature > 0
				then moving;

			transition first moving then idle;
		}

		transition first off then operating;
		transition first operating then off;
	}
}

package MachineViews {
	private import StandardViewDefinitions::*;

	// The rendering kind comes from the standard view definition, with no
	// rendering member of its own.
	view vehicleStates : StateTransitionView {
		expose Machines::VehicleStates;
	}
}
