package test {
	private import ScalarValues::*;

	// A flow in a branch leaves the body's own statements unordered.
	calc def Ord {
		return : Integer;
		attribute y : Integer := 1;
		assign y := y * 10;
		assign y := y + 2;
		if y < 0 {
			action a { assign y := y + 1; }
			action b { assign y := y * 2; }
			first b then a;
		}
		y
	}

	action def Use {
		out attribute r : Integer := 0;
		assign r := Ord();
	}
}
