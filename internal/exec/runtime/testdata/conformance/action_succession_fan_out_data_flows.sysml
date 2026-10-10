package test {
	private import ScalarValues::*;

	// p is one performance with two successions out of it, so its output is moved
	// once, before its token splits, and each flow out of it delivers to its own
	// target's pin.
	action pipe {
		attribute runs : Integer = 0;
		attribute got : Integer = 0;
		attribute doubled : Integer = 0;

		first start;
		then action p { out v : Integer; assign v := 21; assign runs := runs + 1; }
		action q { in v : Integer; assign got := v; }
		action r { in v : Integer; assign doubled := v * 2; }

		succession first p then q;
		succession first p then r;
		flow p.v to q.v;
		flow p.v to r.v;
	}
}
