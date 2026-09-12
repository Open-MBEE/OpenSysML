- **An unnamed transition with a body is an anonymous member of its state.** `transition first
  off then on { … }` had a scope for its body but no symbol, so nothing that walks a state's
  members by declaration — a `Route` annotation in the body among them — could find it, while
  the same transition named `t` was found. It is now registered as a named one is, under no
  name, so it resolves nothing new and is listed and annotated as the state's own feature (SysML
  v2 §7.19.2); one with no name and no body, effect or trigger still declares nothing.
