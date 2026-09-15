- **A join runs the effect of every transition into it.** The transitions into a join and the one
  out of it are segments of one compound transition, but firing the join ran only the effect of
  the incoming transition that completed last and dropped the others'. Every incoming segment now
  fires — its source exited, then its effect — before the state owning the join is exited and the
  outgoing segment's effect runs; the order among the incoming segments is a region-order choice
  the scheduling policy draws (`choice join <name>` in traces, source declaration order by
  default), and a failing effect on any incoming segment fails the step. The static footprint of a
  transition into a join folds in the other incoming effects too.
