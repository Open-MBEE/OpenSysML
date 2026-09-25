package test {
	private import ScalarValues::*;

	// Each branch of a fork may state a flow of its own; the join waits for both
	// inner flows, not merely for the branch nodes to be entered.
	action par {
		out attribute leftValue : Integer;
		out attribute rightValue : Integer;

		fork split;
		action left {
			action leftStep { assign leftValue := 1; }
			first leftStep;
		}
		action right {
			action rightStep { assign rightValue := 2; }
			first rightStep;
		}
		join sync;
		action tally { assign leftValue := leftValue + rightValue; }
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then tally;
		succession first tally then done;
	}
}
