package test {
	private import ScalarValues::*;

	// A binding at a node's input pin supplies its value ahead of the default the
	// node's own declaration states, and a default written in terms of the bound
	// pin reads the bound value.
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		attribute x : Integer = 5;
		out attribute result : Integer;
		out attribute untouched : Integer;

		bind add.a = x;

		first start;
		then action add : Adder {
			in a = 1;
			in b = a * 10;
		}
		then action plain : Adder {
			in a = 1;
			in b = a * 10;
		}
		then action fin {
			assign result := add.sum;
			assign untouched := plain.sum;
		}
		then done;
	}
}
