// The steps of a constraint body run in declaration order before its
// conditions are evaluated in the state they left: (1 * 10) + 2 == 12, which
// no other order of the two assignments answers.
package test {
	private import ScalarValues::*;

	constraint ordered {
		attribute y : Integer = 1;
		assign y := y * 10;
		then assign y := y + 2;
		y == 12
	}
}
