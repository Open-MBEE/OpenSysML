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
  Time: 0.0
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

**Action debugging commands:**
- `%action <name> [<object>]` — Start an action debugging session, optionally performed by an instantiated object
- `%step` — Advance all tokens one step
- `%continue` — Run to completion, or to the first breakpoint hit
- `%tokens` — Show active tokens with data
- `%break <node>` — Set breakpoint on a named node, one an `if` branch or a loop body declares included; `%continue` stops when a token reaches it, or before a body performs it
- `%stop` — Stop debugging

**State machine debugging commands:**
- `%state <name> [<object>]` — Start a state machine debugging session; naming an instantiated object runs the machine on behalf of that object, so what it sends routes over that object's connections. Naming the machine the object exhibits attaches to its running machine instead (see [below](#an-object-runs-the-behaviors-its-type-exhibits))
- `%send <signal>[(<p>=<expr>, ...)] [to <object>]` — Send a signal to an object's machine over the runtime's message bus; by default to the object being debugged
- `%events` — Show event queue and signals in flight
- `%current` — Show current state, stack, data
- `%advance <time>` — Advance simulation time by `<time>` units, processing every event due
- `%stop` — Stop debugging

For complete workflows, see
[examples/action-executor-demo.sysml](../../examples/action-executor-demo.sysml),
[examples/orthogonal-regions-demo.sysml](../../examples/orthogonal-regions-demo.sysml) and
[examples/pseudostates-demo.sysml](../../examples/pseudostates-demo.sysml).

## When a model has more than one valid run

A behavior is a set of performances under a *partial* order, not a program with one next
instruction. The KerML Kernel Semantic Library orders three things and nothing else:

- **Successions.** `succession first a then b` is a `HappensBefore` link: `a` completes before `b`
  begins. A fork's branches all follow the fork; a join follows every branch into it.
- **Send before accept.** A message is accepted after it was sent, so an `accept` that waits for a
  `send` in another branch follows that send.
- **Ancestor priority.** When a substate's transition and its enclosing state's are both enabled
  by one event, the innermost fires — UML/SysML order, not a pick.

Everything else two performances could do in either order, they may: which of two fork branches
steps first (*token interleaving*), which of two holding guards a decision follows (*overlapping
guards*), which of two transitions out of one state fires on one event (*competing transitions*),
which of two orthogonal regions reacts first to an event both accept (*region order*), and whose
value stands when two branches assign one feature in one step (*same-step writes*). A model with
any of these has several valid runs, and a run that took one of them is not wrong for it — but a
tool that showed only that run, and called its result *the* outcome, would be. The rest of this
section is how the executor keeps that honest: it reports every such pick as a *choice point*,
lets you take another one (`seed:<n>`), lets you see them all (`explore`), and lets a test state
the whole set of outcomes it admits.

The examples below are one fixture from the conformance suite, three branches writing one feature
between a fork and a join,
[`action_explore_three_writers.sysml`](../../internal/core/runtime/testdata/conformance/action_explore_three_writers.sysml):

```sysml
package test {
	private import ScalarValues::*;

	action race {
		attribute x : Integer = 0;
		attribute aRan : Boolean = false;
		attribute bRan : Boolean = false;
		attribute cRan : Boolean = false;

		first start;
		fork split;
		action a { assign x := 1; assign aRan := true; }
		action b { assign x := 2; assign bRan := true; }
		action c { assign x := 3; assign cRan := true; }
		join sync;
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}
}
```

The library fixes that `split` precedes each branch, that `sync` follows all three, and that each
branch runs once; it does not fix the order of the three writes to `x`. Six orders, three
values, all valid.

### Reading a choice point

Where the executor has to pick, it follows one fixed rule — reverse token order, first holding
guard, first declared transition, so a run replays exactly — and records the pick rather than
passing it off as the only outcome. A plain run ends with a count:

```console
$ sysml -action test::race action_explore_three_writers.sysml
✓ package test
✓ Started action executor for "test::race"
  State: Running
  Tokens: 1
✓ Action completed
  Final state: Completed
  2 choice points; %trace on to see them
  Results:
    aRan = true
    bRan = true
    cRan = true
    x = 1
```

The same line closes `%step`, `%continue` and `%advance` in the REPL, and reads `2 choice points`
without the hint once `%trace on` is showing them. Under `-trace` (or `%trace on`) each choice
is a `choice` line naming what was open, every alternative, and the one taken:

