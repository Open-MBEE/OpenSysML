# 6. Behavior: actions and state machines

Actions and state machines are executed, not just parsed. A debugger steps through
them, and the non-interactive `-action` and `-state` flags run them to completion and report the
values they produce. A behavior can be performed by an object, in which case the messages it sends
are routed over that object's connections.

**Action execution (step-by-step):**
```sysml
sysml> action SimpleWorkflow {
  ...>     attribute result = 0;
  ...>     first start;
  ...>     then action compute { assign result := 42; }
  ...>     then done;
  ...> }
✓ action SimpleWorkflow

sysml> %action SimpleWorkflow
✓ Started action executor for "SimpleWorkflow"
  State: Running
  Tokens: 1

Use %step to advance, %tokens to inspect, %continue to run to completion

sysml> %step
✓ Step complete
  State: Running
  Tokens: 1

sysml> %tokens
Active tokens (1):
  Token 1 @ compute
  Values:
    result = 0

sysml> %continue
✓ Action completed
  Final state: Completed
  Results:
    result = 42
```

**State machine execution:**

A machine completes when a transition reaches `done`, the terminal state the standard
library provides for every state machine. Entering it runs the exit actions, and then the
machine reports itself completed. With orthogonal regions, each region has its own `done`,
and the machine completes only once every region has reached it.

```sysml
sysml> state TrafficLight {
  ...>     entry; then start;
  ...>     state start;
  ...>     state green { accept after 25 [SI::s] then yellow; }
  ...>     state yellow { accept after 5 [SI::s] then red; }
  ...>     state red { accept after 30 [SI::s] then done; }
  ...>     succession first start then green;
  ...> }
✓ state TrafficLight

sysml> %state TrafficLight
✓ Started state machine executor for "TrafficLight"
  Current state: start
  Time: 0.0
  Events: 1

Use %events to see queue, %current for state, %advance <time> to step

sysml> %advance 25
✓ Advanced to 25.0 (2 event(s) processed)
  Current state: yellow
  Last event at: 25.0
  Remaining events: 1

sysml> %current
Current state: yellow
Time: 25.0
Last event at: 25.0
Execution state: Running

sysml> %advance 5
✓ Advanced to 30.0 (1 event(s) processed)
  Current state: red
  Last event at: 30.0
  Remaining events: 1

sysml> %advance 30
✓ Advanced to 60.0 (1 event(s) processed)
  Current state: done
  Last event at: 60.0
  Remaining events: 0

✓ State machine completed (a transition reached `done`)
```

**Sending a signal.** A transition that waits on an `accept` is driven from the prompt with
`%send`, which puts the signal on the runtime's message bus exactly as a `send` from an action
would, so nothing has to be written in the model just to fire it:

```sysml
sysml> package Lamps {
  ...>     private import ScalarValues::*;
  ...>     attribute def go;
  ...>     attribute def Dim { attribute level : Integer; }
  ...>     state def Lamp {
  ...>         attribute brightness : Integer = 0;
  ...>         entry; then off;
  ...>         state off;
  ...>         transition off_on first off accept go then on;
  ...>         state on;
  ...>         transition on_dim first on accept d : Dim do assign brightness := d.level then dimmed;
  ...>         state dimmed;
  ...>     }
  ...>     part def Bulb { exhibit state lamp : Lamp; }
  ...>     part bulb : Bulb;
  ...> }
✓ package Lamps

sysml> %instantiate bulb
✓ Created instance of Lamps::bulb
  ID: 1
  Use %features bulb to inspect

sysml> %state bulb
✓ Debugging state machine "lamp" exhibited by object #1 of "Lamps::bulb"
  Current state: off
  Time: 0.0
  Events: 0

sysml> %send go
✓ Sent go to object #1 of "Lamps::bulb"
  Accepted by state machine "Lamp" in state off

Use %step or %advance <time> to dispatch it

sysml> %events
Signals in flight: 1
  go
Use %advance <time> to process next event

sysml> %advance 1
✓ Advanced to 1.0 (1 event(s) processed)
  Current state: on
  Last event at: 0.0
  Remaining events: 0

sysml> %send Dim(level=3+4)
✓ Sent Dim(level=7) to object #1 of "Lamps::bulb"
  Accepted by state machine "Lamp" in state on

sysml> %step
✓ Event dispatched
  Current state: dimmed
  Time: 1.0
  Events: 0

sysml> %send go
error: object #1 of "Lamps::bulb" accepts no signal go now: state machine "Lamp" in state dimmed
```

