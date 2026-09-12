- **The bounded model checker's independence relation counts message order.** The design note
  (`docs/internals/design/bounded-model-checking.md`) now declares any two sends, and any two
  accepts, dependent whatever their receivers: the bus is one context-wide list in arrival
  order and an accept takes the oldest match, so neither pair reaches the same captured state
  in both orders.
