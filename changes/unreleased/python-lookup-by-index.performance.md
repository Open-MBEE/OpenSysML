- **The Python client answers `Model.find`, `Model.get`, `model[name]` and `name in model` from
  the service's index instead of walking the symbol tree one RPC at a time.** A qualified name
  is one `GetSymbol` call; a short name is one `Query` on the effective `name` followed by one
  fetch, so a lookup costs the same on a model of ten symbols and one of ten thousand, where the
  walk took tens of milliseconds. What is found is unchanged: the outermost symbol of a shared
  short name, and among those the one declared first. A library symbol is now reachable by its
  qualified name too (`model.get("ISQBase::mass")`), with a name declared in the model winning over
  a library package of the same spelling. A service without the `query` capability is still
  walked. `import opensysml` no longer loads the HTTP stack, which only a release download
  needs; `opensysml.binary.fetch` imports it when one happens.
