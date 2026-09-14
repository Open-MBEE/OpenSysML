- **The `first` end of an action body's `first a then b;` is a reference to the succession's
  source, not a name the node declares.** The symbol table no longer registers a label under `a`
  for the two-ended form in an action body, so `a` resolves like any other succession end: a
  source nothing declares is reported as `unresolved reference` where it is written instead of
  being taken for a member the node declares, and `a` is collected as a reference for
  find-references and rename. A one-ended `first a;` still marks the start of the flow under `a`'s
  name, and a state machine's `first start then off;` still declares its `start` for transitions
  to name. The AST keeps the name after `first` as a qualified name (`InitialNode.First`), so the
  RDF mapping, the REPL and the language server read the source through the one binding; no
  notation, output or graph changed.
