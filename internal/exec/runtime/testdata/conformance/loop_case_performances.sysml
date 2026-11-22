package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	// A case is an action: the loops its body reaches are performances of the case.
	analysis def Counted {
		subject s : Integer = 0;
		attribute n : Integer = 0;
		loop action body {
			action inc assign n := n + 1;
		} until n >= 3;
		return r : Integer = size(loops) * 100 + size(loops.body) * 10 + size(loops.body.subperformances);
	}
	analysis def Nested {
		subject s : Integer = 0;
		attribute n : Integer = 0;
		attribute t : Integer = 0;
		while n < 2 {
			assign n := n + 1;
			for i in (1, 2) {
				action add assign t := t + i;
			}
		}
		return r : Integer = size(loops) * 100 + size(loops.body.loops) * 10 + size(loops.body.loops.body.subperformances);
	}
	part def K {
		analysis counted : Counted;
		analysis nested : Nested;
		attribute countedShape : Integer = counted.r;
		attribute nestedShape : Integer = nested.r;
	}
	part k : K;
}
