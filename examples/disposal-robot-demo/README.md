# Bomb-disposal robot demo

[`robot.sysml`](robot.sysml) is one bomb-disposal robot modelled far enough to
drive every part of OpenSysML from a single file: its structure, the behavior it
performs, the budget a solver can answer about, and the views it is presented
through. This walkthrough runs it from the CLI, then from the REPL, then from
Python, and every output below is what the commands print.

The solver sections need `z3` or `cvc5` on `PATH` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional).
`%optimize` needs z3 specifically.

The four packages:

| Package | What it holds |
| --- | --- |
| `Robot` | the platform: battery, power pack, mobility with four wheels, mast and camera, manipulator and gripper, computer, radio, the power line and data link between them, and two robots of it |
| `RobotBehavior` | three calculations, the `DisposalRun` action a call-out runs, the `Modes` state machine, and a controller that exhibits it and logs its video passes |
| `RobotSolver` | the call-out energy budget, a call-out nobody can plan, three variation points, and three optimization cases |
| `RobotViews` | one view per rendering kind, a viewpoint the robots are checked against, and a view reaching only the critical parts |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/disposal-robot-demo/robot.sysml -validate
```

```
✓ package Robot
✓ package RobotBehavior
✓ package RobotSolver
✓ package RobotViews
✓ examples/disposal-robot-demo/robot.sysml: no errors
```

**Plan the approach.** `-calc` invokes a calculation with positional arguments:
the distance from the staging point to the device, and what driving it costs at
the robot's draw.

```bash
./bin/sysml examples/disposal-robot-demo/robot.sysml \
  -calc "RobotBehavior::Approach(120.0, 90.0)" \
  -calc "RobotBehavior::energyForDrive(150.0, 0.42, 120.0)"
```

```
✓ RobotBehavior::Approach(120.0, 90.0)
  = 150.0
✓ RobotBehavior::energyForDrive(150.0, 0.42, 120.0)
  = 11.904761904761907
```

**Run the call-out.** `DisposalRun` forks the survey and the retrieval, joins
them, tallies what the call-out cost, and streams video home only if the power
budget left room for it. The retrieval branch is a single node stating a flow of
its own — the gripper closes, then the device is stowed — so the join waits for
that whole flow, not merely for the node to be entered.

```bash
./bin/sysml examples/disposal-robot-demo/robot.sysml -action RobotBehavior::DisposalRun
```

```
✓ Action completed
  Final state: Completed
  Results:
    armFrames = 3
    chargeAvailable = 1200.0
    chargeLeft = 1040.0
    framesHeld = 15
    handlingCost = 120.0
    streamed = 15
    surveyCost = 40.0
    surveyFrames = 12
```

**Run the modes.** The transitions are timed, so `-advance` says how much
simulated time to run for; without it only the initial transition is taken.

```bash
./bin/sysml examples/disposal-robot-demo/robot.sysml -state RobotBehavior::Modes -advance 40
```

```
✓ Advanced to 40.0 (4 event(s) processed)
  Current state: done
✓ State machine completed (a transition reached `done`)
```

Add `-instantiate RobotBehavior::controller -state "RobotBehavior::Modes controller"`
to run the machine as that object's own, and `-trace` to see every step.

**Check what holds.** A verdict is about an object, so instantiate the robot the
question is about; `-satisfy=<name>` evaluates the assertions one element states.

```bash
./bin/sysml examples/disposal-robot-demo/robot.sysml \
  -instantiate Robot::heavyMockup -constraint Robot::Platform::withinMassBudget
./bin/sysml examples/disposal-robot-demo/robot.sysml -satisfy=RobotSolver::plans
```

```
✗ Constraint Robot::Platform::withinMassBudget failed (on Robot::heavyMockup ID: 1)
  Assertion evaluated to false: mass <= 400.0
✓ satisfy runIsBudgeted by run12 holds (on RobotSolver::run12 ID: 1)
✗ satisfy runIsBudgeted by run13 fails (on RobotSolver::run13 ID: 2)
  Required condition evaluated to false: plan.driveEnergy + plan.toolEnergy + plan.housekeepingEnergy <= 1200
