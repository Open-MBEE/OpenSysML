// A part performing an action definition: the flow the debugger runs belongs to
// the definition typing the perform usage, not to the usage itself.
package Perform {
	private import ScalarValues::*;

	action def Trek {
		out attribute legs : Integer;

		first leg;

		action leg {
			action step { assign legs := 3; }
			first step;
		}

		succession first leg then done;
		done;
	}

	part def Hiker {
		perform action walk : Trek;
	}

	part hiker : Hiker;
}
