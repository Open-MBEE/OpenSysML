# Analysis results demo: saving runs into the model and reporting them

[`lander-results.sysml`](lander-results.sysml) asks one question: **can the
results of analysis runs be saved into the model itself, so a generated
document can tabulate them later?**

The honest answer is *yes — by recording them, which the tool can now do
for you*. `-analysis` and `-sweep` print each run's inputs, outputs and
verdicts, but a run leaves nothing a document query can read until it is
**recorded**: `%record` and `-record-run` write the run back into the model as
a `part` usage typed by a result-record definition in the bundled
`AnalysisRecords` library — inputs, outputs, objective status, verdicts and
evaluations, annotated with provenance metadata and a `ref` to the part the
run was about. This demo's records are the same shape, written by hand (why,
below). [`report.md`](report.md) is what the document then renders.

## The recording pattern

The `Records` package builds the demo's record definitions on the bundled
`AnalysisRecords` library, which supplies `RecordedRun` (provenance
metadata: `runAt`, `tool`, `command`, `kind`), `AnalysisRun` (`caseName`,
`kind`, `'objective'`, `iteration`, `'subject'`, `subjectName`, `verdict`,
plus `verdicts` and `evaluations` collections), `VerdictRecord` and
`EvaluationRecord`:

```sysml
part def DemoRun :> AnalysisRecords::AnalysisRun {
    attribute command : String;
}

part def FuelBudgetRun :> DemoRun {
    ref part lander : Lander;
    attribute burnTime : Real;
    attribute fuelUsed : Real;
    attribute wetMass : Real;
    attribute fuelLeft : Real;
    attribute liveFuelLeft : Real = lander.fuel - burnTime * lander.burnRate;
    attribute drift : Real = liveFuelLeft - fuelLeft;
    attribute stale : Boolean = drift != 0.0;
}
```

Each run is then one usage at the top level of the `Records` package — the
same package `-record-run` writes its records into — filled in from the
printed output. The definitions themselves sit in `Records::Vocab`, one
nesting level down: the report's queries walk the package's direct members,
and a `part def` matching `WhereType` would surface its own unbound features
as a bogus row.

```sysml
part scoutRun : FuelBudgetRun {
    @AnalysisRecords::RecordedRun {
        runAt = "2025-11-02T09:14:00Z";
        tool = "sysml";
        command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget";
        kind = "run";
    }
    attribute :>> caseName = "Descent::scoutBudget";
    attribute :>> kind = "run";
    attribute :>> 'objective' = "satisfied";
    ref :>> 'subject' = Landers::scout;
    ref part :>> lander = scout;
    attribute :>> subjectName = "Landers::scout";
    attribute :>> burnTime = 40.0;
    attribute :>> fuelUsed = 120.0;
    attribute :>> wetMass = 730.0;
    attribute :>> fuelLeft = 130.0;
    part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
        attribute :>> kind = "objective";
        attribute :>> name = "reserveHeld";
        attribute :>> status = "satisfied";
    }
}
```

Two limitations shape the record. The annotation's attribute values are
not projectable — `Project(properties = ("runAt"))` reports
`unknown property` — so `command` is also carried as a plain attribute on
`DemoRun` for the provenance table to show; the `@AnalysisRecords::RecordedRun`
metadata still answers `WhereMetadata` filters and keeps the provenance
machine-readable. And the library's untyped `'subject'` ref cannot drive the
`liveFuelLeft` formula, so `FuelBudgetRun` keeps a typed `lander` ref and each
record binds both to the same part. Sweep records set the library's
`iteration` attribute (1, 2, 3); the trade-study record carries its scores as
`EvaluationRecord` usages under `evaluations`, the same shape `-record-run`
emits.

## The runs that were recorded

One baseline run per candidate, at `FuelBudget`'s default 40 s burn. **Two of
these transcripts are records, not what the commands print today**: the relay
run and the trade study were captured before `relay.fuel` was edited from
`180.0` to `210.0`, which is what makes the staleness section below possible.
Each is labelled; the scout and hauler runs print the same numbers now.

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget
```

```
✓ Descent::scoutBudget
  fuelUsed = 120.0
  wetMass = 730.0
  fuelLeft = 130.0
  objective reserveHeld: satisfied
```

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::haulerBudget
```

```
  fuelUsed = 320.0
  wetMass = 1980.0
  fuelLeft = 580.0
```

