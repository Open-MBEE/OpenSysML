package test {
	private import ScalarValues::*;

	action def P0 {
		attribute c : Integer = 0;
		first start then a;
		action a { assign c := c + 1; }
		then done;
	}

	action def W { action x : P0; }
}
