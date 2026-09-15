# Satellite-network stress test

How far the toolchain scales over a model that looks like one a systems
engineer would write: a satellite constellation in which every spacecraft is
modeled down to its components, with the connections between them, a mass and
a power budget, requirements with satisfy assertions and a mode machine. The
question answered here is how many such objects `sysml` can hold, at what cost
in time and memory, and where each way of using it stops being practical.

All figures were taken on one machine — `Intel Xeon Platinum 8559C`, 8 CPUs,
31 GiB of memory, no swap, Go 1.25, Linux — from the binary `make build`
produces. Treat absolute numbers as ratios elsewhere. Wall times are single
runs or the range over two or three; they are not `benchstat` medians, and the
smallest models carry ±30% of noise from the machine, which the trend does not.

## The workload

`internal/stressmodel` generates the model; `cmd/stress-model` writes it out:

```bash
go run ./cmd/stress-model -planes 8 -satellites 25 -ground-stations 20 -stats > constellation.sysml
# satellites=200 ground-stations=20 components=4080 connections=3175 requirements=600 elements=37552 bytes=2297852
sysml -validate -memstats constellation.sysml
sysml -satisfy -memstats constellation.sysml
```

A constellation is `planes × satellites` spacecraft and a number of ground
stations. Every satellite is its own `part def`, specializing an abstract
`Spacecraft`, and redefines the whole tree beneath it:

- seven subsystems — electrical power, attitude control, command and data
  handling, communications, propulsion, thermal, payload — each a part with
  its own mass and power draw summed from its components;
- twenty components (solar array, battery, power conditioner, star tracker,
  IMU, four reaction wheels, onboard computer, mass memory, transponder,
  crosslink terminal, antenna, propellant tank, thruster, radiator, heater,
  imaging sensor, payload processor), each with an as-built mass, power draw
  and serial number and the attributes its kind adds (capacity, torque,
  pointing accuracy, data rate, thrust, …), every one a quantity with a unit;
- six power feeds and six data-bus links between the subsystems, a payload
  data feed, and a bus voltage on every feed;
- a dry-mass and a total-power-draw attribute computed over the tree, a mass
  margin constraint, and three requirements — a mass budget, a power budget
  and a crosslink-capacity requirement — each with a `satisfy` assertion on
  the satellite's configured usage;
- a `SpacecraftModes` state machine, exhibited by every spacecraft, that
  accepts typed events (`ImagingWindow`, `GroundContact`, `Fault`, …).

Satellites are cross-linked to the next satellite in their plane and to the
same slot in the adjacent plane; every satellite has a downlink to a ground
station, and every station carries a dish, a modem, a server and a UPS. RF
links use conjugated ports with flows in both directions and carry a data rate
and a slant range. The whole network is one `part def Network` with a usage of
every satellite and station.

Per satellite that comes to roughly **187 declared elements, 20 components and
13 internal connections**, plus its two to four links. The element count reported by `-stats` is
the number of declarations the source makes — definitions, usages, attributes,
connections, requirements, assertions, states and transitions — so it is
comparable to the element counts in `docs/internals/performance.md`. It is
not a count of objects at run time: a run of `-satisfy` materializes each
satellite's tree of about 30 objects when its subject is instantiated.

The model is deliberately regular. Real models have fewer identical
satellites and more variety in their definitions, deeper expression nesting
and more textual documentation; the regularity is what makes the scaling
curve readable, and the cost it hides is discussed under limitations.

## Loading and validating

`sysml -validate -memstats`: parse, scope building, name resolution and every
validation pass, reporting nothing. *Allocated* is cumulative allocation;
*RSS* is the peak resident size measured from outside with `/usr/bin/time`.

