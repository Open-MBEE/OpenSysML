package test {
	private import ScalarValues::*;
	calc def Pow { in base : Real; in exp : Real; return : Real = base ** exp; }
	calc def Apply {
		in calc f { in base : Real; in exp : Real; return : Real; }
		in x : Real;
		return : Real = f(exp = 2.0, base = x);
	}
	calc def UseApply { return : Real = Apply(x = 3.0, f = Pow); }
}
