- A state-space dynamics action performed again in one `sysml` process — a second `-action`, or
  an `-action` after an `-analysis` or another action — now integrates from its bound initial
  state for its whole `stopTime` and ends at the same values as the first performance. `stopTime`
  and `time` (`StateSpaceIntegration::FixedStepDynamics`) are durations from the performance's
  own start, not instants of the shared clock, so a performance begun after the clock had
  advanced no longer ends at its initial state, drops a step or reports a `time` offset by the
  earlier runs. A negative `stopTime` is refused as a typed error.
