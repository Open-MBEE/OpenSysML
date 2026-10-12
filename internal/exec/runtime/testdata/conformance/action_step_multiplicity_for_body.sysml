// Each iteration of a for loop is one performance of its body, which performs `a`
// twice with that iteration's `i`.
package test {
	private import ScalarValues::*;

	action def ForRep {
		attribute c : Integer = 0;
		first start then worker;
		action worker {
			for i : Integer in (1, 2, 3) {
				first start then a;
				action a[2] { assign c := c + i; }
			}
		}
		then done;
	}
}
