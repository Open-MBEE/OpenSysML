package X {
	attribute def AD;
	calc def G2 { in a : AD; in b : AD; return r : AD; }
	part def P2 { attribute px : AD; attribute py : AD; }
	part def E {
		attribute d1 : AD; attribute d2 : AD; part ps : P2;
		attribute eop : AD = d1 + d2;
		attribute einv : AD = G2(d1, d2);
		part enew : P2 = new P2(d1, d2);
		attribute echain : AD = (ps).px;
		attribute elit = 1;
		attribute eidx = d1#(1);
		attribute esel = d1.?{in v : AD; true};
		attribute ecol = d1.{in v : AD; v};
		attribute econd = if true ? d1 else d2;
		attribute enest : AD = G2(d1 + d2, G2(d1, d2));
		calc cu : G2 { in a = d1; in b = d2; }
		constraint cc { d1 == d2 }
	}
}
