package test {
	private import ScalarValues::*;

	// A node's pin holds a value only once the node has been performed: reading it
	// from a node that runs earlier is reported, not answered with nothing.
	action outer {
		out attribute total : Integer;

		first start;
		then action early { assign total := late.v; }
		then action late { out v : Integer; assign v := 1; }
		then done;
	}
}
