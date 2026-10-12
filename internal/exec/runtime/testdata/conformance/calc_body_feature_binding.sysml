package test {
	private import ScalarValues::*;

	calc def Recompute {
		in initial : Integer;
		attribute current : Integer := initial;
		attribute doubled : Integer = current * 2;
		assign current := 6;
		return : Integer = doubled;
	}

	calc c : Recompute { in initial = 3; }
}
