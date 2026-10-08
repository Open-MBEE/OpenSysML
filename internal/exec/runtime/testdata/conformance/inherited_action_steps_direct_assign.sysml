package test {
	private import ScalarValues::*;

	action def Base {
		attribute c : Integer = 0;
		first start then s;
		action s;
		then assign c := c + 5;
		then done;
	}

	action def Direct :> Base;
}
