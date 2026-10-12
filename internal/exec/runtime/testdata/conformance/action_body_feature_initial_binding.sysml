package test {
	private import ScalarValues::*;

	action run {
		attribute source : Integer := 2;
		attribute snapshot : Integer := source;
		attribute fallback : Integer default source;
		out attribute result : Integer;

		first start;
		then action change {
			assign source := 5;
			then assign fallback := 9;
			then assign result := snapshot * 10 + fallback;
		}
		then done;
	}
}
