package test {
	private import ScalarValues::*;

	// An `if` branch's `bind`s address its own action nodes' pins: add.a is seeded
	// from x before add runs, add.sum is bound into mirror.n, and a sibling reads add.sum.
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		attribute x : Integer = 5;
		out attribute result : Integer = 0;
		out attribute mirrored : Integer = 0;

		first start;
		then action choose {
			if x > 0 {
				bind add.a = x;
				action add : Adder { in b = 6; }
				bind add.sum = mirror.n;
				action mirror { in n : Integer; assign mirrored := n; }
				assign result := add.sum;
			}
		}
		then done;
	}
}
