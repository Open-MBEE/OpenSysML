package test {
	private import ScalarValues::*;

	calc def Descend {
		in n : Integer;
		return : Integer;
		attribute y : Integer := 0;
		assign y := y * 10;
		assign y := y + 2;
		if n > 0 {
			return : Integer = Descend(n - 1);
		}
		y
	}

	action def Use {
		out attribute r : Integer := 0;
		assign r := Descend(2);
	}
}
