// A calc body dividing Integers through the non-normative OpenSysML extension
// library's quotient: the exact ratio truncated toward zero, so -7 by 2 is -3,
// and exact above 2^53 where a Real quotient would round.
package test {
	private import ScalarValues::*;
	private import OpenSysMLMathFunctions::*;

	calc def splitLoad {
		in load : Integer;
		in ways : Integer;
		return : Integer = quotient(load, ways) + quotient(-7, 2) + quotient(27021597764222979, 3);
	}
}
