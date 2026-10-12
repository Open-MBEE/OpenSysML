package test {
	private import ScalarValues::*;

	// A [0] step is no performance, so nothing waits on it: both of its successors
	// still run, each once.
	action def ZeroFanOut {
		attribute c : Integer = 0;
		attribute d : Integer = 0;
		first start then a;
		action a[0];
		then q;
		then r;
		action q { assign c := 7; }
		action r { assign d := 8; }
	}
}
