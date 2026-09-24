# Recording analysis runs

`%record` at the prompt — `-record-run` on the command line — runs an analysis
case exactly as `%analysis`/`-analysis` does, reports the same verdict, and then
writes the run **into the model** as elements of the bundled `AnalysisRecords`
library: a record definition, one part per run carrying every input bound and
output produced, and provenance metadata stating when the run was made, by what
tool, with what command, and of which kind (`run`, `trade`, `sweep` or `runs`).

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

The run goes into a `Records` package in the package enclosing the case's —
`Demo::Records` when the case is `Demo::timed`, a top-level `Records` when it
has no enclosing package — and into a record definition named for the case
(`TimedRun`), specializing `AnalysisRecords::AnalysisRun`. `%record ... into
<pkg>` names the package instead. The record part holds a redefinition for
each input and output of the case, its `caseName`, `kind` and `iteration`, and
a `ref` to its subject; verdicts a trade study or verification made become
`VerdictRecord`/`EvaluationRecord` parts under `verdicts`/`evaluations`:

```sysml
package Records {
    part def TimedRun :> AnalysisRecords::AnalysisRun {
        attribute caseName : String;
        attribute gain : ScalarValues::Real;
        attribute x : ScalarValues::Real;
    }
    part timed_run1 : TimedRun {
        @AnalysisRecords::RecordedRun {
            runAt = "2026-01-01T00:00:00Z";
            tool = "sysml dev";
            command = "%record Demo::timed";
            kind = "run";
        }
        attribute :>> caseName = "Demo::timed";
        attribute :>> kind = "run";
        attribute :>> 'objective' = "undecided";
        attribute :>> iteration = 1;
        ref :>> 'subject' = Demo::probe;
        attribute :>> gain = 2.0;
        attribute :>> x = 5.0;
    }
}
```

Recording a second run of the same case reuses the definition and numbers the
part on (`timed_run2`) — including after the model was saved and reloaded.

## Sweeps and Monte Carlo

On the command line the run a `-record-run` makes takes the same bounds the
matching check takes: with `-sweep` the case runs once per row as `-sweep`
makes it, and one record per row is written (`kind = "sweep"`, `iteration`
the row); with `-runs <n>` and `-seed` a `Simulation::MonteCarlo` case is
sampled as `-runs` does and each seeded run is recorded (`kind = "runs"`):

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
queries see the records. A run that fails records nothing and leaves the
model untouched — the record submission is atomic: the diagnostics it produced
are reported and the model is as it was.

## Reading the records

The `@AnalysisRecords::RecordedRun` metadata makes every record findable by
`WhereMetadata`, and its features are ordinary values `WhereFeature` and
`Project` read — see [Which query is which](query-kinds.md#object-rows-and-verdict-rows):

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

`internal/workspace/libs/stdlib/OpenSysML Libraries/AnalysisRecords.sysml`,
bundled like `DocumentQueries`, declares what a record specializes:

- `AnalysisRecords::AnalysisRun` — the record definition's supertype; carries
  `caseName`, `kind`, `iteration` and `subjectName`, subsets `verdicts` and
  `evaluations`.
- `AnalysisRecords::RecordedRun` — the metadata annotation carrying
  `runAt`, `tool`, `command` and `kind`.
- `AnalysisRecords::VerdictRecord` — one asserted verdict: `subjectName`,
  `condition`, `status`, `reason`.
- `AnalysisRecords::EvaluationRecord` — one trade-study evaluation:
  `alternativeName`, `score`, `selected`.

## Limitations

- A value that is not a scalar, enum literal, quantity or resolvable reference
  is recorded as its printed String.
- A quantity is recorded as its Real magnitude plus a `<name>Unit` String
  companion naming the unit.
- An input or output left unset is declared on the record but not redefined.
- Records join only a package whose header is a plain `package Name`.
- Recording is exposed at the REPL and CLI; the gRPC surface does not expose
  it yet.
