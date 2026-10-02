package test {
	private import ScalarValues::*;

	action def Base {
		attribute c : Integer = 0;
		first start then a;
		action a { assign c := c + 1; }
		then done;
	}

	action def Left :> Base { action l { assign c := c + 10; } }
	action def Right :> Base { action r { assign c := c + 100; } }
	action def Diamond :> Left, Right;
}