| satellites | elements | source | wall | allocated | peak RSS |
| ---------- | -------- | ------ | ---- | --------- | -------- |
| 2 | 635 | 34 KB | 0.06 s | 49 MiB | 77 MB |
| 10 | 2 115 | 124 KB | 0.12–0.14 s | 77 MiB | 93 MB |
| 50 | 9 517 | 577 KB | 0.47–0.52 s | 213 MiB | 160 MB |
| 100 | 18 862 | 1.15 MB | 0.92–0.97 s | 389 MiB | 250 MB |
| 200 | 37 552 | 2.30 MB | 1.9–2.1 s | 738 MiB | 400 MB |
| 400 | 74 857 | 4.59 MB | 4.1–5.3 s | 1.4 GiB | 700 MB |
| 800 | 149 617 | 9.18 MB | 8.6–8.7 s | 2.8 GiB | 1.35 GB |
| 1 600 | 299 137 | 18.4 MB | 19.0 s | 5.5 GiB | 2.7 GB |
| 3 200 | 598 177 | 36.8 MB | 38–42 s | 11.0 GiB | 5.1 GB |
| 6 400 | 1 196 257 | 73.6 MB | 86 s | 22.0 GiB | 9.9 GB |
| 12 800 | 2 392 417 | 147 MB | 318 s | 44.0 GiB | 20.6 GB |

Above the process floor the cost is close to linear in the model: **about
55 µs, 19 KiB allocated and 8.5 KB of peak RSS per element**, or 10–13 ms,
3.5 MiB and 1.6 MB per fully modeled satellite. Doubling the model doubles
the time from 100 to 6 400 satellites (0.95 → 1.95 → 4.1 → 8.6 → 19 → 40 →
86 s); the last doubling to 12 800 costs 3.7× rather than 2×. That run was
not profiled; the most plausible reading is the collector working against a
20 GB heap on a 31 GiB machine rather than a change in the algorithm.

The next doubling — 25 600 satellites, 4.8 million elements — would need
about 40 GB resident and was not attempted. **Peak memory, not time, is the
hard limit of a batch validation**: at 8.5 KB of RSS per declared element a
machine holds roughly `memory / 8.5 KB` elements, so 16 GiB validates about 1.9
million elements (10 000 of these satellites) and 32 GiB about 3.8 million.

### What a loaded model holds

The peak RSS above is mostly transient: the parser's and the resolver's
garbage. What a session keeps alive once the model is loaded — the size that
bounds a long-lived REPL, LSP or gRPC session rather than a single run — is
measured in-process by `BenchmarkLoad` in `internal/stressmodel`, which
collects before and after a load and reports the difference:

```bash
go test ./internal/stressmodel -run '^$' -bench Load -benchmem -benchtime 3x
```

| satellites | elements | load wall | live heap held | held / element | allocated / element |
| ---------- | -------- | --------- | -------------- | -------------- | ------------------- |
| 32 | 6 227 | 0.29 s | 18.8 MiB | 3.1 KiB | 19.3 KiB |
| 128 | 24 167 | 1.21 s | 63.1 MiB | 2.7 KiB | 18.7 KiB |
| 512 | 95 927 | 5.23 s | 242 MiB | 2.6 KiB | 18.6 KiB |

A loaded satellite costs **about 490 KiB held**, so a session can hold a
2 000-satellite constellation in a gigabyte. Held memory is about a third of
the peak RSS a load of the same model reaches; the other two thirds are what
the collector has not yet returned when the run ends. These satellites hold
less per element than the 8 KiB the figures in `docs/internals/performance.md`
report for the synthetic model there, because most of their elements are
attribute redefinitions with a literal value rather than definitions with
bodies of their own.

## Running: instantiation, state machines and satisfaction

`sysml -satisfy -memstats` loads and validates the model, then for every
`satisfy` assertion instantiates its subject — a satellite's configured usage,
with its subsystem and component tree — starts the mode machine the spacecraft
exhibits, evaluates the summed mass and power over the instance and checks the
requirement's constraint against it. Three assertions per satellite; every one
holds.

| satellites | assertions | wall | of which load | allocated | peak RSS |
| ---------- | ---------- | ---- | ------------- | --------- | -------- |
| 2 | 6 | 0.10–0.15 s | 0.06 s | 65 MiB | 90 MB |
| 10 | 30 | 0.23–0.25 s | 0.13 s | 122 MiB | 118 MB |
| 50 | 150 | 0.90–0.98 s | 0.49 s | 402 MiB | 210 MB |
| 100 | 300 | 1.86–1.95 s | 0.95 s | 762 MiB | 310 MB |
| 200 | 600 | 3.9–4.1 s | 2.0 s | 1.5 GiB | 530 MB |
| 400 | 1 200 | 8.3 s | 4.5 s | 2.9 GiB | 975 MB |
| 800 | 2 400 | 17.5–17.9 s | 8.7 s | 6.0 GiB | 1.83 GB |
| 1 600 | 4 800 | 38.1 s | 19.0 s | 12.8 GiB | 3.8 GB |
| 3 200 | 9 600 | 83 s | 42 s | 28.9 GiB | 7.6 GB |

