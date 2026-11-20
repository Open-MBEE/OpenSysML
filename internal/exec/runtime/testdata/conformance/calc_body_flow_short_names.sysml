package test {
	private import ScalarValues::*;

	// The ends of a succession name action nodes by their short names.
	calc def Short {
		in n : Integer;
		attribute x : Integer := 0;
		action <t> { assign x := x * 3; }
		action <d> add { assign x := x + n; }
		action <s> seed { assign x := 1; }
		first s then d;
		first d then t;
		return : Integer = x;
	}
}
