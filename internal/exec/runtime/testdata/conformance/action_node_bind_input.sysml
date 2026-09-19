package test {
	private import ScalarValues::*;

	// A binding connector whose end is a pin of a later node gives that node's
	// input its value when the node starts, from the feature at the other end.
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		attribute x : Integer = 5;
		attribute y : Integer = 6;
		out attribute result : Integer;

		bind add.a = x;
		bind y = add.b;

		first start;
		then action add : Adder;
		then action fin { assign result := add.sum; }
		then done;
	}
}
