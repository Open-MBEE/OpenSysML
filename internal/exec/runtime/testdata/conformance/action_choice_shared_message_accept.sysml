package test {
	private import ScalarValues::*;

	// Two accepts of one type park in parallel branches while a third branch
	// sends one Integer at a time. The first message answers both accepts, so
	// which one takes it is the executor's order: a choice point naming both
	// accepts. The second message finds one accept waiting: no choice.
	action listen {
		attribute a : Integer = 0;
		attribute b : Integer = 0;

		first start;

		fork split;

		action sendOne send 1;

		action sendTwo send 2;

		action left accept x : Integer;

		action right accept y : Integer;

		join sync;

		action recorder {
			assign a := x;
			assign b := y;
		}

		done;

		succession first start then split;
		succession first split then sendOne;
		succession first split then left;
		succession first split then right;
		succession first sendOne then sendTwo;
		succession first sendTwo then sync;
		succession first left then sync;
		succession first right then sync;
		succession first sync then recorder;
		succession first recorder then done;
	}
}
