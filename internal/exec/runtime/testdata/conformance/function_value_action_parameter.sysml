package test {
	private import ScalarValues::*;

	calc def Unary { in v : Real; return : Real; }
	calc def Sq :> Unary { in :>> v; return : Real = v * v; }
	calc def Cube :> Unary { in :>> v; return : Real = v * v * v; }
	calc sq : Sq;
	calc cube : Cube;

	// An action's `in calc` parameters hold the functions bound to them, and a
	// body applies them as it applies any calc.
	action def Apply {
		in calc f { in v : Real; return : Real; }
		in calc g : Unary;
		in a : Real;
		out y : Real;

		first step;
		action step { assign y := f(a) + g(a); }
	}

	action outer {
		out attribute result : Real;

		first start;
		then action apply : Apply {
			in f = sq;
			in g = cube;
			in a = 2.0;
		}
		then action fin { assign result := apply.y; }
		then done;
	}
}
