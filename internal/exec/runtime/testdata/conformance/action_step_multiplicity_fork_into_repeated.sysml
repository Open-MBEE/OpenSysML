package test {
	private import ScalarValues::*;

	// The fork's lone outgoing succession into `a` is bijective, so the ends
	// force the fork to `a`'s count: it performs once per arrival, and `b`,
	// behind the written [*] to [1] succession's barrier, runs once after
	// the last performance.
	action def U {
		attribute c : Integer = 0;
		first start then f;
		fork f;
		succession first f then a;
		action a[3] { assign c := c + 1; }
		succession first [*] a then [1] b;
		action b { assign c := c + 10; }
	}
}
