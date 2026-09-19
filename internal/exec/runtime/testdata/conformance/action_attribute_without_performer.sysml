// A directly executed action has no performance occurrence, so its declared
// attributes remain executor-local feature values.
package test {
	private import ScalarValues::*;

	action def Bump {
		attribute count : Integer = 0;

		action step {
			assign count := count + 1;
		}
		first step;
	}
}
