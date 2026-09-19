package test {
	private import ScalarValues::*;

	// An action declared in a loop body may state a flow of its own (`first`,
	// successions, `done`): each iteration performs that flow to completion in
	// the node's frame, whose nodes read each other's pins, before the statement
	// after it reads step.v. total = 6 + 7 = 13.
	action outer {
		out attribute total : Integer = 0;

		first start;
		then action iterate {
			for i in 1..2 {
				action step {
					out v : Integer;
					first start;
					then action one { out w : Integer; assign w := i + 4; }
					succession one then two;
					action two { assign v := one.w + 1; }
					then done;
				}
				assign total := total + step.v;
			}
		}
		then done;
	}
}
