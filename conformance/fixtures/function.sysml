package F {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Cube { in v : Real; return : Real = v * v * v; }
	calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc apply : Apply;
	calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
	attribute pick = Identity(Sq);
	attribute nine = Apply(Sq, 3.0);
	part def Scaler {
		attribute k : Real = 2.0;
		calc scale { in x : Real; return : Real = x * k; }
	}
	part holder : Scaler;
	attribute scaler = holder.scale;
}
