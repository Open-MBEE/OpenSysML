# Running external programs from a model

An analysis case's action can be *performed by a program outside OpenSysML* — a
solver, a simulator wrapper, a script — rather than by a body written in SysML.
This chapter walks the whole loop on the worked example in
[`examples/external-tool-demo/`](https://github.com/Open-MBEE/OpenSysML/tree/develop/examples/external-tool-demo):
registering the program in a manifest, previewing what a run would give it,
running the case, recording the run with its tool provenance, and tabulating
the record in a document. The manifest members, the protocol the exchange
speaks and every failure's shape are reference material in [External
tools](../reference/environment.md#external-tools); this is the story.

## Declaring the tool

An action declares it is performed by a tool with the
`AnalysisTooling::ToolExecution` metadata, its `toolName` and `uri` carried to
the tool verbatim; each parameter taking part in the exchange carries
`@AnalysisTooling::ToolVariable` naming it as the tool's manifest spells it:

```sysml
action def Solve {
    metadata ToolExecution {
        toolName = "ThermalSolver";
        uri = "thermal://demo/solve";
    }
    in mass : MassValue           { @ToolVariable { name = "mass"; } }
    in power : PowerValue         { @ToolVariable { name = "power"; } }
    in ambient : TemperatureValue { @ToolVariable { name = "ambient"; } }
    out tMax : TemperatureValue   { @ToolVariable { name = "tMax"; } }
    out margin : TemperatureValue { @ToolVariable { name = "margin"; } }
}
```

The tools a `sysml` or `sysml-grpc` process may run are the entries of the
directory `OPENSYSML_TOOLS` names, read once at startup — one JSON file per
tool, in a directory outside every workspace, writable by its owner alone:

```json
{
  "toolName": "ThermalSolver",
  "version": "1.0",
  "executable": "python3",
  "variables": ["mass", "power", "ambient", "tMax", "margin"],
  "invocation": {
    "args": ["thermal.py", "--mass", "{mass}", "--power", "{power}", "--ambient", "{ambient}"],
    "cwd": ".",
    "stdin": "none"
  },
  "reply": {
    "format": "csv",
    "source": "stdout",
    "outputs": {
      "tMax": {"column": "tmax", "type": "number", "unitColumn": "tmaxUnit"},
      "margin": {"column": "margin", "type": "number", "unit": "K"}
    }
  }
}
```

The `invocation` block composes the command: each `args` entry renders to
exactly one argument with `{mass}` placeholders filled from the call's values,
`cwd` confined to the manifest's directory (`.` — the script sits beside the
entry), `stdin` saying what the process reads. The `reply` block says the
answer is the CSV record on standard output and where each output lives in it.
An entry with no block at all speaks the `object` protocol — one JSON object
each way on stdin/stdout.

`-engines` lists the tool as `tool:ThermalSolver`, its `protocol` column
spelling the composition — here `argv+none/csv`:

```text
tool:ThermalSolver  tool  argv+none/csv  observed  compute  ready (ThermalSolver 1.0 at /usr/bin/python3)
```

## Previewing the call

`-tool-dry-run` at the CLI, `%tool` in the REPL, runs the case or action as
`-analysis` would until the performance first reaches a tool — then prints
what the call would have been given, without starting the process. What the
preview performed is discarded: the session is as it found it.

The `env` block lists the manifest's own `env` entries with their rendered
values; a variable taken from this process (`PATH`, `HOME`, `TMPDIR`, `LANG`
and each `OPENSYSML_TOOL_ENV_PASSTHROUGH` names) is listed by name only, as
`NAME=<from this process>`.

```text
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
  env:
    ...
  cwd: /home/me/.opensysml-tools
  stdin: none
  inputs:
    ambient = 290 [K]
    mass = 12.5 [kg]
    power = 250 [W]
  outputs:
    margin
    tMax
  reply: csv from stdout, header, delimiter ","
    margin: column "margin", row last, type number, unit K
    tMax: column "tmax", row last, unitColumn "tmaxUnit", type number
  the process was not started
```

A manifest fault, an unregistered tool or an input the call does not send
reports the same typed error the real run would fail with — the preview is a
diagnostic surface, not a guess. A run that reaches no tool says so.

## Running and recording it

`-analysis` runs the case; the tool's process is started, the inputs bound,
the reply read:

```text
✓ ThermalDemo::heating
  tMax = 306.0 [SI::K]
  margin = 94.0 [SI::K]
  objective marginHeld: satisfied
  standing: value (observed: 1 run under reverse)
```

A tool's answer stands at strength *observed*: nothing in OpenSysML knows
what the tool should have computed, so equal inputs answering differently are
reported as a divergence in the run's notes.

`-record-run` records the run as an `AnalysisRecords` element — see
[Recording analysis runs](recording-analysis-runs.md) — and its `RecordedRun`
annotation's `tools` names every external tool call the run made, one element
per call in call order (`tool version from manifest: executable argv`):

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

Recording composes with document rendering — `-record-run` beside
`-render-document` records first, so the document's queries see the record —
and `Project` tabulates the record's own features. An annotation's attribute
values are not projectable, so `tools` stays in the record's source rather
than a table column:

```bash
./bin/sysml examples/external-tool-demo/thermal.sysml \
    -record-run ThermalDemo::heating \
    -render-document Reporting::HeatingReport -o report.md
```

```markdown
| name | caseName | kind | mass | power | ambient | tMax | margin | objective |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| heating\_run1 | ThermalDemo::heating | run | 12.5 | 250 | 290 | 306 | 94 | satisfied |
```