Without `to <object>`, the signal goes to the object whose machine the `%state` session is
debugging (`%send go to bulb` names it explicitly, and is the form to use when no session is
active; the object is any object reference, `to #1` or `to rack.lamp` included). Payload features
are written `<parameter>=<expression>` as for `%invoke`, and are checked against the signal's
declaration: `%send Dim(lvl=1)` is refused because `Dim` carries no `lvl`. A
signal nothing in the machine's current state accepts is refused up front, with the state named,
rather than queued to be silently dropped — and so is one whose every triggered transition is
held back by its guard, decided as the dispatch would decide it, with the payload bound: with
`transition on_dim first on accept d : Dim if d.level > 0 ...`, `%send Dim(level=0)` is refused
while `%send Dim(level=3)` is in flight. A guard that cannot be evaluated is a `%send` error. If
the state or the data a guard reads changes between the send and the dispatch, the `%step` or
`%advance` that drops the signal says so.

**One clock.** Simulation time belongs to the runtime the session executes in, not to any one
behavior: every action and state machine the session runs — the ones `%action` and `%state`
debug, the ones instantiated objects exhibit, the ones a body performs — reads and waits on the
same clock, which starts at `0.0` and counts in `SI::s`. An action body waits on it as a
transition does: `accept after 5 [SI::s]` parks the token until the clock has moved five
seconds past the moment the accept was reached, and `accept at t` (a `Time::TimeInstantValue`)
until the clock reads `t` — an instant already passed is due at once. A duration or instant
that is not a time (`accept after 5 [SI::m]`) is refused when the accept is reached, as it is in
a transition. `%step` does not move the clock, so a token waiting only on time is reported as
such, with what would move it; `%advance <time>` moves the clock of the session's runtime by
`<time>` seconds, and everything due along the way runs, whichever debugger it belongs to:

```sysml
sysml> package Timed {
  ...>     item def Ping;
  ...>     action pinger {
  ...>         attribute count = 0;
  ...>         first start;
  ...>         then action wait accept after 5 [SI::s];
  ...>         then action tick assign count := count + 1;
  ...>         then send new Ping() to listener;
  ...>         then done;
  ...>     }
  ...>     state listener {
  ...>         entry; then idle;
  ...>         state idle;
  ...>         transition idle_pinged first idle accept Ping then pinged;
  ...>         state pinged;
  ...>     }
  ...> }
✓ package Timed

sysml> %action Timed::pinger
✓ Started action executor for "Timed::pinger"
  State: Running
  Tokens: 1

sysml> %step
✓ Step complete
  State: Running
  Tokens: 1

sysml> %step
Nothing to step: the action waits on the clock, which %step does not move
  Token 1: accept after waiting since step 2 for the clock to reach t=5.0
  Use %advance 5.0 to move the clock from t=0.0 to the earliest wait

sysml> %state Timed::listener
✓ Started state machine executor for "Timed::listener"
  Current state: idle
  Time: 0.0
  Events: 0

sysml> %advance 2
✓ Advanced to 2.0 (0 event(s) processed)
  Current state: idle
  Last event at: 0.0
  Remaining events: 0
  Action state: Waiting
  Tokens: 1
  Waiting on the clock:
    t=5.0: action pinger, accept after waiting since step 2 for the clock to reach t=5.0

sysml> %advance 3
✓ Advanced to 5.0 (1 event(s) processed)
  Current state: pinged
  Last event at: 5.0
  Remaining events: 0
  Action state: Completed
  Tokens: 0
  Action steps taken: 4

✓ Action completed
  Results:
    count = 1
```

