package test {
	private import ScalarValues::*;

	analysis def Ordered {
		return : Integer;
		attribute x : Integer := 1;
		action s1 { assign x := x * 10; }
		action s2 { assign x := x + 2; }
		first s1;
		then s2;
		x
	}
}
