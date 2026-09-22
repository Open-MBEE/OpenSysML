- **The PSSM referee's refusal of a guard that acts on the model is settled, not provisional.**
  *Choice 005* traces its four guards to show when a junction's and a choice's are read.
  Recording the runtime's own guard reads as the referee's observable was tried and refused:
  the suite reads the junction on the entered state's default entry before the incoming
  transition's effect and the state's entry, where the runtime and `StatePerformances.kerml`
  read a transition inside a state after its entry, and the runtime's trace keeps only the
  first read at each vertex, the others rolled back with the probe that made them. A `calc def`
  with a side effect is refused too: a v2 expression is pure, and UML 2.5.1 §14.5.11 calls a
  guard with a side effect ill formed. The alignment note tables the three candidates against
  the admitted trace, and no bucket, reason or trace moves.
