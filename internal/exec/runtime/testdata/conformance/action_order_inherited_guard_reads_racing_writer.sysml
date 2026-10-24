package test {
	private import ScalarValues::*;

	// The run is the specific's, whose decision, guards and racing writers are all
	// inherited from the general. The inherited guard is read in the general's scope,
	// where it was declared, so what the taken branch writes stands on the order of
	// the writes of `go`; `later`, written past the merge from a literal, depends on
	// no order.
	action def Derived :> Base;

	action def Base {
		attribute go : Boolean = false;
		attribute mode : Integer = 0;
		attribute later : Integer = 0;

		first start;
		fork split;
		action a { assign go := true; }
		action b { assign go := false; }
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
		if go then high;
		else low;
	}
}
