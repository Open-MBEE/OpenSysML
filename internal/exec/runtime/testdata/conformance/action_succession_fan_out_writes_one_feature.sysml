package test {
	private import ScalarValues::*;

	// The fan-out counterpart of a fork whose branches write one feature: two
	// successions out of the ordinary node split order it before left and right,
	// and nothing orders left against right, so which write of x stands is open.
	action clash {
		attribute x : Integer = 0;
		attribute leftRan : Boolean = false;
		attribute rightRan : Boolean = false;

		first start;
		action split;
		action left { assign x := 1; assign leftRan := true; }
		action right { assign x := 2; assign rightRan := true; }
		join sync;
		done;

		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}
