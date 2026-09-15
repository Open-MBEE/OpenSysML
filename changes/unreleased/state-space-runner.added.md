- **A continuous model runs by time-stepping.** An action specializing the analysis library's
  `ContinuousStateSpaceDynamics` or `DiscreteStateSpaceDynamics` is run as a fixed-step
  state-space simulation: the bundled `StateSpaceIntegration` library adds `FixedStepDynamics`
  (`timeStep`, an optional `stopTime`, a `time` the run writes), the integrators `Euler` and `RK4`
  a model binds to `getNextState`'s `integrate` (RK4 when it binds none) and the `ZeroCrossing`
  event; discrete dynamics step by `getDifference`. Each step advances the runtime's shared clock,
  so a state machine exhibited beside the dynamics sees the same time, its `accept after`/`at`
  triggers fire in step order, and a step and a trigger due together are a `due order` choice
  point the scheduling policy decides. An `event occurrence` typed by `ZeroCrossing` posts an
  event of its type when its `guard` changes sign at a step, which a machine's `accept` takes, and
  ends the dynamics when `terminal`. The run records `state: <action> t=<instant> x=<state>
  y=<output>` per step in the execution trace and reports `stateSpace`, `output` and `time` as
  the action's outputs. A shape the runner cannot run — a state, input, derivative or output that
  is not a vector, a protocol calc left abstract, an integrator the runtime does not provide, a
  step that is absent, zero or negative, a state that leaves a step non-finite — is a typed error
  naming the action and the member at fault.
