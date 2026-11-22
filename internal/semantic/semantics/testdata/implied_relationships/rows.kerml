package K {
	datatype D; class C; struct S;
	assoc A { end a : C; end b : C; }
	assoc A2 specializes A { end a2 : C; end b2 : C; }
	assoc A3 { end a : C; end b : C; end c : C; }
	assoc AX { end [1] feature xa : C; end [0..*] feature xb : C; }
	feature fd : D; feature fc : C; feature fs : S;
	class Owner {
		feature od : D; feature oc : C; composite feature occ : C; composite feature os : S;
		var feature v : D; feature valued : D = od; feature m : D [0..2];
	}
	struct SOwner { composite feature so : S; feature sref : S; }
	behavior B { in x : D; out y : D; }
	behavior B2 specializes B { in x2 : D; out y2 : D; }
	function F { in p : D; return r : D; }
	function F2 specializes F { in p2 : D; return r2 : D; }
	class CC { feature x : C; feature y : C; connector cn : A from x to y; }
	feature p1 : C; feature p2 : C;
	connector top : A from p1 to p2;
	connector within : A from Owner::oc to Owner::occ;
	flow fl of D from Owner::od to Owner::valued;
}
