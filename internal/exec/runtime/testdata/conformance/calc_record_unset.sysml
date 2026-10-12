package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	attribute def Point { attribute x : Real; attribute y : Real; }
	attribute def Seg { attribute a : Point; attribute b : Point; }
	attribute def Def { attribute x : Real; attribute y : Real default 3.0; }

	// A required feature no argument binds holds one unset value: not null, not empty, equal only to itself.
	calc def NotNull { in a : Real; return : Boolean = new Point(a).y == null; }
	calc def Distinct { in a : Real; return : Boolean = new Point(a).y == new Point(a).y or new Point(a).y === new Point(a).y; }
	calc def Alias { in a : Real; attribute p : Point = new Point(a); return : Boolean = p.y == p.y and p.y === p.y and not (p.y != p.y); }
	calc def VsValue { in a : Real; return : Boolean = new Point(a).y != a and not (new Point(a).y == a); }
	calc def Size { in a : Real; return : Integer = size(new Point(a).y); }
	calc def Empty { in a : Real; return : Boolean = isEmpty(new Point(a).y) or not notEmpty(new Point(a).y); }
	calc def Kept { in a : Real; attribute p : Point = new Point(a); return : Boolean = (p.y ?? 5.0) === p.y; }
	calc def Nested { in a : Real; return : Boolean = new Seg(new Point(a), new Point(a, a)).a.y == null; }
	calc def Defaulted { in a : Real; return : Real = new Def(a).y; }

	action def Unset {
		out notNull : Boolean = NotNull(1.0);
		out distinct : Boolean = Distinct(1.0);
		out alias : Boolean = Alias(1.0);
		out vsValue : Boolean = VsValue(1.0);
		out size : Natural = Size(1.0);
		out empty : Boolean = Empty(1.0);
		out kept : Boolean = Kept(1.0);
		out nested : Boolean = Nested(1.0);
		out defaulted : Real = Defaulted(1.0);
	}
	action unset : Unset;
}
