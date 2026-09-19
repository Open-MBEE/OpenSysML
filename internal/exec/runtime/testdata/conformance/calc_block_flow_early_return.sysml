// A `return` reached inside a nested action declared in a loop body ends the
// calc, so the flow of the block, the loop around it and the body all stop: the
// second iteration returns 2 and the third never runs.
package test {
	private import ScalarValues::*;

	calc firstAbove {
		attribute seen : Integer = 0;
		for i in 1..3 {
			assign seen := i;
			action check {
				if seen > 1 {
					return : Integer = seen;
				}
			}
			assign seen := 0;
		}
		return : Integer = seen;
	}
}