```

A failing check exits nonzero, so these run in CI as they stand.

**Render the views.** `-render` writes one view in the form its `render` member
states, `-render-form` asks for another, and `-render-all` writes every view of
the model into a directory.

```bash
./bin/sysml examples/disposal-robot-demo/robot.sysml -render RobotViews::interfaces
./bin/sysml examples/disposal-robot-demo/robot.sysml -render RobotViews::partsTable -render-form text
./bin/sysml examples/disposal-robot-demo/robot.sysml -render-all rendered
```

```
%% RobotViews::interfaces — interconnection rendering (render asInterconnectionDiagram)
flowchart LR
  subgraph n0 ["part def Robot::Platform"]
    n3["part battery (Battery)"]
    n5["part mobility (Mobility)"]
    …
  end
  n3 ---|"drivePower"| n5
  n6 ---|"imageryLink"| n8
  n3 -.->|"of Charge"| n5
  n6 -.->|"of Frame"| n8
```

```
wrote rendered/RobotViews.interfaces.mmd (mermaid, 549 bytes)
wrote rendered/RobotViews.overview.mmd (mermaid, 914 bytes)
wrote rendered/RobotViews.overview.interfaceSubview.mmd (mermaid, 565 bytes)
wrote rendered/RobotViews.modes.mmd (mermaid, 631 bytes)
wrote rendered/RobotViews.run.mmd (mermaid, 678 bytes)
wrote rendered/RobotViews.partsTable.md (markdown, 835 bytes)
wrote rendered/RobotViews.criticalParts.mmd (mermaid, 878 bytes)
```

**Identify elements with an OSLC query.** `-query` takes
[OSLC Query text](../../docs/reference/oslc-query.md), either as a bare
where-clause or as a URL-encoded parameter string, and reports one element per
line as qualified name, metamodel type, and the properties `oslc.select` names.

```bash
./bin/sysml examples/disposal-robot-demo/robot.sysml \
  -query 'oslc.where=rdf:type%3Dsysml:PartUsage+and+sysml:multiplicityUpper%3E1&oslc.select=sysml:type,sysml:multiplicityUpper'
./bin/sysml examples/disposal-robot-demo/robot.sysml \
  -query 'oslc.where=sysml:owner%3D%22Robot::Platform%22&oslc.orderBy=-sysml:name&oslc.select=sysml:name'
```

```
Robot::Mobility::wheels  PartUsage  type=Robot::Wheel  multiplicityUpper=4
```

```
Robot::Platform::withinMassBudget  ConstraintUsage  name=withinMassBudget
Robot::Platform::radio  PartUsage  name=radio
Robot::Platform::pack  PartUsage  name=pack
…
Robot::Platform::arm  PartUsage  name=arm
```

A value is a literal, so a qualified name needs its quotes:
`sysml:owner="Robot::Platform"`, not `sysml:owner=Robot::Platform`. A query
matching nothing says `no elements matched` on standard error, so the result
rows that follow the validation report on standard output remain one line per
match.

## From the REPL

```bash
./bin/sysml
```

```
%load examples/disposal-robot-demo/robot.sysml
```

### The robot as an object

`%instantiate` builds the object and starts the behaviors its type exhibits;
`%features` shows what it holds, down to the four wheels and both ends of every
connection.

```
%instantiate Robot::fielded
%features Robot::fielded
%eval in Robot::fielded : mass
```

### Stepping the call-out

`%action` starts a debugging session, `%tokens` shows where the tokens are,
`%step` advances one step and `%continue` runs to the end.

```
%action RobotBehavior::DisposalRun
%tokens
%step
%continue
```

```
Active tokens (1):
  Token 1 @ start
  Values:
    chargeAvailable = 1200.0
    …
✓ Action completed
  Results:
    chargeLeft = 1040.0
    framesHeld = 15
    streamed = 15
```

`%break <node>` stops at a node — `%break sync` to see the join wait for both
branches, one of them still inside the retrieval flow:

```
%action RobotBehavior::DisposalRun
%break sync
%continue
%tokens
```

```
Active tokens (2):
  Token 2 @ sync
  Token 3 @ grip
  Values:
    armFrames = 0
    chargeAvailable = 1200.0
    handlingCost = 0.0
    streamed = 0
    surveyCost = 40.0
    surveyFrames = 12
```

### Driving the machine of an object

`%instantiate` starts the machine the controller exhibits, `%state <part>` binds
to that object's own running machine, and `%advance` dispatches the timed
triggers. `%current` shows the active configuration, which is nested while the
robot is approaching.

```
%instantiate RobotBehavior::controller
%state RobotBehavior::controller
%advance 5
%current
```

```
✓ Advanced to 5.0 (1 event(s) processed)
  Current state: rolling
Current state: rolling
State stack (active configuration):
  0. approach
  1. rolling

