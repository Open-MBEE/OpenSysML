# Modeling fleets and repeated structure

A fleet — a constellation of satellites, a production run of vehicles, a rack of
identical servers — is one design, or a few, built many times with values that
differ per unit. This chapter shows how to write that in SysML v2 so the model
declares the design once and the units as *occurrences* of it, what the choice
costs in declared elements against writing every unit out, and what the
runtime does with a fleet of 12 800 occurrences today.

## One definition, a multiplicity

The unit of a fleet is a `part def`. The fleet is a usage of it with a
multiplicity:

```sysml
part def Spacecraft {
    attribute catalogId : Integer;
    part comms : CommsSubsystem;
    attribute dryMass : Real = eps.mass + adcs.mass + comms.mass;
}

part def OrbitalPlane {
    part sats : Spacecraft[400] ordered;
}
```

`sats` is one declared element whatever the number in the brackets. Every
occurrence has the shape `Spacecraft` gives it — the same features, the same
computed `dryMass`, the same mode machine — and the runtime creates the 400
objects when the plane is instantiated, not when the model is read.

Occurrences are addressed by position in an ordered collection and as a
whole for anything that applies to all of them:

```sysml
sysml> %eval network.plane1.sats#(3).catalogId
sysml> %eval network.plane0.sats.dryMass      // one value per occurrence
```

A connector whose ends are collection paths connects the occurrences as a
whole, and an end multiplicity says how many of them each link joins:
`connect [1] sats.comms.crosslinkTx to [1] sats.comms.crosslinkRx` declares
links that each join one transmitter to one receiver over the fleet's ports,
and `connect plane0.sats.comms.rf to gs0.uplink` gives every satellite in a
plane a downlink to one station. What such a connector cannot say is *which*
occurrence pairs with which: a connector end is a feature chain, not an
expression, so `sats#(1).comms.crosslinkTx` is not an end, and the pairing a
per-unit model writes out — unit `i` to unit `i + 1`, unit `i` to the same
slot of the next plane — has no compact form. When the pairing matters,
name the occurrences it involves (`part unit0 :> sats`, below) and connect
those; the runtime realizes a collection connector as one link whose ends
hold the collections, not as a link per pair.

## Variants as specializations

A fleet is rarely one design. Write each variant as a specialization that
sets what differs — as *defaults*, so an occurrence that says nothing inherits
the variant's values and one that diverges can still redefine them:

```sysml
part def BlockA :> Spacecraft {
    attribute :>> catalogId default = 40000;
    part :>> comms {
        part :>> crosslinkTerminal {
            attribute :>> mass default = 3.1 [kg];
            attribute :>> serialNumber default = "CROSSLINKTERMINAL-00000-1";
        }
    }
}

part def BlockB :> Spacecraft { /* … */ }
```

A plane of block-A spacecraft redefines its fleet to the variant, and the
occurrence count travels with the plane:

```sysml
part plane0 : OrbitalPlane {
    part :>> sats : BlockA { attribute :>> plane = 0; }
}
part plane1 : OrbitalPlane {
    part :>> sats : BlockB { attribute :>> plane = 1; }
}
```

A value written with `=` on the variant is fixed for every occurrence and
cannot be redefined beneath it; write `default =` for anything a unit may
state for itself, and `=` for what is genuinely a property of the design.

## Per-occurrence values

A unit whose as-built values differ from its variant's — a heavier terminal,
a serial number — is a member of the fleet that subsets it and redefines the
values it owns:

```sysml
part plane0 : OrbitalPlane {
    part :>> sats : BlockA { attribute :>> plane = 0; }
    part unit16 :> sats {
        attribute :>> catalogId = 40016;
        attribute :>> slot = 16;
        part :>> comms {
            part :>> crosslinkTerminal {
                attribute :>> mass = 19.9 [kg];
                attribute :>> serialNumber = "CROSSLINKTERMINAL-00016-1";
            }
        }
    }
}
```

`unit16` is one of the occurrences of `sats` — the collection still has 400
members, and the units that subset it are its leading members, so with
`unit0` and `unit16` declared, `sats#(1)` is `unit0`, `sats#(2)` is `unit16`
and `sats#(3)` onward read the block's defaults. A diverging unit inherits
everything it does not state, and it is checked by whatever checks the
fleet. The cost of the model is now proportional to how many units
*diverge*, not to how many there are.

What the language also allows, and this implementation does not yet, is to
carry the per-unit values as one table the fleet binds to — a sequence of
serial numbers bound to `sats.comms.crosslinkTerminal.serialNumber` so that
the *n*th occurrence reads the *n*th entry. A binding to a collection path is
accepted, but a target feature that already carries a default is a binding
conflict, and a sequence of 12 800 literals written in the source is as long
as the fleet it describes. Until the runtime can bind a table over defaults,
write diverging units as above and keep the variant's values as defaults.

## The stress-test constellation, both ways

