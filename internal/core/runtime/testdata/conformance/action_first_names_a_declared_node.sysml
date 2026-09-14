package test {
	private import ScalarValues::*;

	// The one-ended `first s1;` marks s1 as the node the flow starts at, and the
	// succession out of it is written separately.
	action seq {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		action s1 assign x := 7;
		action s2 assign y := 9;

		first s1;
		succession first s1 then s2;
	}
}
