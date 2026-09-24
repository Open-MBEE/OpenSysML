package test {
	private import ScalarValues::*;

	// An action declared in a loop body is performed anew each iteration, in a
	// frame of its own: its `in n = i` reads the loop variable, its `out s` is read
	// by the statement after it through the node's name, and the value reported for
	// the node is its latest performance's. A typed node in the loop binds the
	// action it performs the same way, and its output is read back through it.
	action def Square {
		in x : Integer;
		out sq : Integer;

		first step;
		action step { assign sq := x * x; }
	}

	action sum {
		out attribute total : Integer = 0;
		out attribute cubes : Integer = 0;

		first start;
		then action iterate {
			for i in 1..3 {
				action square { in n = i; out s : Integer; assign s := n * n; }
				action cube : Square { in x = i; }
				assign total := total + square.s;
				assign cubes := cubes + cube.sq * i;
			}
		}
		then done;
	}
}
