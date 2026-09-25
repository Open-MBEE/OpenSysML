package test {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Cube { in v : Real; return : Real = v * v * v; }
	calc def Identity { in calc f { in v : Real; return : Real; } return r = f; }
	calc def Same {
		in calc f { in v : Real; return : Real; }
		in calc g { in v : Real; return : Real; }
		return : Boolean = f == g;
	}
	part def Holder {
		attribute fn = Identity(Sq);
		attribute same : Boolean = Same(Sq, Sq);
		attribute other : Boolean = Same(Sq, Cube);
	}
}
