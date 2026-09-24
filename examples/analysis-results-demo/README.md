# Analysis results demo: saving runs into the model and reporting them

[`lander-results.sysml`](lander-results.sysml) asks one question: **can the
results of analysis runs be saved into the model itself, so a generated
document can tabulate them later?**

The honest answer is *not by itself, but yes by pattern*. `-analysis` and
`-sweep` print each run's inputs, outputs and verdicts and then discard them:
a run leaves no element in the model and no held object a document query can
read, and `-render-document` cannot be combined with `-analysis`, so the
document never sees a run happen. What a document *can* see is anything the
model declares — so the recording pattern is to write each run back yourself:
a `part` usage typed by a result-record definition, holding the run's inputs,
outputs and objective as attribute values, annotated with provenance metadata
and pointing at the part the run was about. [`report.md`](report.md) is what
the document then renders.

## The recording pattern

The `Records` package declares the vocabulary:

```sysml
metadata def RecordedRun {
    attribute runAt : String;
    attribute tool : String;
    attribute revision : String;
    attribute command : String;
}

part def AnalysisRun {
    attribute caseName : String;
    attribute kind : String;        // "run" | "sweep" | "trade"
    attribute 'objective' : String; // "satisfied" | "not satisfied" | "undecided"
    attribute command : String;
}

part def FuelBudgetRun :> AnalysisRun {
    ref part lander : Lander;
    attribute subjectName : String;
    attribute burnTime : Real;
    attribute fuelUsed : Real;
    attribute wetMass : Real;
    attribute fuelLeft : Real;
    attribute liveFuelLeft : Real = lander.fuel - burnTime * lander.burnRate;
    attribute drift : Real = liveFuelLeft - fuelLeft;
    attribute stale : Boolean = drift != 0.0;
}
```

Each run is then one usage in the `Results` package, filled in from the
printed output:

```sysml
part scoutRun : FuelBudgetRun {
    @RecordedRun {
        runAt = "2025-11-02T09:14:00Z";
        tool = "sysml";
        revision = "v0.8";
        command = "./bin/sysml examples/analysis-results-demo/lander-results.sysml -analysis Descent::scoutBudget";
    }
    ref part :>> lander = scout;
    attribute :>> caseName = "Descent::scoutBudget";
    attribute :>> kind = "run";
    attribute :>> 'objective' = "satisfied";
    attribute :>> subjectName = "scout";
    attribute :>> burnTime = 40.0;
    attribute :>> fuelUsed = 120.0;
    attribute :>> wetMass = 730.0;
    attribute :>> fuelLeft = 130.0;
}
```

Two limitations shape the record. The annotation's attribute values are not
projectable — `Project(properties = ("runAt"))` reports `unknown property` —
so `command` is also carried as a plain attribute for the provenance table to
show; the `@RecordedRun` metadata still answers `WhereMetadata` filters and
keeps the provenance machine-readable. And `objective` is a reserved word,
written `'objective'` wherever a name is needed.

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
  -run-query "Reporting::StaleRuns root=results"
```

```
✓ Query Reporting::StaleRuns returned 1 row
  Columns: name, subjectName, burnTime, fuelLeft, liveFuelLeft, drift
  Row 1: Results::results::relayRun
    fuelLeft = 80.0
    liveFuelLeft = 110.0
    drift = 30.0
```

The relay edit made a second record stale too: `lightestRun` still reports
`relayScore = 630.0` while `-analysis Selection::lightest` now scores relay
`660.0`. Nothing flags it — `TradeStudyRun` rederives nothing, so it has no
`liveFuelLeft` to compare against. That is the limitation the paragraph above
describes, made concrete: drift detection only exists where the record
definition recomputes the value itself, which is exactly what an automated
record step would have to emit for every output it saves.

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
render byte-for-byte. It shows a grouped table of every fuel-budget record by
subject, the sweep rows alone, the stale-records table (exactly `relayRun`),
a provenance table over `WhereMetadata(... 'metadata' = "Records::RecordedRun")`,
the trade-study record, and — the contrast — a `Verdicts` table of the
assertions about `scout` **evaluated live at render time**: the records say
what a run printed; the verdicts say what holds now.

HTML and PDF render the same document tree:

```bash
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -render-document Reporting::AnalysisReport -doc-form html -o report.html
./bin/sysml examples/analysis-results-demo/lander-results.sysml \
  -render-document Reporting::AnalysisReport -doc-form pdf -o report.pdf
```

## Why isn't this automatic?

Because nothing bridges two surfaces the tool already has — an implementation
gap, not an architectural limitation. The runtime already holds each
analysis run's results as typed values, `%save` already serializes the
session model, and document queries already read declared elements and held
objects; but analysis output is printed and discarded, no step writes it
back, and `-render-document` cannot run alongside `-analysis`. The pattern
this demo records by hand — a `part` usage typed by a result-record
definition, provenance metadata, a `ref part` to the subject — is the shape
an automated record step would emit.

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
