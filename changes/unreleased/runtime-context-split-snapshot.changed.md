- **The runtime's `Context` is split into a shared `Model` and a run's own state, and a run
  can be snapshotted and restored.** `runtime.Model` holds what the runtime derives from the
  model alone — the semantic model, the resolver, the memoized calc shapes, write and
  invocation targets, literal caches, compiled calc closures and effective features — and is
  built once per analysis worker; `runtime.NewContext(model, maxSteps)` allocates only what a
  run mutates (objects, lifetimes, variants, the message bus, the clock, the scheduler, the
  trace), so a fresh context rebuilds none of the model's tables. `Context.Snapshot`, and the
  action and state executors' `Snapshot`, mark the run's journal between steps and capture the
  executors' tokens, frame tree, configuration, event queue, timers and `do` progress;
  `Restore` rolls the run back to the mark as often as asked, keeping every object's identity,
  until `Release`. A snapshot asked for inside a step or of a body paused mid-statement is the
  typed `ErrSnapshotMidRun` or `ErrSnapshotPausedBody`. The conformance suite proves the round
  trip on every case at every step, and `OPENSYSML_SCHEDULE_SEEDS` widens its scheduling sweep
  to further seeds. No flag, command, RPC, field or line of output changed.
