package test {
	private import ScalarValues::*;
	action def G {
		attribute level : Integer = 0;
		action raise { assign level := 3; }
		assert constraint raised { level == 3 }
		action lower { assign level := 1; }
		assert not constraint high { level > 2 }
		first start then raise;
		first raise then raised;
		first raised then lower;
		first lower then high;
		first high then done;
	}
	action def S :> G;
}
