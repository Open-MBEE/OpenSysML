package test {
	// A guard that does not evaluate to a Boolean is reported rather than treated
	// as either verdict: `bool guard[*]` admits no other value. The guard names an
	// untyped feature, so only the executor can see the value it holds.
	action seq {
		attribute x;
		attribute y = 0;

		action s0 assign x := 7;
		action s1 assign y := 9;
		action s2 assign y := 1;

		first s0 then s1;
		succession first s1 if x then s2;
	}
}