Checking the whole constellation costs **about 2.0× a validation** of the
same model at every size, and the extra is linear: about 3.2 ms, 1.2 MiB
allocated and 0.45 MB of peak RSS per assertion — that is, per instantiation
of a satellite with its twenty components and a running state machine. A CPU
profile at 400 satellites puts the run's own share (30% of samples, the rest
being the load) almost entirely in `runtime.(*Context).Instantiate`:
materializing the parts that run behaviors (`materializeBehavingParts`,
`runsBehaviors`) and shaping the features of each type (`FeaturesOf`,
`semantics.(*Model).ShapeFeatures`). Evaluating the budgets is a small part.

Re-checking a loaded constellation is much cheaper than the first check,
because the runtime's per-type memoization — feature shapes, which types run
behaviors, the verification cases under each scope — is then warm.
`BenchmarkSatisfy` measures the warm re-check of every assertion:

| satellites | assertions | warm re-check | allocated |
| ---------- | ---------- | ------------- | --------- |
| 32 | 96 | 3.5 ms | 1.9 MiB |
| 128 | 384 | 20 ms | 11.2 MiB |
| 512 | 1 536 | 144 ms | 104 MiB |

That is under 0.1 ms per assertion warm, against 3.2 ms cold: the first check
pays for building the runtime's view of every type, and a session that keeps
the model loaded — the REPL, the gRPC service — amortizes it.

## The same constellation as a fleet

Everything above declares a `part def` per satellite. The generator's
`-fleet` form states the same constellation the way a fleet is engineered
— a few spacecraft blocks carrying the as-built values as defaults, each
orbital plane as `part sats : Block[N] ordered`, as-built values only on the
units that diverge from their block (every sixteenth), the ring link as one
connector over the collection, one inter-plane link per adjacent pair of
planes and one downlink per plane and station — the collection connectors
with `[1]` ends, so each link joins one satellite to one satellite or
station, though not which to which — and the three requirements
declared once per block and asserted on the block's configuration and on
every diverging unit. `-stats` reports both forms alike; the two new fields
are the spacecraft definitions and the units that state values of their own.
The guide chapter [modeling fleets](../guide/modeling-fleets.md) shows the
source of both forms.

```bash
go run ./cmd/stress-model -planes 32 -satellites 400 -ground-stations 20 -stats > legacy.sysml
# satellites=12800 definitions=12800 units=12800 ground-stations=20 components=256080 connections=204400 requirements=38400 elements=2354827 bytes=145364954
go run ./cmd/stress-model -planes 32 -satellites 400 -ground-stations 20 -fleet -stats > fleet.sysml
# satellites=12800 definitions=4 units=800 ground-stations=20 components=960 connections=724 requirements=12 elements=12467 bytes=770621
```

| satellites | planes × per plane | form | definitions | units | elements | source | `-validate` wall | allocated | peak RSS |
| ---------- | ------------------ | ---- | ----------- | ----- | -------- | ------ | ---------------- | --------- | -------- |
| 1 600 | 8 × 200 | one definition per satellite | 1 600 | 1 600 | 294 627 | 18.1 MB | 17.5 s | 5.5 GiB | 2.6 GB |
| 1 600 | 8 × 200 | fleet | 4 | 104 | 3 203 | 193 KB | 0.17 s | 93 MiB | 106 MB |
| 12 800 | 32 × 400 | one definition per satellite | 12 800 | 12 800 | 2 354 827 | 145 MB | 301 s | 43.5 GiB | 20.1 GB |
| 12 800 | 32 × 400 | fleet | 4 | 800 | 12 467 | 771 KB | 0.57 s | 254 MiB | 175 MB |

The single-definition rows here are the plane and station layout the fleet
uses, so the two forms describe the same planes and stations; the validation table above
(299 137 and 2 392 417 elements, 19.0 s and 318 s) was taken over a layout
with a different split into planes and stations, and so slightly more links
and station components. The fleet form
declares **190 times fewer elements** at 12 800 satellites and validates in
0.57 s and 175 MB rather than 301 s and 20.1 GB: validation is a function of
what the source declares, and the fleet source is the size of four
spacecraft, twenty stations and the links between thirty-two planes.

