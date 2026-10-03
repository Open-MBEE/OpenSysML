package test {
	private import ScalarValues::*;

	action run {
		attribute source : Integer := 1;
		attribute live : Integer = source * 2;
		out attribute result : Integer;

		first start;
		then action update {
			assign live := 99;
			assign source := 5;
			assign result := live;
		}
		then done;
	}
}
