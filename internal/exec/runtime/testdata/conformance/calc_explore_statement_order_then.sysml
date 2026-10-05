package test {
	private import ScalarValues::*;

	calc def Then {
		return : Integer;
		attribute y : Integer := 1;
		assign y := y * 2 + 1;
		then assign y := y * 3 + 2;
		y
	}

	action def Use {
		out attribute r : Integer := 0;
		assign r := Then();
	}
}
