# Recording analysis runs

`%record` at the prompt — `-record-run` on the command line — runs an analysis
case exactly as `%analysis`/`-analysis` does, reports the same verdict, and then
writes the run **into the model** as elements of the bundled `AnalysisRecords`
library: a record definition, one part per run carrying every input bound and
output produced, and provenance metadata stating when the run was made, by what
tool, with what command, and of which kind (`run`, `trade`, `sweep`, `runs`
or `sample`).

The records are ordinary model elements, so a document query finds them and a
document renders them — a run log lives in the model beside the cases it
records.

## Recording a run

```sysml
package Demo {
    private import ScalarValues::*;
    part def Probe { attribute t : Real = 3.0; }
    part probe : Probe;

    analysis def Check {
        subject s : Probe;
        in gain : Real;
        out x : Real = s.t + gain;
    }
    analysis timed : Check { subject s = probe; in gain = 2.0; }
}
```

```text
%record Demo::timed
✓ Demo::timed
  x = 5.0
  standing: value (observed: 1 run under reverse)
  recorded Records::timed_run1 (Records::TimedRun)
```

The run goes into a `Records` package beside the package enclosing the case's —
a top-level `Records` when the case is `Demo::timed`, `A::Records` for a case in
a package `A::Descent` declares — and into a record definition named for the case
(`TimedRun`), specializing `AnalysisRecords::AnalysisRun`. `%record ... into
<pkg>` names the package instead. The record part holds a redefinition for
each input and output of the case, plus the `caseName`, `kind`, `'objective'`
and `subjectName`/`subject` features `AnalysisRun` declares — `iteration`
only on a sweep or sample's records — and a `ref` to its subject; verdicts and
evaluations a trade study or verification made become
`VerdictRecord`/`EvaluationRecord` parts under `verdicts`/`evaluations`. A
parameter declared `inout` is one member — the record carries the value the
run left in it — plus a `<name>In` companion carrying the value it was bound
with, just as a quantity carries a `<name>Unit` companion naming the unit. A
verification case's record also carries what its body decided — the `verdict`
attribute (`"pass"`, `"fail"`, `"inconclusive"` or `"error"`) — and one
`VerdictRecord` row apiece for the body's verdict (`kind` `"verification"`)
and each subcase's (`kind` `"subcase"`):

```sysml
package Records {
    part def TimedRun :> AnalysisRecords::AnalysisRun {
        attribute :>> caseName default = "Demo::timed";
        attribute gain : ScalarValues::Real;
        attribute x : ScalarValues::Real;
    }
    part timed_run1 : TimedRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2026-09-24T02:21:14Z";
            tool = "sysml dev";
            command = "%record Demo::timed";
            kind = "run";
        }
        attribute :>> caseName = "Demo::timed";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "undecided";
        ref :>> 'subject' = Demo::probe;
        attribute :>> subjectName = "Demo::probe";
        attribute :>> gain = 2.0;
        attribute :>> x = 5.0;
    }
}
```

The definition's `caseName` marks the case it records. A sibling case of the
same name (`Demo::B::check` beside `Demo::A::check`) gets a definition of its
own, named from its owner (`B_checkRun`), rather than taking the first case's
over.

Recording a second run of the same case reuses the definition and numbers the
part on (`timed_run2`) — including after the model was saved and reloaded.

## Sweeps and Monte Carlo

On the command line the run a `-record-run` makes takes the same bounds the
matching check takes: with `-sweep` the case runs once per row as `-sweep`
makes it, and one record per row is written (`kind = "sweep"`, `iteration`
the row); with `-runs <n>` and `-seed` a `Simulation::MonteCarlo` case is
sampled as `-runs` does and each seeded run is recorded (`kind = "runs"`),
with the sample's conclusion — the statistics, the result and the checks that
are the sample's — recorded once more beside them (`kind = "sample"`):

```bash
$ sysml model.sysml -record-run "Demo::timed" -sweep "gain=1..3" -convert sysml -o saved.sysml
sweep Demo::timed — 3 run(s)
  recorded 3 runs as Records::timed_run1 … timed_run3
wrote saved.sysml (sysml, 2278 bytes)
```

