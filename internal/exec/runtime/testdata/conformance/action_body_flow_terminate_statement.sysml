package test {
	private import ScalarValues::*;

	action run {
		attribute sum : Integer = 0;
		attribute after : Integer = 0;
		attribute completed : Integer = 0;
		first start;
		then action iterate {
			for i in 1..3 {
				action a { assign sum := sum + i; } then terminate;
			}
			assign after := 1;
		}
		then action finish { assign completed := 1; }
		then done;
	}
}
