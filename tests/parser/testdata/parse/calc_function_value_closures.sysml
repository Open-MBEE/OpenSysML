package Closures {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc sqU : Sq;
	calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def Stored { in b : Boolean; attribute g = sqU; if b { assign g := sqU; } return : Real = Apply(g, 2.0); }
	calc def Mk { in k : Real; calc s { in v : Real; return : Real = v * k; } return r = s; }
	calc def Rec { in k : Real; calc s { in v : Real; return : Real = if v > 3.0 ? v else s(v + k); } return r = s(1.0); }
	calc def Same { in k : Real; return : Boolean = Mk(k) == Mk(k) or Mk(k) === sqU; }
	calc def Text { in s : String; return : String = StringFunctions::Substring(s + "!", 1, 2); }
}