State data:
  faults = 0
  framesCaptured = 0
  odometer = 21.0
```

Entering `rolling` entered `approach` around it and ran the entry action that
turned the odometer. Carry on to the end of the call-out, then call an operation
of the object with `%invoke`:

```
%advance 10
%advance 20
%current
%invoke RobotBehavior::controller recordPass frames=15
%features RobotBehavior::controller
%eval in RobotBehavior::controller : modes.odometer
%eval in RobotBehavior::controller : log.frames
```

```
✓ State machine completed (a transition reached `done`)
✓ Invoked recordPass on object #1 of "RobotBehavior::controller"
Instance: RobotBehavior::controller (ID: 1)
Features:
  odometer = 0.0
  framesHeld = 15
  log = Instance(ID: 3)
    frames = 15
    passes = 1
  modes = Instance(ID: 2)
    odometer = 21.0
    framesCaptured = 12
    faults = 0
    idle = <unknown>
    approach = Instance(ID: 4)
      rolling = <unknown>
      holding = <unknown>
    handling = <unknown>
    safing = <unknown>
    idle_to_rolling = <unknown>
    rolling_to_holding = <unknown>
    rolling_to_safe = <unknown>
    approach_to_handling = <unknown>
  recordPass = Instance(ID: 5)
    frames = <unset>
    store = <unknown>
✓ modes.odometer (on RobotBehavior::controller ID: 1)
  = 21.0
✓ log.frames (on RobotBehavior::controller ID: 1)
  = 15
```

An entry action writes the attribute of the machine it is declared in, not the
like-named one of the object exhibiting it. `%current` reports the executor's
current state data, while `%features` and `%eval` read the same value through
the materialized `modes` performance occurrence. The controller's distinct
`odometer` remains `0.0`.

`recordPass` writes two things: the controller's own tally, and — through the
chain its assignment names — the `log` part beside it.

A write has to fit what it is written to, so the wrong kind of value is a refusal
rather than a stored surprise — and the refusal comes from analysis, before
anything runs. Editing `store` to say `assign framesHeld := "fifteen";` and
reloading reports:

```
examples/disposal-robot-demo/robot.sysml:345:38: error: cannot bind String value to a feature typed by Integer
                assign framesHeld := "fifteen";
                                     ^~~~~~~~~
```

### Evaluating and solving the call-out budget

`%constraint` and `%satisfy` evaluate what *does* hold of an object; `%check` and
the commands after it ask the solver what *can* hold.

```
%calc RobotBehavior::streamable 15 8.0
%instantiate RobotSolver::run12
%constraint RobotSolver::run12::fitsTheBattery
%satisfy RobotSolver::plans
```

`%check` answers whether the whole budget can hold at all, and `%solve` fills in
a plan that satisfies it:

```
%check RobotSolver::RunBudget
%solve RobotSolver::RunBudget
```

```
✓ Requirement RunBudget is satisfiable (z3, 8ms)
  RobotSolver::RunBudget::'plan.driveEnergy' = 200
  RobotSolver::RunBudget::'plan.housekeepingEnergy' = 300
  RobotSolver::RunBudget::'plan.toolEnergy' = 150
✓ Requirement RunBudget has values satisfying it (z3, 7ms)
  Synthesised:
    RobotSolver::RunBudget::'plan.driveEnergy' = 200
    RobotSolver::RunBudget::'plan.housekeepingEnergy' = 300
    RobotSolver::RunBudget::'plan.toolEnergy' = 150
  One witness: a solver may answer with any of the assignments that satisfy it.
```

`OverbookedRun` asks for more than the battery holds, so `%explain` names the
conditions that conflict, minimally:

```
%check RobotSolver::OverbookedRun
%explain RobotSolver::OverbookedRun
```

```
✗ Requirement OverbookedRun is unsatisfiable: 4 conditions conflict (z3, 33ms)
  Every condition below is needed: dropping any one leaves the rest satisfiable.
  1. required condition: `plan.driveEnergy + plan.toolEnergy + plan.housekeepingEnergy <= 1200` …
  2. required condition: `plan.driveEnergy >= 600` …
  3. required condition: `plan.toolEnergy >= 500` …
  4. required condition: `plan.housekeepingEnergy >= 300` …
