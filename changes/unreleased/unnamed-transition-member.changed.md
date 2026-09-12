- **An unnamed transition is an anonymous member of its state.** `transition first off then
  on { … }` had a scope for its body but no symbol, and one with no body had neither, so nothing
  that walks a state's members by declaration — a `Route` annotation in the body among them —
  could find it, while the same transition named `t` was found. Every transition is now
  registered as a named one is, an unnamed one under no name, so it resolves nothing new and is
  listed and annotated as the state's own feature (SysML v2 §7.19.2).
