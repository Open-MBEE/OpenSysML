package test {
	private import ScalarValues::*;

	calc def Twice {
		in k : Real;
		out d = k * 2.0;
	}

	action def CalcOrder {
		attribute v : Real := 1.0;
		attribute doubled : Real := 0.0;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): the assignment reading the calc
		// usage and the one writing its input are unordered, so `t.d` may bind `v` before it is written.
		action s {
			assign v := 2.0;
			calc t : Twice { in k = v; }
			assign doubled := t.d;
		}
		then done;
	}
}
