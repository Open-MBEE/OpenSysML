# Bomb-disposal team demo

[`team.sysml`](team.sysml) is the team around the robot of
[disposal-robot-demo](../disposal-robot-demo/README.md): the truck that carries
it, the console that commands it, the callout they answer, and the cases stated
about them. Where the robot demo drives one part end to end, this one is written
for the notation the robot demo does not reach — quantities with units,
collection operations, messages crossing a connector between two parts,
occurrences with snapshots and timeslices, and the four kinds of case.

The solver sections need `z3` or `cvc5` on `PATH` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional).
`%optimize` needs z3 specifically.

The four packages:

| Package | What it holds |
| --- | --- |
| `Team` | the truck, its two cradles and two robots, a payload budget in kilograms, and two calculations over the fleet — one `select`, one `reduce` |
| `TeamComms` | the command item, a port definition and its conjugate, the interface joining them, the console and unit state machines that send and accept command items, the site that connects one console to one unit, and the squad site whose connector fans out to two units |
| `TeamMission` | the callout occurrence, the individual that happened, and a snapshot and a timeslice holding the crew it had |
| `TeamCases` | the payload requirement and the satisfaction assertion, the calculation usages, the use case, the verification case, and the analysis case the solver optimizes |

## From the command line

Analyse the model:

```bash
./bin/sysml examples/disposal-team-demo/team.sysml -validate
```

```
✓ package Team
✓ package TeamComms
✓ package TeamMission
✓ package TeamCases
✓ examples/disposal-team-demo/team.sysml: no errors
```

**Ask the fleet calculations.** `FleetReach` selects the robots reaching past a
floor and reads their reach; `Endurance` reduces the charges to one total and
divides it by the draw. Both are asked through usages that bind their inputs to
the loaded truck, so `-calc` needs no arguments.

```bash
./bin/sysml examples/disposal-team-demo/team.sysml \
  -calc TeamCases::usableReach -calc TeamCases::teamEndurance
```

```
✓ TeamCases::usableReach
  usable = [2.1 [m], 2.1 [m]]
✓ TeamCases::teamEndurance
  minutes = 60.0
```

**Check the payload budget.** The truck's payload is the sum of what it carries,
in kilograms; the constraint compares it against the limit, and the requirement
states the same of the truck it is bound to.

```bash
./bin/sysml examples/disposal-team-demo/team.sysml \
  -instantiate TeamCases::loadedTruck -constraint Team::Truck::withinPayload
./bin/sysml examples/disposal-team-demo/team.sysml -requirement TeamCases::payloadHolds -satisfy
```

```
✓ Constraint Team::Truck::withinPayload passed (on TeamCases::loadedTruck ID: 1)
✓ Requirement TeamCases::payloadHolds satisfied
✓ satisfy payloadAsserted by loadedTruck holds (on TeamCases::loadedTruck ID: 6)
✓ satisfy payloadHolds holds
```

## From the REPL

**The console commands the unit.** Instantiating the site materializes both
parts, and each exhibits its own state machine: the console's states send its
`approach` then its `disarm` command item through its own port, and the connector
the site declares carries each one to the unit's conjugated port, where the
transition accepts it and stores its code.

```
sysml> %load examples/disposal-team-demo/team.sysml
sysml> %instantiate TeamComms::site
sysml> %features TeamComms::site
```

```
Instance: TeamComms::site (ID: 1)
Features:
  console = Instance(ID: 2)
    command = Instance(ID: 8)
      issued = Instance(ID: 9)
        code = <unset>
    issued = 2
    approach = Instance(ID: 4)
      code = 1
    disarm = Instance(ID: 7)
      code = 2
    dispatch = Instance(ID: 3)
      ready = <unknown>
      approaching = <unknown>
      disarming = <unknown>
  unit = Instance(ID: 5)
    command = Instance(ID: 11)
      issued = Instance(ID: 12)
        code = <unset>
    accepted = 2
    lastCode = 2
    duty = Instance(ID: 6)
      standingBy = <unknown>
      working = <unknown>
  link = Instance(ID: 14)
    commanding = Instance(ID: 8)
      issued = Instance(ID: 9)
        code = <unset>
    commanded = Instance(ID: 11)
      issued = Instance(ID: 12)
        code = <unset>
```

`accepted = 2` and `lastCode = 2` are the unit's own values: the messages were
delivered on the unit's identity, not the console's, which is what a connector
between two parts means. They are also the trace of the machine's path — the
states print as `<unknown>` because `%features` reads values, not the
configuration — so the unit entered `working` twice, its unguarded transition
returning it to `standingBy` each time.

