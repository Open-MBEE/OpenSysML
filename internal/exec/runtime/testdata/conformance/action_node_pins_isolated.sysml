package test {
	private import ScalarValues::*;

	// Each action node is a performance of its own, holding the parameters it
	// declares: two sibling nodes may both declare `v` without one overwriting the
	// other, and a later node reads each through the node's name.
	action outer {
		out attribute total : Integer;

		first start;
		then action p { out v : Integer; assign v := 3; }
		then action q { out v : Integer; assign v := 4; }
		then action fin { assign total := p.v + q.v; }
		then done;
	}
}
