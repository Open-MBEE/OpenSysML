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
	item def Sig; part def PQ { attribute q : AD; }
	action def W { in c : Boolean; action wl while c { } until c; }
	action def L { in c : Boolean; action ll loop { } until c; }
	action def I { in c : Boolean; action ifa if c { } else { } }
	action def I2 { in c : Boolean; action ifb if c { } }
	action def I3 { in c : Boolean; action ifc if c action t { } else action e { } }
	action def W2 { in c : Boolean; action wm while c action step { } }
	action def Snd { in s : Sig; part p : PQ; action sa send s via p to p; }
	action def Acc { part p : PQ; action aa accept m : Sig via p; }
	action def Ter { part p : PQ; action ta terminate p; action tb terminate; }
	action def AsgC { part p : PQ; in v : AD; action ac assign p.q := v; }
	action def LoopT { in xs : AD[*]; action lt for j : AD in xs { } }
	state def ST { state sa; state sb; transition tr first sa then sb; }
	part def Sat { part sv : Veh; requirement sr : R; satisfy sr by sv; }
	calc def CR { in c : AD; return res : AD; c }
	calc def CDf { in a : AD; in b : AD default a; return r : AD; }
	part def E2 { attribute d1 : AD; attribute einvd : AD = CDf(d1); attribute edef : AD default d1; attribute einit : AD := d1; }
	action def AccT { in c : Boolean; part q : PQ; action aw accept when c via q; }
	part def Sat2 { requirement sr2 : R; satisfy sr2; }
}
