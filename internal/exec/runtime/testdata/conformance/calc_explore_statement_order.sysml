package test {
	private import ScalarValues::*;

	calc def Ord {
		return : Integer;
		attribute y : Integer := 1;
		assign y := y * 10;
		assign y := y + 2;
		y
	}

	action def Use {
		out attribute r : Integer := 0;
		assign r := Ord();
	}
}
