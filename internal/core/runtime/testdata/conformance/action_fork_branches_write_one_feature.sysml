package test {
	import ScalarValues::*;
	// Oracle (docs/project/behavior-semantic-oracle.md): both flags end true;
	// the library leaves the final `x` open, so its value only pins the runtime's order.
	action clash {
		attribute x : Integer = 0;
		attribute leftRan : Boolean = false;
		attribute rightRan : Boolean = false;

		first start;
		fork split;
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
