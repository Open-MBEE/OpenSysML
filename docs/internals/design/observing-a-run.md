# Observing a run: an event stream, a live configuration, and model-declared listeners

A design for reading a behavior *while it runs* — the value an attribute holds now, the states
active now in every region of every machine, the action steps live now — from a client outside
the process, and for letting the model itself say what happens at those moments: a call into a
co-simulated service, a hook a diagram animates on. The note fixes the observation vocabulary
(one event stream and one snapshot shape, shared by the REPL, the public Go package and the
wire), the wire session that carries it, the metadata by which a model declares a listener, what
a listener may and may not do to the run it observes, and how the referees keep their standing
when a listener is attached. It answers the question *how can execution state be collected* the
way the [surface-parity](api-surface-parity.md) note's Stage 4 asked for: in its own note before
code is written.

**Status.** A proposal. Nothing here is implemented; the section
[What exists to build on](#what-exists-to-build-on) is the inventory of what is.

## The problem this answers

A behavior is run today in three ways, and each answers a different question about it:

- `ExecuteAction` / `ExecuteState` (the wire), `-action` / `-state` (the CLI) run it to completion
  and answer with the **result**: outputs, `states_visited`, `final_context`, `final_time`, and
  under `explore` the set of outcomes. Nothing in the middle is visible.
- `%action` / `%state` (the REPL) run it **one step at a time** and print, on request, the
  current state (`%current`), the tokens (`%tokens`), the queue (`%events`) and the objects'
  feature values (`%features`, `%eval`). Everything in the middle is visible, to a person at a
  prompt, as text.
- `opensysml.OpenSession` (the public Go package) runs it in process and answers **facts** —
  `ActiveStates`, `Feature`, `Transitions`, `Accepts`, the `ChoicePoint`s an `Advance` drew — to
  Go code linked into the binary.

What none of them gives is a program outside the process a *stream of what happened* and a
*snapshot of what is*, at the moments a behavior changes, with the run paused or not as the
program chooses. That is the shape every downstream use has:

- **Co-simulation.** On entering `Heating`, call a thermal service with the current `setpoint`
  and feed the reply back as a signal; on each `do` step of `Sample`, read a value from a
  running plant model.
- **Animation.** Highlight the active states on the state diagram, the live action nodes on the
  action diagram, the attribute values in the part tree, as the run moves — in the VS Code
  panel ([visual modeling](vscode-visual-modeling.md)), in a SysON or Cameo canvas
  ([syson-plugin](syson-plugin.md), [cameo-plugin](cameo-plugin.md)), in a notebook.
- **Collection.** Record every transition with its instant and payload, every assignment, into
  a table a script analyzes afterward; or a subset of them, chosen by kind and by element.
- **Steering.** Pause at a state or a node, inspect, inject a signal, resume — the debugger a
  notebook drives instead of a prompt.

The precedent that shows the developer experience wanted is the SCXML route: translate the
state machine to SCXML, run it on an SCXML engine, and attach that engine's listeners, each an
arbitrary function called on enter, exit and transition, free to call any service and to
redraw any diagram. The experience is right. The architecture is not the one to adopt, for
reasons given under [Alternatives considered](#alternatives-considered): it replaces the
semantics we verify with the ones the target engine has, covers only the state layer, and
leaves every other question this project answers — choice points, `explore`, `check`, `smt`,
the referees — on a runtime the listener cannot see. So the design below puts the listener on
*this* runtime, and gives it the same three things the SCXML listener had: the moment, the
element, and the values.

## What exists to build on

- **The trace is already an event stream, not a log.** `TraceRecord` (`internal/exec/runtime/trace.go`)
  carries a `TraceKind` — `transition`, `entry`, `exit`, `do`, `accept`, `send`, `choice`,
  `guard`, and `line` for the printed-only records — a `TraceOrigin` (the clock instant, the
  object whose behavior made it, that behavior), the state or the transition's `From`/`To`, the
  event and its typed `Payload`, and the `Target` of a send. `NewEventRecorder` already keeps the
  stream for queries (`DocumentQueries`, the checker's state stream) without printing it; the
  REPL's `%trace on` and the CLI's `-trace` print it. Every kind a listener needs to fire on is
  already recorded at the point it happens; what is missing is a *sink other than the slice*.
- **The configuration is already a fact the executor answers.** `StateExecutor.ActiveStates()`
  and `ActiveLeaves()` give the active-state set with its region structure (a composite with
  regions is active with one leaf per region — [orthogonal regions](orthogonal-regions.md));
  `CurrentTime()` the clock; the action executor its tokens and the nodes they sit on
  (`%tokens`). `Instance` feature values are what `%features` and `Instantiate` already serialize
  as `FeatureValue`.
- **The in-process session already has the verbs.** `client/opensysml/session.go` opens a
  `Session` over a parsed model and keeps clock, schedule and objects between `Instantiate`,
  `Send`, `Advance`, `Perform`, `Feature`, `SetFeature`, `Evaluate`, `ActiveStates`,
  `Transitions`, `Accepts`, answering fact-shaped values (`Acceptance`, `Advancement` with its
  `ChoicePoint`s, `Performance` with its `Branch`es). Surface parity's Stage 4 named this the
  in-process half of a wire session; this note designs the other half.
- **The call-out exists for one moment.** The `tool:<name>` engine
  ([analysis framework, External tools](analysis-framework.md#external-tools);
  [bring your own engine, Tools](bring-your-own-engines.md#tools-composed-invocations-and-structured-replies))
  spawns a process per performance of an action annotated `@ToolExecution`, hands it the
  inputs as JSON, binds its outputs. That is a listener that fires on exactly one kind of moment
  (an action's performance) and must answer (its outputs bind). The listener designed here
  generalizes the moment and relaxes the answer.
- **Metadata as the model-side declaration.** `Simulation::Configuration`, `AnalysisTooling::ToolExecution`,
  `AnalysisRecords::RecordedRun` all show the pattern this project uses for tool-facing
  declarations: a non-normative library package of `metadata def`s the model annotates with,
  read by the runtime, ignored harmlessly by a tool that does not know them.
- **The REPL's session rules.** A debugger session ends when the behavior it steps or the
  object it runs on is superseded by a submission (`dropStaleDebugSessions`); `%advance` of any
  session drives the whole runtime's clock; a `%state` on an object attaches to the machine
  that object exhibits. These are the lifecycle rules the wire session inherits rather than
  reinvents.

## The contract

Three things are fixed here, and the surfaces are then adapters over them: the **event**, the
**snapshot**, and the **listener**.

### Events

An `Event` is one `TraceRecord`, made public and given an identity:

```
Event {
  seq        : uint64            // position in the session's stream, dense, from 1
  at         : double            // the clock instant, seconds
  kind       : entry | exit | transition | do | accept | send | assign |
               token | choice | performance_start | performance_end |
               run_start | run_end | breakpoint
  object     : InstanceRef?      // the object whose behavior made it; absent when none
  behavior   : SymbolRef?        // the machine or action performing
  element    : SymbolRef?        // the state, transition, node or feature the event is about
  from, to   : SymbolRef?        // a transition's endpoints; a token's move
  event      : string            // the trigger, signal or operation name
  payload    : map<string, Value>
  target     : InstanceRef?      // a send's addressee
  value      : Value?            // an assign's new value
  choice     : ChoicePoint?      // a choice's record, as scheduling.md defines it
  note       : string            // free text, the printed line for a line record
}
```

Two kinds are new against `TraceKind`. `assign` is the write of a feature value, reported with
the feature (`element`) and the new `value`; today an assignment is a printed `line`, and the
step write ledger already notes it for `ChoiceWriteOrder`, so the record is a promotion, not a
new hook. `token` is one token's move onto a node, which the action executor reports through
`%tokens` today and prints under `%trace` as a line; promoting it gives the animation its live
action nodes. `performance_start`/`performance_end` bracket a `perform`ed action or a `do`
behavior; `run_start`/`run_end` bracket the session's run; `breakpoint` is the event a paused
session emits last before it waits. `line` records are not events: what prints and what is
observed are separated, as `NewEventRecorder` already separates them.

The `seq` is the session's, not the recorder's; a client that reconnects asks for the stream
from a `seq` and gets what the server still holds (bounded as the recorder's `limit` bounds
it), or a `gap` marker with the count dropped. Every event carries the clock instant because
the clock is the one order every consumer agrees on; `seq` is the order the run emitted them
in, which under one linearization is total.

### Snapshots

A `Snapshot` is what is, at a `seq`:

```
Snapshot {
  seq          : uint64
  at           : double
  objects      : [Instance]            // each with feature_values, as Instantiate returns
  machines     : [MachineState]        // one per running state machine
  actions      : [ActionState]         // one per live action performance
  queue        : [Pending]             // events and clock waits not yet dispatched
  paused_at    : Breakpoint?
}
MachineState {
  object       : InstanceRef?
  behavior     : SymbolRef
  active       : [ActiveState]         // the configuration
  status       : running | suspended | completed | terminated
}
ActiveState {
  state        : SymbolRef
  regions      : [ [ActiveState] ]     // one list per orthogonal region, empty for a leaf
  since        : double                // the instant it was entered
}
ActionState {
  object       : InstanceRef?
  behavior     : SymbolRef
  tokens       : [Token]               // node, data, waiting_on
  locals       : map<string, Value>
}
```

`active` is a tree, not a flat list, because a configuration is one: a composite state with
three regions is one `ActiveState` with three region lists, and a client that flattens it can;
a client given a flat list cannot rebuild the regions. `ActiveLeaves()` is the leaves of this
tree and `ActiveStates()` its nodes, so the executor already computes both. `since` is what an
animation needs to show dwell time and what a co-simulation needs to compute a duration
without subscribing to every entry.

A snapshot is answered on request (a unary call, or a `snapshot` command on the stream) and is
*not* sent with every event: a run that makes ten thousand transitions must not serialize ten
thousand copies of every object. A subscription may ask for a *delta* instead — the
`MachineState`s and objects an event changed — and a client that wants the snapshot on every
event asks for it, paying for it knowingly.

### Sessions

A `Session` is opened over a model hash and lives until closed, idle-evicted or the model is
evicted from the cache (surface parity's lifecycle rule). It owns one runtime `Context` — one
clock, one message bus, one set of objects — so several machines and actions run under it and
`Advance` moves them all, as `%advance` does in the REPL. Its verbs are the in-process
`Session`'s, with three additions:

| Verb | Answers | In-process today |
|---|---|---|
| `Instantiate(symbol)` | the object | yes |
| `Start(behavior, object?)`, `Perform(action, object?, inputs)` | the machine or performance started; a machine started runs from its entry, an action from its start node, both to their first wait | `Perform` runs to completion; `Start` is new |
| `Send(object, signal, args)` | the acceptance | yes |
| `Advance(seconds)`, `Step()`, `Continue()` | what ran: the events emitted, the choices drawn | `Advance` yes; `Step`/`Continue` are `%step`/`%continue` |
| `Feature`, `SetFeature`, `Evaluate`, `ActiveStates`, `Transitions`, `Accepts` | facts | yes |
| `Snapshot()` | the snapshot | new |
| `Break(element)`, `Unbreak(element)` | the breakpoint set on a state or node by symbol id; the session pauses before entering it or before a token performs it | new (REPL: `%break`) |
| `Subscribe(filter, from_seq?)` | the event stream | new |
| `Close()` | — | yes |

`Subscribe` is a server stream; every other verb is unary. That is a deliberate departure from
Stage 4's "bidirectional streaming as the natural shape": the [transport evaluation](transport-evaluation.md)
§8 shows a browser can consume only server-streaming, and a Python notebook does not want a
command stream to write into — it wants to call `advance()` and have its callbacks fire. Unary
commands plus one server stream serve gRPC, Connect over HTTP/1.1 and the browser with the
same messages, and a bidirectional `Drive` stream can be added later as a multiplexing of the
same messages if a client turns out to need it.

A `filter` names event kinds, and optionally the objects, behaviors or elements (by symbol id,
with `**` for a subtree) the client wants; the default is every kind but `token`, which is
noisy and wanted by an action animation only. A `pause_on` in the filter is a list of kinds at
which the session pauses *after* emitting the event and waits for `Continue` — the listener's
way to hold the run while it makes a call whose answer the run needs, without setting
breakpoints element by element. Pausing has a deadline (the same idle bound as the session);
past it the session resumes and emits a `resumed_after_timeout` note, so an abandoned
subscriber cannot hold a runtime indefinitely.

The REPL's `%state`, `%action`, `%step`, `%advance`, `%send`, `%break`, `%current`,
`%tokens`, `%events` become one implementation of these verbs over an in-process session,
printing what they print today from the `Snapshot` and `Event` values; `%trace on` becomes a
subscription that prints. This is Stage 1 of surface parity applied to the debuggers: one
assembly, four surfaces.

### Listeners

A **listener** is a subscription with a body: a function called for each event that passes
the filter, allowed to read the snapshot, allowed to `Send` into the run, and — under
`pause_on` — allowed to hold the run until it returns. Two homes:

**In the client.** The Python client (and the Go and Java clients) get:

```python
with model.session() as s:
    car = s.instantiate("Demo::Car")
    s.start("Demo::Car::modes", car)

    @s.on("entry", element="Demo::Car::modes::Heating")
    def heating(ev, snap):
        r = requests.post(THERMAL, json={"setpoint": snap.feature(car, "setpoint")})
        s.send(car, "Demo::PlantReply", {"temp": r.json()["temp"]})

    @s.on("transition")
    def redraw(ev, snap):
        canvas.highlight(snap.machines[0].active)

    s.advance(30.0)
```

`on(kind, element=..., object=..., behavior=..., pause=False)` registers a callback; the client
keeps one `Subscribe` stream per session and dispatches. `pause=True` sets `pause_on` for that
kind and `Continue`s when the callback returns; an exception in a paused callback still
`Continue`s (the run is not left hung by a client bug) and is re-raised to the caller of
`advance()` afterward. A callback that calls `send()` posts onto the bus like any external
sender: the message is *accepted after it was sent*, at the clock instant the run is at, so it
is ordered by the library's one rule and is not a hidden choice point.

**In the model.** A non-normative library package `Observation` declares the metadata by which a
model names its listeners, in the pattern of `Simulation::Configuration` and `ToolExecution`:

```sysml
library package Observation {
    enum def Moment { entry; exit; transition; do; assign; performance; }

    metadata def Listener {
        attribute on   : Moment[1..*];
        attribute tool : String;            // a tool name the OPENSYSML_TOOLS manifest resolves
        attribute hold : Boolean[0..1];     // pause the run until the tool replies (default false)
        attribute send : String[0..1];      // a signal the tool's reply is delivered as
    }
}
```

```sysml
state def Modes {
    @Observation::Listener { on = Moment::entry; tool = "thermal"; hold = true; send = "PlantReply"; }
    state heating;
    ...
}
```

The annotated element is the filter (`element` = the state, `on` = the kinds); the `tool` is
resolved through the engines manifest exactly as `tool:<name>` resolves one, and is invoked
with the `Event` and the `Snapshot` as JSON on stdin (the `invocation` shape of bring-your-own
engine's version-2 entries, with `event` and `snapshot` fields beside `inputs`). Its reply,
when `send` names a signal, is delivered as that signal's payload to the annotated element's
object; otherwise its reply is recorded as a `line` and discarded. `hold` is `pause_on` for
that element. This is the "listener declared with a language keyword" of the SCXML precedent
done without a keyword: the language is unchanged, a tool that does not know `Observation`
reads an ordinary metadata usage, and the model can be run with the listeners live or ignored
(`-listeners off`) without editing it.

A model-declared listener is not a *replacement* for a client callback; it is the same
subscription made by the runtime on the model's behalf, over the same process protocol the
`tool:` engine speaks. A notebook that wants Python instead of a subprocess uses `on()`; a
model that must carry its co-simulation binding with it uses `@Listener`; both go through one
`Subscribe`.

### What a listener may do to the run

The runtime's standing with the referees (PSSM, the pilot evaluator, the corpora) rests on the
run being the runtime's. A listener must not become a second scheduler. So:

- A listener **observes** through the stream and the snapshot; both are read-only views.
- A listener **acts** only by `Send` (and, explicitly, `SetFeature`), each of which is an
  external occurrence the run already admits — a signal from outside, a value set from
  outside — and each of which is itself recorded as an event (`send` with `object` absent and a
  `note` naming the subscription; `assign` likewise), so a trace with a listener attached shows
  exactly where the outside touched the run.
- A listener that **holds** the run holds the clock: nothing due is dispatched until it
  `Continue`s. Holding is at an event boundary, never inside a run-to-completion step, so the
  configuration a held listener reads is a stable one (RTC's atomicity is kept).
- A listener cannot change the **schedule**: it cannot pick which of two enabled transitions
  fires or reorder regions. Those remain choice points drawn by the policy; the listener sees
  them as `choice` events and, wanting another, runs again under another policy.
- Under `explore`, `check` and `smt` the runtime runs the behavior many times or not at all as
  an interpreter; a listener is **not attached** to those engines' runs, and a
  `@Observation::Listener` with `send` set makes the outcome table *observed* rather than
  *checked* if it is attached — the same downgrade a `tool:` output causes, for the same reason
  (bring-your-own engine, [the standing of an external answer](bring-your-own-engines.md#the-standing-of-an-external-answer)).
  `run` (one linearization, the debugger's engine) is where listeners live.

The consequence that matters most for co-simulation: because a listener's `Send` is an
ordinary external send at the current instant, a co-simulated exchange is *reproducible* from
the trace. The trace records the send with its payload; replaying the trace's sends at their
instants without the external service reproduces the run. That is what the SCXML route could
not give — its listener's side effects were outside anything the engine recorded.

## Wire shape

The service gains a `Session` capability:

```proto
rpc OpenSession(OpenSessionRequest) returns (OpenSessionResponse);   // model_hash, schedule → session_id
rpc SessionCommand(SessionCommandRequest) returns (SessionCommandResponse);
rpc SessionSnapshot(SessionSnapshotRequest) returns (Snapshot);
rpc SessionSubscribe(SessionSubscribeRequest) returns (stream SessionEvent);
rpc CloseSession(CloseSessionRequest) returns (CloseSessionResponse);
```

`SessionCommandRequest` is a `oneof` over `Instantiate`, `Start`, `Perform`, `Send`, `Advance`,
`Step`, `Continue`, `SetFeature`, `Break`, `Unbreak`; `SessionCommandResponse` a matching
`oneof` over their fact-shaped answers plus the `[]Event` the command emitted (so a client
without a subscription still sees what an `Advance` did — the `%advance` printout). `Evaluate`,
`Feature`, `ActiveStates`, `Transitions`, `Accepts` are the existing unary RPCs taking an
optional `session_id`, so a stateless client and a session client share them.

`SessionEvent` is `oneof { Event event; Gap gap; Paused paused; Closed closed; }`.

`Value`, `Instance`, `FeatureValue`, `Diagnostic`, `ChoicePoint` are the existing messages;
the note adds `Event`, `Snapshot`, `MachineState`, `ActiveState`, `ActionState`, `Token`,
`Pending`. Session ids are opaque strings; a session is bound to the connection's model hash
and refused across hashes (`INVALID_ARGUMENT`), and `NOT_FOUND` after eviction. The server's
bounds — sessions open, idle life, events retained — are the model cache's, exposed under the
capability's reported limits so a client can read them rather than guess.

The Python client's `Session` wraps these; `Model.session()` opens one. The Go public package's
`opensysml.Session` becomes an interface with two implementations, in-process and dialed, as
surface parity foresaw; `Dial`'s `OpenSession` stops returning `CodeUnimplemented`.

## Test contract

- **The stream is the trace.** For every fixture the trace tests run (`trace_test.go`, the
  conformance cases), the events a subscription receives with `kind` in the printed kinds,
  rendered by `TraceRecord.Line`, equal the `-trace` output line for line. One recorder, two
  readers.
- **The snapshot is the executor.** After each `Advance` of the orthogonal-region and
  pseudostate fixtures, the `Snapshot.machines[i].active` tree flattened equals
  `ActiveStates()` and its leaves equal `ActiveLeaves()`; `objects` equals what `%features`
  prints.
- **A held listener does not move the clock.** A subscription with `pause_on = [entry]` on a
  timed machine: between the `entry` event and `Continue`, `Now()` is unchanged and no further
  event is emitted; after `Continue`, the run's remaining events equal the unheld run's.
- **A listener's send is an ordinary send.** A machine driven by a Python callback that
  `send`s on `entry` produces a trace whose `send` records, replayed as `%send`s at their
  instants with no callback attached, produce an identical `states_visited` and
  `final_context`. Under `explore` with the callback attached the answer is refused with the
  message that names why.
- **`@Observation::Listener` is `on()`.** A model annotating a state with a `tool` listener,
  the tool a test executable that echoes a fixed reply, produces the same event stream and
  final context as the same model with no annotation and a Python callback doing the same.
- **The REPL is unchanged to a user.** The REPL's existing debugger tests pass over the
  session-backed implementation with their expected output byte for byte.
- **Eviction and bounds.** A session past the idle bound is `NOT_FOUND`; a subscription past
  the retention bound receives a `Gap` whose count equals the recorder's `dropped`; a held
  session past the hold deadline resumes and notes it.
- **The referees are untouched.** The PSSM referee, the pilot execution referee and the corpus
  gates run with no session open and their committed counts do not move; a run with a
  read-only subscription attached produces the same referee verdicts as one without.

## Stages

1. **Promote the recorder.** `assign`, `token`, `performance_*`, `run_*` as `TraceKind`s; a
   `TraceSink` interface the recorder fans out to beside its slice; `Snapshot()` on `Context`
   assembled from `ActiveStates`/`ActiveLeaves`, the action executors' tokens and the instances.
   Tests: the stream-is-the-trace and snapshot-is-the-executor rows. No surface changes.
2. **One debugger assembly.** The REPL's `%state`/`%action` family re-implemented over the
   in-process `Session` extended with `Start`, `Step`, `Continue`, `Break`, `Snapshot`,
   `Subscribe`; the REPL prints from the values. Tests: the REPL byte-for-byte row.
3. **The wire session.** The five RPCs, the new messages, the session table with its bounds,
   the Python `Session` with `on()`. Tests: the held-listener, send-is-a-send, eviction rows;
   a notebook example under `examples/` that animates a state diagram and calls a stub service.
4. **Model-declared listeners.** The `Observation` library, its reading by the runtime into
   subscriptions, the tool invocation with `event`/`snapshot`, the `-listeners` flag. Tests:
   the `@Listener`-is-`on()` row and the `explore` refusal.
5. **Consumers.** The VS Code panel subscribes for live highlighting; the SysON and Cameo plans
   take `SessionSubscribe` as the run-feedback channel their notes leave open.

Stage 1 is the load-bearing one and is independently useful (a `-trace-json` flag falls out of
it for free). Stages 3 and 4 are independent of each other once 2 is in.

## What this does not change

- The language. No keyword is added; `Observation` is metadata a conforming tool ignores.
- The semantics of a run. A listener cannot pick a linearization; the policies and choice
  points of [scheduling](scheduling.md) are what they were, and are visible to the listener as
  events.
- The stateless RPCs. `ExecuteAction`/`ExecuteState` keep answering results without a session.
- The REPL's incremental-declaration model. Surface parity's exclusion stands: a client that
  changes the model re-parses and opens a new session.
- The strength vocabulary. A listener that acts downgrades a claim exactly as a tool output
  does; one that only observes downgrades nothing.

## Alternatives considered

**Translate to SCXML and run on an SCXML engine, attaching that engine's listeners.** This is
the precedent that motivated the note, and it works well as a notebook experience. It is
rejected as the architecture for four reasons. (1) *Semantics.* SCXML's datamodel is untyped
scripting over a flat event queue; SysML v2's is typed feature values with units, objects with
parts and connectors that route signals, a clock with `accept at/after/when`, deferred events,
completion events ordered before time triggers, and the Kernel Semantic Library's partial
order — each is either lost in translation or reimplemented beside the engine, and every gap
is a translation bug no referee sees. (2) *Coverage.* It runs state machines. Actions, calcs,
analysis and verification cases, and the `explore`/`check`/`smt` engines run on one runtime
here; a listener architecture that covers only the state layer would leave the animation of an
action diagram, the value of a calc result, and the outcome table with no hook. (3) *The
referees.* PSSM, the pilot evaluator and the corpora check the runtime the user runs; a
translated model's behavior is checked against nothing. (4) *Reproducibility.* An SCXML
listener's side effects live outside anything the engine records; here a listener's `Send` is a
recorded external send, so a co-simulated run is replayable from its trace. What the SCXML
route got right — an arbitrary function at every enter/exit/transition, declared in the model,
free to call any service — is taken whole, on this runtime.

**Send the snapshot with every event.** Simplest for the client; ruinous for a long run over
many objects. Rejected in favor of snapshot-on-request plus optional deltas.

**Bidirectional stream as the only session shape.** Stage 4's first instinct. Rejected as the
*only* shape because a browser cannot open one and a notebook does not want one; kept as a
possible later multiplexing of the same messages.

**Listeners as a `TraceSink` in Go only, no wire.** Gives the VS Code extension nothing (it
talks to `sysml-lsp` over a protocol, not to a Go package), and gives the notebook nothing.
The Go sink is Stage 1; it is not the end.

**A new keyword (`when … listen …`) in the notation.** Would put an OpenSysML extension into
the grammar every other tool parses. The metadata route reads as a standard model everywhere and
is the pattern the project already uses for `Simulation::Configuration` and `ToolExecution`.
