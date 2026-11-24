package R {
	attribute def AD; part def PD; item def ID;
	part def Veh { attribute mass : AD; part eng : PD; ref part drv : PD; item cargo : ID; port p; attribute m2 : AD [0..2] = mass; }
	state def SD { entry action en; do action dd; exit action ex; state s1; }
	action def Asg { attribute x : AD; in v : AD; action asg assign x := v; }
	requirement def R { subject s : Veh; }
	use case def UC { subject u : Veh; objective o : R; }
	use case def UC2 :> UC { subject u2 : Veh; objective o2 : R; }
	view def VD { render rendering r; }
	part def Var { variation part choice : PD { variant part v1 : PD; variant part v2 : PD; } }
	variation part def VPD :> PD { variant part w1; }
	action def Act { in a : AD; out b : AD; }
	action def Act2 :> Act { in a2 : AD; out b2 : AD; }
	calc def Calc { in c : AD; return res : AD; }
	part def Conn { part p1 : PD; part p2 : PD; connection cx connect p1 to p2; }
	item def Msg; 
	part def Fl { part src : PD { out item o : Msg; } part dst : PD { in item i : Msg; } flow fl of Msg from src.o to dst.i; }
	occurrence def Occ { attribute t : AD; }
	action def Loop { action lp for i in (1,2,3) { } }
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
