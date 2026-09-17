- **The SysML v1 migration writes behaviors that run.** An Activity becomes an `action def`
  the action executor performs — `first start`, nested `action x : Def;` calls with `bind`/`flow`
  for their pins, `fork`/`join`/`decide`/`merge`, `send new Sig() to this.part`, `accept p : Sig`,
  `accept after 2.0 [SI::s]`, `accept when c`, `action x terminate;` for a final node, `if`
  guards where the guard parses and resolves and the guard text as a comment where it does not —
  and a block's classifier behavior is performed by a `perform action` usage of its `part def`.
  A `DurationConstraint` on an action is a wait before it, `accept after lo [SI::s]` for a point
  interval and `accept after RandomFunctions::uniform(lo, hi) [SI::s]` otherwise, with `1s`,
  `80ms`, `2 min` literals scaled to seconds; «Probability» on the edges out of a decision is
  `@Stochastic::Probability { p = … }` when every edge carries one, scaled when they do not sum
  to one. A StateMachine becomes a `state def` the state debugger steps — nested states, the
  regions of an orthogonal state as sub-states of a `parallel` state, a submachine state as a
  `state` usage typed by the referenced machine's `state def`, `entry`/`do`/`exit` behaviors,
  `transition first s accept sig : Sig if g do e then t;` with relative time and change events
  as triggers, a deferrable signal trigger as `defer Sig;` — exhibited by an `exhibit state`
  usage of its block. An Operation is an `action def` owned by the block with its parameters,
  its method as body and its conditions as `assert constraint`s; a `CallOperationAction` on an
  object performs it on that object through
  `perform action x ::> target.op;`. An OpaqueBehavior or FunctionBehavior whose body is a v2
  expression is a `calc def`; an Interaction whose messages are all signal sends to parts is a
  scenario `action def` of `send`s; a Reception is a comment naming its signal. Absolute time
  events, internal transitions, entry points and history pseudostates, synchronous interaction
  messages and a tool's time variable have no v2 form and stay comments the report accounts for.
- **`perform action x ::> part.action;` and `exit part.action;` run on the part.** An action
  usage referencing a feature chain performs the chain's last action on the object the chain
  reaches from the performer, as a state's entry, do or exit behavior does; an empty or
  many-valued receiver, a chain ending in no action and a destroyed receiver are refused with
  typed errors.
- **`terminate;` and `terminate x;` end a performance from inside it.** A `terminate`
  statement ends the running action (the named nested action when one is given), cancelling the
  tokens its forks and joins hold, so a migrated activity final node ends the activity as it does
  in v1; the trace records each termination.
