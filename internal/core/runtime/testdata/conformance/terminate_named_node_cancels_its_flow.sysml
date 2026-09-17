// `terminate b` ends a sibling node while its own flow waits, nested two deep,
// on the clock: the wait is cancelled, b's remaining steps never run, and the
// join sees b as ended so the flow completes.
package test {
	private import ScalarValues::*;

	action def A {
		out x : Integer := 0;
		out y : Integer := 0;
		out z : Integer := 0;

		first start;
		fork f;
		action aw accept after 1 [SI::s];
		action a { terminate b; assign x := 1; }
		action b {
			first start;
			action inner {
				first start;
				action bw accept after 5 [SI::s];
				action bset { assign y := 2; }
				done;
				succession first start then bw;
				succession first bw then bset;
				succession first bset then done;
			}
			done;
			succession first start then inner;
			succession first inner then done;
		}
		join j;
		action tail { assign z := 3; }
		done;

		succession first start then f;
		succession first f then aw;
		succession first aw then a;
		succession first f then b;
		succession first a then j;
		succession first b then j;
		succession first j then tail;
		succession first tail then done;
	}
}
