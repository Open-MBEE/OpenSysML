# Examples

This directory contains example SysML v2 models.

## OMG Training Examples

The official OMG SysML v2 training examples are **not included in this repository**.

**Download them with:**

```bash
./scripts/download-training-examples.sh
```

That fetches `sysml/src/training` from the pinned pilot release
(https://github.com/Systems-Modeling/SysML-v2-Pilot-Implementation) into
`sysml-v2-training/`, which is gitignored.

**Status:** the corpus gate's current result is in
[docs/project/training-examples.md](../docs/project/training-examples.md), with the files that
still report errors and why.

## Walkthroughs

Each of these is a model and a walkthrough of the commands that exercise it.

| Model | Walkthrough | What it demonstrates |
| --- | --- | --- |
| [disposal-robot-demo/robot.sysml](disposal-robot-demo/robot.sysml) | [disposal-robot-demo/README.md](disposal-robot-demo/README.md) | one bomb-disposal robot, end to end: structure, calculations, an action with a fork/join, a branch and a nested flow, a hierarchical state machine an object exhibits, assignment through a feature chain, the solver commands, the view renderings, and [the same questions from Python](disposal-robot-demo/robot_demo.py) |
| [disposal-team-demo/team.sysml](disposal-team-demo/team.sysml) | [disposal-team-demo/README.md](disposal-team-demo/README.md) | the team around that robot, written for what the robot demo does not reach: quantities with units, `select` and `reduce` over a fleet, a message crossing the connector two parts are joined by, an occurrence with a snapshot and a timeslice, and a requirement, use case, verification case and analysis case over the same subject |
| [relay-probe-demo/mission.sysml](relay-probe-demo/mission.sysml) | [relay-probe-demo/README.md](relay-probe-demo/README.md) | one individual probe across its mission phases: event occurrences ordered in time, snapshots and a timeslice of one individual, occurrences with multiplicity, a calculation reading across two snapshots, a requirement whose subject is a snapshot, and a beacon inside a timeslice sending telemetry through its probe's own port |
| [analysis-demo/lander.sysml](analysis-demo/lander.sysml) | [analysis-demo/README.md](analysis-demo/README.md) | analysis cases, asked every way the tool answers them: an analysis whose action steps feed each other and whose objective is a requirement, run bound, with arguments and on an object; a verification case whose body decides its verdict beside its objective; a parameter sweep and a seeded sample; two trade studies choosing among three landers; an action and a state machine due at the same instant of one clock, under each scheduling policy and explored; `-trace`, `-json`, the REPL forms and [the same questions from Python](analysis-demo/lander_demo.py) |
| [solver-demo.sysml](solver-demo.sysml) | [SOLVER-DEMO.md](SOLVER-DEMO.md) | `%check`, `%explain`, `%solve`, `%configure` and `%optimize` — what conditions *can* hold, which conflict, what satisfies them, which variants are permitted, what is best (needs z3 or cvc5) |
| [oosem-demo/oosem-demo.sysml](oosem-demo/oosem-demo.sysml) | [oosem-demo/README.md](oosem-demo/README.md) | the `OOSEM` library on a small Earth-observation mission: as-is and to-be enterprise, causal analysis, stakeholder needs derived down to component requirements with `#moe`/`#mop`, the black-box system context and its use case, the logical scenario and components, and the physical architecture distributed over nodes |
| [mosa-demo/mosa-demo.sysml](mosa-demo/mosa-demo.sysml) | [mosa-demo/README.md](mosa-demo/README.md) | the `MOSA` library on a modular ground vehicle: the major system platform, its major system components and a modular autonomy system, the modular system interfaces between them (one written as `#keyInterface`), the consensus standards they conform to, data rights and proprietary elements, interface control, MOSA requirements with their traces, a conformance assessment, the MOSA views and a generated interface control document; `-validate` reports the openness gaps the model leaves on purpose |
| [views-demo.sysml](views-demo.sysml) | [VIEWS-DEMO.md](VIEWS-DEMO.md) | `%view` and `%render` — the five rendering kinds, the text/Mermaid/Markdown forms, viewpoint conformance and filtered exposure |
| [expressions-demo.sysml](expressions-demo.sysml) | [EXPRESSIONS-DEMO.md](EXPRESSIONS-DEMO.md) | the expression forms worked through one payload: `as` casts that select rather than convert, `*` as the unbounded value, `.metadata` on an annotated part, calculations passed and invoked as function values, a `Set` with no order and no repeats, a rank-three tensor quantity indexed and scaled, and `collect`/`select`/`reduce` bodies typed by what they return |
| [action-executor-demo.sysml](action-executor-demo.sysml) | [ACTION-EXECUTOR-DEMO.md](ACTION-EXECUTOR-DEMO.md) | executing actions, and stepping one in the REPL |
| [self-model/](self-model/) | [self-model/README.md](self-model/README.md) | OpenSysML's own architecture in SysML v2: the analysis pipeline as parts, ports and item flows onto the Go packages that implement it, the validation tiers and the two execution engines as state machines, the [AGENTS.md](../AGENTS.md) architecture invariants as requirements the tool evaluates, and the views `make self-model` renders the architecture diagrams from |
| `parser_features_demo_*.sysml`/`.kerml` | [PARSER_FEATURES_DEMOS.md](PARSER_FEATURES_DEMOS.md) | the notation the parser accepts, feature by feature |

## Other Examples

Additional example models may be added to this directory. SysML v2 files use the `.sysml`
extension and KerML files use `.kerml`.
