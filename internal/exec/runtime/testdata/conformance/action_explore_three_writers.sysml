package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): three branches each
	// write `x` once between one fork and one join; the library orders none of
	// them against another, so the six orders of the three writes leave three
	// values open — the last write stands — and each branch runs exactly once.
	action race {
		attribute x : Integer = 0;
		attribute aRan : Boolean = false;
		attribute bRan : Boolean = false;
		attribute cRan : Boolean = false;

		first start;
		fork split;
		action a { assign x := 1; assign aRan := true; }
		action b { assign x := 2; assign bRan := true; }
		action c { assign x := 3; assign cRan := true; }
		join sync;
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}
}
