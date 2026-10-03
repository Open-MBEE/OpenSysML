package test {
	private import ScalarValues::*;

	// Two successions out of the ordinary node p, one guard false: only the link to
	// r exists, so p's performance is followed by r alone, beside the fork's other
	// branch w; which of r and w writes x last is open.
	action oneHolds {
		attribute armed : Boolean = false;
		attribute x : Integer = 0;

		first start;
		then fork f;
		action p;
		action q { assign x := 3; }
		action r { assign x := 1; }
		action w { assign x := 2; }
		join j;
		done;

		succession first f then p;
		succession first f then w;
		succession first p if armed then q;
		succession first p then r;
		succession first r then j;
		succession first w then j;
		succession first j then done;
	}
}
