package test {
	private import ScalarValues::*;

	action def B1 {
		attribute c : Integer = 0;
		first start then a;
		action a { assign c := c + 1; }
		then done;
	}

	action def P2 :> B1 {
		attribute z : Integer = 0;
	}
}
