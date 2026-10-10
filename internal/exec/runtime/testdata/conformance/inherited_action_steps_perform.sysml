package test {
	private import ScalarValues::*;

	action def P0 {
		out attribute c : Integer = 0;
		assign c := c + 1;
	}

	action def W { perform action p : P0; }
}
