package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): `both` is one performance per
	// performance of Converge, after that performance's `l1` and `l2`; two performances
	// of Converge running at once each count it once: one.hits = 1, two.hits = 1.
	action def Converge {
		out hits : Integer = 0;

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

	action outer {
		attribute total : Integer = 0;

		first start;
		fork split;
		action one : Converge;
		action two : Converge;
		join sync;
		action tally { assign total := one.hits + two.hits; }
		done;

		succession first start then split;
		succession first split then one;
		succession first split then two;
		succession first one then sync;
		succession first two then sync;
		succession first sync then tally;
		succession first tally then done;
	}
}
