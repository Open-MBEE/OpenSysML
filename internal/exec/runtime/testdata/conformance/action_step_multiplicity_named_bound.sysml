package test {
	private import ScalarValues::*;

	attribute three : Integer = 3;
	action def NamedBound {
		attribute c : Integer = 0;
		first start then a;
		action a[three] { assign c := c + 1; }
		then done;
	}
}