What the current runtime does with the 12 800 occurrences, on the same
machine:

| satellites | operation | wall | allocated | peak RSS |
| ---------- | --------- | ---- | --------- | -------- |
| 1 600 | `-instantiate` the network | 0.47 s | 220 MiB | 168 MB |
| 1 600 | `-satisfy`, 324 assertions | 0.95 s | 513 MiB | 269 MB |
| 12 800 | `-instantiate` the network | 2.34 s | 1.0 GiB | 692 MB |
| 12 800 | `-satisfy`, 2 412 assertions | 23.4 s | 14.4 GiB | 1.36 GB |
| 12 800 | `%eval` of `plane<i>.sats.dryMass`, all 32 planes | 252 s | 73.8 GiB | 4.9 GB |

The runtime shares one shape — the effective feature list `FeaturesOf`
caches per type — between the occurrences of a block, and nothing else: each
occurrence is an object with a value slot per feature, materialized lazily.
Instantiating the network is therefore linear and cheap (about 50 KB per
occurrence; the walk of the created object's feature values stops at the
materialization budget and says so). Checking is not: a `satisfy` on a unit
reads the unit through the network object, evaluates its summed mass and
power — materializing its subsystems and components and starting their
behaviors — and then drains the behaviors every object of the network runs,
so each check costs more the more of the fleet earlier checks have touched
(0.9 MiB allocated per assertion in a network of one plane of 400, 6 MiB in
one of 32 planes). A CPU profile of the 16-plane `-satisfy` spends 57% of
its samples evaluating the requirements' expressions, 41% of the total under
`startClassifierBehaviors` for the parts that evaluation materializes, and
31% in `ObjectBehavior.hasPendingWork` / `StateExecutor.hasDueEvent`
polling the running mode machines. Reading one summed attribute over every
occurrence evaluates it over the full tree of each — the cost the
single-definition form paid at validation, paid here at the first read.

Three limits of the current language and runtime shape the fleet form:

- A connector end is a feature chain, so the fleet form cannot write the
  single-definition form's pairing — `ring<i>To<i+1>` closing each plane,
  `plane<i>To<j>` between the same slots of adjacent planes, `downlink<i>To<k>`
  to station `i mod G` — without naming every occurrence. It declares one
  connector over each collection instead, and the runtime realizes that as
  one link whose ends hold the collections (`%eval network.plane0.ring.a`
  is every transmitter of the plane), whatever the `[1]` ends declare. The
  topology the two forms state is therefore not the same: the fleet says
  each satellite is linked within its plane, to the next plane and to the
  stations, not to which neighbour or station.

- A `satisfy` whose subject is a collection (`satisfy blockAMass by
  plane0.sats`) is rejected — the subject must denote one object — so the
  fleet asserts each requirement on the block's configuration, which stands
  for every occurrence inheriting the block's values, and on each diverging
  unit.
- A collection whose lower bound exceeds 1 000 (`maxMaterializedLowerBound`)
  is not materialized: a plane of `Spacecraft[1600]` validates, but checking
  a unit of it reports `multiplicity violation: lower bound too large or
  infinite`. The 12 800-satellite fleet is therefore 32 planes of 400.

`BenchmarkFleetInstantiate` and `BenchmarkFleetSatisfy` in
`internal/stressmodel` measure, warm, instantiating the fleet network and
reading `sats.dryMass` over four planes, and re-checking every assertion:

```bash
go test ./internal/stressmodel -run '^$' -bench Fleet -benchmem -benchtime 3x
```

| satellites | elements | instantiate + read four planes | per satellite | allocated | assertions | warm re-check | allocated |
| ---------- | -------- | ------------------------------ | ------------- | --------- | ---------- | ------------- | --------- |
| 32 | 1 179 | 74 ms | 2.3 ms | 20.3 MiB | 24 | 0.8 ms | 0.5 MiB |
| 128 | 1 715 | 268 ms | 2.1 ms | 83.0 MiB | 36 | 1.4 ms | 1.1 MiB |
| 512 | 3 947 | 1.43 s | 2.8 ms | 579 MiB | 108 | 6.4 ms | 7.4 MiB |

