// A loop body of a calculation stating its own flow: each iteration is a new
// performance of it, and the succession runs `take` before `count` although
// `count` is declared first.
package test {
	private import ScalarValues::*;

	calc summed {
		in n : Integer;
		attribute i : Integer := n;
		attribute acc : Integer := 0;
		while i > 0 {
			action count { assign i := i - 1; }
			action take { assign acc := acc * 2 + i; }
			first take then count;
		}
		return : Integer = acc;
	}
}
