package test {
	private import ScalarValues::*;

	action def Base {
		attribute c : Integer = 0;
		first start then a;
		action a { assign c := c + 1; }
		then done;
	}

	action def Narrow :> Base { action :>> a[2]; }
}
