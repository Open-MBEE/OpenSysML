package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	// A step declaring a short and a primary name is one feature of the body iteration
	// that performs it, so each iteration has one subperformance for it.
	action def Run {
		attribute t : Integer = 0;
		out bodyCount : Integer;
		out bodySteps : Integer;

		first start;
		then for i in (1, 2) {
			action <a> add[1] assign t := t + i;
		}
		then assign bodyCount := size(loops.body);
		then assign bodySteps := size(loops.body.subperformances);
		then done;
	}
}
