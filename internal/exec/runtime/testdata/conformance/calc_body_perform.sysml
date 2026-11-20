// A `perform` in a calculation body performs the action (SysML v2 7.19.1: a
// calculation is an action), and a later step and the result read its output.
package test {
	private import ScalarValues::*;

	package Lib {
		action def Scale { in a : Integer; out r : Integer = a * 10; }
	}

	calc def Scaled {
		in n : Integer = 3;
		attribute x : Integer := 0;
		perform action i : Lib::Scale { in a = n; out r; }
		assign x := i.r + 1;
		return : Integer = x;
	}
}
