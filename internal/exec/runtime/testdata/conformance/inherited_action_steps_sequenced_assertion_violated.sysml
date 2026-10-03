package test {
	private import ScalarValues::*;
	action def G {
		attribute level : Integer = 0;
		action raise { assign level := 3; }
		assert constraint raised { level == 4 }
		action lower { assign level := 1; }
		first start then raise;
		first raise then raised;
		first raised then lower;
		first lower then done;
	}
	action def S :> G;
}
