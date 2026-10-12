package test {
	private import ScalarValues::*;

	// A fork with one outgoing succession still replaces the token it receives,
	// beside the outer fork's live branch w; which of r and w writes x last is open.
	action oneBranch {
		attribute x : Integer = 0;

		first start;
		then fork f;
		action p;
		fork g;
		action r { assign x := 1; }
		action w { assign x := 2; }
		join j;
		done;

		succession first f then p;
		succession first f then w;
		succession first p then g;
		succession first g then r;
		succession first r then j;
		succession first w then j;
		succession first j then done;
	}
}
