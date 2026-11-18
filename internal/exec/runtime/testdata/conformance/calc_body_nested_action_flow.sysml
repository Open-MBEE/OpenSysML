// A calculation is an action (SysML v2 7.19.1) and its body admits action usages
// (CalculationBodyItem), so the actions it declares are performed and a flow
// between their pins carries a value: a.o = 4 reaches b.p, and x ends 3 + 4.
package test {
	private import ScalarValues::*;

	calc def Tally {
		in n : Integer = 3;
		attribute x : Integer := n;
		action a { out o : Integer = n + 1; }
		action b { in p : Integer; assign x := x + p; }
		flow a.o to b.p;
		return : Integer = x;
	}
}
