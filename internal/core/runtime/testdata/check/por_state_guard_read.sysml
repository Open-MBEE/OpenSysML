// An action's write to a feature a transition guard reads: which branch of the
// choice the machine takes depends on whether the writer went first, so the two
// are dependent and both routes are reached.
package test {
	private import ScalarValues::*;

	part def Dial {
		attribute level : Integer = 0;
	}

	part dial : Dial;

	action turner {
		attribute other : Integer = 0;
		first start;
		fork split;
		action turn { assign dial.level := 9; }
		action busy { assign other := 1; }
		join sync;
		done;
		succession first start then split;
		succession first split then turn;
		succession first split then busy;
		succession first turn then sync;
		succession first busy then sync;
		succession first sync then done;
	}

	state router {
		attribute route : Integer = 0;
		entry; then idle;
		state idle;
		choice pick;
		state low { entry { assign route := 1; } }
		state high { entry { assign route := 2; } }
		succession first idle then pick;
		transition first pick if dial.level < 5 then low;
		transition first pick if dial.level >= 5 then high;
	}
}
