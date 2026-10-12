package test {
	private import ScalarValues::*;

	action addThird {
		in x;
		out y;
		first start;
		action inner {
			assign y := x + 1 / 3;
		}
		then done;
		succession first start then inner;
	}
}
