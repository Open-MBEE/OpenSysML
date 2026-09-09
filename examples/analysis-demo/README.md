# Lander analysis demo

[`lander.sysml`](lander.sysml) is a small model built to be *asked* rather than
read: a lander definition with three candidates, the analysis and verification
cases stated about them, a trade study over the three, and a ground action that
runs on the same clock as the lander's flight mode. Each section below runs one
case, shows what the tool prints, and says what to look for in the output. It is
the worked companion to [running an analysis case](../../docs/guide/06-behavior.md#running-an-analysis-case)
in the behavior guide and [running behavior](../../docs/guide/03-command-line.md#running-behavior)
in the command-line guide.

Nothing here needs a solver. The Python section needs the `opensysml` client
and a `sysml-grpc` service — see [clients](../../docs/guide/09-clients.md).

The four packages:

| Package | What it holds |
| --- | --- |
| `Landers` | `Lander`, with masses, burn rate, payload and touchdown speed, and a `flightMode` state machine that coasts five seconds and then brakes; the `scout`, `hauler` and `relay` candidates |
| `Descent` | `FuelBudget`, an analysis whose two action steps compute the fuel a burn leaves and whose objective is the `FuelReserve` requirement; `scoutBudget`, bound to the scout; the `SoftLanding` requirement and `TouchdownCheck`, a verification case whose body decides its verdict with `VerificationCases::PassIf` |
| `Selection` | two `TradeStudy` usages over the three candidates — `lightest` minimizes, `mostValuable` maximizes and takes a `fuelCost` parameter |
| `Timing` | `GroundWatch`, an action that waits five seconds and reads whether the scout is braking, and `watchDescent`, an analysis that performs it |

Build the tool first with `make build`; every command below runs from the
repository root.

## Run an analysis case

`FuelBudget` is a definition: a subject, an input, two `then`-chained action
steps that feed each other through their parameters, two `out` values, an
objective and a `return`. `scoutBudget` binds the subject to the scout, so it
runs by name:

```bash
./bin/sysml examples/analysis-demo/lander.sysml -analysis Descent::scoutBudget
```

```
✓ package Landers
✓ package Descent
✓ package Selection
✓ package Timing
✓ Descent::scoutBudget
  fuelUsed = 120.0
  wetMass = 730.0
  fuelLeft = 130.0
  objective reserveHeld: satisfied
```

(The four `✓ package` lines say the model loaded and analysed cleanly; every
run prints them, and the rest of this page leaves them out.)

The steps ran in order — `burn` used `3.0 × 40.0`, `remaining` read `burn.used`
— and then the `out` values and the return were evaluated over what the steps
left behind. The objective is a requirement usage; it is evaluated last, over
the same values, and reported on its own line. It decides the exit status: `0`
here.

**Pass arguments.** Inputs are given in parentheses, by name. Burning twice as
long leaves 10 kg, under the 20 kg reserve, and the run exits `1`:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -analysis "Descent::scoutBudget(burnTime = 80.0)"
```

```
✗ Descent::scoutBudget(burnTime = 80.0)
  fuelUsed = 240.0
  wetMass = 610.0
  fuelLeft = 10.0
  objective reserveHeld: not satisfied: fuelLeft >= reserve
```

**Run the definition on an object.** `FuelBudget` itself has no subject, so
running it bare is refused with a message saying what to do (exit `2`, the
status for a run that could not start). Instantiate a candidate and name the
object after the case:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -instantiate Landers::hauler -analysis "Descent::FuelBudget Landers::hauler"
```

```
✓ Created instance of Landers::hauler
  ID: 1
  Use %features Landers::hauler to inspect
✓ Descent::FuelBudget on object #1 of "Landers::hauler"
  fuelUsed = 320.0
  wetMass = 1980.0
  fuelLeft = 580.0
  objective reserveHeld: satisfied
```

**Watch it run.** `-trace` prints every binding, step and evaluation. Two
things it shows that the summary does not: the subject is *materialized* — the
scout's `flightMode` state machine starts when the case does — and the
objective is evaluated after the `return`:

```bash
./bin/sysml examples/analysis-demo/lander.sysml -trace -analysis Descent::scoutBudget
```

```
[trace] enter analysis Descent::scoutBudget
[trace] materialize: scout #1
[trace] start: exhibited state machine flightMode of #1
…
[trace]   bind lander = instance#1 [default]
[trace]   bind burnTime = 40.0 [default]
[trace] enter action node: analysis Descent::scoutBudget
…
[trace]     stmt assign used
…
[trace]       eval operator * -> 120.0
…
[trace]     stmt assign left
…
[trace]       eval operator - -> 130.0
[trace] leave action node: analysis Descent::scoutBudget
[trace]   stmt return
[trace] exit analysis Descent::scoutBudget -> 130.0
[trace] output Descent::scoutBudget.fuelUsed = 120.0
[trace] output Descent::scoutBudget.wetMass = 730.0
…
[trace] eval operator >= -> true
```

**Machine-readable output.** `-json` wraps the same run: `status` and `exit`
at the top, and per check the printed `lines` plus a `values` list with one
entry per output and objective.

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -json -analysis "Descent::scoutBudget(burnTime = 80.0)"
```

```json
{
  "status": "fails",
  "exit": 1,
  "checks": [
    {
      "subject": "Descent::scoutBudget(burnTime = 80.0)",
      "status": "fails",
      "values": [
        { "name": "fuelUsed", "value": "240.0" },
        { "name": "wetMass", "value": "610.0" },
        { "name": "fuelLeft", "value": "10.0" },
        { "name": "objective reserveHeld", "value": "not satisfied: fuelLeft >= reserve" }
      ],
      …
    }
  ],
  …
}
```

## Verify a requirement

A verification case reports two things, and they are decided differently. Its
*objective* verifies a requirement and is evaluated like the analysis objective
above. Its *verdict* comes from running the body, where
`VerificationCases::PassIf(<condition>)` produces `VerdictKind::pass` or
`VerdictKind::fail`. `checkScout` binds the subject to the scout, whose 1.2 m/s
is under the 1.5 m/s limit:

```bash
./bin/sysml examples/analysis-demo/lander.sysml -analysis Descent::checkScout
```

```
✓ Descent::checkScout
  result = VerdictKind::pass
  objective obj: satisfied
  ✓ Verification Descent::checkScout verdict: pass
```

The hauler lands at 1.9 m/s. Running the definition on a hauler object fails
both ways, and the exit status is `1`:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -instantiate Landers::hauler -analysis "Descent::TouchdownCheck Landers::hauler"
```

```
✗ Descent::TouchdownCheck on object #1 of "Landers::hauler"
  result = VerdictKind::fail
  objective obj: not satisfied: lander.touchdownSpeed <= 1.5
  ✗ Verification Descent::TouchdownCheck verdict: fail
```

The two can disagree — a body that returns `PassIf(false)` next to an objective
that holds is a case whose body does not measure what its objective states —
and that is the point of printing both. A body without `PassIf` or a verdict
value is `inconclusive`, and a body that errors reports the error as its
verdict.

Verification cases also run when their requirement is asked about: `-requirement`
and `-satisfy` report the body verdict of every case that verifies the
requirement beside the satisfaction result.

```bash
./bin/sysml examples/analysis-demo/lander.sysml -requirement Descent::scoutLandsSoftly
```

```
✓ Requirement Descent::scoutLandsSoftly satisfied
✓ Verification Descent::checkScout verdict: pass
```

## Sweep a parameter

`-sweep <param>=<from>..<to>[:<step>]` runs the case once per value and prints
one row per run — inputs, outputs, return, verdicts, wall time. Any row whose
objective is unsatisfied makes the exit status `1`, so a sweep doubles as a
check of where the design stops holding:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -analysis Descent::scoutBudget -sweep "burnTime=20.0..80.0:20.0"
```

```
sweep Descent::scoutBudget — 4 run(s)
burnTime | fuelUsed | wetMass | fuelLeft | verdict                    | time
---------+----------+---------+----------+----------------------------+--------
20.0     | 60.0     | 790.0   | 190.0    | reserveHeld: satisfied     | 0.142ms
40.0     | 120.0    | 730.0   | 130.0    | reserveHeld: satisfied     | 0.082ms
60.0     | 180.0    | 670.0   | 70.0     | reserveHeld: satisfied     | 0.075ms
80.0     | 240.0    | 610.0   | 10.0     | reserveHeld: not satisfied | 0.073ms
```

**Sample instead of enumerating.** `-samples <n>` draws `n` values uniformly
from the range in place of stepping through it; `-seed` fixes the draw, so the
same seed reproduces the same rows on any machine. Leave the step off the range
when sampling.

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -analysis Descent::scoutBudget -sweep "burnTime=20.0..80.0" -samples 4 -seed 7
```

```
samples Descent::scoutBudget — 4 run(s), seed 7
burnTime          | fuelUsed           | wetMass           | fuelLeft           | verdict                    | time
------------------+--------------------+-------------------+--------------------+----------------------------+--------
21.90164941545121 | 65.70494824635364  | 784.2950517536464 | 184.29505175364636 | reserveHeld: satisfied     | 0.087ms
78.07478446864098 | 234.22435340592295 | 615.775646594077  | 15.775646594077045 | reserveHeld: not satisfied | 0.073ms
42.7187552135554  | 128.1562656406662  | 721.8437343593338 | 121.84373435933381 | reserveHeld: satisfied     | 0.069ms
51.35890304495193 | 154.07670913485578 | 695.9232908651443 | 95.92329086514422  | reserveHeld: satisfied     | 0.067ms
```

Sweeps take several `-sweep` ranges (one row per combination), work over
`-calc` as well as `-analysis`, and with `-json` the table arrives as `rows`
with each row's `inputs`, `outputs`, `verdicts` and `milliseconds`. The `time`
column is wall time and varies run to run; everything else is deterministic.

## Choose between alternatives

A `TradeStudy` from the `TradeStudies` library scores every alternative with
its `evaluationFunction` and selects by its objective. `lightest` lists the
three candidates as its subject, minimizes, and scores mass on the pad:

```bash
./bin/sysml examples/analysis-demo/lander.sysml -analysis Selection::lightest
```

```
✓ Selection::lightest
  selectedAlternative = Landers::relay (object #5)
  objective tradeStudyObjective: satisfied
  evaluationFunction(Landers::scout (object #1)) = 850.0
  evaluationFunction(Landers::hauler (object #3)) = 2300.0
  evaluationFunction(Landers::relay (object #5)) = 630.0 [selected]
```

Each alternative is materialized (the object numbers are the parts and their
state machines), scored in the order the subject lists them, and reported; the
selection is the `return`. `objective tradeStudyObjective: satisfied` says the
library's own check — that the selected score is the best one — holds. Two
alternatives with the same best score are reported `[tied]`, and an evaluation
the function cannot compute is reported as that alternative's error with the
objective undecided, rather than silently dropped.

**Parameterize the study.** `mostValuable` maximizes payload net of what its
fuel costs, at a rate the caller supplies:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -analysis "Selection::mostValuable(fuelCost = 0.1)"
```

```
✓ Selection::mostValuable(fuelCost = 0.1)
  selectedAlternative = Landers::hauler (object #3)
  objective tradeStudyObjective: satisfied
  evaluationFunction(Landers::scout (object #1)) = 15.0
  evaluationFunction(Landers::hauler (object #3)) = 210.0 [selected]
  evaluationFunction(Landers::relay (object #5)) = 7.0
```

Because `fuelCost` is a case parameter, the study sweeps like any other case,
and the table shows where the choice flips:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -analysis Selection::mostValuable -sweep "fuelCost=0.0..0.5:0.25"
```

```
sweep Selection::mostValuable — 3 run(s)
fuelCost | selectedAlternative         | verdict                        | evaluations                                                   | time
---------+-----------------------------+--------------------------------+---------------------------------------------------------------+--------
0.0      | Landers::hauler (object #3) | tradeStudyObjective: satisfied | …scout…) = 40.0; …hauler…) = 300.0 [selected]; …relay…) = 25.0   | 5.592ms
0.25     | Landers::hauler (object #3) | tradeStudyObjective: satisfied | …scout…) = -22.5; …hauler…) = 75.0 [selected]; …relay…) = -20.0 | 0.261ms
0.5      | Landers::relay (object #5)  | tradeStudyObjective: satisfied | …scout…) = -85.0; …hauler…) = -150.0; …relay…) = -65.0 [selected] | 0.254ms
```

(The `evaluations` column is abridged here; the tool prints each
`evaluationFunction(…)` call in full.)

This is a different question from `%optimize`, which asks a solver for the
values of free attributes that optimize an analysis's objectives. A trade study
has no free values: it evaluates a function over the finite alternatives the
model names and needs no solver.

## Two behaviors, one clock

Actions and state machines share the runtime's simulation clock. The scout's
`flightMode` coasts for five seconds and then brakes; `groundWatch` waits five
seconds and then reads `scout.braking`. Instantiate the scout so its state
machine is running, start the action, and advance the clock:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -instantiate Landers::scout -action Timing::groundWatch -advance 5
```

```
✓ Created instance of Landers::scout
  ID: 1
  Use %features Landers::scout to inspect
✓ Started action executor for "Timing::groundWatch"
  State: Running
  Tokens: 1
✓ Advanced to 5.0 (1 event(s) processed)
  Action state: Completed
  Tokens: 0
  1 choice point; %trace on to see them
  Action steps taken: 5
✓ Action completed
  Results:
    armed = true
    sawBraking = false
```

Both timers came due at `t=5.0`. Which runs first is not fixed by the model —
it is a *choice point*, and the default scheduling policy ran the action first,
so it read `braking` before the state machine set it. `-trace` names the choice:

```
[trace] choice at t=5.0: due state machine flightMode of object #1, action GroundWatch (unordered; ran action GroundWatch first)
```

**Pick the other order.** `-schedule` sets the policy for every choice point in
the run. Under `declared` the state machine, which the scout started first,
goes first, and the watch sees the brake:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -schedule declared -instantiate Landers::scout -action Timing::groundWatch -advance 5
```

```
  Results:
    armed = true
    sawBraking = true
```

**Enumerate every order.** `-schedule explore` runs each linearization of the
choice points and groups the runs by outcome, with a witness — the choices one
run reaching that outcome made:

```bash
./bin/sysml examples/analysis-demo/lander.sysml \
  -schedule explore -instantiate Landers::scout -action Timing::groundWatch -advance 5
```

```
✓ explored Timing::groundWatch: 2 outcomes
outcome                          | linearizations | witness
---------------------------------+----------------+----------------------------------------------------------------------------------------------------------------
armed = true; sawBraking = false | 1              | t=5.0: action GroundWatch first of action GroundWatch, state machine flightMode of object #1
armed = true; sawBraking = true  | 1              | t=5.0: state machine flightMode of object #1 first of action GroundWatch, state machine flightMode of object #1
complete (2 runs)
```

`complete (2 runs)` says every order within the run and depth budgets was
tried, so an outcome not listed cannot be reached by reordering alone. A
one-line answer to "does the order matter?" is one outcome or two.

**The same thing inside an analysis.** `watchDescent` performs `GroundWatch`
as a step. The subject binding materializes the scout, whose state machine then
runs on the case's clock, and an analysis run advances that clock itself when
its body waits on time — no `-advance` is needed:

```bash
./bin/sysml examples/analysis-demo/lander.sysml -analysis Timing::watchDescent
./bin/sysml examples/analysis-demo/lander.sysml -schedule explore -analysis Timing::watchDescent
```

```
✓ Timing::watchDescent
  sawBraking = false
✓ explored Timing::watchDescent: 2 outcomes
outcome            | linearizations | witness
-------------------+----------------+----------------------------------------------------------------------------------------------------------------
sawBraking = false | 1              | t=5.0: action GroundWatch first of state machine flightMode of object #1, action GroundWatch
sawBraking = true  | 1              | t=5.0: state machine flightMode of object #1 first of state machine flightMode of object #1, action GroundWatch
complete (2 runs)
```

This is the form to reach for when a case's answer depends on how concurrent
behavior interleaves: with `-json`, the exploration arrives as `outcomes`
(values, linearization count, witness) and an `exploration` record saying
whether it completed.

## The same questions from the REPL

Every command above has a REPL form; `%help` lists them. Start the REPL on the
model and ask:

```
sysml> %analysis Descent::scoutBudget(burnTime = 60.0)
sysml> %sweep Descent::scoutBudget burnTime=20.0..80.0:20.0
sysml> %samples 4 7 Descent::scoutBudget burnTime=20.0..80.0
sysml> %analysis Selection::lightest
sysml> %schedule declared
sysml> %instantiate Landers::scout
sysml> %action Timing::groundWatch
sysml> %advance 5
```

Two things differ from the one-shot command line. `%schedule <policy>` sets the
policy for the rest of the session. And objects stay live: the three landers
`%analysis Selection::lightest` materialized are still coasting when
`%advance 5` runs, so their state machines fire alongside the scout's and the
watch's — "4 event(s) processed", four choice points — where the command line
reported one. `%trace on` shows each of them.

## The same questions from Python

[`lander_demo.py`](lander_demo.py) asks the same questions through the
`opensysml` client and prints one section per case:

```bash
python examples/analysis-demo/lander_demo.py
```

```
== the fuel budget of one descent
fuel left     130.0 kg after 120.0 kg burned
reserve held  True
80 s burn     10.0 kg left, reserve held False
              ✗ objective reserveHeld fails (on Landers::scout ID: 1): condition evaluated to false: fuelLeft >= reserve
the hauler    580.0 kg left, reserve held True

== how far the budget stretches
burnTime  20.0  fuelLeft  190.0  reserve holds
burnTime  40.0  fuelLeft  130.0  reserve holds
burnTime  60.0  fuelLeft   70.0  reserve holds
burnTime  80.0  fuelLeft   10.0  reserve fails
seed 7: 4 draws, 3 within the reserve

== does the scout land softly?
body verdict  pass
objective     satisfied
the hauler    fail

== which lander?
Landers::scout     850.0
Landers::hauler   2300.0
Landers::relay     630.0 [selected]
fuel at 0.0   -> Landers::hauler
fuel at 0.25  -> Landers::hauler
fuel at 0.5   -> Landers::relay

== the watch and the flight mode on one clock
sawBraking = False (1 order): t=5.0: action GroundWatch first of state machine flightMode of object #1, action GroundWatch
sawBraking = True  (1 order): t=5.0: state machine flightMode of object #1 first of state machine flightMode of object #1, action GroundWatch
```

The calls map one to one onto the command line: `Model.run_analysis(case,
subject=…, named_arguments=…)` is `-analysis`, returning an `AnalysisResult`
whose `outputs`, `verdicts`, `verifications`, `evaluations` and `selected` are
the lines printed above; `Model.run_sweep(case, {param: (from, to[, step])},
samples=, seed=)` is `-sweep`, returning a `SweepTable` of `SweepRow`s that are
truthy when their verdicts hold; `Model.explore_analysis(case)` is
`-schedule explore -analysis`, returning an `Exploration` of `Outcome`s. Load
with `strict=True` so a model that does not analyse cleanly raises at
`opensysml.load` rather than at the first case that touches the fault.

## Where to read more

- [Running an analysis case](../../docs/guide/06-behavior.md#running-an-analysis-case),
  with its [verification cases](../../docs/guide/06-behavior.md#verification-cases)
  and [trade studies](../../docs/guide/06-behavior.md#trade-studies): what a run
  binds, in what order it evaluates, and how verdicts are decided.
  [When a model has more than one valid run](../../docs/guide/06-behavior.md#when-a-model-has-more-than-one-valid-run)
  covers choice points, the shared clock, scheduling policies and `explore`.
- [Running behavior](../../docs/guide/03-command-line.md#running-behavior) and
  [machine-readable results](../../docs/guide/03-command-line.md#machine-readable-results)
  in the command-line guide; the [CLI reference](../../docs/reference/cli.md) has
  every flag used above, [sweeping a parameter](../../docs/reference/cli.md#sweeping-a-parameter),
  [exploring every linearization](../../docs/reference/cli.md#exploring-every-linearization)
  and the [exit statuses](../../docs/reference/cli.md#exit-status).
- [REPL command reference](../../docs/reference/repl-commands.md): `%analysis`,
  `%sweep`, `%samples`, `%schedule`, `%advance`, and how `%optimize` differs.
- [Clients](../../docs/guide/09-clients.md): installing the Python client and
  how it finds or starts a `sysml-grpc` service.
