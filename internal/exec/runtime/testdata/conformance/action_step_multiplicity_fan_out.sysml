package test {
	private import ScalarValues::*;

	// Two successions out of a repeated step, each with written end multiplicities:
	// all three performances of a precede the one of q and the one of r.
	action def FanOut {
		attribute sum : Integer = 0;
		attribute seenByQ : Integer = 0;
		attribute seenByR : Integer = 0;
		action a[3] { assign sum := sum + 1; }
		action q { assign seenByQ := sum; }
		action r { assign seenByR := sum; }
		succession first start then a;
		succession first [*] a then [1] q;
		succession first [*] a then [1] r;
	}
}
