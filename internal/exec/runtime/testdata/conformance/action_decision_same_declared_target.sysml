package test {
	private import ScalarValues::*;

	action guarded {
		attribute c : Boolean = true;
		attribute result : Integer = 0;

		first start;
		action shared {
			assign result := 42;
		}
		decide check;

		succession first start then check;
		succession first check then shared;
		succession first shared then done;
		if c then shared;
		else shared;
	}
}
