package test {
	private import ScalarValues::*;

	// A merge performs on every arrival (Actions::MergeAction is one MergePerformance
	// per incoming link), so its body runs for each of the two tokens; the guard is a
	// DecisionTransitionAction whose TPCGuardConstraint only prunes the outgoing link,
	// so the first arrival is retired after its body ran and the second passes.
	action orJoin {
		attribute ready : Boolean = false;
		attribute mergeRuns : Integer = 0;
		attribute passed : Integer = 0;

		first start;
		fork split;
		action slow;
		action slower { assign ready := true; }
		merge gate { assign mergeRuns := mergeRuns + 1; }
		action tail { assign passed := mergeRuns; }
		done;

		succession first start then split;
		succession first split then gate;
		succession first split then slow;
		succession first slow then slower;
		succession first slower then gate;
		succession first gate if ready then tail;
		succession first tail then done;
	}
}
