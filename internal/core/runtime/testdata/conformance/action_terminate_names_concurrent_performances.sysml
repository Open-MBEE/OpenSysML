package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	// Two tokens reach slow through the merge one step apart, so two performances of
	// slow are paused at once, each holding its own nap. `terminate slow;` names the
	// node, so it ends every ongoing performance of it, the earlier begun first: both
	// naps are dropped, neither assignment runs, and both tokens take slow's succession.
	action host {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;

		fork split;
		action pre;
		merge gate;
		action slow {
			if x == 0 {
				action inner {
					first start;
					then action nap accept after 10 [s];
					then done;
				}
				assign x := x + 1;
			}
		}
		action pre1;
		action pre2;
		action pre3;
		action killer { terminate slow; assign y := 1; }
		succession first start then split;
		succession first split then gate;
		succession first split then pre;
		succession first pre then gate;
		succession first gate then slow;
		succession first slow then done;
		succession first split then pre1;
		succession first pre1 then pre2;
		succession first pre2 then pre3;
		succession first pre3 then killer;
		succession first killer then done;
	}
}
