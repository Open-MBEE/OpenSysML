- **A namespace-level object usage of several occurrences denotes its objects.** `part wheels :
  Wheel[2];`, `item links : Link[3];` or `part hubs : Hub[1..*];` declared in a package with no
  value now denotes its lower bound of objects for the run (KerML 1.0 §7.3.4.3 Multiplicities — a
  feature of multiplicity `[2]` has exactly two values), created once in declaration order and
  reused on every read, through the same materializer that fills a collection nested in an object;
  a `[0..*]` usage still denotes none. `all Wheel` counts them beside the `[1]` usages, a usage of
  exact count reads as the sequence (or set) of those objects, and `wheels#(1)`, `wheels.radius`
  and `size(wheels)` read them, in the REPL and over gRPC alike, while a usage of open count read
  directly (`hubs`, `size(hubs)`) stays undetermined of that count, as a nested collection of open
  count does; the trace names each member after its usage (`wheels #1`, `wheels #2`). It used to denote nothing, so `all Wheel` was refused with
  `ErrExtentUnavailable` and `wheels.radius` was undetermined. A valued usage (`part wheels :
  Wheel[2] = (new Wheel(), new Wheel());`) is bound to its value instead, and a value whose count
  breaks the declared multiplicity is refused with `ErrMultiplicityViolation` naming the usage. A
  member that cannot be constructed, a lower bound over the materialized-collection cap
  (`part many : Wheel[10000];`) or over the element budget is that typed error naming the usage
  and leaves no object, behavior or record behind. A port at namespace level still denotes no
  object and keeps its `ErrExtentUnavailable` refusal.
