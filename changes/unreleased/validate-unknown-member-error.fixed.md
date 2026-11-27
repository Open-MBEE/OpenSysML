- `sysml -validate=V::Nope`, `%validate V::Nope` and the other commands and RPCs that take an
  object reference report a name that resolves to nothing inside a package as unresolved
  (`unresolved reference: V::Nope`), as `-constraint` does. They answered
  `no instance of "V" (use %instantiate first)`, naming the package, which is never an object.
  A name that runs past a held object is still read as a path from that object, on the REPL and
  over gRPC alike (`V::car has no feature "P"`).
