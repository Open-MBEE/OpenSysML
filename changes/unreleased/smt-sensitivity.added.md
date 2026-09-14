- **The `smt` engine decides whether the schedule decides a feature, under the one flag both
  checkers read.** `-check-diverge <feature>` (repeatable) and `%check-diverge` now make the
  checked action's question one of *sensitivity* for whichever engine answers: `check` searches
  the schedules for two that end the feature differently as before, `smt` asks a solver for the
  two at once — two copies of the action's unrolled relation sharing the initial state and the
  free inputs, both completing within the depth, the feature's final values differing — and
  `-engine all` puts the one question to both and composes their answers. A *sensitive* verdict
  names the two final values, the first step at which the two schedules part and the move each
  took there, and comes with both schedules as witnesses (`witness A:`/`witness B:`, `-json`'s
  `witness` and `contrast`), each replayed through the interpreter before it is claimed and each
  written by `-check-witness` as a `-A`/`-B` file that `-schedule replay:` and `%replay` follow.
  A feature every schedule ends alike is *holds*, *proved* when no schedule was cut by the
  depth, unroll or slot bounds and `no sensitivity found within k moves` (*bounded*) otherwise,
  with the live schedule or the cut loop as the reason; a deadlock or typed error every schedule
  reaches is reported as that violation instead. The refusal of `-check-diverge` under `-engine
  smt` alone is gone; `-check-states` under `smt` alone stays refused. Two expectations of the
  `check` engine moved with the shared question: a clean exhaustive search under
  `-check-diverge` now stands as `holds (bounded over schedules …)`, the same claim `smt` spells
  for its bounded negative, where it stood as `outcomes (bounded …)`; the `✓ … no violation,
  exhaustive` line is unchanged. A feature of the performing object and an action with the
  clock or a paused nested flow are refused by `smt` naming the construct until later stages
  encode them.