```

`%configure` asks which robots the build constraint permits — one consistent
selection, a caller-specified selection, or all of them:

```
%configure RobotSolver::robotFamily::buildIsConsistent
%configure RobotSolver::robotFamily::buildIsConsistent tool=gripper
%configure RobotSolver::robotFamily::buildIsConsistent all
```

```
✓ the chosen variants are consistent with Constraint buildIsConsistent (z3, 9ms)
  Already fixed:
    RobotSolver::robotFamily::tool = …::tool::gripper  (chosen)
  Synthesised:
    RobotSolver::robotFamily::uplink = …::uplink::radio
    RobotSolver::robotFamily::wheels = …::wheels::titanium
✓ Constraint buildIsConsistent permits 5 selections of variants, which are all of them (z3, 12ms)
```

Choosing the gripper forced the radio uplink and the heavier wheels: of the eight
builds the three variation points spell, five are consistent.

`%optimize` improves an analysis case's objectives, lexicographically when it
states several:

```
%optimize RobotSolver::BestRun
%optimize RobotSolver::LightestRobot
%optimize RobotSolver::FarthestThenTool
```

```
✓ Analysis BestRun is optimized (z3, 8ms)
  maximize mostToolTime = `toolEnergy`: 700
  RobotSolver::BestRun::driveEnergy = 200
  RobotSolver::BestRun::housekeepingEnergy = 300
  RobotSolver::BestRun::toolEnergy = 700
✓ Analysis LightestRobot is optimized (z3, 6ms)
  minimize lightest = `mass`: 300000.0 [gram]
✓ Analysis FarthestThenTool is optimized (z3, 8ms)
  maximize farthest = `standoff`: 120
  maximize mostToolTime = `toolEnergy`: 420
```

The best call-out drives the least the plan allows and spends the rest on tool
time; the two-objective case keeps the longest standoff first, and takes the most
tool time among the plans that keep it. `LightestRobot`'s optimum is a quantity,
and is reported in grams, the unit the runtime normalizes mass to.

### Views

`%view` shows what a view exposes and how it conforms to the viewpoints it
satisfies; `%render` draws it.

```
%view RobotViews::overview
```

```
view RobotViews::overview
  exposes
    Robot::fielded (partUsage)
    Robot::heavyMockup (partUsage)
  nested views
    RobotViews::overview::interfaceSubview (viewUsage)
  viewpoint conformance
    satisfy operationsPerspective: violated
      concern mass: violated
        Robot::heavyMockup: … require condition evaluated to false: robot.mass <= maxMass
```

```
%render RobotViews::modes
%render RobotViews::run mermaid
%render RobotViews::criticalParts
%render RobotViews::partsTable markdown
```

```
RobotViews::modes - state rendering (view def StateTransitionView)

state def RobotBehavior::Modes
  start
  state idle (initial)
  state approach
    state rolling (entry)
    state holding
  state handling (entry)
  state safing (entry)
  state done (completes)

transitions:
  start of RobotBehavior::Modes -> idle
  idle -> rolling: idle_to_rolling: after 5 [s]
  approach -> handling: approach_to_handling: after 20 [s]
  rolling -> holding: rolling_to_holding: after 10 [s]
  rolling -> safing: rolling_to_safe: [faults > 0]
  handling -> done
```

`criticalParts` exposes `Robot::**[@Robot::Critical]`, so it reaches only the
parts marked with the `Critical` metadata: the battery, the wheels, the mobility
system and the computer.

## From Python

[`robot_demo.py`](robot_demo.py) asks the same questions through the `opensysml`
client, which talks to the `sysml-grpc` service
([guide chapter 9](../../docs/guide/09-clients.md#from-python) covers installing both):

```bash
pip install opensysml
python examples/disposal-robot-demo/robot_demo.py
```

```
== the robot as an object
mass          340.0 kg
drive power   120.0 W at 0.42 m/s
wheels        4
battery       1200.0 of 1600.0 Wh

== planning the approach
150.0 m costs 11.9 Wh of the 1200.0 Wh aboard

== running the call-out
frames held   15
arm frames    3 (the retrieval branch ran through)
streamed      15
charge left   1040.0 Wh

== running the mode machine
states        idle -> approach -> rolling -> holding -> handling -> done
wrote         faults=0, framesCaptured=12, odometer=21.0

== checking the call-out budget
✓ satisfy runIsBudgeted by run12 holds (on RobotSolver::run12 ID: 1)
✗ satisfy runIsBudgeted by run13 fails (on RobotSolver::run13 ID: 2): condition evaluated to false: …
both hold     False
heavy mockup  over the mass budget
```

The client covers loading, instances, calculations, action and state execution,
and verification. The solver commands and view rendering are CLI and REPL only,
so a script that needs them shells out to `sysml`.