An advance runs what comes due in the order it comes due; at one instant, each executor runs its
own work in the order it always has (a machine dispatches its due event and runs its do behavior,
an action moves its tokens), and which *executor* goes first when several are due at the same
instant is a *choice point* (below). Once the definite work at an instant has
settled, the change conditions state machines and action tokens watch (`accept when`) are polled,
so a condition another executor has just made true fires at that instant — also for the executor
driving the clock itself, and whatever timer it has waiting beside the condition. Which watcher
polls first is the same choice point, as what one does on its condition is what the next sees. The advance stops early, saying
so and how to raise the bound, when it exhausts the event, do-step or step budget
([environment](../reference/environment.md)); a wait due after the deadline stays queued and
is listed under `Waiting on the clock`; an advance with nothing waiting just moves the clock.
`%continue` runs an action to completion on its own, moving the clock to each of its waits as it
reaches them, and moving with it every other behavior of the same runtime that comes due.

**Action debugging commands:**
- `%action <name> [<object>]` — Start an action debugging session, optionally performed by an instantiated object
- `%step` — Advance all tokens one step; a token waiting only on the clock is reported with the `%advance` that would move it
- `%continue` — Run to completion, or to the first breakpoint hit
- `%tokens` — Show active tokens with data
- `%break <node>` — Set breakpoint on a named node, one an `if` branch or a loop body declares included; `%continue` stops when a token reaches it, or before a body performs it
- `%stop` — Stop debugging

**State machine debugging commands:**
- `%state <name> [<object>]` — Start a state machine debugging session; naming an instantiated object runs the machine on behalf of that object, so what it sends routes over that object's connections. Naming the machine the object exhibits attaches to its running machine instead (see [below](#an-object-runs-the-behaviors-its-type-exhibits))
- `%send <signal>[(<p>=<expr>, ...)] [to <object>]` — Send a signal to an object's machine over the runtime's message bus; by default to the object being debugged
- `%events` — Show event queue and signals in flight
- `%current` — Show current state, stack, data
- `%advance <time>` — Advance the runtime's simulation clock by `<time>` seconds, running every state event, action token, change-condition poll and do behavior due along the way, in every debugging session of the runtime
- `%stop` — Stop debugging

**Choice points.** The library orders some things and leaves others open: a succession says
which step comes first, but nothing says which of two fork branches steps first, which of two
waiting accepts takes the one message both answer to, which of two holding guards a decision
follows, which of two transitions out of one state fires on the same event, whose write
stands when several branches assign one feature in one step, or which of two executors — an
action token and a state transition, two state machines, two actions — due at the same instant
of the clock runs first. Where the
executor has to pick, it follows one fixed rule — reverse token order, first holding guard, first
declared transition, the executor created last first, so a run replays exactly — and records a
*choice point* rather than passing
the pick off as the only outcome. `%step`, `%continue` and `%advance` end with a count of the
choices they made (`2 choice points; %trace on to see them`), `%trace on` shows each as a
`choice` line naming the alternatives and the one taken (`choice step 3: tokens 2@left, 3@right
(unordered; took 3@right first)`; `choice at t=5.0: due action watcher, state machine blinking
of object #1 (unordered; ran state machine blinking of object #1 first)`), and the gRPC responses
carry each as an informational diagnostic. One executor alone due at an instant is not a choice
and is not reported, so a model with a single behavior runs and traces exactly as it did before
the clock was shared. A run with no choice points has the one outcome the model states; one with choice
points has the outcome this executor's rule produces, and the lines say where another rule would
diverge. The innermost-transition-wins rule between a substate and the state enclosing it is
spec-defined order, not a choice, and is not reported. Reporting never changes the run: once a
guard or transition holds, the ones after it are read in a preview that is undone, and one that
cannot be evaluated there — a division by zero, say — is not an alternative and not an error (a
guard with no result is not true, so its branch is not taken); it is counted beside the choices
(`1 guard not evaluable`) and shown in the trace as an `unevaluable guard` line. The first guard
read is the run's own, and its failure fails the run as it always has.

