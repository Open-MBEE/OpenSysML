package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): `converge` is reached over two
	// successions, but the decision takes one branch, so the succession from `low` orders
	// nothing before it — `converge` performs once for the one arrival, no deadlock.
	action choose {
		attribute mode : Integer = 2;
		attribute took : Integer = 0;
		attribute hits : Integer = 0;

		decide pick;
		action low { assign took := 1; }
		action high { assign took := 2; }
		action converge { assign hits := hits + 1; }

		succession first start then pick;
		succession first pick if mode < 2 then low;
		succession first pick if mode >= 2 then high;
		succession first low then converge;
		succession first high then converge;
		succession first converge then done;
	}
}
