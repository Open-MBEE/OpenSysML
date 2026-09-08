package test {
	private import ScalarValues::*;
	calc cube { in v : Real; return : Real = v * v * v; }
	calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def UseCube { return : Real = Fn(cube, 2.0); }
}
