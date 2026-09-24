// An exhibited machine whose nested state is a typed usage: the inherited
// content runs, and the machine's attribute is still a feature of the state
// performance occurrence.
package test {
	private import ScalarValues::*;

	state def Inner {
		entry; then i1;
		state i1;
	}

	state def Modes {
		attribute distance : Integer = 0;

		entry; then driving;
		state driving {
			entry action move {
				assign distance := distance + 21;
			}
			entry; then nested;
			state nested : Inner;
		}
	}

	part def Rover {
		exhibit state modes : Modes;
	}
}
