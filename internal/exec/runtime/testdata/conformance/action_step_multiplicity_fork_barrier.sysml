package test {
	private import ScalarValues::*;

	// The fork declares no multiplicity, and its ends force no other count, so
	// it takes the executor's one-performance reading — an unwritten step
	// performs once per arrival — and fires once after the last performance
	// of `a`, behind the written [*] source end's barrier.
	action def U {
		attribute c : Integer = 0;
		first start then a;
		action a[3] { assign c := c + 1; }
		succession first [*] a then f;
		fork f;
		then x;
		then y;
		action x { assign c := c + 10; }
		action y { assign c := c + 100; }
	}
}
