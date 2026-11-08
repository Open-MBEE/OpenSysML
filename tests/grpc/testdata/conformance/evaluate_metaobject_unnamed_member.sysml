package Anonymous {
  part def V;
  part outer : V { part : V; part named : V; }
  attribute features [*] = (outer meta KerML::Type).ownedFeature;
}
