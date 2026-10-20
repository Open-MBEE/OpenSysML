package K {
  class Cl; datatype DT; struct St; assoc As { end a : Cl; end b : Cl; } assoc struct AS { end a : St; end b : St; }
  behavior Bh; function Fn; predicate Pr; metaclass Mc;
  feature f; feature fc : Cl; feature fs : St; feature fd : DT; step sp; expr ex; bool bl; connector cx : As from fc to fc;
  class Oc { feature nested; step nstep; expr nexpr; bool nbool; }
}
