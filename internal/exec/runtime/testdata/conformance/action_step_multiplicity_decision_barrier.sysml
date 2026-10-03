package test {
	private import ScalarValues::*;

	// A decision declares no multiplicity of its own, and a succession into it
	// does not fix its count: `a`'s three performances cannot order with
	// respect to `d`, so the flow is refused rather than run behind one barrier.
	action def U {
		attribute c : Integer = 0;
		first start then a;
		action a[3] { assign c := c + 1; }
		succession first [*] a then d;
		decide d;
		if c >= 3 then x;
		else y;
		action x { assign c := c + 10; }
		action y { assign c := c + 100; }
	}
}