Warm, instantiating a fleet and reading a summed attribute over its
occurrences costs **about 2 ms and 1 MiB per satellite** — the per-satellite
cost of a cold `-satisfy` over the single-definition form — because every
occurrence's component tree is still materialized to evaluate the sum. What
would change that is sparse per-occurrence values and verification over
distinct shapes ([scaling to very large models](large-model-scaling-design.md),
one definition, many occurrences): an occurrence whose feature holds its
block's default storing nothing for it, and a check over N occurrences that
read only block-level values evaluating once.

## Editing: what an editor pays per keystroke

An editor does not validate once; it re-validates the open file after every
change, with the rest of the project indexed beside it. `BenchmarkEditBeside`
opens the constellation as one workspace document, opens a second small file
that imports it (`package Ops { private import SatelliteNetwork::Constellation::*; part spare : Sat0; }`),
and measures one edit to the small file followed by its diagnostics — what the
LSP server does on `didChange`:

| satellites in the workspace | elements | per edit of the small file | allocated per edit |
| --------------------------- | -------- | -------------------------- | ------------------ |
| 32 | 6 227 | 52 ms | 21 MiB |
| 128 | 24 167 | 263 ms | 79 MiB |
| 512 | 95 927 | 1.53 s | 311 MiB |

**The cost of editing a two-line file grows linearly with the size of the
model it sits beside**: about 16 µs per element in the workspace, per
keystroke. The reasons are structural, not incidental:

- `model.(*Workspace).invalidateLocked` drops every cached diagnostic and
  the reverse-reference index on any change, on the correct grounds that a
  change anywhere can alter what a name elsewhere resolves to. The next
  diagnostics request re-analyzes from a fresh semantic model with cold
  memoization.
- Several passes are workspace-wide by design: a CPU profile of the 128-
  satellite case spends 24% in the OOSEM method audit (`OOSEMMethodPass`),
  which gathers the kind of every symbol in every workspace document to
  check derivation and satisfaction relationships across files, 10% in the
  inherited-name conflict pass and 3% in the MOSA audit; between them they
  compute `semantics.(*Model).FeatureTypeSet` over the whole constellation
  to analyze a file that declares one part. Name resolution of the small
  file's own imports is 16%.

The interactive limit is therefore set by the workspace, not the file being
edited. With one large model file beside the one being typed in, 100–200 of
these satellites (20 000–40 000 elements) keep a keystroke under 250–500 ms;
at 500 satellites every keystroke costs 1.5 s and the editor is no longer
usable. A project that splits the constellation over many files pays the
same per-file analysis for each open file it publishes diagnostics for, so a
workspace with `n` open files costs about `n` times the figures above.

## Where the time goes

A CPU profile of `sysml -validate` at 1 600 satellites (299 137 elements,
19 s):

| share of samples | where |
| ---------------- | ----- |
| 56% | validation passes (`passes.AnalyzeWithOptions`), of which |
| — 23% | name resolution (`resolve.(*Resolver).ResolveDocument`) |
| — 13% | inherited-name conflict checking (`W9CInheritedNameConflictPass`) |
| — 5% | the type checker |
| 12% | parsing |
| 8% | indexing the document and expanding wildcard imports |
| 25% | the collector (`runtime.scanobject` and its callees), spread through the above |
| 12% | map access (`runtime.mapaccess2_fast64` and the hashing beneath it) |

Nothing in the profile is super-linear at this size; the doubling series
above confirms it. Two things that *were* super-linear in an earlier build
were fixed along the way and are worth recording, since a model of this shape
is what exposes them:

- **Implicit parameter naming rescanned every anonymous member of a scope
  for every name resolved through it.** A `part def` here declares a dozen
  anonymous interface usages, and resolving each of the hundred or so names
  in its body walked all of them, so the cost per satellite grew with the
  satellite. `resolve.(*Resolver).implicitParameters` now collects the
  anonymous members that can carry an implicit redefinition once per scope
  and journals the entry so transient resolution does not retain it. This
  took the 1 600-satellite validation from 30 s to 19 s, and it was 25% of
  the samples before.
- **Every satisfy assertion walked the whole model for verification cases.**
  `runtime.(*Context).VerificationsOf` collected the verification cases under
  the model root on every call, so checking `n` assertions walked `n × the
  model`. `runtime.(*Model).verificationCasesIn` now memoizes the walk per
  scope, in declaration order, for the life of the runtime model. This took
  the 200-satellite `-satisfy` from 5.9 s and 2.2 GiB allocated to 3.9 s and
  1.5 GiB, and the quadratic term was 18% of the samples before.

