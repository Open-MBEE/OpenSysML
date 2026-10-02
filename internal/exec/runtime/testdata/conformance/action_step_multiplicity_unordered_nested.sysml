package test {
	private import ScalarValues::*;

	action def U {
		attribute c : Integer = 0;
		action a[2] {
			action x { assign c := c + 1; }
			action y { assign c := c + 1; }
		}
	}
}
