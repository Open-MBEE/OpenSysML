package test {
	private import ScalarValues::*;

	action def B1 {
		attribute c : Integer = 0;
		first start then a;
		action a { assign c := c + 1; }
		then done;
	}

	action def P3 :> B1 {
		action b { assign c := c + 100; }
	}
}
