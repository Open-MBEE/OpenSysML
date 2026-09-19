- **The SysML v1 migration writes interactions, receptions and the rest of a state machine
  executably.** An Interaction owned by a block is a scenario `action def` of every message
  kind: a signal send, a `synchCall`/`asynchCall` of an operation as a typed perform on the
  lifeline's object — `perform action spin : Motor::Spin ::> drive.motor.spin { in rpm = 30.0; }`,
  the arguments bound to the operation's `in` parameters by name or position — and a `reply`
  as the assignment of the call's result to the caller lifeline's attribute; a lifeline is
  resolved to the feature path through the block's parts, ports and references or to an `in`
  parameter, and `alt`/`opt`/`loop`/`par` fragments are `if`/`for`/`while`/`fork` structures
  when their guards parse and resolve, a reply answering the latest open call of its operation
  between its lifelines, never one another alternative or a concurrent operand made; a duration
  constraint between two messages with steps
  between them is a wait forked after the earlier step and joined before the later, so those
  steps count toward the interval. A lifeline or guard that does not resolve, a create or
  delete message and a message-less timing trace are refused with the reason, as is a call
  or signal message leaving an `in` parameter or signal attribute — inherited ones included —
  with no default and a lower bound above zero unbound; two parts of
  one type are two paths, so a lifeline standing for a part of that type is ambiguous. A Reception is
  an `action def` of the block that accepts its signal and runs its method with the signal's
  attributes bound to the method's parameters of the same name, so a signal sent to the object
  runs the method against the object; where the method requires a value no attribute supplies,
  the reception only accepts the signal and says so. State machines gain transitions across regions and
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
- **A migrated state values its entry and do parameters from the signal that enters it.** When
  every transition into a state accepts the same signal and its attributes fit the behavior's
  parameters in order, type and multiplicity — the attributes the signal inherits from its
  generals counted with its own — the `state def` keeps the signal in an item
  (`item setPoint : SetPoint;`) each transition assigns and the parameters read
  (`in target : ScalarValues::Real = setPoint.level;`); a state entered without a signal or with
  one that does not fit is reported with the transition or attribute that is the reason. A
  region holding no vertex is skipped when the machine's states are named as when they are
  written, so a machine whose other region is populated is written inline and a transition
  across nesting levels names its far end by a path that exists. A
  trigger naming no port is also written accepting via each port of the owner its signal arrives
  at — one the document's connectors and delegations carry a send of it to, one an item flow
  conveys it to, or one whose type (generals and realized interfaces included) declares an inward
  flow property or a reception of it — with the ports that declare nothing and receive no send
  reported as left unrouted, and an activity whose required input pin only
  parameters nothing values flow into is reported as never firing instead of written to wait.
- **A `via` path can start at a bound reference, and delegated, redefined and untyped ports
  route.** `send … via ctx.p` from a behavior whose `ctx` is bound to another object leaves that
  object's port even when the performer owns a feature of the same name, the binding shadowing
  it as it does in every other expression, while `via this.ctx.p` stays the performer's own;
  a part's port is known to the connectors its type inherits under the name the
  part was declared with before redefinition; a `ref` usage holds what is bound to it rather than
  an object of its own; and an untyped `port` materializes as a `Ports::Port`, so a binding
  connector can join it and a signal sent inward over it reaches the bound part's machine.
- **A migrated call or send that v1 fires without a required value keeps its place and performs
  nothing.** A call passing no argument for a parameter that must hold a value, or a call or
  signal send passing none for a signal attribute that must, or one whose pin is fed only by flows no value travels — from a parameter nothing values,
  an unmigrated opaque or value specification action, or a callee whose own activity gives that
  `out` parameter no value, judged through any depth of nesting — is written as an empty action carrying the token,
  with the reason in its comment and report line, and the object flow is kept as a comment
  rather than written from a feature that will hold nothing. Control and buffer nodes only
  object flows lead to route their values from source to pin, a control node no edge leaves
  ends the token as `done` does, and an action fed by an object flow from outside its control
  path waits for the value only when the producer runs on every pass of the surrounding loop.
