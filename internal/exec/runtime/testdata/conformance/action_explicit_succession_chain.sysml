package test {
	private import ScalarValues::*;
	action seq {
		attribute result : Integer = 0;
		action compute { assign result := 42; }
		succession first start then compute;
		succession first compute then done;
	}
}
