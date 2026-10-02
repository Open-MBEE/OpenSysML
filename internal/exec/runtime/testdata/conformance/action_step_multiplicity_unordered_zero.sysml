package test {
	private import ScalarValues::*;

	action def U {
		attribute c : Integer = 0;
		action z[0] { assign c := c + 1; }
	}
}
