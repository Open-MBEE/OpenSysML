# Runtime showcase

Four small models, each built around a question that no amount of reading the
model answers. Every file here validates: names resolve, types conform,
multiplicities hold. What each one shows is what happens next, when the runtime
materializes the instances, evaluates the values, performs the actions and runs
the clock — and, in each, one deliberate case where a well-formed model turns
out not to run, with the runtime saying why.

| Model | The question | What the runtime does |
| --- | --- | --- |
| [`mass-rollup.sysml`](mass-rollup.sysml) | What does the stack weigh? | materializes a launch vehicle, rolls a recursive `totalMass` up through stages and engines, and reports the one part whose mass was declared but never given |
| [`delta-v-budget.sysml`](delta-v-budget.sysml) | Does it reach orbit? | runs the rocket equation with units, evaluates an analysis whose objective is a requirement, asserts that requirement of an object, and refuses a result whose dimension is not a speed |
| [`reliability.sysml`](reliability.sysml) | Will the crew come home? | folds `e^(-λt)` over the critical components, asserts a threshold of two missions, and tells `e` from a quantity that happens to share its name |
| [`mission-sequence.sysml`](mission-sequence.sysml) | Where does the mission end? | performs the flight on an object, branching on the budget left after lunar orbit insertion; advances a clocked state machine through the phases; and stops at an action with no starting step |

The closing section runs the same tool over the published
[Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2): 7 200
lines of real systems-engineering model, valid throughout, whose calculations
and mission action each fail to run for a reason the runtime names.

