- **An `individual` definition keeps its modifier through the RDF mapping.** The definition
  exporter now writes `sysml:isIndividual` for an `individual part def`, `individual item def`,
  `individual occurrence def` and every other definition kind the modifier may prefix — the same
  property a usage already carried — and the importer writes the modifier back from it, so a
  `.ttl` stripped of its `sysx:sourceText` no longer comes back as a plain `part def` with
  `An individual must be typed by one individual definition.` on each usage typed by it. An
  `individual def` carries the flag too and reads back by its keyword alone; a definition without
  the modifier still carries no flag.
