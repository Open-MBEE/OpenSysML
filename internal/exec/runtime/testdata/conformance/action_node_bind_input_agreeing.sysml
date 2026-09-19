package test {
	private import ScalarValues::*;

	// Two binding connectors ending at one input pin are two equalities: when
	// their other ends agree the pin holds that value once, whichever is stated
	// first, and the node reads it as usual.
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		attribute x : Integer = 5;
		attribute alsoX : Integer = 5;
		out attribute result : Integer;

		bind add.a = x;
		bind alsoX = add.a;

		first start;
		then action add : Adder { in b = 6; }
		then action fin { assign result := add.sum; }
		then done;
	}
}
