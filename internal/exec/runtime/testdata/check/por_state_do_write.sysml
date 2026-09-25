// A state's do behavior writes a feature an action reads while the state is
// active: each do step is a move dependent on the read, and each reading is reached.
package test {
	private import ScalarValues::*;

	part def Meter {
		attribute count : Integer = 0;
	}

	part meter : Meter;

	action reader {
		attribute saw : Integer = -1;
		attribute other : Integer = 0;
		first start;
		fork split;
		action look { assign saw := meter.count; }
		action busy { assign other := 1; }
		join sync;
		done;
		succession first start then split;
		succession first split then look;
		succession first split then busy;
		succession first look then sync;
		succession first busy then sync;
		succession first sync then done;
	}

	state counter {
		entry; then counting;
		state counting {
			do {
				assign meter.count := 1;
				assign meter.count := 2;
			}
		}
		state rested;
		succession first counting then rested;
	}
}
