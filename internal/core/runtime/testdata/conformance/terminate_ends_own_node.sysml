// `terminate` with no target ends the action node stating it: the statement
// after it does not run, the node's succession is taken, and a concurrent
// branch goes on.
package test {
	private import ScalarValues::*;

	action def A {
		out x : Integer := 0;
		out y : Integer := 0;
		out z : Integer := 0;

		first start;
		fork f;
		action a { assign x := 1; terminate; assign x := 8; }
		action b { assign y := 2; }
		join j;
		action tail { assign z := 3; }
		done;

		succession first start then f;
		succession first f then a;
		succession first f then b;
		succession first a then j;
		succession first b then j;
		succession first j then tail;
		succession first tail then done;
	}
}
