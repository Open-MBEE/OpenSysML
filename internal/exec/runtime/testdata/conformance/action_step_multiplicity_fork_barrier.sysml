package test {
	private import ScalarValues::*;

	// The written [*] source end admits every performance of `a` behind one
	// barrier: `f` fires once after the last, so each branch runs once.
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
