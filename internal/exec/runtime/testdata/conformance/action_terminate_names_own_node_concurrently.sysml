package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	// Two tokens reach slow through the merge one step apart, so the first performance
	// of slow is paused in its nap when the second runs `terminate slow;`. Naming its
	// own node, the statement ends every ongoing performance of it: the paused one is
	// dropped with its nap, its own unwinds, and both tokens take slow's succession.
	action host {
		out attribute n : Integer = 0;
		out attribute y : Integer = 0;
		out attribute late : Integer = 0;

		fork split;
		action pre;
		merge gate;
		action slow {
			assign n := n + 1;
			if n == 1 {
				action inner {
					first start;
					then action nap accept after 10 [s];
					then done;
				}
				assign late := late + 1;
			} else {
				terminate slow;
				assign late := late + 10;
			}
		}
		action after { assign y := y + 1; }
		succession first start then split;
		succession first split then gate;
		succession first split then pre;
		succession first pre then gate;
		succession first gate then slow;
		succession first slow then after;
		succession first after then done;
	}
}
