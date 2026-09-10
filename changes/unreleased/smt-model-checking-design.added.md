- **A design note for SMT bounded model checking of behaviors**
  (`docs/internals/design/smt-model-checking.md`). It proposes unrolling an action's token flow
  to a bounded number of moves and asking the SMT solver whether any schedule, for any value of
  the inputs the model leaves unbound, violates a requirement, deadlocks or leaves a feature's
  final value depending on the order of two moves. It fixes what a verdict may claim — proved
  within a stated bound, violated with a witness the interpreter replays, sensitive with the two
  schedules, or not covered with the reason — a per-construct coverage table, the referee gate
  against `-schedule explore`, and the stages. Nothing is implemented; the note exists to be
  reviewed before code is written.
