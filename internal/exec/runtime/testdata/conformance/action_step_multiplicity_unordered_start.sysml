package test {
	private import ScalarValues::*;

	action def U {
		attribute c : Integer = 0;
		action a[3] { assign c := c + 1; }
	}
}
