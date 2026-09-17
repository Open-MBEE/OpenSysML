- **The SysML v1 migration writes interactions, receptions and the rest of a state machine
  executably.** An Interaction owned by a block is a scenario `action def` of every message
  kind: a signal send, a `synchCall`/`asynchCall` of an operation as a typed perform on the
  lifeline's object — `perform action spin : Motor::Spin ::> drive.motor.spin { in rpm = 30.0; }`,
  the arguments bound to the operation's `in` parameters by name or position — and a `reply`
  as the assignment of the call's result to the caller lifeline's attribute; a lifeline is
  resolved to the feature path through the block's parts, ports and references or to an `in`
  parameter, and `alt`/`opt`/`loop`/`par` fragments are `if`/`for`/`while`/`fork` structures
  when their guards parse and resolve. A lifeline or guard that does not resolve, a create or
  delete message and a message-less timing trace are refused with the reason. A Reception is
  an `action def` of the block that accepts its signal and runs its method with the signal's
  attributes bound to the method's parameters of the same name, so a signal sent to the object
  runs the method against the object. State machines gain transitions across regions and
  nesting levels named by path, `junction`/`choice`/`fork`/`join`/`history`/`deep history`
  pseudostates, entry and exit points of a submachine as states of its `state def` addressed
  by path, internal transitions as self transitions where re-entry is not observable, and
  absolute time events as `accept at <instant>` over a `Time::TimeInstantValue` attribute of
  the behavior. A `CallOperationAction` over a port performs the operation on the part a
  connector of the caller's block joins to that port, the way connector paths resolve.
- **The migration report counts what nothing refers to apart from gaps.** An event no trigger
  names is skipped as a model element nothing refers to, counted apart from profile and library
  content in the summary line, rather than reported as unmapped.
- **A typed action usage that references a feature chain performs the chain on the object it
  reaches.** `perform action x : Def ::> part.action { in p = v; }` runs the part's action with
  the part as performer and binds the callee's inputs from its body, and an accept payload is
  visible from the body of a typed usage in the same action body, so a nested typed action can
  read the accepted message (`in level = msg.level`).
