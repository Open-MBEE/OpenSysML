// A step's multiplicity counts its performances per performance of the body that
// features it: each pass of the loop body performs `a` three times and `skip`
// never, so two passes add six to `c`.
package test {
	private import ScalarValues::*;

	action def LoopRep {
		attribute c : Integer = 0;
		attribute passes : Integer = 0;
		first start then worker;
		action worker {
			while passes < 2 {
				first start then a;
				action a[3] { assign c := c + 1; }
				action skip[0] { assign c := c + 100; }
				succession first [*] a then [1] tally;
				action tally { assign passes := passes + 1; }
			}
		}
		then done;
	}
}
