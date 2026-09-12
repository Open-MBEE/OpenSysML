- **The bounded model checker's independence relation counts message order.** The design note
  (`docs/internals/design/bounded-model-checking.md`) now declares two sends that may reach one
  receiver, and two accepts that may match one message, dependent: the bus keeps arrival order
  and an accept takes the oldest match, so neither pair reaches the same state in both orders.
