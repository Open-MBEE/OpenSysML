package test {
	private import ScalarValues::*;
	private import RealFunctions::*;
	calc def Fn { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def FnI { in calc f { in v : Real; return : Integer; } in a : Real; return : Integer = f(a); }
	calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
	part def Holder {
		attribute root : Real = Fn(sqrt, 16.0);
		attribute floored : Integer = FnI(floor, 2.75);
		attribute fn = Identity(sqrt);
	}
}
