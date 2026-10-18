// A calc's parameter is a feature of the calc, not of the action invoking it: the
// body of `Scale` reads its own `k` — the argument — and a parameter default
// written in the calc (`base = k + 1`) reads the calc's `k` too (KerML 1.0
// §8.2.3.5.3). A nested calc usage binding `in k = k` reads the enclosing
// action's `k` for the value (the inputs being bound are not in scope of their
// own values), and a bare `k` in the action body stays the action's.
package test {
	private import ScalarValues::*;

	calc def Scale {
		in k : Integer;
		return r : Integer = k * 10;
	}

	calc def Offset {
		in k : Integer;
		in base : Integer = k + 1;
		return : Integer = base;
	}

	action run {
		attribute k : Integer = 7;
		out scaled : Integer = 0;
		out offset : Integer = 0;
		out usage : Integer = 0;
		out own : Integer = 0;
		calc s : Scale { in k = k; }

		first start;
		action apply {
			assign scaled := Scale(2);
			then assign offset := Offset(2);
			then assign usage := s.r;
			then assign own := k;
		}
		done;

		succession first start then apply;
		succession first apply then done;
	}
}
