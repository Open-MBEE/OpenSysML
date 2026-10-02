package test {
	private import ScalarValues::*;

	// A repeated step ends behind one barrier at a decision too: `d` decides
	// once after the last performance of `a`, where the guard already holds.
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
