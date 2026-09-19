package test {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;

	// The first performance of slow naps, the second reaches slow and naps longer; the
	// first wakes and runs `terminate slow;`. Naming its own node ends the ongoing
	// performances earliest first: the first unwinds and completes, then the second is
	// ended in place with its nap dropped, and both tokens take slow's succession.
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
				terminate slow;
				assign late := late + 1;
			} else {
				action inner2 {
					first start;
					then action nap2 accept after 100 [s];
					then done;
				}
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
