# FMI models (FMUs)

[Functional Mock-up Interface](https://fmi-standard.org/) — FMI — ships a model as a
**Functional Mock-up Unit**, a `.fmu` archive carrying a `modelDescription.xml` and the
binaries that run the model. OpenSysML reads a model description, imports the FMU as a SysML
`calc def`, and hands its evaluation to an external **FMI runner** you install: a co-simulation
or model-exchange execution is a process boundary away, never code loaded into this one.

## Importing an FMU

`-convert` accepts `fmu` as an input format — `.fmu` is recognized from the extension — and
`-convert sysml` writes the imported declaration:

```console
$ sysml -convert sysml -o bouncingball.sysml bouncingball.fmu
```

The archive's model description (FMI 1.0, 2.0 or 3.0) is read and each variable becomes a
parameter, in document order:

```sysml
package BouncingBall {
	private import ScalarValues::*;
	private import AnalysisTooling::*;

	calc def BouncingBall {
		doc /* A bouncing ball. */
		metadata ToolExecution {
			toolName = "fmi";
			uri = "file:///opt/fmus/bouncingball.fmu";
		}
		// parameters and inputs of the model, in document order
		in g : Real = -9.81 { @ToolVariable { name = "g"; } }
		in e : Real = 0.7 { @ToolVariable { name = "e"; } }
		// the simulation experiment
		in startTime : Real = 0.0 { @ToolVariable { name = "fmi:startTime"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		in stepSize : Real = 0.01 { @ToolVariable { name = "fmi:stepSize"; } }
		// outputs at stopTime
		return h : Real { @ToolVariable { name = "h"; } }
		out v : Real { @ToolVariable { name = "v"; } }
	}
}
```

- **Parameters and inputs** — FMU variables of `parameter` or `input` causality — are `in`
  parameters, carrying their start values where declared.
- **The experiment** is four reserved parameters: `startTime` and `stopTime` always, `stepSize`
  and `tolerance` where the description's `DefaultExperiment` declares them. They take the
  reserved `fmi:` variable names below; a model variable of the same name is suffixed.
- **Outputs** — `output` and `calculatedParameter` causality — are `out` parameters; the first
  is the `return`. A description with no outputs gets `return time : Real` bound to `fmi:time`.
- **Types** map `Real`→`Real`, `Integer` and `Enumeration`→`Integer`, `Boolean`→`Boolean`,
  `String`→`String`. Names that are not valid SysML identifiers, or collide with a keyword or
  another parameter, are sanitized and suffixed; every parameter's `@ToolVariable` records the
  FMU name it binds.
- **Skipped** variables — structural parameters, arrays, `Binary` and `Clock` values — leave a
  `//` comment where they would have stood rather than disappearing silently.

`uri` is the path the FMU was read from (as a `file:` URL); edit it to where the FMU will sit
when the calc runs. `fmu` is input-only — it is read and imported, never written — so
`-convert fmu` names the same refusal every read-only format does.

## The `tool:fmi` engine

The imported `calc def` is a `ToolExecution` performance like any
[external tool's](../manual/running-external-programs.md): evaluating it routes a compute
question to the `tool:fmi` engine, which is always registered and listed by `-engines` and
`%engines`. It answers the question only when the FMU it names can be read, every parameter it
binds is a variable or a reserved name, and the runner is granted.

The URI resolves as a `file:` URL or a bare path; a relative path resolves against the
directory of the file the `calc def` stands in, and is refused where the declaration has no
file — as with a `calc def` written straight into the REPL.

## Granting the runner

Like `OPENSYSML_SMT`, executing another program is an explicit grant:
`OPENSYSML_FMI_RUNNER` names the runner executable. Unset, an `fmi` performance refuses — it is
never silently run — with a message naming the variable:

```console
$ OPENSYSML_FMI_RUNNER=/usr/local/bin/fmi-runner sysml -calc 'BouncingBall::BouncingBall()' model.sysml
```

The runner is a subprocess: one process per evaluation, the minimal tool environment (`PATH`,
`HOME`, `TMPDIR`, `LANG`, plus the names `OPENSYSML_TOOL_ENV_PASSTHROUGH` lists), bounded by
`OPENSYSML_TOOL_TIMEOUT` and `OPENSYSML_TOOL_MAX_OUTPUT` like any external process.

### The runner protocol

The runner reads one JSON object on standard input and writes one JSON object on standard
output, nothing else on stdout:

```json
{"protocol":1,"fmu":"/abs/path/model.fmu","interface":"coSimulation",
 "experiment":{"startTime":0.0,"stopTime":3.0,"stepSize":0.01},
 "start":{"g":-9.81,"e":0.7},"outputs":["h","v"]}
```

- `protocol` is `1`. `fmu` is the archive's absolute path. `interface` is `coSimulation`,
  `modelExchange` or `scheduledExecution`, whichever the description serves — the first, in
  that order.
- `experiment` carries `startTime` and `stopTime` always, `stepSize` and `tolerance` only when
  the declaration binds them.
- `start` maps every remaining `in` variable name to the value bound for it; `outputs` lists
  the `out` variable names in declaration order.

The reply is either the result or the error:

```json
{"protocol":1,"time":3.0,"outputs":{"h":0.0,"v":-29.43}}
{"protocol":1,"error":"the FMU failed to initialize"}
```

`time` is the simulated time the outputs were read at; `outputs` maps each requested name to
its value — a JSON number for `Real`, `Integer` and `Enumeration`, `true`/`false` for
`Boolean`, a string for `String`. An `error` reply fails the performance as a
`*fmi.RunnerError`; any other protocol violation — no reply, a wrong protocol version, a value
of the wrong shape — fails it as malformed output. A `Real` variable's value is never truncated
to fit an `Integer` one, and a declared unit on an input is checked: a value measured in
another unit is refused rather than converted silently.

### Reserved `fmi:` variables

Four names carry the experiment rather than a model variable:

| `ToolVariable` name | Direction | Carries |
|---|---|---|
| `fmi:startTime` | input | the experiment's start time, default `0.0` |
| `fmi:stopTime` | input | the experiment's stop time, default `1.0` |
| `fmi:stepSize` | input | the step size — only when `DefaultExperiment` declares one |
| `fmi:tolerance` | input | the solver tolerance — only when declared |
| `fmi:time` | output | the reply's `time` — bound when the FMU declares no output |

## Limitations

- The runner is external and the FMU is never loaded in-process: no FMI calls cross into
  OpenSysML, so a co-simulation runs one `doStep`-style exchange per evaluation through the
  runner you provide.
- Structural parameters, array variables, `Binary` and `Clock` values are not imported;
  `local` and `independent` variables are read but not bound.
- Unit names recorded on FMU variables annotate the imported parameters and are checked on
  inputs; they are not projected as SysML quantity types.
- `fmu` is an input format only.
