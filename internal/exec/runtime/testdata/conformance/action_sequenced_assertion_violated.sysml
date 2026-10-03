package test {
	private import ScalarValues::*;
	action check {
		attribute level : Integer = 0;
		action raise { assign level := 3; }
		action lower { assign level := 1; }
		assert constraint raised { level == 3 }
		first start then raise;
		first raise then lower;
		first lower then raised;
		first raised then done;
	}
}
