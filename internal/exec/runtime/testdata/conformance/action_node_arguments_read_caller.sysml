// An invocation's arguments are the enclosing action's expressions: `a` in
// `Adder(a, 1)` is the caller's `a`, even when the node's own input pin `a`
// already holds a bound value that the argument then replaces.
package test {
	private import ScalarValues::*;

	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		attribute a : Integer = 7;
		attribute hundred : Integer = 100;
		out attribute result : Integer;

		bind add.a = hundred;

		first start;
		then action add : Adder = Adder(a, 1);
		then action fin { assign result := add.sum; }
		then done;
	}
}
