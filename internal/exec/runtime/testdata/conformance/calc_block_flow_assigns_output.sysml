// A calc whose output is assigned only inside a nested action declared in a loop
// body still computes: the assignment lives in the block's own token flow, and
// the shape of the calc counts it as much as one written in the body directly.
package test {
	private import ScalarValues::*;

	calc lastSeen {
		out reached : Integer;
		for i in 1..3 {
			action note {
				assign reached := i * 10;
			}
		}
	}
}
