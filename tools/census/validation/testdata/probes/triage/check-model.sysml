package P {
  attribute def T; item def I; part def D; port def PD; action def A; state def S; constraint def C;
  requirement def R; concern def CO; calc def K; case def CS; analysis def AN; verification def VF;
  use case def UC; view def V; viewpoint def VP; rendering def RD; metadata def MD; occurrence def O;
  enum def E { e1; }
  connection def CN { end a : D; end b : D; } interface def IF { end a : PD; end b : PD; }
  allocation def ALD { end a : D; end b : D; }
  attribute u_attribute; item u_item; part u_part; port u_port; action u_action; state u_state;
  constraint u_constraint; requirement u_requirement; concern u_concern; calc u_calc; case u_case;
  analysis u_analysis; verification u_verification; use case u_usecase; view u_view; viewpoint u_viewpoint;
  rendering u_rendering; occurrence u_occurrence; part d1 : D; part d2 : D; port p1 : PD; port p2 : PD; connection u_connection connect d1 to d2; interface u_interface connect p1 to p2;
  allocation u_allocation allocate d1 to d2; ref u_ref;
  part def Holder { part sub; item it; port q; action act; state st; occurrence oc; snapshot sn; timeslice ts;
    constraint cc; requirement rr; calc kk; attribute atr; analysis an; verification vf; port pt; connection cnx connect sub to it; interface ifx connect q to q; allocation alx allocate sub to it; viewpoint vpt; }
}
