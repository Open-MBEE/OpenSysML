- **A persistent session in the public Go API.** `opensysml.OpenSession` opens a `Session` over
  a model a `New` client parsed: an interactive run that keeps its clock, its scheduling policy
  and the objects it instantiated between calls, where `ExecuteAction` and `ExecuteState` run a
  whole behaviour and return. `SetSchedule` governs the turns from then on, the machines and
  clock already running included (`runtime.Context.Reschedule`), where they stand kept.
  `Instantiate` makes an object and starts the state machines it
  exhibits; `ActiveStates` and `Transitions` say where each machine stands and what could fire
  next, by name; `Accepts` says whether a signal would be taken, read from the machines dispatch would let take it
  — one whose guards all fail yields it to a sibling that would fire on or defer it — whether a
  transition is triggered by it and whether a guard holds now;
  `Send` posts it and `Advance` dispatches it, completion transitions included; `Perform` runs an
  action on the object and reports its outputs, the `ChoicePoint`s the schedule resolved and the
  `Branch` each decision left by, with `TurnedAway()` for an action that declined at its opening
  decision; `Feature`, `SetFeature`, `Evaluate` and `Members` read and write the state the runs
  left. Every answer is a fact copied out of the engine, never one of its graphs or objects, and
  misuse — a closed session, a signal no transition accepts, an unknown action, an exploration
  policy — is a typed refusal. The session is opened from a `Client` but is not part of the
  `Client` interface: a `Dial` client refuses it with `CodeUnimplemented`, because the service
  exposes no RPC for state held between calls, and the parity contract on `Client` is untouched.
- **The Legend of the Red Dragon's browser game is written against the public API.**
  `examples/lord-demo/web` imports `client/opensysml` and nothing under `internal/`, playing the
  model in a `Session` — the same menus, refusals, deterministic dice and rules, all still in
  `lord.sysml`. The WebAssembly binary grows to about 57 MB with the public client linked in.
