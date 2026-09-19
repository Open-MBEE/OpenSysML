// A feature a node declares shadows a sibling node of the same name: in `read`,
// `pick` is the node's own object-valued input, so `pick.mark` reads that object's
// member, not a pin of the sibling node `pick`.
package test {
	private import ScalarValues::*;

	part def Cell {
		attribute mark : Integer = 5;
	}

	action def Read {
		in sample : Cell;
		out seen : Integer;
		out picked : Integer;

		first pick;
		action pick {
			out mark : Integer = 100;
		}
		succession pick then read;
		action read {
			in pick : Cell = sample;
			assign seen := pick.mark;
		}
		succession read then tally;
		action tally {
			assign picked := pick.mark;
		}
	}

	part def Host {
		part cell : Cell;

		perform action reading : Read {
			in sample = cell;
		}

		attribute total : Integer = reading.seen;
		attribute fromNode : Integer = reading.picked;
	}
}