The relay run, **as recorded** (when `relay.fuel` was `180.0`):

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::relayBudget
```

```
  fuelUsed = 100.0
  wetMass = 530.0
  fuelLeft = 80.0
```

The same command **now prints** — the difference the `relayRun` record
detects:

```
  fuelUsed = 100.0
  wetMass = 560.0
  fuelLeft = 110.0
```

A sweep of `scoutBudget`, one record per row (this README's tables omit the
`time` column, which is wall time):

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -analysis Descent::scoutBudget -sweep "burnTime=40.0..80.0:20.0"
```

```
burnTime | fuelUsed | wetMass | fuelLeft | verdict
---------+----------+---------+----------+----------------------------
40.0     | 120.0    | 730.0   | 130.0    | reserveHeld: satisfied
60.0     | 180.0    | 670.0   | 70.0     | reserveHeld: satisfied
80.0     | 240.0    | 610.0   | 10.0     | reserveHeld: not satisfied
```

The 80 s row leaves the objective unsatisfied, so the sweep exits `1`; the
transcripts here and above show the result lines only, not the package
loading and `standing` lines every run also prints.

And the trade study, **as recorded** before the relay edit:

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Selection::lightest
```

```
✓ Selection::lightest
  selectedAlternative = Landers::relay (object #3)
  objective tradeStudyObjective: satisfied
  evaluationFunction(Landers::scout (object #1)) = 850.0
  evaluationFunction(Landers::hauler (object #2)) = 2300.0
  evaluationFunction(Landers::relay (object #3)) = 630.0 [selected]
```

The same command **now prints** `660.0` for relay (`450.0 + 210.0`) — the
selection is still `relay`, but the score the record saved no longer is.

## Records go stale — and can say so

The `relay` part was edited *after* its run was recorded: `fuel` went from
`180.0` to `210.0` (the attribute carries a comment saying so). The record
still holds the printed `fuelLeft = 80.0`, but `liveFuelLeft` recomputes the
same formula from the model as it now stands — `210.0 - 40.0 * 2.5 = 110.0` —
so `drift = 30.0` and `stale = true`. That is the one thing a printed report
can never give you: a record that notices the model moved.

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -run-query "Reporting::StaleRuns"
```

```
✓ Query Reporting::StaleRuns returned 1 row
  Columns: name, subjectName, burnTime, fuelLeft, liveFuelLeft, drift
  Row 1: Records::relayRun
    fuelLeft = 80.0
    liveFuelLeft = 110.0
    drift = 30.0
```

The relay edit made a second record stale too: `lightestRun`'s third
`EvaluationRecord` still says `score = 630.0` while `-analysis
Selection::lightest` now scores relay `660.0`. Nothing flags it —
`EvaluationRecord` rederives nothing, so it has no `liveFuelLeft` to compare
against. That is the limitation the paragraph above describes, made concrete:
drift detection only exists where the record definition recomputes the value
itself, which neither the library's records nor `-record-run`'s output does —
a recompute-def like `FuelBudgetRun` is something a modeler writes on purpose.

The comparison is a derived Boolean on the record definition rather than a
`Column` expression, because computed columns do not support `!=`. Any model
edit can produce drift this way — only records typed by a definition that
recomputes an output can detect it, and only for the values it rederives.

## Rendering the records

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -render-document Reporting::AnalysisReport -o examples/analysis-results-demo/report.md
```

[`report.md`](report.md) is committed so the test suite can compare the
render byte-for-byte. It opens with a table of **every recorded run** —
`WhereMetadata(... 'metadata' = "AnalysisRecords::RecordedRun")` filtered to
`WhereType(... type = "AnalysisRecords::AnalysisRun")` over the `Records`
package's direct members — then a grouped table of every fuel-budget record
by subject, the sweep rows alone, the stale-records table (exactly
`relayRun`), a provenance table over the same `WhereMetadata` filter, the
trade-study record and its `EvaluationRecord`s, and — the contrast — a
`Verdicts` table of the assertions about `scout` **evaluated live at render
time**: the records say what a run printed; the verdicts say what holds now.

HTML and PDF render the same document tree:

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -render-document Reporting::AnalysisReport -doc-form html -o report.html
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -render-document Reporting::AnalysisReport -doc-form pdf -o report.pdf
```

## Recording runs automatically

Everything above was written by hand; the same vocabulary is what
`-record-run` emits. Running a case with `-record-run` records it into a
`Records` package beside the case and, with `-convert sysml`, writes the
model — records included — back out:

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -record-run "Descent::scoutBudget" -convert sysml -o recorded.sysml
```

```
✓ Descent::scoutBudget
  fuelUsed = 120.0
  wetMass = 730.0
  fuelLeft = 130.0
  objective reserveHeld: satisfied
  standing: value (observed: 1 run under reverse)
  recorded Records::scoutBudget_run1 (Records::ScoutBudgetRun)
```

The generated record is the same shape this demo writes by hand — a def per
case specializing `AnalysisRecords::AnalysisRun`, the library annotation, the
subject ref, and a `VerdictRecord` per check:

```sysml
part def ScoutBudgetRun :> AnalysisRecords::AnalysisRun {
    attribute :>> caseName default = "Descent::scoutBudget";
    attribute burnTime : ScalarValues::Real;
    ...
}
part scoutBudget_run1 : ScoutBudgetRun {
    @AnalysisRecords::RecordedRun {
        runAt = "2026-09-24T06:08:21Z";
        tool = "sysml v0.8.1-2129-ge8b389eea";
        command = "-record-run \"Descent::scoutBudget\"";
        kind = "run";
    }
    ...
    ref :>> 'subject' = Landers::scout;
    part verdict1 : AnalysisRecords::VerdictRecord :> verdicts {
        attribute :>> kind = "objective";
        attribute :>> name = "reserveHeld";
        attribute :>> status = "satisfied";
    }
}
```

`-record-run` targets the `Records` package because it is the plain
top-level package beside `Descent` — the same package this demo's
hand-written records live in, so the generated record lands where the
report's queries already walk. Render the document in the same invocation
and the new record joins it:

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -record-run "Descent::scoutBudget" \
  -render-document Reporting::AnalysisReport -o recorded-report.md
```

The *Every recorded run* table — annotated `AnalysisRun`s, whatever their
definition — picks it up as an eighth row:

```
| name | caseName | kind | subjectName | objective |
| --- | --- | --- | --- | --- |
| scoutRun | Descent::scoutBudget | run | Landers::scout | satisfied |
| haulerRun | Descent::haulerBudget | run | Landers::hauler | satisfied |
| relayRun | Descent::relayBudget | run | Landers::relay | satisfied |
| scoutSweep40 | Descent::scoutBudget | sweep | Landers::scout | satisfied |
| scoutSweep60 | Descent::scoutBudget | sweep | Landers::scout | satisfied |
| scoutSweep80 | Descent::scoutBudget | sweep | Landers::scout | not satisfied |
| lightestRun | Selection::lightest | trade |  | satisfied |
| scoutBudget\_run1 | Descent::scoutBudget | run | Landers::scout | satisfied |
```

The fuel-budget, sweep and stale tables do *not* pick it up — they filter
`WhereType(... type = "FuelBudgetRun")`, and the generated def specializes
`AnalysisRun`, not `FuelBudgetRun` — and the provenance table lists it with
an empty `command` cell, since only the hand-written `DemoRun` declares that
projectable attribute. The REPL form is `%record`; sweeps record one record
per row (`recorded 3 runs as Records::scoutBudget_run1 …`), though `-convert`
refuses a sweep — write the model out after a single `-record-run`, or
record each row explicitly. The full flag reference is
[the manual](../../docs/manual/recording-analysis-runs.md).

These records stay hand-written for two reasons: the stale-relay story needs
values from *before* `relay.fuel` changed — a `-record-run` today would
record `fuelLeft = 110.0` and no drift — and the derived `liveFuelLeft` /
`drift` / `stale` columns live on the shared `DemoRun`/`FuelBudgetRun` defs,
while `-record-run` writes one def per case with no recompute.

## Where to read more

- [analysis-demo](../analysis-demo/README.md): the same lander asked with
  `-analysis`, `-sweep`, trade studies and scheduling policies — everything
  this demo records.
- [verdicts-demo](../verdicts-demo/README.md): `Verdicts(...)` rows in depth.
- The [query cookbook](../../docs/manual/query-cookbook.md):
  [metadata filters](../../docs/manual/query-cookbook.md#metadata-filters),
  [property filters](../../docs/manual/query-cookbook.md#property-filters),
  [sorting](../../docs/manual/query-cookbook.md#sorting) and
  [derived values](../../docs/manual/query-cookbook.md#derived-values), and
  the [authoring manual](../../docs/manual/authoring.md) for
  `Table`, `groupBy` and `Definitions`.
