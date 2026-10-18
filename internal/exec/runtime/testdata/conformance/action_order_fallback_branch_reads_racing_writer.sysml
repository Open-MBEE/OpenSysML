package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once; a decision
	// after the join reads `x` in its one guard and otherwise takes the unguarded
	// branch. Whether the guard fails is the open order of the writes, so `mode`
	// stands on the same choice points as `x` on the unguarded branch too.
	action route {
		attribute x : Integer = 0;
		attribute mode : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action enabled { assign mode := 10; }
		action other { assign mode := 20; }
		merge rejoin;
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then choose;
		succession first enabled then rejoin;
		succession first other then rejoin;
		succession first rejoin then done;

		decide choose;
		if x == 1 then enabled;
		else other;
	}
}
