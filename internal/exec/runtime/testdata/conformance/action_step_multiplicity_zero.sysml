package test {
	private import ScalarValues::*;

	action def Zero {
		attribute c : Integer = 0;
		first start then a;
		action a[0];
		then q;
		action q { assign c := 7; }
		then done;
	}
}