**One send, every unit.** `SquadSite` connects the console's port to
`units.command`, where `units : Unit[2]` — a connector end reached through a
multi-valued feature denotes every element it holds, so each send is delivered
once per unit, on that unit's own identity.

```
sysml> %instantiate TeamComms::squad
sysml> %features TeamComms::squad
```

```
Instance: TeamComms::squad (ID: 1)
Features:
  console = Instance(ID: 3)
    ...
    issued = 2
    ...
  units = [Instance(ID: 6), Instance(ID: 7)]
    command = Instance(ID: 12)
      issued = Instance(ID: 16)
        code = <unset>
    accepted = 2
    lastCode = 2
    duty = Instance(ID: 8)
      standingBy = <unknown>
      working = <unknown>
    command = Instance(ID: 13)
      issued = Instance(ID: 18)
        code = <unset>
    accepted = 2
    lastCode = 2
    duty = Instance(ID: 9)
      standingBy = <unknown>
      working = <unknown>
  (anonymous connector) = Instance(ID: 2)
    source = Instance(ID: 11)
    target = [Instance(ID: 12), Instance(ID: 13)]
```

The console issued two commands, and each unit accepted both — `accepted = 2`
and `lastCode = 2` on each element, not on the collection: every delivery keeps
the element's own port and identity.

**The callout that happened.** An individual is one occurrence, and its snapshot
and timeslice are objects of their own, each holding the crew it had while the
individual holds the district throughout.

```
sysml> %instantiate TeamMission::callout17
sysml> %features TeamMission::callout17
```

```
Instance: TeamMission::callout17 (ID: 1)
Features:
  district = "Harbour"
  arrival = Instance(ID: 2)
    crewOnSite = 2
  approach = Instance(ID: 3)
    crewOnSite = 5
  crewOnSite = <unset>
  unit = Instance(ID: 5)
    mass = 340.0 [kg]
    reach = 2.1 [m]
    charge = 1200.0
```

**What the solver answers.** `%check` translates the requirement through the
subject it is bound to; `%optimize` maximizes the analysis objective under its
constraints, in the base units of the quantity.

```
sysml> %check TeamCases::payloadHolds
sysml> %optimize TeamCases::HeaviestPair
```

```
✓ Requirement payloadHolds is satisfiable (z3, 6ms)
  TeamCases::payloadHolds::'truck.payload' = 0.0 (in the base units of M)
  TeamCases::payloadHolds::'truck.payloadLimit' = 0.0 (in the base units of M)
✓ Analysis HeaviestPair is optimized (z3, 8ms)
  maximize heaviest = `robotMass`: 540000.0 [gram]
  TeamCases::HeaviestPair::cradleMass = 60000.0 [gram]
  TeamCases::HeaviestPair::robotMass = 540000.0 [gram]
```

The values `%check` prints are a witness the solver chose, not the model's
defaults: the requirement's subject leaves `payload` and `payloadLimit` free, and
`0.0 <= 0.0` satisfies it. `-constraint withinPayload` and `-satisfy` above are
what evaluate the declared masses.

The heaviest pair the budget allows is 540 kg a robot once the two cradles take
their 60 kg each — `2 * 540 + 2 * 60 = 1200`.

## What this model found

Three defects, all fixed with the change that added this example:

- **A send did not cross a connector its owner declared.** The console's state
  sends through its own port, and the connector joining that port to the unit's
  lives on the site holding both. Routing only consulted the connections of the
  behavior and of the sending object, so the send reported `send reaches no
  receiving port`. Deliveries now also follow the connections of the objects
  holding the sender, on the peer object's identity
  (`internal/exec/runtime/routing.go`).
- **A bound subject did not carry the subject's type.** `requirement payloadHolds
  : PayloadReq { subject truck = loadedTruck; }` redefines the definition's
  subject, so it is typed by `Truck` — but the redefinition was not among the
  usage's supertypes, and `%check` refused the requirement with ``payload` names
  no member of `truck``. Implicit role redefinitions are now direct supertypes
  (`internal/semantic/semantics/model.go`).
- **An item object could not be sent.** `send approach via command`, where
  `approach` is an `item approach : Command { … }` of the console, reported
  `message of kind instance has no signal type`: a message took its type from a
  scalar value only, so an object had none. An object's message is now typed by
  the definition it materializes, which is the type an accept of it names
  (`internal/exec/runtime/signal.go`).
