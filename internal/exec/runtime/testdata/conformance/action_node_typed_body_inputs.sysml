package test {
	private import ScalarValues::*;

	// The `in x = …` members of a typed node bind the parameters of the action it
	// performs, and the node's frame holds what that performance produced.
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		out attribute result : Integer;

		first start;
		then action add : Adder {
			in a = 3;
			in b = 4;
		}
		then action fin { assign result := add.sum; }
		then done;
	}
}