`-convert sysml` writes the session text the records joined — the model plus
the `Records` package — formatted; loading `saved.sysml` and recording again
produces `timed_run4`. `-record-into <pkg>` names the records' package;
`-render-document` composes the same way, recording first so the document's
queries see the records. A failed sweep row is skipped and counted; a sampled
run whose declared output could not be read is not recorded — `run N not
recorded: <err>` names it. An output bound to a statistic of the sample
(`mean`, `deviation`, …) is the sample's and appears only on its record. An
output whose binding draws a random value is evaluated afresh on every read,
as the runtime's checks and conclusion do: its recorded value is one such
evaluation, made without moving the draws the sample's runs and conclusion see.
A run that fails records nothing and leaves the
model untouched — the record submission is atomic: the diagnostics it produced
are reported and the model is as it was.

## Reading the records

The `@AnalysisRecords::RecordedRun` metadata makes every record findable by
`WhereMetadata`, and its features are ordinary values `WhereFeature` and
`Project` read — see [Which query is which](query-kinds.md#object-rows-and-verdict-rows).
This run goes into `Demo::Log` instead, so the query's `root=Demo` subtree
contains it:

```text
%record Demo::timed into Demo::Log
✓ Demo::timed
  x = 5.0
  standing: value (observed: 1 run under reverse)
  recorded Demo::Log::timed_run1 (Demo::Log::TimedRun)
```

```sysml
calc def TimedRuns :> DocumentQueries::Query {
    in root : Element;
    Project(source = WhereFeature(
        source = WhereMetadata(
            source = Descendants(source = root, maxDepth = 10),
            'metadata' = "AnalysisRecords::RecordedRun"),
        'feature' = "caseName",
        operator = "=",
        value = "Demo::timed"),
        properties = ("name", "gain", "x"))
}
```

```text
%run-query TimedRuns root=Demo
✓ Query Demo::TimedRuns returned 1 row
  Columns: name, gain, x
  Row 1: Demo::Log::timed_run1
    name = "timed_run1"
    gain = 2.0
    x = 5.0
```

## The AnalysisRecords library

`internal/workspace/libs/stdlib/OpenSysML Libraries/AnalysisRecords.sysml`, a
non-normative OpenSysML extension bundled like `DocumentQueries`, declares the
vocabulary the records are written in:

- `RecordedRun` — the metadata annotation a record carries: `runAt` (the UTC
  timestamp), `tool`, `command`, `kind` (`"run"`, `"trade"`, `"sweep"`,
  `"runs"` or `"sample"`), and `tools` (each external tool call the run made,
  in call order, as `tool version from manifest: executable argv` — see
  [Running external programs from a model](running-external-programs.md)).
- `AnalysisRun` — the record definition's supertype: `caseName`, `kind`,
  `'objective'` (the run's objective verdict, `"undecided"` when the case
  declares none), `iteration` (its position in a sweep or sample), `'subject'`
  and `subjectName` (the object it ran on), `verdict` (what a verification
  case's body decided — `"pass"`, `"fail"`, `"inconclusive"` or `"error"`;
  unset for a case that is not a verification), and `verdicts`/`evaluations`.
- `VerdictRecord` — one check a run made: `kind` (`"objective"`, `"assertion"`,
  `"verification"` or `"subcase"`), `name`, `status`, `detail` (the violated
  condition, or why an undecided check could not be evaluated).
- `EvaluationRecord` — one trade-study evaluation: `function`, `alternative`,
  `score`, `result`, `selected`, `tied`, `error`.

## Limitations

- A value that is not a scalar, enum literal, quantity or resolvable reference
  is recorded as its printed String.
- A quantity is recorded as its Real magnitude plus a `<name>Unit` String
  companion naming the unit; an `inout` parameter as the value the run left
  plus a `<name>In` companion carrying the value it was bound with.
- An input or output left unset is declared on the record but not redefined.
- A member's type is settled from the values the runs supply; Integer and
  Real are one numeric family for it — either way the member is Real (an
  Integer literal is valid under it). A recorded value that is a scalar-valued
  enum literal keeps the literal (`= Grade::high`), not the scalar it equals.
- Records join only a package whose header is a plain `package Name`.
- Recording is exposed at the REPL and CLI; the gRPC surface does not expose
  it yet.
