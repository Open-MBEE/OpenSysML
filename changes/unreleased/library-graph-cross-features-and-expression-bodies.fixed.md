- **Every bundled library file converts back from its graph without source text.** Reading a
  `.ttl` stripped of `sysx:sourceText` refused seven library files. A cross feature an `end`
  declares in its head (`end happensWhile subsets timeCoincidentOccurrences feature …`) was
  refused whenever its id had to be written, since the head holds no annotation: the reader
  now writes the id as `metadata : IdentityMetadata::ElementId about <end> { … }` in the end's
  body, and a normative library id, which the rebuilt notation implies on its own, is not
  written at all; a graph that gives the cross feature a body or members of its own is still
  reported. A declaration inside an expression body (`{ in x; attribute k = 2; x * k }`) was
  carried only as its notation and refused without it: it is now a `sysx:bodyMember` node of
  its own, typed by its metaclass and carrying what a namespace member carries, nested bodies
  included, and is written back from that structure. An invocation of a named function
  (`f(a, b)`, `x->f()`, `new T()`) was refused: it is now spelled from its `sysml:function`
  link by the same rule as every other reference, so a function the graph does not define, or
  that no spelling reaches, is still refused rather than misspelled. The inherited target of a
  cross feature's `subsets` is resolved in the end that owns it, so the spelling written back
  is the one the library wrote.
