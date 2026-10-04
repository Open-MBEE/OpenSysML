package test {
	private import ScalarValues::*;
	calc def Ordered {
		return : Integer;
		attribute y : Integer := 1;
		assign y := y * 10;
		then assign y := y + 2;
		assign y := y + 3;
		y
	}
}
