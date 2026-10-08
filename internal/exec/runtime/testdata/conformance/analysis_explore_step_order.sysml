package test {
	private import ScalarValues::*;

	analysis def An {
		return : Integer;
		attribute x : Integer := 1;
		action s1 { assign x := x * 10; }
		action s2 { assign x := x + 2; }
		x
	}

	action def Use {
		out attribute r : Integer := 0;
		assign r := An();
	}
}
