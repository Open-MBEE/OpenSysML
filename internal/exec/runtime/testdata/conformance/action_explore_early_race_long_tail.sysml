package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): two branches between one
	// fork and one join each open by writing `x`, the last write standing, then go
	// on for two steps writing their own feature in turn; the library orders neither
	// branch, so the two orders of the writes leave two values open while every order
	// of the steps after them ends the same. A run meets the open choice first, closed ones after.
	action race {
		attribute x : Integer = 0;
		attribute p : Integer = 0;
		attribute q : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action p1 { assign p := 1; }
		action p2 { assign p := 2; }
		action b { assign x := 2; }
		action q1 { assign q := 1; }
		action q2 { assign q := 2; }
		join sync;
		done;

		succession first start then split;
		succession first split then a;
		succession first a then p1;
		succession first p1 then p2;
		succession first p2 then sync;
		succession first split then b;
		succession first b then q1;
		succession first q1 then q2;
		succession first q2 then sync;
		succession first sync then done;
	}
}
