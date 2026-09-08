- **The scheduling policy a run resolves its choice points under is selectable.** Where the
  library orders nothing — several steppable tokens in one step, several holding guards at a
  decision, several enabled transitions out of one state for one event, and so whose same-step
  write to one feature stands — the executors follow a named policy: `reverse` (the default:
  reverse token order, first holding guard, first enabled transition, so every existing result and
  trace is unchanged), `declared` (tokens in spawn order, guards and transitions in declaration
  order) or `seed:<n>` (a pseudo-random order the seed fixes, so one seed replays one run on every
  platform and two seeds may take two linearizations). The spelling is the same everywhere: `sysml
  -schedule <policy>` for `-action`, `-state` and `-analysis` (a calc's body performs nothing, so
  `-calc` has no choice to make); `%schedule [<policy>]` in the REPL, shown with no argument and
  applied to the runs started after it while a debugging session under way keeps its own; a
  `schedule` field on `ExecuteActionRequest`, `ExecuteStateRequest` and `RunAnalysisRequest`,
  empty for the default and advertised as the `schedule` capability, with the Go and Python
  clients taking it as an option (`opensysml.WithSchedule`, `opensysml.Schedule`, `schedule=`);
  and a `schedule` pin on a conformance case, which the harness runs under. A policy changes only
  which alternative each choice takes: every choice point a run reaches is reported and each `took
  …` is what the policy took, though another linearization may reach other choice points. A
  spelling naming no policy — an unknown name, `seed` or `seed:` without a number, `seed:-1`,
  `seed:abc` — is refused before anything runs, as `INVALID_ARGUMENT` on the wire. `explore`, the
  bounded exhaustive replay, is reserved and refused by name until it exists. The conformance
  suite also runs whole under `declared` and `seed:1`, requiring every case that pins no policy
  and lists no `outcomes` to produce its default outputs. Two accepts racing for two sends now
  list both pairings as `outcomes`, with the derivation in the semantic oracle; a send to a
  same-named port pins `reverse` until the via-less accept that over-matches it is fixed.
