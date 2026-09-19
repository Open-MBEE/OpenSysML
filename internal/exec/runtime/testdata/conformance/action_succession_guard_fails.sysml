package test {
	// The same guarded succession with a guard that does not hold: its link is
	// `HappensBefore[0..1]`, so no link occurs, s2 does not run and the flow ends
	// at s1. That the flow ends rather than failing is tool-defined.
	action seq {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		action s1 assign x := 7;
		action s2 assign y := 9;

		first s1 if x > 100 then s2;
	}
}
