package Records {
    private import ScalarValues::*;
    attribute def Point { attribute x : Real; attribute y : Real; }
    attribute def Scaled { attribute k : Real; calc scale { in v : Real; return : Real = v * k; } }
    calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
    calc def Sum { in a : Real; attribute p : Point = new Point(y = 2.0, x = a); return : Real = p.x + new Point(1.0, a).y; }
    calc def Same { in a : Real; attribute p : Point = new Point(a, a); return : Boolean = p == p and p !== new Point(a, a); }
    attribute def Seg { attribute a : Point; attribute b : Point; }
    calc def Nest { in a : Real; attribute p : Point = new Point(a, 1.0); attribute s : Seg = new Seg(p, new Point(2.0, a)); return : Boolean = s.a == p and s.b.y == a; }
    calc def Direct { in a : Real; attribute s : Scaled = new Scaled(a); return : Real = s.scale(2.0) + Apply(s.scale, 3.0); }
}
