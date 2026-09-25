// A result written in a loop body that also declares a nested action returns from
// the calc where it stands: the first iteration notes 10 and returns it, so the
// result the body states after the loop is never reached.
package test {
	private import ScalarValues::*;

	calc firstNoted {
		attribute noted : Integer = 0;
		for i in 1..3 {
			action note {
				assign noted := i * 10;
			}
			return : Integer = noted;
		}
		return : Integer = 0;
	}
}
