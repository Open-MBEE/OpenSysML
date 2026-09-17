// `terminate` is a statement of an action body (SysML.xtext
// TerminateActionUsage): bare, it ends the performing occurrence; with a
// target, the named one. It stands wherever a statement does — in a body, a
// conditional, a loop, a state's entry action — and keeps its target.
package Termination {
	private import ScalarValues::*;

	action def Halt {
		out n : Integer := 0;

		first start;
		fork f;
		action a { assign n := 1; terminate; }
		action b { terminate a; }
		action c { terminate Halt; assign n := 9; }
		action d {
			while n < 3 {
				assign n := n + 1;
				if n == 2 { terminate; }
			}
		}
		join j;
		done;

		succession first start then f;
		succession first f then a;
		succession first f then b;
		succession first f then c;
		succession first f then d;
		succession first a then j;
		succession first b then j;
		succession first c then j;
		succession first d then j;
		succession first j then done;
	}

	state def M {
		entry; then s;
		state s {
			entry action { terminate; }
		}
	}
}
