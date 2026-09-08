package test {
	private import ScalarValues::*;

	calc def Unary { in x : Real; return : Real; }
	calc def Fn { in calc f { in x : Real; return : Real; } in a : Real; return : Real = f(a); }

	// A calc declared in an action body closes over the performance it is read in;
	// passed on as a function, it still reaches that performance's inputs and locals,
	// and so does a second body-local calc it applies.
	action def Scaler {
		in k : Real;
		out y : Real;
		attribute offset : Real = 100.0;

		calc byK { in x : Real; return : Real = x * k; }
		calc shifted { in x : Real; return : Real = byK(x) + offset; }

		first step;
		action step { assign y := Fn(shifted, 3.0) + Fn(byK, 2.0); }
	}

	action outer {
		out attribute result : Real;

		first start;
		then action scale : Scaler { in k = 10.0; }
		then action fin { assign result := scale.y; }
		then done;
	}
}
