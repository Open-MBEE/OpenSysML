- **A design record on protocol state machines** (`docs/project/protocol-state-machines.md`). It
  establishes that SysML v2 has no counterpart to UML's protocol state machine and needs none for
  the half it can express: the legal order of receptions on a port or part is an ordinary exhibited
  state machine, which the runtime already runs on parts with `accept … via` and on port definitions
  with a bare `accept`; an arrival the active state does not accept is dropped and reported rather
  than raised. Post-conditions, conformance between machines, static sequence checking and the gating
  of operation calls by state have no SysML v2 spelling. The record specifies one optional follow-up,
  an opt-in typed error for a refused arrival, with its proof fixtures. The roadmap item and the
  compliance bullet now point at it.
