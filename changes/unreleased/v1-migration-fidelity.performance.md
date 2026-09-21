- **A state machine's poll of the signals in flight is memoized.** A run holding many active
  objects probed every state machine's transitions against every queued message at each
  scheduling step; the probe's answer is now kept until a queue, a write, a nested call, a
  rollback or another executor changes what it could see, which makes a long stochastic run of
  a model with many active parts several times faster with the same trace.
