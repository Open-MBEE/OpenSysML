package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): `sync` follows one performance
	// of each of `a`, `b` and `c3`; the two that arrive in one step do not fire it, so
	// `after` reads arrived = 3.
	action three {
		attribute arrived : Integer = 0;
		attribute seen : Integer = 0;

		first start;
		fork split;
		action a { assign arrived := arrived + 1; }
		action b { assign arrived := arrived + 1; }
		action c1;
		action c2;
		action c3 { assign arrived := arrived + 1; }
		join sync;
		action after { assign seen := arrived; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c1;
		succession first c1 then c2;
		succession first c2 then c3;
		succession first a then sync;
		succession first b then sync;
		succession first c3 then sync;
		succession first sync then after;
		succession first after then done;
	}
}
