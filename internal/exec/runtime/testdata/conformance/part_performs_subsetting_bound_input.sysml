package test {
	private import ScalarValues::Integer;
	action def Step { in x : Integer; out y : Integer; first start; then assign y := x + 1; }
	action t : Step { in x = 9; }
	part def P {
		attribute src : Integer = 4;
		perform action pt : Step :> t;
		bind pt.x = src;
	}
	part p : P;
}