```console
$ sysml -trace -action test::race action_explore_three_writers.sysml
…
[trace] step 2: token 2@a, token 3@b, token 4@c
…
[trace] choice step 3: writes x := 1 by token 2, x := 2 by token 3, x := 3 by token 4 (unordered; x := 1 by token 2 stood)
[trace] choice step 3: tokens 2@a, 3@b, 4@c (unordered; took 4@c first)
[trace] step 3: token 2@sync, token 3@sync, token 4@sync
[trace] step 4: token 5@done
[trace] step 5: no active tokens
```

`tokens 2@a, 3@b, 4@c` names the tokens by id and node; `took 4@c first` is the reverse-order
rule. The write line lists each token's last write to `x` and which one stood — `x = 1`, because
token 2 stepped last. The other kinds read the same way: a decision with two holding guards is
`choice step 2: decision select branches 1->warn, 2->alarm hold (unordered; took 1->warn)`, two
transitions out of one state enabled by one event are `choice state idle on accept Go: transitions
1->left, 2->right (unordered; took 1->left)`, and two regions reacting to one event are
`choice on accept Go: states a1, b1 react (unordered; took b1 first)` — that last kind only under
`explore`, since the fixed policies all take declaration order. Over gRPC and
Connect the same choice is an informational diagnostic with code `choice-point` and the message
`choice point: step 3: tokens 2@a, 3@b, 4@c (unordered; took 4@c first)`, placed at the node,
decision, feature or state that made it — a finding about the run, never an error.

Reporting never changes the run. Once a guard or transition holds, the ones after it are read in
a preview that is undone, and one that cannot be evaluated there — a division by zero, say — is
not an alternative and not an error: a guard with no result is not true, so its branch is not
taken. It is counted beside the choices (`1 guard not evaluable`), traced as
`unevaluable guard step 2: decision select branch 2->alarm: division by zero (not selected)`, and
carried as a diagnostic with code `guard-unevaluable`. The first guard read is the run's own, and
its failure fails the run as it always has. And the case the library does order is not reported:
a substate's transition outranking its enclosing state's makes no `choice` line under any policy.

### Taking another linearization: `seed:<n>`

The fixed rule is one *scheduling policy*, named `reverse`, and the executor can be told to
resolve every choice point under another. `declared` takes tokens in the order they were spawned
and guards and transitions in declaration order; `seed:<n>` draws each pick from a pseudo-random
sequence the non-negative integer `n` fixes, so `seed:1` replays the same run every time and on
every platform while `seed:2` may take another linearization:

```console
$ sysml -schedule seed:1 -action test::race action_explore_three_writers.sysml
✓ package test
✓ Started action executor for "test::race"
  State: Running
  Tokens: 1
✓ Action completed
  Final state: Completed
  2 choice points; %trace on to see them
  Results:
    aRan = true
    bRan = true
    cRan = true
    x = 2
```

