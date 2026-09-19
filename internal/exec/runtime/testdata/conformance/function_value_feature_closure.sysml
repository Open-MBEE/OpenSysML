package test {
	private import ScalarValues::*;
	part def Scaler {
		attribute k : Real;
		calc scale { in x : Real; return : Real = x * k; }
	}
	part twice : Scaler { attribute :>> k = 2.0; }
	part thrice : Scaler { attribute :>> k = 3.0; }
	calc def Fn { in calc f { in x : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def UseScalers { return : Real = Fn(twice.scale, 5.0) + Fn(thrice.scale, 5.0); }
}
