// A pin holding an object is chained through like any feature: `pick.target.mark`
// reads a member of the object node `pick` put out at `target`.
package test {
	private import ScalarValues::*;

	part def Cell {
		attribute mark : Integer = 5;
	}

	action def Read {
		in sample : Cell;
		out seen : Integer;

		first pick;
		action pick {
			out target : Cell = sample;
		}
		succession pick then read;
		action read {
			assign seen := pick.target.mark + 1;
		}
	}

	part def Host {
		part cell : Cell;

		perform action reading : Read {
			in sample = cell;
		}

		attribute total : Integer = reading.seen;
	}
}
