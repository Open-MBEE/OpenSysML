package test {
	private import ScalarValues::*;

	calc def Pair {
		return : Integer;
		attribute a : Integer := 0;
		attribute b : Integer := 0;
		assign a := a + 2;
		assign b := b + 3;
		a + b
	}

	action def Use {
		out attribute r : Integer := 0;
		assign r := Pair();
	}
}
