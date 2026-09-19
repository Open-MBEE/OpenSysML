// A nested state's entry action writes the root state performance occurrence.
package test {
	private import ScalarValues::*;

	state def Modes {
		attribute distance : Integer = 0;

		entry; then driving;
		state driving {
			entry; then rolling;
			state rolling {
				entry action move {
					assign distance := distance + 21;
				}
			}
		}
	}

	part def Rover {
		exhibit state modes : Modes;
	}
}
