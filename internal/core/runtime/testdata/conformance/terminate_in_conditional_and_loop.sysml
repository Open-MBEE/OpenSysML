// `terminate` unwinds the conditional and the loop it stands in: the loop
// stops at the iteration stating it and the statements after both are skipped.
package test {
	private import ScalarValues::*;

	action def A {
		out n : Integer := 0;
		out after : Integer := 0;

		first start;
		action count {
			while n < 10 {
				assign n := n + 1;
				if n == 3 { terminate; }
			}
			assign after := 1;
		}
		done;

		succession first start then count;
		succession first count then done;
	}
}
