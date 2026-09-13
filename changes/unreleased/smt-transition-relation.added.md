- **An SMT model checker for actions on concrete inputs.** `internal/core/smt` encodes the
  lowered action graph — straight-line bodies, fork, join, merge, decisions, body loops unrolled
  to a bound, pins and object flows — as a transition relation over `k` moves and asks an SMT
  solver whether any schedule violates a requirement or constraint, or deadlocks. `unsat` is
  *proved* when every schedule finishes within the bounds and *bounded* otherwise; a `sat` model
  is decoded to the interpreter's choice lines and replayed through it, and is *violated* only
  when the replay reaches the state the solver described; `unknown`, a timeout, a body the
  encoding cannot express (clock, messages, nested flows, object-valued assignments, calc
  invocation, nonlinear arithmetic) and a replay that disagrees are *not covered* with the
  reason. The `smt` engine implements the analysis `Engine` contract over the solver
  `OPENSYSML_SMT`, `z3` or `cvc5` names, with `k` from `Budget.Depth`, the solver time from
  `Budget.Solver` and an unroll bound of its own (4), and is refereed against `explore` over the
  conformance corpus: outcome sets must agree, every witness must replay, and a proof must agree
  with exhaustive exploration. It is not yet registered with the default engines, so no verdict,
  golden or exploration changes; `-engine smt` follows.
- **`replay:<file>` scheduling policy.** A file of choice lines — as `explore`'s witness column
  spells them, one per line up to the first blank line, or `no choice points` alone for a run
  that met none — is followed move for move, then the run continues as `reverse`. A move the run cannot make (a pick not offered, a step already passed,
  a line left over at the end) fails the run with `replay refused: move <n> (<choice>): <what
  the run faced>` rather than running another linearization. Accepted by `sysml -schedule`,
  `%schedule` and a conformance case's `schedule` pin; over the wire, where a request carries no
  file of the caller's, the spelling is `INVALID_ARGUMENT`.
