package test {
	private import ScalarValues::*;

	// `first s1 then s2;` is the succession s1 -> s2 and marks no start. The body
	// names no start, so the flow begins at s1, the one node nothing leads to.
	action seq {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		action s1 assign x := 7;
		action s2 assign y := 9;

		first s1 then s2;
	}
}
