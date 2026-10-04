package test {
	private import ScalarValues::*;

	constraint def Ok {
		in x : Integer;
		attribute y : Integer := x;
		assign y := y * 10;
		assign y := y + 2;
		y == 12
	}

	action def A {
		out attribute r : Boolean := false;
		assign r := Ok(1);
	}
}