`internal/stressmodel` (`cmd/stress-model`) generates the
[satellite-network stress test](../project/satellite-network-stress-test.md)
in both forms: `-fleet` selects the fleet form. Both state the same
spacecraft — seven subsystems, twenty components with mass, power draw and
serial number, the power and data connections between them, a mass and a
power budget, three requirements with `satisfy` assertions, a mode machine
every spacecraft exhibits — and the same ground stations. The links differ
in what they can state: the per-unit form writes every ring link (satellite
`i` to `i + 1`, closing the ring), every inter-plane link (slot `i` to slot
`i` of the next plane) and every downlink (satellite `i` to station `i mod
G`) as its own connector, and the fleet form declares one connector per
collection with `[1]` ends — each link joins one satellite to one satellite
or station, but the connector does not say which pairs, and the runtime
realizes it as one link over the collections.

**One definition per satellite.** Every satellite is its own `part def`
specializing `Spacecraft` and redefines the whole tree beneath it to state
its serial numbers and as-built masses; the network is a usage of each:

```sysml
part def Sat0 :> Spacecraft {
    attribute :>> catalogId = 40000;
    part :>> eps {
        part :>> solarArray {
            attribute :>> mass = 2.0 [kg];
            attribute :>> serialNumber = "SOLARARRAY-00000-0";
            // …
        }
        // …
    }
    // … about 180 declared elements per satellite, its links and requirements included
}
part def Sat1 :> Spacecraft { /* … */ }
part def Network {
    part sat0 : Sat0;
    part sat1 : Sat1;
    interface ring0To1 : RFLink connect sat0.comms.crosslinkTx to sat1.comms.crosslinkRx;
    // …
}
```

**Fleet.** Four spacecraft blocks carry the as-built values as defaults; each
orbital plane is `part sats : Block[N] ordered` with one ring connector over
the collection, one connector between adjacent planes and one downlink
connector per plane and station, their ends declared `[1]` so that each link
joins one satellite to one satellite or station; every sixteenth unit states
a catalog number, a slot and a crosslink terminal of its own — the fifteen
units between read the block's values, so the fleet carries fewer distinct
per-unit values than the per-satellite form, the trade-off a bound value
table would remove; the requirements are declared once per block and
asserted on the block's configuration and on every diverging unit:

```sysml
part def OrbitalPlane {
    attribute plane : Integer;
    part sats : Spacecraft[400] ordered;
    interface ring : RFLink connect [1] sats.comms.crosslinkTx to [1] sats.comms.crosslinkRx;
}
part def Network {
    part plane0 : OrbitalPlane {
        part :>> sats : BlockA { attribute :>> plane = 0; }
        part unit0 :> sats { /* as-built values */ }
        part unit16 :> sats { /* … */ }
    }
    // …
    interface plane0To1 : RFLink connect [1] plane0.sats.comms.crosslinkTx to [1] plane1.sats.comms.crosslinkRx;
    interface downlink0To0 : RFLink connect [1] plane0.sats.comms.rf to [1] gs0.uplink;
}
satisfy blockAMass by blockAConfig;
satisfy blockAMass by network.plane0.unit16;
```

`-stats` reports the element count of either form — the number of
declarations the source makes:

```bash
go run ./cmd/stress-model -planes 8 -satellites 200 -ground-stations 20 -stats > legacy.sysml
# satellites=1600 definitions=1600 units=1600 ground-stations=20 components=32080 connections=25400 requirements=4800 elements=294627 bytes=18135413
go run ./cmd/stress-model -planes 8 -satellites 200 -ground-stations 20 -fleet -stats > fleet.sysml
# satellites=1600 definitions=4 units=104 ground-stations=20 components=264 connections=220 requirements=12 elements=3203 bytes=191418
```

| satellites | planes × per plane | form | definitions | units stating values | elements | source |
| ---------- | ------------------ | ---- | ----------- | -------------------- | -------- | ------ |
| 1 600 | 8 × 200 | one definition per satellite | 1 600 | 1 600 | 294 627 | 18.1 MB |
| 1 600 | 8 × 200 | fleet | 4 | 104 | 3 203 | 193 KB |
| 12 800 | 32 × 400 | one definition per satellite | 12 800 | 12 800 | 2 354 827 | 145 MB |
| 12 800 | 32 × 400 | fleet | 4 | 800 | 12 467 | 771 KB |

The fleet form of the 12 800-satellite constellation is **12 467 declared
elements against 2 354 827** — a factor of 190 — and what remains grows with
the number of planes (links, downlinks) and of diverging units, not with the
number of satellites. (The
[stress-test record](../project/satellite-network-stress-test.md)'s
single-definition counts, 299 137 and 2 392 417, are the same constellation
split differently into planes and stations; the counts here are the same
layout in both forms.)

## What it costs to validate

All figures below were taken on one machine — `Intel Xeon Platinum 8559C`,
8 CPUs, 31 GiB of memory, no swap, Go 1.25, Linux — from the binary
`make build` produces, as single runs; peak RSS is measured from outside with
`/usr/bin/time`.

