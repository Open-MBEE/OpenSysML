- **The architecture self-model now shows the runtime from the inside.** A new
  `examples/self-model/execution.sysml` models the instance layer as the six units it is — the
  schema built once per type, the allocator, the lazy reader, the binding propagator, admission
  and the dependency tracker — with the value flows between them, the scheduler with its kinds
  of choice and policy spellings, and one action executed as an interaction from the surface's
  request through lowering, stepping, scheduling, evaluation and every feature read or written.
  `behavior.sysml` gains `ReadFeatureValue`, one feature read whose decision nodes are the cases
  a feature can be in (undeclared, bound, held, a variation, a `default` yielding to
  contributions, a stated value, a connector, a composite). Three views render them —
  `instanceLayer` as an interconnection diagram, `featureReadFlow` as an action flow and
  `actionExecution` as a sequence diagram — and the architecture document embeds all three in
  its validation-and-execution section. The self-model test checks the modelled layer against
  the runtime: the effective feature's field count, the schema's memoization, the refusals the
  reader and admission spell, the scheduler's choice kinds and policy spellings, and every path
  of the feature read.
