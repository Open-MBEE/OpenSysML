- **A conformance case can admit several outcomes and constrain the order of its trace.** Where
  the Kernel Semantic Library leaves more than one result open, `.expected.json` lists every
  admissible result under `outcomes` and must cite, in `admissible`, the section of the behavior
  semantic oracle deriving them; the run must match exactly one, and a missing or unresolvable
  citation fails the case. A `<case>.trace.order` file of `a < b` lines states the partial order
  the recorded trace must satisfy, beside or instead of an exact golden. The fork case that writes
  one feature from two branches now admits both `x = 1` and `x = 2`; the default schedule and every
  exact golden are unchanged.
