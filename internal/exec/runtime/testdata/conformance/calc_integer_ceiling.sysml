// Rounding up through the extension library's ceiling: -2.5 is -2, and the
// least Integer is a value rather than the overflow -floor(-x) reports.
package test {
	private import ScalarValues::*;
	private import OpenSysMLMathFunctions::*;

	calc def slotsFor {
		in load : Real;
		in perSlot : Real;
		return : Integer = ceiling(load / perSlot) + ceiling(-2.5) + ceiling(-9223372036854775808.0) - ceiling(-9223372036854774784.0);
	}
}
