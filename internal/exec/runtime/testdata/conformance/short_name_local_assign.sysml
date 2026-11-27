package test {
	private import ScalarValues::*;

	// A local declaring a name and a short name is one feature, so one binding: an
	// assignment through either name is read through the other.
	action outer {
		out attribute viaName : Integer = 0;
		out attribute viaShort : Integer = 0;
		out attribute afterName : Integer = 0;
		out attribute unvalued : Integer = 0;
		out attribute shadowed : Integer = 0;
		out attribute inner : Integer = 0;

		first start;
		then action body {
			attribute <s> x : Integer := 1;
			attribute <u> late : Integer;
			first start;
			then assign s := 5;
			then assign viaName := x;
			then assign viaShort := s;
			then assign x := x + 2;
			then assign afterName := s;
			then assign u := 3;
			then assign unvalued := late;
			then if true {
				// This x is another feature: the short name still writes the outer one.
				attribute x : Integer := 100;
				assign s := s * 2;
				then assign inner := x;
			}
			then assign shadowed := x;
		}
		then done;
	}
}