**Scheduling policies.** The fixed rule is one *scheduling policy*, named `reverse`, and the
executor can be told to resolve every choice point under another: `declared` takes tokens in the
order they were spawned, guards and transitions in declaration order and executors due together
in the order they were created, and `seed:<n>` draws each
pick from a pseudo-random sequence the non-negative integer `n` fixes, so `seed:1` replays the same
run every time and on every platform while `seed:2` may take another linearization. The policy is
spelled the same everywhere — `sysml -schedule declared` for `-action`, `-state` and `-analysis`
(a calc's body performs nothing, so `-calc` has no choice to make), `%schedule seed:7` in the
REPL for the runs started after it (a debugging session under way keeps its own), a `schedule`
field on the gRPC execution requests, and a `schedule` pin on a conformance case — and changes
only which alternative each choice takes: every choice point the run reaches is reported, and
each `took …` is what the named policy took, so running a model under two policies and comparing
the outcomes is how a scheduling artefact is told from a bug. A guard the policy picks past the
first was only previewed, so the run reads it once more for real before taking its branch (the
trace shows that reading), as a transition's guard is always read again as it fires. Another
linearization can reach
other choice points — which tokens are steppable in a step depends on the order the earlier ones
moved — so the count is not fixed across policies, only the reporting is. An unknown spelling —
`random`, `seed` without a number, `seed:-1` — is refused before anything runs rather than falling
back to the default. Where the library orders the alternatives
— the innermost transition over its enclosing state's — there is no choice, and every policy
follows that order.

For complete workflows, see
[examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml),
[examples/orthogonal-regions-demo.sysml](../../examples/orthogonal-regions-demo.sysml) and
[examples/pseudostates-demo.sysml](../../examples/pseudostates-demo.sysml).

## An object runs the behaviors its type exhibits

A type that exhibits a state machine or performs an action binds that behavior to every object of
the type: instantiating an object gives it an execution of its own, tied to its identity. Two
objects of the same type run two independent machines, each with its own current state, event
queue and feature values, and an assignment in a behavior body writes the feature value of the
object performing it.

```sysml
sysml> part def Monitor {
  ...>     attribute count = 0;
  ...>     exhibit state modes {
  ...>         entry; then idle;
  ...>         state idle {
  ...>             entry action bump { assign count := count + 1; }
  ...>             accept after 10 [SI::s] then awake;
  ...>         }
  ...>         state awake { entry action mark { assign count := count + 10; } }
  ...>     }
  ...>     action bumpBy { in n; action apply { assign count := count + n; } first apply; then done; }
  ...> }
✓ part def Monitor

sysml> %instantiate Monitor
✓ Created instance of Monitor
  ID: 1
  Use %features Monitor to inspect

sysml> %state Monitor
✓ Debugging state machine "modes" exhibited by object #1 of "Monitor"
  Current state: idle
  Time: 0.0
  Events: 1

sysml> %step
✓ Event dispatched
  Current state: awake
  Time: 10.0
  Events: 0

sysml> %features Monitor
Instance: Monitor (ID: 1)
Features:
  count = 11
Behaviors:
  modes: exhibited state machine, current state awake
  bumpBy: action, not running
```

`%instantiate` started the machine, and `%state Monitor` attached the debugger to *that object's*
machine rather than to a detached run of the usage. `%step`, `%advance`, `%current` and `%events`
therefore drive that machine, and `%features` shows the values its entry actions wrote: `1` from
`idle`, then `10` more from `awake` once the timer fired. The machine and the operation are not
values the object holds, so they are listed under `Behaviors:` with what the object is doing with
each: the exhibited machine's current state is the one `%current` reports, and `bumpBy`, which the
type declares but does not perform, is not running.

The two-argument form does the same when the machine it names is the one the object exhibits:
`%state Monitor::modes Monitor` attaches to the running machine and says so in a `note:` line,
rather than performing `modes` a second time against the same feature values (which would run
its entry actions again, leaving `count` at `2` instead of `1`). Only a machine the object does
not exhibit — one it merely performs — is started as a detached performance by that form. When
the object exhibits one definition as several usages (`exhibit state front : Blink; exhibit
state rear : Blink;`), naming the definition names no one machine, so `%state Blink lamp` refuses
and names `Lamp::front` and `Lamp::rear` to name instead.

Naming the machine alone attaches the same way when one held object exhibits it: `%state modes`
(or `%state Monitor::modes`) after `%instantiate Monitor` drives that object's running machine,
so what its `do` and `entry` actions write shows up in `%features Monitor`. It never performs
the machine detached from its object: with no object exhibiting it (`%state modes` before any
`%instantiate`), or several (a second `Monitor` held as a part of another object), `%state`
refuses and names the objects — or, before any exists, the type exhibiting the machine — and you
name one with `%state <object>` or `%state <machine> <object>`. Only a `state def` no type
exhibits is started as a detached performance by that form.

The object can also be a part reached through composition, or an id. With `part def Driver {
part r : Monitor; }`, `part driver : Driver;` and `%instantiate driver`, `%state driver.r` debugs the nested part's own
machine, and `%state #2` the same by the id `%features driver` prints for it (`r =
Instance(ID: 2)`). A path that stops short of an object says which segment failed, in the words
every command uses for an [object reference](../reference/repl-commands.md#object-references):

```sysml
sysml> %state driver.x
error: driver has no feature "x" (its features are r, and 13 more the library declares)
```

Naming a usage whose *definition* alone was instantiated is reported as such, with what to
instantiate instead. With `part monitor : Monitor;` declared:

```sysml
sysml> %instantiate Monitor
sysml> %state Monitor::modes monitor
error: no instance of the usage "monitor": object #1 of "Monitor" is of its definition "Monitor", not of the usage — use %instantiate monitor to create the usage's object, or name Monitor to address it
```

**When a machine starts, and how far it runs.** The object's feature values are built and its
constant defaults evaluated first, so an entry action sees the declared initial values. The
machine is then initialized and run until it is *quiescent*: no event is due at the current
time, no do action can run, and no message is in flight. A machine waiting on a timer or an
`accept` is quiescent, and advancing time is what lets it proceed. Objects that signal one
another are run together until they all settle, within the event and do-step budgets described in
[reference/environment.md](../reference/environment.md); an exchange that never settles reports a
budget error rather than hanging. Instantiating the same name twice creates a second object with
its own identity and its own machines: `%instantiate` reports the new object, and the name then
refers to it, while the first object keeps running and is still addressed by its id
(`%state #1`, `%invoke #1 bumpBy n=4`, `%features #1`; see
[addressing an object](04-repl.md#addressing-an-object)). A `%state` or `%action` session
started on the first object stays with it — it now knows the object as `#1` — and it ends only if
that object is later dropped. A machine a nested part exhibits is
debugged by naming that part through its owner, `%state Monitor.sensor` or `%state #1.sensor`.
An exhibited machine with no initial state is reported as such. A performed action
that declares no flow has nothing to step, but the object is still created. A performed action
waiting at an `accept` is also quiescent, and a message sent later by a sibling object
wakes it up.

**Editing the model while an object runs.** Submitting an unrelated declaration keeps the object
(its identity survives the rebuilt analysis) but not the execution it was running. An execution
belongs to the graph, names and message bus of the analysis it started in, so the object's
behaviors are **restarted from their initial states** in the rebuilt analysis, and any values the
discarded run wrote are dropped with it. The restart is reported (`note: the exhibited state machine
modes of object #1 was restarted from its initial state because the model was rebuilt`), and a
`%state` session follows the object onto its restarted machine, so a restarted behavior exchanges
messages with objects instantiated after the edit in the usual way. Redeclaring what the object
runs (its type's features, or the body of a machine or action it runs) produces a different
object, so the original is dropped with a stated reason and `%instantiate` creates a new one.

**Invoking an operation.** `%invoke <object> <op> [<p>=<expr>]` runs an action owned by the
object's type, performed by that object — named as `%state` names one, so `%invoke driver.r bumpBy
n=4` and `%invoke #2 bumpBy n=4` reach a nested part:

```sysml
sysml> %invoke Monitor bumpBy n=4
✓ Invoked bumpBy on object #1 of "Monitor"

sysml> %features Monitor
Instance: Monitor (ID: 1)
Features:
  count = 15
Behaviors:
  modes: exhibited state machine, current state awake
  bumpBy: action, not running
```

Each argument is written as `<parameter>=<expression>`. An unbound parameter, an argument that
names no parameter, and an operation the type does not own are each reported as errors. A `calc`
or `constraint` cannot yet be invoked this way.

## Running an analysis case

An analysis case is a case, a case is a calculation, and a calculation is an action (SysML v2
§7.22, §7.21, §7.19), so running one is performing an action whose result is its `out` and
`return` parameters. `%analysis` and `-analysis` run a case the way `%calc` invokes a calc:
the case's `subject` is an `in` parameter the usage binds (`subject s = ship;`) or the object
named after the case supplies, the other `in` parameters take arguments in parentheses,
positionally or by name, or their declared defaults, and the report lists what the case computed
with its units and, after that, the verdict of its `objective`.

```sysml
package test {
    private import ScalarValues::*;
    part def Ship { attribute cost : Real = 5.0; }
    part ship : Ship;

    analysis steps {
        subject s = ship;
        action a {
            out p : Real;
            assign p := s.cost + 1.0;
        }
        then action b {
            in q : Real = a.p;
            out w : Real;
            assign w := q * 10.0;
        }
        out total : Real = b.w + a.p;
        return : Real = b.w;
    }
}
```

```bash
$ sysml -analysis test::steps steps.sysml
✓ package test
✓ test::steps
  total = 66.0
  result = 60.0
```

The body's `action` steps run through the same executor `%action` debugs: `then` sequences them,
each later step reading an earlier one's output by `step.pin`, and a body that states no
succession performs its steps in declaration order, as a calc body does. A nested `analysis`
usage is a step too; one that binds no subject of its own runs on the enclosing case's subject,
and the enclosing case reads its outputs as features of it (`out total : Real = inner.m * 2.0;`).
The `out` parameters and `return` are evaluated in the case's frame after the steps complete, so
they may read the subject, the `in` parameters, the steps' outputs and call `calc def`s.
`%trace on` shows the order: the case is entered, its subject bound, each step performed as an
action node, then `return` and every `out` evaluated.

The `objective` is a requirement the case frames; it is evaluated, not executed. After the body
runs, its `require` and `assume` constraints are checked against the case's results by the same
engine `%requirement` uses, and the verdict is printed beside them: `satisfied`, `not satisfied`
with the condition that failed, or `undecided` with the reason a condition could not be evaluated
(a feature the condition reads has no value). An `assert constraint { ... }` inside the body is
checked the same way, against the values the run bound once its steps have completed. `%optimize`
still asks a solver which
values would make the objectives best; `%analysis` reports what they are for the values the model
has.

An objective typed by a requirement definition binds that definition's `subject` as a requirement
usage does, in every spelling — `objective : MassLimit { subject = ship; }`, `subject s = ship;`
or `subject :>> s = ship;` — and the binding may read the case's subject, its `in` parameters and
locals, and its steps' outputs — a nested case's (`subject = inner.picked;`) or an action's
(`subject = weigh.m;`). The case's own result, named or not,
is readable by its qualified name as the OMG examples write it — `subject = MassCase::result;` in
the objective, `MassCase::result < limit` in an `assert constraint`, `inner.result` from the case
performing `inner` as a step. The qualifier names whose result it is: `MassCase::result` (or
`Cases::Case::result`) is the running case's, while a sibling usage's `light::result` is the
sibling's own run, never the running case's value. An objective that binds no subject takes the
library's default for it: the case's result (`Cases::Case::obj` declares `subject subj default
Case::result`, SysML v2 §7.22). So an objective typed by `MassLimit` in a case that `return`s a `Ship` checks the ship
returned, while in a case that returns a `Real` it is `undecided`, saying so: `subject s defaults
to the case's result (Cases::Case::obj): type mismatch: 1000.0 (a Real) is not a Ship`. The result
must also fit the subject's multiplicity: one `Ship` for a `subject pair : Ship[2]` is `undecided`
as a `multiplicity violation` (an objective redeclaring the subject without one, `subject :>> pair;`,
keeps the `[2]`). A case that returns nothing leaves such a subject unbound, and the
verdict says to bind it or return a result. Bound either way, an object is held as a value of the
subject, so one declared a `Ship` and bound to a `subject t : Tanker` is a `Tanker` for the
conditions, its `cargo` answering `t.cargo`, exactly as a requirement usage's `subject = ship;`
holds it; a value the subject cannot hold at all (a `Buoy` for a `Tanker`) is refused as a
`type mismatch` before any condition is read, and an expression yielding more or fewer values
than the subject declares (one `Ship` for a `subject pair : Ship[2]`, or none) as a
`multiplicity violation`, just as the default is. The object a satisfaction assertion supplies
with `by` is held to the subject's declaration the same way: classified by its type, refused as a
`type mismatch` where it cannot be, and as a `multiplicity violation` where one object is too few.

```sysml
requirement def MassLimit {
    subject s : Ship;
    attribute limit : Real = 2000.0;
    require constraint { s.hullMass < limit }
}
analysis def MassCheck {
    subject ship : Ship;
    objective : MassLimit { subject = ship; }
    return r : Real = ship.hullMass;
}
analysis light : MassCheck { subject ship = O::ship; }
```

```bash
$ sysml -analysis O::light mass.sysml
✓ package O
✓ O::light
  r = 1000.0
  objective obj: satisfied
```

A case usage owned by a part definition (`part def Holder { analysis inner : CostAnalysis {
subject s = h; } part h : Ship; }`) is a feature of every object of that type, as a `calc` usage
owned by a part is: `holder.inner.total` runs the case on first read and keeps the result until a
value it depends on changes, and an attribute redefined from an analysis output (`attribute :>>
fuelEconomy = cityAnalysis.fuelEconomyResult;`) evaluates through the same path.

What stops a case is reported as an error naming it, never as a silent empty result: a
`subject` nothing binds (`analysis An::CostAnalysis: s subject is unbound: bind it
(`subject s = <element>`) or run it on an object`), an `in` parameter with neither argument nor
default, a step that fails, a body that deadlocks or exhausts the step budget, and a case whose
body runs itself. A case that recurses without bound — through a nested `analysis` step that
performs its own definition, or a `calc def` through a `calc` usage member typed by itself — hits
the calc depth limit, and the error collapses the repeated frames to one line as `-calc`'s does:
`analysis An::rec: node again: analysis An::Rec::again: … 9999 frames: node again: calc recursion
limit exceeded: calc An::Rec::again nested 10000 deep (unbounded recursion?; raise
OPENSYSML_MAX_CALC_DEPTH to allow more)`.

### Verification cases

A verification case body uses the same grammar and runs the same way: `%analysis` and `-analysis`
accept a `verification def` or `verification` usage, bind its `subject` and `in` parameters as they
bind an analysis case's, run its body over the same action graph, and check its `objective` and
`assert constraint`s afterwards. Beside those verdicts they report the `VerdictKind` the body
produced, which is what running the body answered:

```sysml
verification def SpeedCheck {
    subject lander : Lander;
    objective { verify touchdown; require constraint { lander.verticalSpeed <= 1.5 } }
    VerificationCases::PassIf(lander.verticalSpeed <= 1.5)
}
verification checkSlow : SpeedCheck { subject lander = L::slowLander; }
```

```bash
$ sysml -analysis L::checkSlow landing.sysml
✓ package L
✓ L::checkSlow
  result = VerdictKind::pass
  objective obj: satisfied
  ✓ Verification L::checkSlow verdict: pass
```

The two verdicts are independent: an objective stating no condition of its own —
`objective { verify touchdown; }` alone — stays `undecided` and leaves the case unresolved, while
the body verdict beside it still reports what the body answered.

A body whose result is a `VerificationCases::PassIf(...)` call is `pass` or `fail` as that library
calculation computes it; one binding `verdict` to a `VerdictKind` literal reports that literal; one
producing no verdict value is `inconclusive`; and one whose run could not be carried out is `error`
carrying the same message the run failed with. Each nested `verification` step is reported on its
own line, marked `(subcase)`, since the library states no roll-up of a subcase's verdict into its
parent's.

`%requirement`, `%satisfy`, `-requirement` and `-satisfy` report those verdicts beside their own,
and their own verdict is unchanged: the requirement engine still decides whether the requirement is
satisfied, and a failing verification body does not make a satisfied requirement violated.

`%sweep` runs the case once per value of a range rather than once, and `%samples <n> <seed>`
draws that many values from it instead — the same tables `-sweep` and `-samples` print. Each row
is an ordinary run of the case with the swept parameter bound to that row's value, so it reports
what that run computed and how long it took:

```
%instantiate An::ship
%sweep An::CostAnalysis An::ship limit=10.0..30.0:10.0
sweep An::CostAnalysis — 3 run(s)
limit | total | verdict                   | time
------+-------+---------------------------+--------
10.0  | 12.0  | affordable: not satisfied | 0.510ms
20.0  | 12.0  | affordable: satisfied     | 0.022ms
30.0  | 12.0  | affordable: satisfied     | 0.012ms

%samples 3 42 An::CostAnalysis An::ship limit=10.0..30.0
samples An::CostAnalysis — 3 run(s), seed 42
limit              | total | verdict                   | time
-------------------+-------+---------------------------+--------
26.509450139960897 | 12.0  | affordable: satisfied     | 0.016ms
10.856399027228605 | 12.0  | affordable: not satisfied | 0.015ms
25.521460994239078 | 12.0  | affordable: satisfied     | 0.013ms
```

The endpoints are expressions evaluated where the session evaluates one, units included, and a
calc is swept the same way (`%sweep An::Sum(2.0) b=0.0..10.0:2.5`). Several ranges run their
cartesian product, a failed run is a row of the table rather than the end of it, and
[reference/repl-commands.md](../reference/repl-commands.md) states each refusal. Sampling is
uniform over the range — the bundled library defines no probability distribution, so a
distribution asked for by name is refused naming what is missing.

## Token-flow patterns

Each model below is in
[examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml), and the output
shown is from
`sysml -action ActionExecutorDemo::<name> examples/action-executor-demo.sysml`.

### Sequential: start → action → done

The sequential flow below uses standard implicit succession notation:

```sysml
action sequential {
    attribute result : Integer = 0;

    first start;
    then action compute { assign result := 42 * 2; }
    then done;
}
```

A single token is created at `start`, moves to `compute`, which runs its body, and is consumed at
`done`:

```
$ sysml -action ActionExecutorDemo::sequential examples/action-executor-demo.sysml
✓ Action completed
  Final state: Completed
  Results:
    result = 84
```

---

### Fork and join: parallel paths

`fork` and `join` are action node literals, so this action is standard notation.

```sysml
action forkJoin {
    attribute task1 : Integer = 0;
    attribute task2 : Integer = 0;
    attribute task3 : Integer = 0;

    first start;
    fork split;
    action left { assign task1 := 10; }
    action middle { assign task2 := 20; }
    action right { assign task3 := 30; }
    join sync;

    succession first start then split;
    succession first split then left;
    succession first split then middle;
    succession first split then right;
    succession first left then sync;
    succession first middle then sync;
    succession first right then sync;
    succession first sync then done;
}
```

`fork` places a token on each outgoing succession. `join` is an AND-join: it waits for a token
on *every* incoming succession before a single token continues. A fork duplicates control, not
values: all three branches are steps of the same performance, so every assignment is visible
when it completes.

```bash
$ sysml -action ActionExecutorDemo::forkJoin examples/action-executor-demo.sysml
✓ Action completed
  Final state: Completed
  Results:
    task1 = 10
    task2 = 20
    task3 = 30
```

If a branch never arrives, that is a deadlock rather than a failure, and the run is reported
as undecided.

---

### Decision and else: conditional branching

`decide` with a guarded branch and an `else` branch is standard notation. The
OpenSysML spelling `decision` is the one that produces a warning.

```sysml
action conditional {
    attribute x : Integer = 15;
    attribute taken : Integer = 0;

    first start;
    action pathA { assign taken := 1; }
    action pathB { assign taken := 2; }

    succession first start then check;
    succession first pathA then done;
    succession first pathB then done;

    decide check;
    if x > 10 then pathA;
    else pathB;
}
```

`decide` evaluates its guards in the order written, with the action's features in scope, and takes
the first guard that holds. The `else` branch is taken when no guard holds. With `x = 15`:

```bash
$ sysml -action ActionExecutorDemo::conditional examples/action-executor-demo.sysml
✓ Action completed
  Final state: Completed
  Results:
    taken = 1
    x = 15
```

Setting `x` to `5` gives `taken = 2`. The state-machine counterparts (orthogonal
regions, choice and junction) appear in
[examples/orthogonal-regions-demo.sysml](../../examples/orthogonal-regions-demo.sysml) and
[examples/pseudostates-demo.sysml](../../examples/pseudostates-demo.sysml), and every case the
executors are tested against lives under `internal/core/runtime/testdata/conformance/`.

A run that stops early, whether through deadlock or by hitting a budget, is reported as an
undecided check rather than a failure. The budgets are documented in
[reference/environment.md](../reference/environment.md).

---

Next: [7. Saving, and converting to RDF](07-saving-and-rdf.md).
