package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): on each of two passes
	// both guards of `pick` hold, and the library says only that exactly one
	// branch follows, not which; the passes are independent, so the four
	// linearizations leave three tallies open, and every pass counts once.
	action tally {
		attribute passes : Integer = 0;
		attribute lefts : Integer = 0;
		attribute rights : Integer = 0;

		first start;
		merge again;
		decide pick;
		action left { assign lefts := lefts + 1; assign passes := passes + 1; }
		action right { assign rights := rights + 1; assign passes := passes + 1; }
		done;

		succession first start then again;
		succession first again then pick;
		succession first pick if passes < 2 then left;
		succession first pick if passes < 2 then right;
		succession first pick if passes >= 2 then done;
		succession first left then again;
		succession first right then again;
	}
}
