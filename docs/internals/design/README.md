# Design notes

How a subsystem is built and which normative reference it answers to. For
maintainers; the behavior a user sees is [the guide](../../guide/).

- **[The action executor](action-executor.md)** — how a token moves through a lowered
  `ActionGraph`
- **[The analysis framework](analysis-framework.md)** — a proposal: one engine contract that
  the interpreter, `explore`, the sweep, the SMT solver, the model checkers and external tools
  register against, one scale for the strength of an answer (proved, bounded, witnessed,
  observed, not covered), dispatch by question with fallback, and runs isolated so they can be
  parallel
- **[Bring your own engine](bring-your-own-engines.md)** — a proposal: a manifest and a
  protocol under which a user's own engine, scheduling policy, sampler or tool registers with
  the analysis framework as a process, a WebAssembly module or Go over the public package; every
  external witness replays through the interpreter, and a universal claim is printed *observed*
  until the site admits more against a referee record
- **[Bounded model checking of behaviors](bounded-model-checking.md)** — a proposal: explore
  every admissible interleaving up to a bound with partial-order reduction, and report the
  requirement violations, deadlocks and schedule-dependent outcomes it finds
- **[Scheduling policies, choice points and exploration](scheduling.md)** — how a run
  resolves what the library leaves unordered, reports each such choice without changing the
  run, takes another linearization under `declared` or `seed:<n>`, and enumerates every one
  within a budget under `explore`
- **[Orthogonal regions](orthogonal-regions.md)** — concurrent substates, an OpenSysML
  extension against UML 2.5.1 semantics
- **[Pseudostates](pseudostates.md)** — choice, junction, fork, join, entry/exit points
  and history
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
