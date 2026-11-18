// A `terminate` in a step of a calculation body ends that step alone (SysML v2
// 7.17.10): the rest of its body is skipped and the step after it is performed.
package test {
	private import ScalarValues::*;

	calc def Stepped {
		attribute x : Integer := 1;
		action a {
			assign x := x + 8;
			then terminate;
			then assign x := 0;
		}
		then action b { assign x := x * 2; }
		return : Integer = x;
	}
}
