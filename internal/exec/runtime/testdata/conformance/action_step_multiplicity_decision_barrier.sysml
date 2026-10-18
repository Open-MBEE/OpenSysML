package test {
	private import ScalarValues::*;

	// The decision declares no multiplicity, and its ends force no other
	// count, so it takes the executor's one-performance reading — an
	// unwritten step performs once per arrival — and decides once after the
	// last performance of `a`, where the guard already holds.
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
