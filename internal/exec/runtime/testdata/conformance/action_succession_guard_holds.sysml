package test {
	// A guard on a succession leaving an ordinary action node (SysML.xtext
	// `GuardedSuccession`): the guard is evaluated when the token leaves s1, so it
	// reads what s1 wrote — false at 3, true at 7 — and the token passes to s2.
	action seq {
		attribute x : Integer = 3;
		attribute y : Integer = 0;

		action s1 assign x := x + 4;
		action s2 assign y := 9;

		first s1 if x > 5 then s2;
	}
}