`sysml -validate -memstats`, both forms over the layouts above:

| satellites | form | elements | wall | allocated | peak RSS |
| ---------- | ---- | -------- | ---- | --------- | -------- |
| 1 600 | one definition per satellite | 294 627 | 17.5 s | 5.5 GiB | 2.6 GB |
| 1 600 | fleet, 8 × 200 | 3 203 | 0.17 s | 93 MiB | 106 MB |
| 12 800 | one definition per satellite | 2 354 827 | 301 s | 43.5 GiB | 20.1 GB |
| 12 800 | fleet, 32 × 400 | 12 467 | 0.57 s | 254 MiB | 175 MB |

Validation is a function of what the source declares, so the fleet form
validates the 12 800-satellite constellation in **0.57 s and 175 MB** where
the single-definition form takes 301 s and 20.1 GB. That is the whole
payoff of writing the model this way, and it is available today.

## What the runtime does with 12 800 occurrences today

Declaring the fleet does not make the satellites free; it moves their cost
from the source to the runtime, which pays it when something asks for the
occurrences. Today the runtime materializes an object per occurrence with a
value slot per feature — the same objects the single-definition form would
create — and shares only the *shape* of the type (its effective feature list)
between them. Measured on the same machine, same layouts as above:

| satellites | operation | wall | allocated | peak RSS |
| ---------- | --------- | ---- | --------- | -------- |
| 1 600 | `-instantiate` the network | 0.47 s | 220 MiB | 168 MB |
| 1 600 | `-satisfy`, 324 assertions | 0.95 s | 513 MiB | 269 MB |
| 12 800 | `-instantiate` the network | 2.34 s | 1.0 GiB | 692 MB |
| 12 800 | `-satisfy`, 2 412 assertions | 23.4 s | 14.4 GiB | 1.36 GB |
| 12 800 | `%eval` of `sats.dryMass` in every plane | 252 s | 73.8 GiB | 4.9 GB |

What the rows say about the current runtime:

- **Instantiating** the network (`sysml -instantiate
  SatelliteNetwork::Constellation::network`) creates the object per
  occurrence in every plane, 2.34 s and 692 MB — about 50 KB per
  occurrence, linear from 1 600 to 12 800. The run then warns that
  materialization is bounded: the walk that reads the created object's
  feature values stops at the runtime's materialization budget, so the
  component trees beneath the occurrences are unread, not checked clean.
- **Checking** an assertion on a unit — `satisfy blockAMass by
  network.plane0.unit16` — reads the unit through the network object and
  evaluates its summed mass and power, which materializes the unit's
  subsystems and components and starts their behaviors. Every check then
  drains the behaviors the network's objects run, so a check costs more the
  more of the fleet earlier checks have touched: 0.9 MiB allocated per
  assertion in a network of one plane of 400, 6 MiB in one of 32 planes. A
  CPU profile of the 16-plane check spends 57% evaluating the requirements'
  expressions (41% of the total in starting the behaviors of the parts that
  evaluation materializes) and 31% polling running state machines for due
  events. The 2 412 assertions of the 12 800-satellite fleet cost 23.4 s and
  14.4 GiB allocated, against 83 s and 28.9 GiB for the 9 600 assertions of
  a 3 200-satellite single-definition constellation; each assertion still
  pays for the fleet around it.
- **Reading a value over every occurrence** — the dry mass of all 12 800
  satellites — evaluates the summed expression over the full component tree
  of every occurrence, 252 s and 73.8 GiB allocated. This is the cost the
  single-definition form paid at validation; the fleet form pays it at the
  first read instead.
- A connector over the collection is realized as **one link whose ends hold
  the collections** (`%eval network.plane0.ring.a` is the sequence of every
  transmitter in the plane), whatever its end multiplicities declare. The
  per-pair topology of the single-definition form — `ring0To1`, `ring1To2`,
  …, `downlink5To0` — is not recovered from it; only what the connector
  states about the collections as a whole is.
- A `satisfy` whose subject is the fleet itself (`satisfy blockAMass by
  plane0.sats`) is rejected: the subject must denote one object. Assertions
  are therefore made on the block's configuration — one check for every
  occurrence that inherits the block's values — and on each diverging unit.
- A collection whose lower bound exceeds 1 000 is not materialized: a plane
  written `Spacecraft[1600]` validates but its assertions report
  `multiplicity violation: lower bound too large or infinite` when checked.
  Keep a fleet under that bound per usage — 32 planes of 400 rather than 8
  of 1 600 — until the runtime holds occurrences sparsely.

What would make these cheap is described in
[scaling to very large models](../project/large-model-scaling-design.md),
one definition, many occurrences: an occurrence whose feature holds its
block's default storing nothing for it, and a check over N occurrences that
read only block-level values evaluating once. Until that lands, model the
fleet as this chapter shows — the declared model is nearly two hundred times
smaller and validates in under a second — and expect instantiation and
checking to cost what they cost for the same number of fully written
satellites.
