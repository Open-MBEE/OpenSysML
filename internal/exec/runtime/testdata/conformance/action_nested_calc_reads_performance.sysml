// A calc declared in an action's body reads the performance of that action it is
// applied in — the one typed by the action def, or the subaction's own — whether
// invoked directly or passed on as a function value.
package test {
	private import ScalarValues::*;

	calc def Fn { in calc f { in x : Real; return : Real; } in a : Real; return : Real = f(a); }

	action def Scale {
		in k : Real;
		out r : Real;
		calc byK { in x : Real; return : Real = x * k; }

		first start;
		then action direct { assign r := byK(3.0); }
		then action nested {
			attribute m : Real = 2.0;
			calc byM { in x : Real; return : Real = x * m * k; }
			assign r := r + byM(1.0);
		}
		then action passed { assign r := r + Fn(byK, 1.0); }
		then done;
	}

	action run {
		out attribute total : Real = 0.0;

		first start;
		then action s : Scale { in k = 10.0; }
		then action tally { assign total := s.r; }
		then done;
	}
}
