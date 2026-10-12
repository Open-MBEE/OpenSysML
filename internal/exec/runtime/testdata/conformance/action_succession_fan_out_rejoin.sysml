package test {
	private import ScalarValues::*;

	// Two successions out of an ordinary action node are two HappensBefore links
	// (Occurrences.kerml): each orders a before its target and neither selects or
	// excludes the other, so b and c are both performed, unordered relative to each
	// other. d, reached over two successions and declaring no multiplicity, is one
	// performance after both.
	action fanOut {
		attribute bRan : Boolean = false;
		attribute cRan : Boolean = false;
		attribute hits : Integer = 0;

		first start;
		then action a;
		action b { assign bRan := true; }
		action c { assign cRan := true; }
		action d { assign hits := hits + 1; }

		succession first a then b;
		succession first a then c;
		succession first b then d;
		succession first c then d;
		succession first d then done;
	}
}
