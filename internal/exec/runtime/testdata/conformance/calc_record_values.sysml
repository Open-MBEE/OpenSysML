package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	attribute def Point { attribute x : Real; attribute y : Real; }
	attribute def Seg { attribute a : Point; attribute b : Point; }
	attribute def Def { attribute x : Real; attribute y : Real default 3.0; }
	attribute def Many { attribute xs : Real[0..*] nonunique; }
	attribute def Scaled { attribute k : Real; calc scale { in v : Real; return : Real = v * k; } }
	calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }

	// Features read off a record, scalar, defaulted, sequence and nested.
	calc def Read { in a : Real; attribute p : Point = new Point(y = 2.0, x = a); return : Real = p.x + p.y; }
	calc def Defaulted { in a : Real; return : Real = new Def(a).y; }
	calc def Seq { in a : Real; return : Real[0..*] nonunique = new Many((a, a, 1.0)).xs; }
	calc def Nested { in a : Real; attribute s : Seg = new Seg(new Point(a, 1.0), new Point(2.0, a)); return : Real = s.b.y + s.a.x; }
	// A data value is equal to itself and its aliases, not to an equal-featured one.
	calc def Identity {
		in a : Real;
		attribute p : Point = new Point(a, a);
		attribute q : Point = p;
		return : Boolean = p == p and p === q and new Point(a, a) != new Point(a, a) and not (new Point(a, a) === new Point(a, a));
	}
	// A calc read off a record closes over that record: equal through an alias, unequal across records.
	calc def Receiver {
		in a : Real;
		attribute s : Scaled = new Scaled(a);
		attribute t : Scaled = s;
		return : Boolean = s.scale(2.0) == 2.0 * a and Apply(s.scale, 3.0) == 3.0 * a
			and s.scale == t.scale and s.scale != new Scaled(a).scale;
	}

	action def Records {
		out read : Real = Read(1.5);
		out defaulted : Real = Defaulted(1.0);
		out seq : Natural = size(Seq(2.5));
		out nested : Real = Nested(4.0);
		out identity : Boolean = Identity(1.0);
		out receiver : Boolean = Receiver(2.0);
	}
	action records : Records;
}
