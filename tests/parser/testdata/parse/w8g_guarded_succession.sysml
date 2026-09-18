action def GuardedSuccessions {
	attribute x = 1;

	action A1;
	action A2;
	action A3;

	succession S first A1
		if x == 0 then A2;

	succession S2 first A2
		if x > 0 then A3 {
			action inner;
		}
}
