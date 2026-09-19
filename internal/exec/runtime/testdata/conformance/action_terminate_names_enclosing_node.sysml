package test {
	private import ScalarValues::*;

	// `terminate outer;` from a node nested two deep ends the enclosing node it
	// names: outer's later node never runs, and the parent continues after outer.
	action host {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;
		out attribute z : Integer = 0;

		first start;
		then action outer {
			action inner {
				assign x := 1;
				terminate outer;
			}
			action late { assign y := 2; }
			first inner;
			succession first inner then late;
		}
		then action after { assign z := 3; }
		then done;
	}
}
