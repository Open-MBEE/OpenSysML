package test {
	private import ScalarValues::*;

	// One fork branch names the node running in the other: `terminate slow;` drops
	// the token parked inside slow's own flow, slow completes with what it assigned
	// so far, and the join fires as after a normal completion.
	action host {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;
		out attribute z : Integer = 0;

		fork split;
		action slow {
			first start;
			then action begun { assign x := 1; }
			then action waiter accept sig : Integer;
			then action late { assign y := 2; }
			then done;
		}
		action pre1 { assign z := 3; }
		action pre2;
		action pre3;
		action killer { terminate slow; }
		join sync;
		succession first start then split;
		succession first split then slow;
		succession first split then pre1;
		succession first pre1 then pre2;
		succession first pre2 then pre3;
		succession first pre3 then killer;
		succession first slow then sync;
		succession first killer then sync;
		succession first sync then done;
	}
}
