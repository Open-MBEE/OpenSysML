package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once; a decision
	// after the join reads `x` in its guards, and each branch writes `mode`. Which
	// guard holds is the open order of the writes, so `mode` stands on the same
	// choice points as `x`; `later`, written past the merge from a literal,
	// depends on no order.
	action route {
		attribute x : Integer = 0;
		attribute mode : Integer = 0;
		attribute later : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action low { assign mode := 10; }
		action high { assign mode := 20; }
		merge rejoin;
		action tail { assign later := 7; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then choose;
		succession first low then rejoin;
		succession first high then rejoin;
		succession first rejoin then tail;
		succession first tail then done;

		decide choose;
		if x == 1 then low;
		if x == 2 then high;
	}
}
