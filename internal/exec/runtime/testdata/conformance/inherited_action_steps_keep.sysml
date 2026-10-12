package test {
	private import ScalarValues::*;

	action def Base {
		attribute c : Integer = 0;
		first start then a;
		action a[3] { assign c := c + 1; }
		then done;
	}

	action def Keep :> Base { action :>> a { assign c := c + 10; } }
}
