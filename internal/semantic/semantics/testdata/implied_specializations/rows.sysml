package R {
	attribute def AD; enum def ED { e1; } occurrence def OD; individual occurrence def IOD;
	item def ID; part def PD; port def PoD; connection def CD3 { end a : PD; end b : PD; end c : PD; }
	connection def CD2 { end a : PD; end b : PD; } interface def IfD { end a : PoD; end b : PoD; }
	allocation def AlD; flow def FD; action def AcD; state def SD; calc def CaD; constraint def CoD;
	requirement def RD; concern def CnD; case def CsD; analysis def AnD; verification def VD;
	use case def UD; view def ViD; viewpoint def VpD; rendering def ReD; metadata def MD;
	part def Sub :> PD; part def Sub2 :> Sub, ID;

	attribute au; occurrence ou; item iu; part pu; port pou; action acu; state su; calc cau;
	constraint cou; requirement ru; concern cnu; case csu; analysis anu; verification vu;
	use case uu; view viu; viewpoint vpu; rendering reu; allocation alu;
	part typed : PD; part subs :> pu; part redundant : PD :> pu;
	connection cu2 : CD2; interface ifu : IfD;

	item def Owner {
		attribute na; occurrence no; item ni; part np; ref part rp; port npo;
		action nac; state ns; calc nca; constraint nco; requirement nr;
	}
	part def POwner {
		part np; port npo; action nac; state ns; perform action pa; exhibit state es;
		assert constraint asc; ref item ri;
	}
	port def PoOwner { port sp; }
	action def AOwner {
		action sa; calc sc; state ss; perform action pa; fork f; join j; merge m; decide d;
		ref action ra;
	}
	state def SOwner { state sub; action sa; }
	state def SPar parallel { state sub; }
	calc def COwner { calc sc; action sa; }
	case def CsOwner { case sc; }
	analysis def AnOwner { analysis sa; action aa; }
	verification def VOwner { verification sv; }
	use case def UOwner { use case su; include use case iu; }
	requirement def ROwner { requirement sr; concern sc; require constraint rc; assume constraint ac; }
	view def ViOwner { view sv; rendering sr; viewpoint vp; }
	rendering def ReOwner { rendering sr; }
	occurrence def OOwner { occurrence so; snapshot sn; timeslice ts; event occurrence ev; }
}
