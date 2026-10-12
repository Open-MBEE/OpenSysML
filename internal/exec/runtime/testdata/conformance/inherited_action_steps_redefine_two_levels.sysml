package test {
	private import ScalarValues::*;

	action def Base {
		attribute c : Integer = 0;
		first start then a;
		action a[2] { assign c := c + 1; }
		then done;
	}

	action def Mid :> Base { action :>> a[2]; }
	action def Leaf :> Mid { action :>> a { assign c := c + 10; } }
}
