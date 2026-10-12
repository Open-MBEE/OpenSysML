package test {
	private import ScalarValues::*;

	action act {
		attribute a : Integer := 0;
		attribute s : Integer = a * 2;
		out attribute r : Integer;

		first start;
		then action write { assign a := 5; }
		then action read { assign r := s; }
		then done;
	}
}
