package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once; a decision
	// after the join reads `x`, and the branch it takes on 1 forks into two actions
	// writing `y` and `z` that join again before the merge; the other branch writes
	// both at once. The fork's tokens and the join's token carry the decision, so
	// `y` and `z` stand on the same choice points as `x`; `later`, written past the merge from a literal, depends on no order.
	action route {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		attribute z : Integer = 0;
		attribute later : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		fork spread;
		action p { assign y := 10; }
		action q { assign z := 5; }
		join gather;
		action high { assign y := 1; assign z := 1; }
		merge rejoin;
		action tail { assign later := 7; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then choose;
		succession first spread then p;
		succession first spread then q;
		succession first p then gather;
		succession first q then gather;
		succession first gather then rejoin;
		succession first high then rejoin;
		succession first rejoin then tail;
		succession first tail then done;

		decide choose;
		if x == 1 then spread;
		if x == 2 then high;
	}
}
