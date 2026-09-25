package test {
	private import ScalarValues::*;

	// A flow addresses the pins of the nodes it joins: the same pin name at both
	// ends names two features, one per node's frame.
	action outer {
		out attribute result : Integer;

		first start;
		then action p { out v : Integer; assign v := 21; }
		then action q { in v : Integer; out w : Integer; assign w := v * 2; }
		then action fin { assign result := q.w - p.v; }
		then done;

		flow p.v to q.v;
	}
}
