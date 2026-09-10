- **A design note for the analysis framework**
  (`docs/internals/design/analysis-framework.md`). It proposes one engine contract that the
  interpreter, the `explore` scheduling policy, the parameter sweep, the SMT constraint solver,
  the proposed model checkers and external analysis tools (`AnalysisTooling::ToolExecution`)
  register against; one scale for the strength of an answer — proved, bounded, witnessed,
  observed, not covered — under which a faster engine's observation never outranks a slower
  engine's proof; dispatch by question with explicit fallback and a referee mode that runs every
  covering engine; and runs isolated over shared immutable model state so sweep rows, explored
  linearizations and solver queries can run in parallel with results identical to the sequential
  ones. Every existing flag, command, RPC and field keeps its meaning. Nothing is implemented;
  the note exists to be reviewed before code is written.
