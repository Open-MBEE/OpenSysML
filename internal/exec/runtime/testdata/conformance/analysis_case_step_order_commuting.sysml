package test {
	private import ScalarValues::*;

	analysis def Commute {
		return : Integer;
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		action s1 { assign x := 1; }
		action s2 { assign y := 2; }
		x + y
	}
}
