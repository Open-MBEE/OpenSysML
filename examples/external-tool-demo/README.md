# External tool demo: a model running a program beside it

[`thermal.sysml`](thermal.sysml) asks one question: **can an analysis case hand
its work to a program that is not OpenSysML, and can the run still be previewed,
recorded and reported?**

Yes. The action `ThermalDemo::Solve` carries `@AnalysisTooling::ToolExecution`
naming the tool `ThermalSolver`, so its performance is given to that tool
instead of a body. The tool here is a three-argument Python script,
[`tools/thermal.py`](tools/thermal.py), that computes a steady-state
temperature and answers as CSV — the manifest entry
[`tools/thermal.json`](tools/thermal.json) says how OpenSysML composes the
command from the model's values (`invocation`) and reads the reply (`reply`).

## Setup

```bash
make build-sysml    # writes bin/sysml
```

A manifest directory is read only when the environment names one **outside
every workspace** — one under the working directory you load the model from is
refused, since a model must not be able to register a program — so copy the
demo's `tools/` somewhere outside the checkout before pointing
`OPENSYSML_TOOLS` at it:

```bash
cp -r examples/external-tool-demo/tools ~/.opensysml-tools
export OPENSYSML_TOOLS=~/.opensysml-tools
```

The entry's `executable` is the bare name `python3`, looked up on `PATH`; its
`cwd` is `.`, the manifest directory itself, which is why `thermal.py` lives
beside `thermal.json` — a `cwd` path must stay inside the manifest directory.

## What the manifest buys

`-engines` lists the tool as `tool:ThermalSolver`, and the `protocol` column
says how the entry composes the process: `argv+none/csv` — arguments over
argv, nothing on stdin, a CSV reply:

```bash
$ ./bin/sysml -engines
engine              kind      protocol       authority  answers                     status
...
tool:ThermalSolver  tool      argv+none/csv  observed   compute                     ready (ThermalSolver 1.0 at /usr/bin/python3)
tool:ThermalSolver 1.0: tool from /home/me/.opensysml-tools/thermal.json, runs python3
```

## Previewing the call

`-tool-dry-run` composes the invocation with the model's current values and
stops short of the process:

```bash
$ ./bin/sysml examples/external-tool-demo/thermal.sysml -tool-dry-run ThermalDemo::heating
✓ ThermalDemo::heating: dry run of tool 'ThermalSolver' for ThermalDemo::Solve
  tool: ThermalSolver 1.0
  protocol: argv+none/csv
  manifest: /home/me/.opensysml-tools/thermal.json
  executable: /usr/bin/python3
  argv:
    "thermal.py"
    "--mass"
    "12.5"
    "--power"
    "250"
    "--ambient"
    "290"
  ...
  reply: csv from stdout, header, delimiter ","
    margin: column "margin", row last, type number, unit K
    tMax: column "tmax", row last, unitColumn "tmaxUnit", type number
  the process was not started
```

## Running it

```bash
$ ./bin/sysml examples/external-tool-demo/thermal.sysml -analysis ThermalDemo::heating
✓ ThermalDemo::heating
  tMax = 306.0 [SI::K]
  margin = 94.0 [SI::K]
  objective marginHeld: satisfied
  standing: value (observed: 1 run under reverse)
```

## Recording it

`-record-run` writes the run into the model as an `AnalysisRecords` element in
the `Records` package beside the case, and `tools` on the `RecordedRun`
annotation names every external tool call the run made — the provenance a
printed report would have lost:

```bash
$ ./bin/sysml examples/external-tool-demo/thermal.sysml \
    -record-run ThermalDemo::heating -convert sysml -o recorded.sysml
✓ ThermalDemo::heating
  ...
  recorded Records::heating_run1 (Records::HeatingRun)
wrote recorded.sysml (sysml, 6030 bytes)
```

```sysml
part heating_run1 : HeatingRun {
    @AnalysisRecords::RecordedRun {
        runAt = "2026-10-15T11:24:00Z";
        tool = "sysml dev";
        command = "-record-run \"ThermalDemo::heating\"";
        kind = "run";
        tools = ("ThermalSolver 1.0 from /home/me/.opensysml-tools/thermal.json: /usr/bin/python3 thermal.py --mass 12.5 --power 250 --ambient 290");
    }
    ...
}
```

## Reporting it

Recording and rendering compose: one invocation records the run, then the
document's queries see the record. `Reporting::HeatingReport` projects every
`@RecordedRun`-annotated `AnalysisRun` in `Records` — an annotation's own
attributes are not projectable, so `tools` is read in the record's source
(above) rather than in the table:

```bash
$ ./bin/sysml examples/external-tool-demo/thermal.sysml \
    -record-run ThermalDemo::heating \
    -render-document Reporting::HeatingReport -o report.md
```

[`report.md`](report.md) is committed as the render the commands produce.

See [Running external programs from a
model](../../docs/manual/running-external-programs.md) for the whole story and
[the `OPENSYSML_TOOLS`
reference](../../docs/reference/environment.md#external-tools) for every
manifest member.
