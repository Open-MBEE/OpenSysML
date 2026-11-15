// A `terminate` in a calculation body ends the calculation's performance (SysML v2
// 7.17.10), so the steps after it are not taken; the result is a binding of the
// calculation, not a step, and reads the value the steps left: 1, 2, 3 and stop.
package test {
	private import ScalarValues::*;

	calc def Capped {
		in cap : Integer = 3;
		attribute x : Integer := 0;
		while x < 10 {
			assign x := x + 1;
			if x == cap { terminate; }
		}
		assign x := 99;
		return : Integer = x;
	}
}
