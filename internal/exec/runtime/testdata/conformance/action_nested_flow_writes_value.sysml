package test {
	private import ScalarValues::*;

	// A node stating a flow of its own runs it to completion before its outgoing
	// succession fires, so a later node reads what it wrote.
	action trek {
		out attribute legs : Integer;
		out attribute total : Integer;

		first leg;

		action leg {
			action step { assign legs := 3; }
			first step;
		}

		action tally { assign total := legs + 1; }

		succession first leg then tally;
		succession first tally then done;
		done;
	}
}
