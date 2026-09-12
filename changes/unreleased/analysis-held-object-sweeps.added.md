- **A sweep over a held object runs on the object as the session holds it, behaviors included.**
  `%sweep` on an object from `%instantiate`, `-instantiate` followed by `-sweep`, and the gRPC
  sweep over such a subject now run when the object's type exhibits a state machine or performs
  an action, where they were refused before. The runtime records whether each execution has
  moved since its start — a token stepped, a body statement run, an accept or wait consumed, a
  transition fired, a timer or change trigger taken, a feature written — and a held object whose
  executions are all unmoved sweeps from its declaration in every row's context, printing the
  table the sequential form printed. A held object that has moved, been written, been sent a
  signal not yet dispatched, or is named by `#<id>` is swept from one image of it and everything
  it holds, taken when the sweep begins and made afresh in each row's context under the same
  identities, with the executors' state and the posted signals: every row starts where the held
  object stands, no row sees another's writes, and the held object is byte for byte as it was
  afterwards. An object the image cannot carry — a destroyed one, a body paused mid-statement, a
  debugger inside a step, a value bound to the run that made it — is refused naming the reason
  before any row runs, never swept on the session's state.
