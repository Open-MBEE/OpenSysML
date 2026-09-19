package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): `sync` follows one
	// performance of `left` and one of `right`, so `after` logs last: log = 12.
	action provenance {
		attribute log : Integer = 0;

		first start;
		fork split;
		action l1;
		action l2;
		action left;
		action r1;
		action r2;
		action r3;
		action right { assign log := log * 10 + 1; }
		join sync;
		action after { assign log := log * 10 + 2; }
		done;

		succession first start then split;
		succession first split then l1;
		succession first split then l2;
		succession first split then r1;
		succession first l1 then left;
		succession first l2 then left;
		succession first r1 then r2;
		succession first r2 then r3;
		succession first r3 then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then after;
		succession first after then done;
	}
}