Build the tool first with `make build`; every command below runs from the
repository root. The `✓ package` lines that open every run — the model loaded
and analysed cleanly — are left out below after the first, and the `standing`
line a failed run closes with, which repeats the error, is elided to `(…)`. For what the
individual flags do, see [running behavior](../../docs/guide/03-command-line.md#running-behavior)
in the command-line guide; the [analysis demo](../analysis-demo/README.md) and the
[disposal robot demo](../disposal-robot-demo/README.md) take the same surfaces
further.

## What the stack weighs

`Component` declares a `mass` and a `totalMass` that defaults to its own mass
plus the sum of its subcomponents' totals. `Stage` binds `subcomponents` to its
engines and redefines `totalMass` to add the propellant it carries,
`LaunchVehicle` binds `subcomponents` to its stages, and `saturnV` is one such
vehicle. No value of `totalMass` is written anywhere; it exists only for an
instance.

```bash
./bin/sysml -quiet \
  -e MassRollup::saturnV.totalMass \
  -e MassRollup::saturnV.stage1.totalMass \
  examples/runtime-showcase/mass-rollup.sysml
```

```
✓ package MassRollup
✓ MassRollup::saturnV.totalMass
  = 2941728.0 [kg]
✓ MassRollup::saturnV.stage1.totalMass
  = 2332000.0 [kg]
```

The 2 332 000 kg is 130 000 kg of stage structure, 2 160 000 kg of propellant
and five 8 400 kg F-1 engines; the total adds the second and third stages the
same way. Evaluating `saturnV.totalMass`
made an instance of the vehicle — three stages, eleven engines — and evaluated
`totalMass` at every level of it.

`saturnVWithIU` is the same stack with an instrument unit added. `InstrumentUnit`
declares its height and inherits `mass`, but gives `mass` no value. Declaring a
feature without a value is ordinary SysML — a definition is allowed to leave
values to its usages — so validation has nothing to say:

```bash
./bin/sysml -validate examples/runtime-showcase/mass-rollup.sysml
```

```
✓ package MassRollup
✓ examples/runtime-showcase/mass-rollup.sysml: no errors
```

Ask for the total and the rollup cannot be finished:

```bash
./bin/sysml -quiet -e MassRollup::saturnVWithIU.totalMass \
  examples/runtime-showcase/mass-rollup.sysml
```

```
sysml: evaluation failed: feature value saturnVWithIU.totalMass: feature value InstrumentUnit.totalMass: no value for feature mass
```

The error names the feature (`mass`), the part it belongs to (`InstrumentUnit`)
and the chain of values that needed it. The model was never wrong; it was
incomplete in a way only an attempt to use it reveals.

## Whether it reaches orbit

`RocketEquation` is Tsiolkovsky's equation with typed inputs: specific impulse
as a time, `g0` as an acceleration, two masses, and a speed back. `StageDeltaV`
applies it to a `Stage` lifting a payload; `TwoStageRocket` sums the two stages;
`saturnIB` fills in the numbers.

Units are computed, not annotated. Seconds times metres per second squared
gives metres per second, and the mass ratio inside the logarithm is a pure
number; the tool checks and carries this at every step:

```bash
./bin/sysml -quiet \
  -calc "DeltaVBudget::RocketEquation(421 [SI::s], 9.80665 [SI::'m/s²'], 133000 [SI::kg], 28000 [SI::kg])" \
  examples/runtime-showcase/delta-v-budget.sysml
```

```
✓ DeltaVBudget::RocketEquation(421 [SI::s], 9.80665 [SI::'m/s²'], 133000 [SI::kg], 28000 [SI::kg])
  = 6432.955324716369 [SI::'m/s']
  standing: value (observed: 1 run under reverse)
```

`AscentBudget` is an analysis: a subject rocket, a `required` speed with a
default, three `out` values and an `objective` that is the `OrbitWithMargin`
requirement. `saturnIBAscent` binds the subject, so it runs by name:

```bash
./bin/sysml -quiet -analysis DeltaVBudget::saturnIBAscent \
  examples/runtime-showcase/delta-v-budget.sysml
```

```
✓ DeltaVBudget::saturnIBAscent
  stage1DeltaV = 3037.6966629706967 [SI::'m/s']
  stage2DeltaV = 6432.955324716369 [SI::'m/s']
  margin = 70.65198768706614 [SI::'m/s']
  objective reachesOrbit: satisfied
  standing: value (observed: 1 run under reverse)
```

Arguments in parentheses rebind the case's inputs. Ask for 400 m/s more and the
same vehicle falls short; the verdict flips, the violated condition is printed,
and the exit status becomes `1`:

```bash
./bin/sysml -quiet -analysis "DeltaVBudget::saturnIBAscent(required = 9800 ['m/s'])" \
  examples/runtime-showcase/delta-v-budget.sysml
```

```
✗ DeltaVBudget::saturnIBAscent(required = 9800 ['m/s'])
  stage1DeltaV = 3037.6966629706967 [SI::'m/s']
  stage2DeltaV = 6432.955324716369 [SI::'m/s']
  margin = -329.34801231293386 [SI::'m/s']
  objective reachesOrbit: not satisfied: margin > 0 ['m/s']
  standing: value (observed: 1 run under reverse)
```

The requirement also stands on its own. `assert satisfy saturnIBOrbit by saturnIB`
claims the rocket satisfies it, and `-satisfy` evaluates every such claim in the
model against an instance of its subject:

```bash
./bin/sysml -quiet -satisfy examples/runtime-showcase/delta-v-budget.sysml
```

```
✓ satisfy saturnIBOrbit by saturnIB holds (on DeltaVBudget::saturnIB ID: 1)
  standing: holds (observed: 1 run under reverse)
```

Two calculations at the end of the file are written to fail, each in a way that
is legal SysML.

`StageDeltaVWithoutGravity` calls `RocketEquation` with three arguments where
it declares four inputs. An invocation may leave a parameter unbound, so this
is well-formed; the pinned OMG pilot validator accepts it without comment
([the discrepancy is recorded](../../docs/project/omg-issues.md#an-invocation-leaving-an-input-parameter-unbound-validates-clean-pilot-2026-07)).
OpenSysML knows the call can never produce a value and warns at validation
time, naming the parameter that positional binding leaves empty:

```
examples/runtime-showcase/delta-v-budget.sysml:93:32: warning: RocketEquation leaves parameter mf unbound, so the call cannot be evaluated
        return :> ISQ::speed = RocketEquation(
                               ^~~~~~~~~~~~~~~
```

`InjectionDeltaV` types a gravitational parameter as a force. Every name
resolves and every operator applies, so there is nothing for a validator to
object to. The runtime computes the number and then refuses to store it, because
the dimension it carries — `M^0.5·T^-1`, the square root of a force over a
length — is not a speed:

```bash
./bin/sysml -quiet \
  -calc "DeltaVBudget::InjectionDeltaV(3.986E14 [SI::N], 6563000 [SI::m], 384400000 [SI::m])" \
  examples/runtime-showcase/delta-v-budget.sysml
```

```
sysml: calc invocation failed: calc DeltaVBudget::InjectionDeltaV: result: type mismatch: cannot write 3135.1638390999387 [kg**0.5/s] (dimension M^0.5·T^-1) to a feature typed by SpeedValue (dimension L·T^-1)
  standing: not covered (…)
```

A modeller who reads that has found a physics error, not a typo.

## Whether the crew comes home

`Spacecraft` has three critical components, each with a failure rate in hertz,
and a `reliability` that is the product over them of `e^(-λt)` for the mission
time. `CrewSafetyReliability` requires that product to be at least 0.99.
`apollo` flies for 195 hours; `longStay` flies the same hardware for 84 days.

```bash
./bin/sysml -quiet -satisfy \
  -e Reliability::apollo.reliability -e Reliability::longStay.reliability \
  examples/runtime-showcase/reliability.sysml
```

```
✓ Reliability::apollo.reliability
  = 0.9902201369662298
✓ Reliability::longStay.reliability
  = 0.9033850540586944
✓ satisfy crewSafety by apollo holds (on Reliability::apollo ID: 9)
  standing: holds (observed: 1 run under reverse)
✗ satisfy crewSafety by longStay fails (on Reliability::longStay ID: 13)
  Required condition evaluated to false: craft.reliability >= threshold
  standing: violated (witnessed: 1 run under reverse)
```

Hours and days were converted to seconds to cancel the hertz; the `collect` body
ran `SurvivalProbability` once per component with the craft's own mission time;
`product` folded the results; and each assertion was decided against an
instance of its subject.

`SurvivalProbabilityFromConstant` writes the same formula as `eulerNumber^(-λt)`.
With `ISQ::*` imported, `eulerNumber` resolves — to the Euler number of
ISO 80000-11, a dimensionless characteristic of pipe flow, which the library
declares and gives no value. The reference resolves, the exponent is a pure
number, the expression is well-typed; nothing in the model is invalid:

```bash
./bin/sysml -quiet \
  -calc "Reliability::SurvivalProbabilityFromConstant(8.0E-9 [SI::Hz], 702000 [SI::s])" \
  examples/runtime-showcase/reliability.sysml
```

```
sysml: calc invocation failed: calc Reliability::SurvivalProbabilityFromConstant: evaluating the returned expression: no value for feature eulerNumber
  standing: not covered (…)
```

## Where the mission ends

`LunarMission` is an action definition: a `budget` in, a `remaining` attribute
that starts at the budget, and a flow of burns that each subtract from it. After
lunar orbit insertion a `decide` node checks whether enough remains to land and
come back up; if not, the flow goes straight to trans-Earth injection. `Flight`
performs the action with its own budget, and `apollo11` and `apollo8` are two
flights.

An action a part performs runs on an instance of that part, so `-instantiate`
makes the object and `-action` names the action and the object:

```bash
./bin/sysml -quiet -instantiate MissionSequence::apollo11 \
  -action "MissionSequence::Flight::mission MissionSequence::apollo11" \
  examples/runtime-showcase/mission-sequence.sysml
```

```
✓ Created instance of MissionSequence::apollo11
  ID: 1
  Use %features MissionSequence::apollo11 to inspect
✓ Started action executor for "MissionSequence::Flight::mission"
  State: Running
  Tokens: 1
✓ Action completed
  Final state: Completed
  Results:
    landed = true
    remaining = 480 ['m/s']
    returned = true
  standing: value (observed: 1 run under reverse)
```

Apollo 8 carries 4 000 m/s less. The same flow, run on that object, takes the
other branch of the decision:

```bash
./bin/sysml -quiet -instantiate MissionSequence::apollo8 \
  -action "MissionSequence::Flight::mission MissionSequence::apollo8" \
  examples/runtime-showcase/mission-sequence.sysml
```

```
  Results:
    landed = false
    remaining = 450 ['m/s']
    returned = true
```

`MissionPhases` is the mission as a state machine on a clock in seconds, and
`mission` is an object that exhibits it. `-advance` runs the clock; the report
says where the machine is, what it has fired, and what it is still waiting for:

```bash
./bin/sysml -quiet -instantiate MissionSequence::mission \
  -state MissionSequence::mission -advance 400000 \
  examples/runtime-showcase/mission-sequence.sysml
```

```
✓ Debugging state machine "phases" exhibited by object #1 of "MissionSequence::mission"
  Current state: prelaunch
  Time: 0.0
  Events: 1
✓ Advanced to 400000.0 (3 event(s) processed)
  Current state: onSurface
  Last event at: 379000.0
  Remaining events: 1
  Waiting on the clock:
    t=457000.0: state machine MissionPhases of object #1, time -> transEarthCoast
  standing: value (observed: 1 run under reverse)
```

Advance to 800 000 s and the machine reaches `recovered`, takes the transition
to `done`, and the run reports `State machine completed`.

`LunarMissionUnordered` lists the same three phases as sub-actions and says
nothing about their order. That is a legal action definition — a definition may
leave sequencing to what refines it — and validation passes it. Asked to
perform it, the runtime has nowhere to begin:

```bash
./bin/sysml -quiet -action MissionSequence::LunarMissionUnordered \
  examples/runtime-showcase/mission-sequence.sysml
```

```
sysml: failed to create executor: initialize action: invalid action flow: no initial node found in action LunarMissionUnordered: no succession leads to "outbound" or to "lunarOps"; 'first' names the step the flow starts at
  standing: not covered (…)
```

Two nodes with nothing before them is ambiguous, and the message names both and
the keyword that would resolve it.

## Apollo 11

The [Apollo 11 SysML v2 model](https://github.com/airbus/apollo-11-sysml-v2) is
a published, 28-file description of the mission — vehicle, crew, functions,
requirements, mission phases, the calculations behind the delta-v and
reliability budgets, and an individual `apollo11MissionIndividual` that
performs the top-level `PerformLunarMission` action. The pinned OMG pilot
validator passes it without a finding.
[Loading it](../../docs/internals/performance.md#a-real-model-apollo-11) takes
OpenSysML 0.43 s and reports 37 warnings and no error, three of them the
unbound-parameter warning shown above, on this model's own calculations.

```bash
git clone https://github.com/airbus/apollo-11-sysml-v2
git -C apollo-11-sysml-v2 checkout 6e9c93f
APOLLO=$(find apollo-11-sysml-v2 -name '*.sysml' | sort)
```

The model's mass-budget step runs as written:

```bash
./bin/sysml -quiet \
  -calc "CalculationsPackage::calculateMassBudgetStep(2900000 [SI::kg], 2000000 [SI::kg], 130000 [SI::kg])" \
  $APOLLO
```

```
✓ CalculationsPackage::calculateMassBudgetStep(2900000 [SI::kg], 2000000 [SI::kg], 130000 [SI::kg])
  = 770000 [SI::kg]
  standing: value (observed: 1 run under reverse)
```

Its other calculations do not, and each failure is a finding about the model
that only an attempt to run it produces. `calculateReliability` is the
`eulerNumber` case above, in the wild — the model means `e` and gets the
ISO 80000 pipe-flow number:

```bash
./bin/sysml -quiet \
  -calc "CalculationsPackage::calculateReliability(5.0E-6 [SI::Hz], 702000 [SI::s])" \
  $APOLLO
```

```
sysml: calc invocation failed: calc CalculationsPackage::calculateReliability: evaluating the returned expression: no value for feature eulerNumber
  standing: not covered (…)
```

`calculateDeltaV` reaches the model's own `naturalLogarithm`, which is declared
with two inputs and a result it never binds — the same body validation warned
about, met at the point of use:

```bash
./bin/sysml -quiet \
  -calc "CalculationsPackage::calculateDeltaV(421 [SI::s], 9.80665 [SI::'m/s²'], 133000 [SI::kg], 28000 [SI::kg])" \
  $APOLLO
```

```
sysml: calc invocation failed: calc CalculationsPackage::calculateDeltaV: evaluating the returned expression: no result expression: calc CoSMAQuantitiesAndUnitsPackage::naturalLogarithm has no return expression: the result parameter binds no value; write the result as the trailing expression of the body, or bind it with `return : DataValue = <expr>;`
  standing: not covered (…)
```

`calculateTliDeltaV` types Earth's gravitational parameter `mu_Earth` as a
force, the error `InjectionDeltaV` reproduces above. The number comes out; the
dimension does not:

```bash
./bin/sysml -quiet \
  -calc "CalculationsPackage::calculateTliDeltaV(3.986E14 [SI::N], 6563000 [SI::m], 384400000 [SI::m])" \
  $APOLLO
```

```
sysml: calc invocation failed: calc CalculationsPackage::calculateTliDeltaV: result: type mismatch: cannot write 3135.1638390999387 [kg**0.5/s] (dimension M^0.5·T^-1) to a feature typed by SpeedValue (dimension L·T^-1)
  standing: not covered (…)
```

And the mission itself. `apollo11MissionIndividual` performs `PerformLunarMission`,
which lists `outbound`, `lunarOps` and `returnJourney` with no succession
between them — `LunarMissionUnordered`, at full scale. Instantiating the
individual starts the performance, and the performance cannot start:

```bash
./bin/sysml -quiet \
  -instantiate Apollo11MissionExecutionPackage::apollo11MissionIndividual \
  $APOLLO
```

```
sysml: instantiation failed: performed action performLunarMission of apollo11MissionIndividual: initialize action: invalid action flow: no initial node found in action PerformLunarMission: no succession leads to "outbound" or to "lunarOps"; 'first' names the step the flow starts at
```

None of these is a validation error, and none should be: each construct is
permitted by the language. They are the questions a model exists to answer —
what does it weigh, does it reach orbit, does the crew come home, where does the
mission end — asked of a model that, as written, cannot answer them, by a tool
that says exactly why.
