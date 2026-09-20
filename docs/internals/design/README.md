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
  implemented (`internal/exec/analysis`), the rest is a proposal
- **[A Cameo Systems Modeler plugin](cameo-plugin.md)** — a proposal: a plugin for Cameo
  2024x Refresh3 that exports the selection, runs `Convert(xmi→sysml)` and `ParseSources` over
  the Java client, executes and verifies on `sysml-grpc`, and lands each verdict on the Cameo
  element it came from through the migration report's `xmi:id → target` accounting; the vendor
  plugin mechanics, UI contribution points, export routes, Simulation Toolkit positioning and
  SysML v2 status answered from public documentation with every unverified claim marked, a
  migration benchmark over twenty public v1 models, a phased plan and the risks
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
- **[Recording the order of orthogonal regions](region-order-scheduling.md)** — the entry and
  exit of a composite state's regions and the units of the firings one occurrence selects
  across regions as recorded choice points drawn on one front, with the kinds, the trace and
  witness lines, what each policy does at each, the rollback of a refused replay, the
  choice-point budget and the alignment row on firing granularity; and the designs, not yet
  implemented, of a do step against a dispatch and of a completion's firing inside the entry
  front
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
- **[Surface parity](api-surface-parity.md)** — a proposal: the REPL, the CLI, the editor, the
  public Go package and the wire inventoried operation by operation, each difference sorted as
  shared already, missing and worth adding, interactive, protocol-bound or local-only, and the
  stages that give every operation one assembly the four surfaces call, add the stateless
  operations the wire lacks (satisfiability, checker options, replay, views, search), design a
  session API for the debuggers apart from the stateless calls, and make the agreement a
  conformance protocol rather than a claim
- **[Transport evaluation](transport-evaluation.md)** — Connect and stdio measured beside gRPC,
  with the lifecycle-code delta and a recommendation
- **[Visual modeling in VS Code](vscode-visual-modeling.md)** — the live diagram panel, the
  diagram-driven edits, and the graphical editor it builds to

The staged plans that built the bindings, kept for their rationale:
[phase 1](python-grpc-phase1-plan.md) (the service),
[phase 2](python-grpc-phase2-plan.md) (the client),
[phase 3](python-grpc-phase3-plan.md) and its [fixes](python-grpc-phase3-fixes.md),
[phase 4](python-grpc-phase4-plan.md).
