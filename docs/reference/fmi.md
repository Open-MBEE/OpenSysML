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
	private import ISQ::*;
	private import SI::*;

	calc def BouncingBall {
		doc /* A bouncing ball. */
		metadata ToolExecution {
			toolName = "fmi";
			uri = "file:///opt/fmus/bouncingball.fmu";
		}
		// parameters and inputs of the model, in document order
		in g : AccelerationValue = -9.81 [m/s^2] { @ToolVariable { name = "g"; } }
		in e : Real = 0.7 { @ToolVariable { name = "e"; } }
		// the simulation experiment
		in startTime : Real = 0.0 { @ToolVariable { name = "fmi:startTime"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		in stepSize : Real = 0.01 { @ToolVariable { name = "fmi:stepSize"; } }
		// outputs at stopTime
		return h : LengthValue { @ToolVariable { name = "h"; } }
		out v : SpeedValue { @ToolVariable { name = "v"; } }
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
  `String`→`String` — but a `Real` variable whose unit resolves (below) types as an ISQ
  quantity type instead. Names that are not valid SysML identifiers, or collide with a
  keyword or another parameter, are sanitized and suffixed; every parameter's `@ToolVariable`
  records the FMU name it binds.
- **Skipped** variables — structural parameters, multi-dimensional and structurally
  dimensioned arrays, `Binary` and `Clock` values — leave a `//` comment where they would
  have stood rather than disappearing silently.

### Units

A `Real` variable's unit is resolved to base-dimension exponents — first through the
description's `UnitDefinitions`/`BaseUnit` (FMI 2 and 3), then a declared type's unit, then
by parsing the unit name itself as a product of the base symbols `kg m s A K mol cd rad`
(FMI 1.0 and unitless-defined names; `N` is not parsed — only base symbols are). A unit that
resolves is required to be **coherent**: a `BaseUnit` with `factor` ≠ 1 or `offset` ≠ 0
(`km`, `degC`) resolves no type and stays a number, its name kept as a comment.

A resolved unit spells a KerML unit expression in a fixed order — `kg m s A K mol cd rad`,
positives then `/` negatives (`m/s^2`, `kg*m/s^2`, `1/s` for `s^-1`) — and the parameter
types as the ISQ value type of that dimension when the table holds one (`LengthValue`,
`MassValue`, `DurationValue`, `ElectricCurrentValue`, `ThermodynamicTemperatureValue`,
`AmountOfSubstanceValue`, `LuminousIntensityValue`, `AreaValue`, `VolumeValue`,
`SpeedValue`, `AccelerationValue`, `FrequencyValue`, `ForceValue`, `PressureValue`,
`EnergyValue`, `PowerValue`, `ElectricChargeValue`, `ElectricPotentialValue`,
`ResistanceValue`, `CapacitanceValue`, `InductanceValue`, `MassDensityValue`,
`AngularVelocityValue`, `MassFlowRateValue`, `VolumeFlowRateValue`), else
`ScalarQuantityValue` — a quantity fixing no dimension. The generated package adds
`private import ISQ::*;` and `private import SI::*;` only when a unit typed a parameter.
An `in` value sent in another unit of the same dimension is converted to the variable's
coherent unit before it is sent (`[km]` to metres); a quantity-typed parameter against a
variable that resolves no unit is refused.

### Arrays

FMI 3.0 one-dimensional arrays with a **fixed** `start` dimension import as ordered
collections — `in u : Real[3] nonunique = (1.0, 2.0, 3.0)` (`nonunique` because an FMU
array may hold repeated values); without a start value the `=` clause is left off. Array
elements type as plain `Real`: a sequence of measured values has no SysML literal, so an
array variable's unit stays a comment. Dimensions declared by `valueReference`
(structural) and multi-dimensional arrays are skipped with a comment. In the protocol,
`start` sends the array as a JSON list and `outputs` answers one the same way; a reply of
the wrong length is a protocol break.

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

An FMU is native code, so granting the runner means trusting every model that names an FMU
on the machine: the grant is for a workspace whose models and archives the operator controls.

### The runner protocol

The runner reads one JSON object on standard input and writes one JSON object on standard
output, nothing else on stdout:

```json
{"protocol":1,"fmu":"/abs/path/model.fmu","interface":"coSimulation",
 "experiment":{"startTime":0.0,"stopTime":3.0,"stepSize":0.01},
 "start":{"g":-9.81,"e":0.7,"u":[1.0,2.0,3.0]},"outputs":["h","v","y"]}
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
{"protocol":1,"time":3.0,"outputs":{"h":0.0,"v":-29.43,"y":[2.7,5.4,8.1]}}
{"protocol":1,"error":"the FMU failed to initialize"}
```

`time` is the simulated time the outputs were read at — a successful reply must carry a
finite one; `outputs` maps each requested name to
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

## Reference runner

A reference runner ships with the Python client; `pip install opensysml[fmi]` installs FMPy and
the `opensysml-fmi-runner` executable, which simulates co-simulation and model-exchange FMUs
(scheduled execution is refused with a named error):

```console
$ pip install opensysml[fmi]
$ export OPENSYSML_FMI_RUNNER=opensysml-fmi-runner
```

Try it over the Modelica Reference-FMUs, downloaded by a script beside the other corpus
downloaders (`examples/reference-fmus/` is gitignored; `OPENSYSML_REQUIRE_REFERENCE_FMUS=1`
turns its absence in the gate into a failure rather than a skip):

```console
$ ./scripts/download-reference-fmus.sh
$ sysml -convert sysml examples/reference-fmus/2.0/BouncingBall.fmu > bouncing.sysml
$ sysml -calc 'BouncingBall::BouncingBall' bouncing.sysml
```

## Limitations

- The runner is external and the FMU is never loaded in-process: no FMI calls cross into
  OpenSysML, so a co-simulation runs one `doStep`-style exchange per evaluation through the
  runner you provide.
- Structural parameters, multi-dimensional and structurally dimensioned array variables,
  `Binary` and `Clock` values are not imported; `local` and `independent` variables are
  read but not bound.
- A `Real` variable's unit projects as an ISQ quantity type only when it resolves to
  coherent base exponents; non-coherent units (`km`, `degC`) and array element units stay
  comments, and an input measured in such a unit is refused rather than converted.
- `fmu` is an input format only.