The policy is spelled the same everywhere — `sysml -schedule` for `-action`, `-state` and
`-analysis` (a calc's body performs nothing, so `-calc` has no choice to make), `%schedule seed:7`
in the REPL for the runs started after it (a debugging session under way keeps the policy it
started with, and its own notes, budget and calc memo, while another run is driven in between), a
`schedule` field on the gRPC execution requests, and a `schedule` pin on a conformance case — and
it changes only which alternative each choice takes. Every choice point the run reaches is still
reported, and each `took …` is what the named policy took. Another linearization can reach other
choice points — which tokens are steppable in a step depends on the order the earlier ones moved —
so the count is not fixed across policies, only the reporting is. A guard the policy picks past
the first was only previewed, so the run reads it once more for real before taking its branch (the
trace shows that reading), as a transition's guard is always read again as it fires. An unknown
spelling — `random`, `seed` without a number, `seed:-1` — is refused before anything runs rather
than falling back to the default.

Use a seed when one other linearization is what you want: to reproduce a run a colleague saw,
to check a fix against the order that exposed the bug, or to pin a conformance case to a
linearization other than the default's. It shows one run per seed, and says nothing about the
runs no seed you tried happened to take.

### Seeing the whole outcome set: `explore`

`explore` replays the behavior once per linearization. The first run records the alternative
taken at each choice point; each later run is a fresh executor of the same loaded model — no
object, message, clock, calc memo or note carries over — that follows the recorded prefix and
takes the next untried alternative at the frontier, depth-first, until every choice sequence is
spent or a budget is hit:

```console
$ sysml -schedule explore -action test::race action_explore_three_writers.sysml
✓ package test
✓ explored test::race: 3 outcomes
outcome                                      | linearizations | witness
---------------------------------------------+----------------+------------------------------------------------------------------
aRan = true; bRan = true; cRan = true; x = 1 | 2              | step 3: 3@b first of 2@a, 3@b, 4@c; step 3: 4@c first of 2@a, 4@c
aRan = true; bRan = true; cRan = true; x = 2 | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 3: 4@c first of 3@b, 4@c
aRan = true; bRan = true; cRan = true; x = 3 | 2              | step 3: 2@a first of 2@a, 3@b, 4@c; step 3: 3@b first of 3@b, 4@c
complete (6 runs)
```

Runs that agree on what the conformance harness compares — an action's outputs; a state machine's
final state, states visited and values; an analysis case's outputs and verdicts — are one
*outcome*, and the table has one sorted row per distinct outcome: the outcome, how many
linearizations reached it, and the choice sequence of one *witness* run (`3@b first of 2@a, 3@b,
4@c` is the first pick, then `4@c first of 2@a, 4@c` among the two that remained). Six
linearizations, three outcomes, two each; `complete (6 runs)` says every choice sequence was
tried. A run that fails under some order is an outcome of its own (`error: …`), not the end of
the exploration; a behavior with no choice point explores in exactly one run (`no choice points`
in the witness column); the same model explores to the same table every time. With `-trace`, the
table is followed by the trace of each outcome's witness run (`trace of outcome 1's witness
(run 4):`). With `-json`, each check carries `outcomes` (values, `linearizations`, `witness`) and
`exploration` (`complete`, `runs`, `budgetsHit`) beside the table's lines.

The budget is 1024 runs and 64 choice points per run unless `explore:runs=N,depth=D` says
otherwise, and hitting it is never silent:

```console
$ sysml -schedule explore:runs=2 -action test::race action_explore_three_writers.sysml
✓ package test
? explored test::race: 2 outcomes
outcome                                      | linearizations | witness
---------------------------------------------+----------------+------------------------------------------------------------------
aRan = true; bRan = true; cRan = true; x = 2 | 1              | step 3: 2@a first of 2@a, 3@b, 4@c; step 3: 4@c first of 3@b, 4@c
aRan = true; bRan = true; cRan = true; x = 3 | 1              | step 3: 2@a first of 2@a, 3@b, 4@c; step 3: 3@b first of 3@b, 4@c
incomplete: runs budget 2 hit after 2 runs
$ echo $?
2
```

`incomplete` names each budget hit (`runs` before `depth`), the table is what was reached so
far and no more, the check is unresolved (`?`) and the exit status is `2` — the status of a run
that decided nothing, as for an unevaluable verdict. Raise the budget it names
(`explore:runs=4096`, `explore:depth=128`, or both) and run again; a model whose exploration stays
incomplete at any budget you can afford has more linearizations than a table can carry, and a
seed is the way to look at some of them.

The same spelling explores over the wire, where the response carries `outcomes` and an
`exploration` status ([wire contract](../reference/wire-contract.md)), and from every client
([clients](09-clients.md)). The REPL refuses it, because its `%action` and `%state` debuggers
step one run and an exploration replays from the start:

```console
sysml> %schedule explore
error: explore replays a behavior from the start once per linearization, which %action and %state, stepping one run, cannot do: run `sysml -schedule explore -action <name>` (or -state, -analysis, -calc), or a request with schedule "explore"
```

### Writing a test that admits several outcomes

A conformance case (see the
[conformance README](../../internal/core/runtime/testdata/conformance/README.md)) that pins one
outcome of a model with choice points pins the default policy's linearization, which is fine when
that is what you mean. When the model admits several, say so with three things beside the
`.sysml`:

**`outcomes` + `admissible` in the `.expected.json`** — the complete set of admissible outcomes,
each written in full (an outcome is never "anything"), and the title of the section of
[the semantic oracle](../project/behavior-semantic-oracle.md) that derives the set from the
library. This is the fixture's own `action_explore_three_writers.expected.json`:

```json
{
	"type": "action",
	"trace": true,
	"outcomes": [
		{
			"outputs": {
				"x": {"type": "Integer", "value": 1},
				"aRan": {"type": "Boolean", "value": true},
				"bRan": {"type": "Boolean", "value": true},
				"cRan": {"type": "Boolean", "value": true}
			}
		},
		{
			"outputs": {
				"x": {"type": "Integer", "value": 2},
				"aRan": {"type": "Boolean", "value": true},
				"bRan": {"type": "Boolean", "value": true},
				"cRan": {"type": "Boolean", "value": true}
			}
		},
		{
			"outputs": {
				"x": {"type": "Integer", "value": 3},
				"aRan": {"type": "Boolean", "value": true},
				"bRan": {"type": "Boolean", "value": true},
				"cRan": {"type": "Boolean", "value": true}
			}
		}
	],
	"admissible": "Three concurrent writers of one feature: six orders, three values"
}
```

`outcomes` replaces `outputs` (or `finalState`, `stateVisits`, `performers` for a state case); a
case may not have both, may not list one outcome, and must cite a section the oracle has. The
harness checks the default run's outcome is exactly one listed member, then explores the case and
fails on a listed outcome no linearization reached (`admissible outcome 2 of 3 is unreachable`),
on a reached outcome the list omits, and on a budget hit — telling you to raise it with
`"exploreBudget": {"runs": N, "depth": D}` in the same file. So the list is exact, not a lower
bound: the three outcomes above are exactly what the six runs of the table reach.

**A `.trace.order` file** — the partial order the library does fix, as `earlier < later`
constraints over trace labels: the first trace entry mentioning `a` comes strictly before the
first mentioning `b` (blank lines and `#` comments are skipped). The harness checks the trace
against them beside the exact golden, so a fixture states what must hold without pinning what may
vary. `action_explore_three_writers.trace.order`:

```text
# The fork precedes every branch; the join waits for all three.
split < a
split < b
split < c
a < sync
b < sync
c < sync
```

**A `.trace.golden`** — with `"trace": true`, the default policy's trace is recorded as usual, so
the linearization the default takes is still pinned exactly (deterministic replay is a feature)
while the `outcomes` say it is one of three. The suite also runs every case under `declared` and
`seed:1`, and a case with `outcomes` gets a `.declared.trace.golden` and a `.seed-1.trace.golden`
of its own.

Which openness a fixture makes observable, and which it leaves to a single-outcome case, is
recorded per fixture in [the semantic oracle](../project/behavior-semantic-oracle.md); the
[compliance table](../project/spec-compliance.md) names the code and tests behind each surface
above.

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
has — and for a [trade study](#trade-studies), which of the listed alternatives the model selects.

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

### Trade studies

A trade study is an analysis case the library defines (`TradeStudies::TradeStudy`, SysML v2
§7.22): its `subject studyAlternatives : Anything[1..*]` lists the alternatives, its
`evaluationFunction` is a calc scoring one of them, and its `tradeStudyObjective` — a
`MinimizeObjective` or `MaximizeObjective` — states which score is best. The library writes the
rest itself: the objective's `best` is `alternatives->minimize {in x; eval(x)}` (or `maximize`),
its requirement is `eval(selectedAlternative) == best`, and the case returns
`studyAlternatives->selectOne {in ref a {} tradeStudyObjective(selectedAlternative = a)}`. A model
supplies the alternatives, the scoring calc and the direction:

```sysml
package Trade {
    private import ScalarValues::*;
    private import TradeStudies::*;
    part def Engine { attribute mass : Real; }
    part a : Engine { attribute :>> mass = 30.0; }
    part b : Engine { attribute :>> mass = 10.0; }
    part c : Engine { attribute :>> mass = 10.0; }
    analysis lightest : TradeStudy {
        subject : Engine[1..*] = (a, b, c);
        objective : MinimizeObjective;
        calc :>> evaluationFunction {
            in part e :>> alternative : Engine;
            return :>> result : Real = e.mass;
        }
        return part :>> selectedAlternative : Engine;
    }
}
```

`%analysis` and `-analysis` run it as they run any case — nothing about the run is specific to
trade studies. The `evaluationFunction` is a calc held as a value, bound into the objective's
`in calc :>> eval` and applied by `eval(x)` inside the library's `minimize`/`maximize` and
`selectOne` bodies; the objective's `require constraint` is inherited from the library and
checked as any objective's is. What is new in the report is the evaluations the run made: each
application of the case's own calc as a value, in subject order, with what it computed and
whether its alternative is the one the case returned:

```bash
$ sysml -analysis Trade::lightest trade.sysml
✓ package Trade
✓ Trade::lightest
  selectedAlternative = Trade::b (object #2)
  objective tradeStudyObjective: satisfied
  evaluationFunction(Trade::a (object #1)) = 30.0
  evaluationFunction(Trade::b (object #2)) = 10.0 [selected]
  evaluationFunction(Trade::c (object #3)) = 10.0 [tied]
```

`b` and `c` score the same. The library's `selectOne` is `select {…}#(1)`, the *first* element
the predicate holds for, so `b` is the pick and the objective is satisfied — and `c` is marked
`[tied]` so the tie is visible rather than a silent first-wins. `%trace on` shows the same order:
the subject bound, `minimize` applying `evaluationFunction` to each alternative in turn, `best`
read, then `selectOne` checking each alternative's `eval(selectedAlternative) == best`.

An `evaluationFunction` that fails for one alternative — a division by zero, a feature with no
value — is an evaluation reported with its error, the alternatives before it keep their values,
nothing is selected, and the objective is `undecided` naming the failure; the run fails with the
same message. One declared without a body (`abstract calc evaluationFunction`, or a redefinition
whose nested rollup calcs bind no result) fails the same way at the first alternative, naming the
calc that has no return expression, so a study that states no way to score its alternatives is
never answered with a fabricated pick. A subject listing no alternative, or one redeclared as
`Engine[1]` and bound to several, is a `multiplicity violation` against the library's `[1..*]`
before any alternative is evaluated; a subject restating no multiplicity (`subject : Engine =
(a, b);`) inherits `[1..*]` from `studyAlternatives` (KerML §7.3.4.5) and runs.

Swept (`%sweep`, `-sweep`, `-samples`), each row carries that run's evaluations beside its
outputs and verdict, so a table shows where the pick changes as a parameter moves — and a row
whose run failed keeps the evaluations it made before failing:

```
%sweep Trade::weighted powerWeight=0.0..1.0:0.5
sweep Trade::weighted — 3 run(s)
powerWeight | selectedAlternative        | verdict                        | evaluations                                                                                                            | time
------------+---------------------------+--------------------------------+------------------------------------------------------------------------------------------------------------------------+--------
0.0         | Trade::light (object #2)  | tradeStudyObjective: satisfied | evaluationFunction(Trade::strong (object #1)) = -30.0; evaluationFunction(Trade::light (object #2)) = -10.0 [selected] | 3.053ms
0.5         | Trade::strong (object #1) | tradeStudyObjective: satisfied | evaluationFunction(Trade::strong (object #1)) = 120.0 [selected]; evaluationFunction(Trade::light (object #2)) = 15.0  | 0.215ms
1.0         | Trade::strong (object #1) | tradeStudyObjective: satisfied | evaluationFunction(Trade::strong (object #1)) = 270.0 [selected]; evaluationFunction(Trade::light (object #2)) = 40.0  | 0.153ms
```

The run and `%optimize` answer different questions. The run is the model's own answer: it
evaluates every alternative the subject lists and reports the one the library's `selectOne`
picks, with no solver. `%optimize` asks a solver for the best *values* a case's conditions admit
over a continuous domain — it is for an objective whose `eval` states an expression over the
case's parameters, not for choosing among listed alternatives — and on a trade study whose
objective applies the case's `evaluationFunction` it refuses, pointing here:

```
%optimize Trade::lightest
error: analysis lightest: objective tradeStudyObjective not optimizable: it applies the calculation `evaluationFunction` to each alternative the case's subject lists, a choice among listed alternatives rather than an optimum over a continuous domain (run the trade study as an analysis (`-analysis`, `%analysis` or RunAnalysis), which evaluates every alternative and reports the one selected) at trade.sysml:9:9
```

The evaluations cross `-json` as each check's `evaluations` and the gRPC API as
`RunAnalysisResponse.evaluations` and each `SweepRow.evaluations` under the `case_evaluations`
capability, so a client reads the same per-alternative table
([reference/wire-contract.md](../reference/wire-contract.md#case-evaluations)).

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
