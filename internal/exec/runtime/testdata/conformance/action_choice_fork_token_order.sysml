package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): the library orders
	// neither branch before the other; each writes its own feature, so the outcome
	// is fixed and only the order the branches were stepped in is the executor's
	// choice, which the trace reports.
	action tally {
		attribute leftCount : Integer = 0;
		attribute rightCount : Integer = 0;

		first start;
		fork split;
		action left { assign leftCount := leftCount + 1; }
		action right { assign rightCount := rightCount + 10; }
		join sync;
		done;

		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}
