# Design notes

How a subsystem is built and which normative reference it answers to. For
maintainers; the behavior a user sees is [the guide](../../guide/).

- **[The action executor](action-executor.md)** — how a token moves through a lowered
  `ActionGraph`
- **[The analysis framework](analysis-framework.md)** — one engine contract that the
  interpreter, `explore`, the sweep, the SMT solver, the model checkers and external tools
  register against, one scale for the strength of an answer (proved, bounded, witnessed,
  observed, not covered), dispatch by question with fallback, and runs isolated so they can be
  parallel; the contract, the registry and the four engines over existing code are
  implemented (`internal/core/analysis`), the rest is a proposal
- **[Bring your own engine](bring-your-own-engines.md)** — a proposal: a manifest and a
  protocol under which a user's own engine, scheduling policy, sampler or tool registers with
  the analysis framework as a process, a WebAssembly module or Go over the public package; every
  external witness is checked by the interpreter against the claim it supports, and a universal
  claim is *observed* only over executions the interpreter replayed, otherwise *not covered*
  until the site admits more against a referee record
- **[Bounded model checking of behaviors](bounded-model-checking.md)** — a proposal: explore
  every admissible interleaving up to a bound with partial-order reduction, and report the
  requirement violations, deadlocks and schedule-dependent outcomes it finds
- **[An embedded target for Class A flight software](embedded-target.md)** — a proposal: a
  closed, serializable behavior IR with a written semantics as the requirement basis, a
  freestanding C profile (no allocation, no recursion, static loop bounds, no extensions,
  structurally coverable emission) over static tables, static refusal of every admissible
  scheduling choice and every unbounded resource, the artifacts and traceability a tool
  qualification argument consumes, and a fixed-step host interface proved under Zephyr on QEMU
  and as an F´ component
- **[Scheduling policies, choice points and exploration](scheduling.md)** — how a run
  resolves what the library leaves unordered, reports each such choice without changing the
  run, takes another linearization under `declared` or `seed:<n>`, and enumerates every one
  within a budget under `explore`
- **[Orthogonal regions](orthogonal-regions.md)** — concurrent substates, in the standard
  `parallel` notation; the bundled libraries give them no performance, so UML 2.5.1 supplies
  the semantics
- **[Pseudostates](pseudostates.md)** — choice, junction, fork, join
  and history
- **[Alignment with the UML precise-semantics specifications](precise-semantics-alignment.md)** —
  an assessment: PSSM, fUML and PSCS mapped clause by clause against the SysML v2 notation, the
  KerML library and what the runtime does, with a verdict per row, a count of where a port could
  change behavior, the PSSM test suite assessed as a referee, options and a recommendation
- **[Python gRPC bindings](python-grpc-bindings.md)** — the service and client design
- **[SMT bounded model checking of behaviors](smt-model-checking.md)** — a proposal: unroll an
  action's token flow to a bounded number of moves and ask an SMT solver whether any schedule and
  any input violates a requirement, with every witness replayed in the interpreter and `explore`
  as the referee
- **[Transport evaluation](transport-evaluation.md)** — Connect and stdio measured beside gRPC,
  with the lifecycle-code delta and a recommendation
- **[Visual modeling in VS Code](vscode-visual-modeling.md)** — the live diagram panel, the
  diagram-driven edits, and the graphical editor it builds to

The staged plans that built the bindings, kept for their rationale:
[phase 1](python-grpc-phase1-plan.md) (the service),
[phase 2](python-grpc-phase2-plan.md) (the client),
[phase 3](python-grpc-phase3-plan.md) and its [fixes](python-grpc-phase3-fixes.md),
[phase 4](python-grpc-phase4-plan.md).
