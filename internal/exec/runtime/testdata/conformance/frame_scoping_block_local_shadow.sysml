// A `while` body is a scope of its own: its `attribute x` shadows the
// performance's `x` for the statements written inside it (KerML 1.0 §8.2.3.5.3),
// a node declared inside the block reads the block's `x`, and once the block has
// completed the enclosing body's `x` is the one it declared, untouched.
package test {
	private import ScalarValues::*;

	action def Loop {
		attribute x : Integer = 100;
		out log : Integer = 0;
		out later : Integer = 0;

		first start;
		action body {
			attribute n : Integer = 2;
			while n > 0 {
				attribute x : Integer = n * 10;
				assign log := log * 100 + x;
				then action peek { assign log := log + x; }
				then assign n := n - 1;
			}
			then assign later := x;
		}
		done;

		succession first start then body;
		succession first body then done;
	}
}
