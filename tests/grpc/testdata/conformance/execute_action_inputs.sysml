package test {
	private import ScalarValues::*;

	action addFive {
		attribute result : Integer = 0;
		first start;
		action inner {
			assign result := result + 5;
		}
		done;
		succession first start then inner;
		succession first inner then done;
	}
}
