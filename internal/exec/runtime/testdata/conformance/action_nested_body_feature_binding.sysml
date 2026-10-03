package test {
	private import ScalarValues::*;

	action outer {
		attribute a : Integer := 2;
		out attribute result : Integer;

		first start;
		then action child {
			attribute x : Integer = a * 3;
			out attribute y : Integer;

			first start;
			then action change { assign a := 4; }
			then action read { assign y := x; }
			then done;
		}
		then action collect { assign result := child.y; }
		then done;
	}
}
