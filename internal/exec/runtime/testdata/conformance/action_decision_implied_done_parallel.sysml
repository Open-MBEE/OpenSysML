package test {
	private import ScalarValues::*;

	action guarded {
		attribute c : Boolean = true;
		attribute result : Integer = 0;

		first start;
		action prepare {
			assign result := 7;
		}
		decide check;

		succession first start then prepare;
		succession first prepare then check;
		if c then done;
		else done;
	}
}
