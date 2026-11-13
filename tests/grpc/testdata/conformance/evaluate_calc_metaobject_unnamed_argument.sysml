package Anonymous {
  private import ScalarValues::*;
  part def V;
  part outer : V { part : V; part named : V; }
  calc def IsTheAnonymousPart { in m : Metaobjects::Metaobject; return : Boolean[1] = m === (outer meta KerML::Type).ownedFeature#(1); }
  calc isTheAnonymousPart : IsTheAnonymousPart;
}
