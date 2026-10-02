package test {
	private import ScalarValues::*;

	action def U {
		attribute c : Integer = 0;
		action a[3] { assign c := c + 1; }
		first start then b;
		action b { assign c := c + 10; } then done;
	}
}
