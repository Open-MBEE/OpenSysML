- **A design record on protocol state machines** (`docs/project/protocol-state-machines.md`). It
  establishes that SysML v2 has no counterpart to UML's protocol state machine and needs none for
  the half it can express: the legal order of receptions on a port or part is an ordinary exhibited
  state machine, which the runtime runs on parts with `accept … via`. It also records what the
  runtime does not yet do: a message a model sends that the active state neither accepts nor defers
  is held on the bus and taken by a later state rather than dropped and reported as a directly
  injected event is, and a machine exhibited by a port definition does not take a model's messages
  routed to that port. Post-conditions, conformance between machines, static sequence checking and
  the gating of operation calls by state have no SysML v2 spelling. The record specifies the runtime
  follow-up with its proof fixtures; the roadmap item stays open and the compliance bullet points at
  the record.
