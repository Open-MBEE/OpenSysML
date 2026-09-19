package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): `both` is one
	// performance that follows both `l1` and `l2`, so hits = 1.
	action converge {
		attribute hits : Integer = 0;

		first start;
		fork split;
		action l1;
		action l2;
		action both { assign hits := hits + 1; }
		done;

		succession first start then split;
		succession first split then l1;
		succession first split then l2;
		succession first l1 then both;
		succession first l2 then both;
		succession first both then done;
	}
}
