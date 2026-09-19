package test {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def UseFn { return : Real = Fn(Sq, 3.0); }
}
