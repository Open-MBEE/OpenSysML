# Relay-probe demo

[`mission.sysml`](mission.sysml) is one deep-space relay probe across its
mission phases, written for the identity and lifecycle side of SysML v2:
`individual` definitions and usages, `event occurrence`s ordering the moments
that matter, `snapshot`s freezing the probe's state at those moments,
`timeslice`s spanning the phases between them, occurrences with multiplicity,
and the places these meet the rest of the language — a calculation reading
across two snapshots, a requirement whose subject *is* a snapshot, and a state
machine inside a timeslice sending telemetry out of the probe's own antenna,
across the mission's connector, to the ground station.

The requirement check needs `z3` or `cvc5` on `PATH` — see
[installing a solver](../../docs/guide/01-install.md#installing-a-solver-optional).

What the one package holds:

| Element | What it is |
| --- | --- |
| `Probe` / `scout` | an `individual part def` and the flight article itself: one probe, not a class of probes |
| `separation`, `closestApproach` | `event occurrence`s — the mission's moments, ordered with `first ... then ...` |
| `postSeparation`, `postFlyby` | `snapshot`s of `scout`, each redefining `mass` to what it was then, ordered in time |
| `cruise` | a `timeslice` of `scout`: the whole cruise phase, with the spin rate it only has then |
| `burns [2]`, `dsnPasses [0..*]` | occurrences with multiplicity — two planned trim burns, any number of tracking passes |
| `massSpent` | a calculation whose defaults read `mass` across the two snapshots |
| `MassBudget` / `flybyMassBudget` | a requirement whose bound subject is the `postFlyby` snapshot |
| `RelayProbe`, `Station`, `Mission` | the probe as flown — a beacon state machine inside its cruise timeslice — the ground station that listens, and the connector between them |

## In the REPL

```bash
./bin/sysml
```

**Materialize the individual.** The portions come with it: each snapshot holds
the mass it redefined, the timeslice holds its own spin rate, and `burns [2]`
fans out to two occurrences.

```
sysml> %load examples/relay-probe-demo/mission.sysml
✓ package RelayMission
sysml> %instantiate scout
✓ Created instance of RelayMission::scout
  ID: 1
  Use %features scout to inspect
sysml> %features scout
Instance: RelayMission::scout (ID: 1)
Features:
  mass = 120.0
  separation = Instance(ID: 2)
    (no features)
  closestApproach = Instance(ID: 3)
    (no features)
  postSeparation = Instance(ID: 4)
    mass = 118.0
  postFlyby = Instance(ID: 5)
    mass = 96.0
  cruise = Instance(ID: 6)
    spinRate = 2.0
  burns = [Instance(ID: 7), Instance(ID: 8)]
    dv = 1.5
    dv = 1.5
  dsnPasses = []
  ownedPorts = []
  …
```

The event occurrences have `(no features)`: a moment carries no configuration
of its own, so there is nothing to hold. `dsnPasses` is an empty collection
until a pass happens, and the features that follow it — `ownedPorts`,
`subparts` and the rest — are the ones every part inherits from the
library. The `first separation then closestApproach;` and `first
postSeparation then postFlyby;` lines are successions over the portions —
they order them in time without shadowing either end.

**Read across the portions.** Both snapshots are the same individual at
different times; the calculation's defaults read one feature at two of them.

```
sysml> %eval scout.postSeparation.mass
  = 118.0
sysml> %eval scout.postFlyby.mass
  = 96.0
sysml> %calc massSpent
✓ massSpent()
  = 22.0
  standing: value (observed: 1 run under reverse)
```

**Check the requirement against a snapshot.** `flybyMassBudget` binds the
subject to `scout.postFlyby`, so the constraint reads the mass the probe had
after the flyby, not the mass it was built with.

```
sysml> %check flybyMassBudget
✓ Requirement flybyMassBudget is satisfiable (z3, 10ms)
  RelayMission::flybyMassBudget::'probeThen.mass' = 90.0
  standing: satisfiable (witnessed: 1 query by solve)
```

**Fly the mission.** Materializing `mission` starts the exhibited machines: the
beacon inside the relay's cruise timeslice sends `Telemetry(frames = 3.0)`
through the probe's `antenna` — a port of the *probe*, not of the timeslice —
and the connector carries it to the station's dish, whose `listen` machine logs
it.

```
sysml> %instantiate mission
✓ Created instance of RelayMission::mission
  ID: 9
  Use %features mission to inspect
sysml> %features mission
Instance: RelayMission::mission (ID: 9)
Features:
  relay = Instance(ID: 10)
    ...
  goldstone = Instance(ID: 13)
    ...
    frames = 3.0
    ...
```

`goldstone.frames = 3.0` is the whole story in one number: a send written
inside a timeslice found the port on its enclosing probe, crossed the
connector, and was accepted as a `Telemetry` occurrence whose `frames` the
transition effect read.

## Known limitations

The notation this model stops short of, honestly:

- `snapshot <name> at <time>` — the timed-snapshot form does not parse; only
  `snapshot <name> { ... }` is supported.
- `happens before` written as a relationship body member does not parse; time
  ordering here is written with `first ... then ...` successions.
- Timeslice *ranges* (a slice bounded by two named events) are not supported;
  a timeslice is declared by name alone.