## Where it stops being practical

| use | comfortable | slow | impractical | bound by |
| --- | ----------- | ---- | ----------- | -------- |
| editing with the model open beside the file | ≤ 100 satellites (20 000 elements, ≤ 250 ms per keystroke) | 200–400 satellites (0.5–1.2 s) | ≥ 500 satellites (≥ 1.5 s per keystroke) | whole-workspace re-analysis per edit |
| a REPL or gRPC session holding the model | ≤ 500 satellites (≤ 5 s to load, 250 MiB held) | 1 000–2 000 satellites (10–25 s to load, 0.5–1 GiB held) | limited by load time, not by memory, until the tens of thousands | load wall time; ~490 KiB held per satellite |
| `sysml -validate` in a build or CI | ≤ 800 satellites (≤ 10 s, 1.4 GB) | 1 600–6 400 satellites (20–90 s, 2.7–10 GB) | 12 800 satellites at 5 min and 21 GB; 25 600 would not fit in 31 GiB | peak RSS, 8.5 KB per element |
| `sysml -satisfy` over every assertion | ≤ 400 satellites (≤ 8 s, 1 GB) | 800–1 600 satellites (18–38 s, 2–4 GB) | 3 200 satellites at 83 s and 7.6 GB; memory runs out about half as far as validation | 2× the validation cost |

For the question as asked — a satellite network with every component modeled
— the practical ceilings on this machine are **a few hundred satellites for
interactive editing, a few thousand for batch validation and checking, and
about ten thousand (two million elements) before a 31 GiB machine cannot hold
a validation**. A small constellation of a dozen satellites is well inside
the interactive band at every operation measured.

## Limitations of the measurement

- One machine, one Go version, one run or a few per point. The trend is
  robust; the absolute figures are ratios to carry elsewhere.
- The workload is regular by construction. Per-element costs of an
  irregular model will differ — deeper expressions and more definitions per
  usage cost more per element, long documentation comments cost less — but
  the shape of the curve (linear load, memory-bound batch, workspace-bound
  editing) does not depend on the regularity.
- The whole constellation is one file. Splitting it over files changes two
  things: the CLI submits files one at a time and reindexes after each, which
  is quadratic in the file count (`docs/internals/performance.md`, notes for
  further work), and an editor pays the per-file analysis once per open file.
- `-satisfy` over the single-definition form instantiates each satellite's
  tree on its own; it does not instantiate the whole `Network` as one object
  with 12 800 satellites and their links. Only the fleet section
  instantiates the network whole, and its occurrences are lazily materialized
  objects whose component trees are read only where a check or an
  evaluation reaches them.
- Runtime execution is a mode machine driven to its initial state per
  instantiation, not a long simulation with events. Event throughput is
  measured separately in `docs/project/execution-performance-2026-09.md`.
- Nothing here measures the LSP server end to end over a socket, the gRPC
  service, or rendering a view or document of the constellation.

## Further work the measurement points at

The design that takes these up — a persistent semantic model invalidated per
document, closed documents held as interface records, parallel batch
validation, and one definition with many occurrences — is in
[scaling to very large models](large-model-scaling-design.md). The three
items below are the ones the profiles point at directly.

- **Scope the workspace-wide passes.** The OOSEM and MOSA audits and the
  inherited-name conflict pass are what make one keystroke cost a
  whole-workspace walk. The audits need the whole workspace only when some
  document declares an artefact of the method's kinds; whether one does is
  computable once per reindex and cached, and a workspace that declares none
  would then pay nothing. That alone removes a quarter of the per-edit cost.
- **Keep the semantic model across edits.** Every diagnostics request after an
  edit starts from cold memoization. Invalidating what a change can reach —
  the documents that import the changed one, transitively — rather than
  everything would let a keystroke in a small file beside a large model cost
  what the small file costs.
- **Reduce allocation per element.** Nineteen KiB allocated per element
  against 2.7 KiB held means a load produces seven times its own weight in
  garbage, and the collector's quarter of the profile is the price. The
  parser's and resolver's per-token and per-name allocations are the largest
  contributors.
