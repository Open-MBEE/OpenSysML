package test {
	private import ScalarValues::*;
	action par {
		attribute leftValue : Integer = 0;
		attribute rightValue : Integer = 0;

		fork split;
		action left { assign leftValue := 1; }
		action right { assign rightValue := 2; }
		join sync;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}
