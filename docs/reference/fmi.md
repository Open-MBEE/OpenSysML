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
 "start":{"g":-9.81,"e":0.7,"u":[1.0,2.0,3.0]},"outputs":["h","v","y"],
 "series":{"samples":5,"outputs":["T"]}}
```

- `protocol` is `1`. `fmu` is the archive's absolute path. `interface` is `coSimulation`,
  `modelExchange` or `scheduledExecution`, whichever the description serves — the first, in
  that order.
- `experiment` carries `startTime` and `stopTime` always, `stepSize` and `tolerance` only when
  the declaration binds them.
- `start` maps every remaining `in` variable name to the value bound for it; `outputs` lists
  the `out` variable names in declaration order; `series`, when present, asks for a
  trajectory of `samples` points and names the variables to sample. With no `times`, the
  runner spaces them evenly over recorded *time* from `startTime` to `stopTime`, so extra
  solver-event rows do not move the instants; with `times`, those exact instants are
  requested and the runner must honour them. An instant outside the simulated run is an
  error reply, never the last recorded value relabelled. A call with no series output
  omits the member entirely.

The reply is either the result or the error:

```json
{"protocol":1,"time":3.0,"outputs":{"h":0.0,"v":-29.43,"y":[2.7,5.4,8.1]},
 "series":{"times":[0.0,0.75,1.5,2.25,3.0],"outputs":{"T":[1.0,2.0,3.0,4.0,5.0]}}}
{"protocol":1,"error":"the FMU failed to initialize"}
```

`time` is the simulated time the outputs were read at — a successful reply must carry a
finite one; `outputs` maps each requested name to
its value — a JSON number for `Real`, `Integer` and `Enumeration`, `true`/`false` for
`Boolean`, a string for `String`. An `error` reply — including a sample instant beyond
the run the FMU actually simulated — fails the performance as a `*fmi.RunnerError`; any
other protocol violation — no reply, a wrong protocol version, a value
of the wrong shape — fails it as malformed output. A `Real` variable's value is never truncated
to fit an `Integer` one, and a declared unit on an input is checked: a value measured in
another unit is refused rather than converted silently.

### Reserved `fmi:` variables

The names below carry the experiment or a trajectory rather than a model
variable; `fmi:stepTime` asks for a session step:

| `ToolVariable` name | Direction | Carries |
|---|---|---|
| `fmi:startTime` | input | the experiment's start time, default `0.0` |
| `fmi:stopTime` | input | the experiment's stop time, default `1.0` |
| `fmi:stepSize` | input | the step size — only when `DefaultExperiment` declares one |
| `fmi:tolerance` | input | the solver tolerance — only when declared |
| `fmi:stepTime` | input | a session's next time — selects an evaluation session instead of a whole run |
| `fmi:samples` | input | the number of points in a trajectory — an integer from 2 to 10000, default 11 |
| `fmi:sampleInterval` | input | an alternative to `fmi:samples`: the spacing in seconds, converted from a measured value and sent as explicit request instants |
| `fmi:time` | output | the reply's `time` — bound when the FMU declares no output |
| `fmi:seriesTime` | output | the trajectory's sample times, bound as a sequence measured in seconds |
| `fmi:series:<name>` | output | the sampled trajectory of the FMU variable `<name>`, bound as a sequence |

These spellings are selectors, but an FMU variable whose archive name is exactly
one of them is still a model variable: the engine gives the FMU's own names first
claim. An output named `fmi:series:y` on an FMU declaring that variable reads its
value at `stopTime`, as the importer's `ToolVariable` name preserves it verbatim.
When both `y` and `fmi:series:y` exist in one FMU, that spelling reads the literal,
so a trajectory of `y` cannot be requested — avoid such names when both are needed.

## Trajectory (series) outputs

A tool-computed calc normally binds each output once, at `stopTime`. To decide a
condition over a whole run — a maximum temperature, a minimum pressure, an area
under a curve — bind an output whose `ToolVariable` name begins `fmi:series:` to
a multi-valued parameter and ask for the sample count or spacing with the
reserved inputs:

```sysml
package BoilerDrive {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ControlFunctions::*;

	calc def Trajectory {
		metadata ToolExecution { toolName = "fmi"; uri = "file:///opt/fmus/boiler.fmu"; }
		in samples : Integer = 5 { @ToolVariable { name = "fmi:samples"; } }
		in stopTime : Real = 3.0 { @ToolVariable { name = "fmi:stopTime"; } }
		return temps : Real[1..*] nonunique { @ToolVariable { name = "fmi:series:T"; } }
	}

	requirement def PeakTemperature {
		in temps : Real[1..*] nonunique;
		in limit : Real;
		require constraint { temps->maximize { in t; t } <= limit }
	}
}
```

The run answers five samples and binds them as a `Real[1..*]` value; the requirement
check decides `max(T) <= limit` from that one trajectory. With `fmi:sampleInterval`
instead of `fmi:samples`, the request carries the exact instants: a 2 s interval over a
0–3 s run samples 0 s and 2 s, not 0 s and 3 s, and a reply at other times is refused.
The scalar answer at `stopTime` is never substituted for a series: if the runner omits
`series`, answers the wrong number of samples or wrong instants, or a caller binds the
trajectory to a single-valued parameter, the performance fails with a malformed-output
error. A series variable must name a scalar output of the FMU; arrays cannot be sampled
into a trajectory.

## Reusable evaluation sessions

A whole-run `tool:fmi` performance starts one runner process, initializes the
FMU and reads every output at `stopTime`. A check that needs a value in the
middle of an experiment can ask for it without starting over: a session
initializes once and then answers "step to `t`" and "read these variables"
from the state it reached. N intermediate readings cost one runner invocation,
not N.

Sessions are an additive Go API in `internal/exec/fmi`, not a change to the
one-shot path. A call that does not bind `fmi:stepTime` still speaks `fmi/1` and
writes byte-identical requests and replies. A call that does bind it goes
through a `SessionManager`:

```go
manager := fmi.NewSessionManager(nil) // nil reads OPENSYSML_FMI_RUNNER
defer manager.Close()
session, err := manager.Open(ctx, "caller-A", call) // one init
session.Step(ctx, 0.5)                             // advance to t
values, err := session.Read(ctx, []string{"h", "v"}) // read from that state
```

`SessionManager.ToolRunner(caller, delegate)` wraps an existing
`runtime.ToolRunner` so a model's annotated `calc def` uses a session whenever
its tool call binds `fmi:stepTime`, and goes to the delegate unchanged for
every other tool or call. Several calc usages that bind different step times
therefore share one session for one caller. A session is keyed by the caller
and the init request, so two callers driving the same FMU keep separate state
and re-opening the same session returns the one already initialized. A stepped
call that reaches the one-shot engine without a session tool runner is refused
with a named error rather than silently answered at `stopTime`.

A stepped call that also names a series variable (`fmi:samples`,
`fmi:sampleInterval`, `fmi:seriesTime` or `fmi:series:<name>`) is refused with
`*fmi.SessionSeriesError`: a session answers scalar outputs at each step, while
a trajectory is the one-shot runner's answer. The combination is undefined by
either surface and is never silently accepted.

A runner that serves sessions reads newline-delimited JSON on standard input
and writes one line per command on standard output (`fmi/2`):

```json
{"protocol":2,"action":"init","fmu":"/abs/model.fmu","interface":"coSimulation",
 "experiment":{"startTime":0.0,"stopTime":3.0},"start":{"g":-9.81}}
{"protocol":2,"action":"step","time":0.5}
{"protocol":2,"action":"read","outputs":["h","v"]}
```

The `init` answer carries the start time; `step` answers the time reached;
`read` answers that time and the outputs:

```json
{"protocol":2,"action":"init","time":0.0}
{"protocol":2,"action":"step","time":0.5}
{"protocol":2,"action":"read","time":0.5,"outputs":{"h":0.95,"v":-4.9}}
```

A rejected command answers `{"protocol":2,"action":"…","error":"…"}`, which
fails the performance as a `*fmi.RunnerError`. A wrong protocol version, a
malformed line, a line over `OPENSYSML_TOOL_MAX_OUTPUT` and a runner that exits
are typed failures; no value is invented. A step backwards or past
`fmi:stopTime` is refused locally with a typed `*fmi.StepError`, without asking
the runner.

Sessions are bounded. A manager holds at most 16 by default
(`NewSessionManagerWith` overrides it), closes a session idle for 60 seconds or
open for 10 minutes, and refuses the 100,001st step. A background janitor
reclaims sessions that pass a bound even when no caller returns; the owner
calls `Close` to end them all and stop the janitor. Each command answers within
`OPENSYSML_TOOL_TIMEOUT` and is bounded by `OPENSYSML_TOOL_MAX_OUTPUT`, as a
one-shot tool is.

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
- Sessions need a runner that speaks `fmi/2`; the reference runner shipped with the Python
  client speaks `fmi/1` only. The whole-run path works with every `fmi/1` runner.
- Structural parameters, multi-dimensional and structurally dimensioned array variables,
  `Binary` and `Clock` values are not imported; `local` and `independent` variables are
  read but not bound.
- A `Real` variable's unit projects as an ISQ quantity type only when it resolves to
  coherent base exponents; non-coherent units (`km`, `degC`) and array element units stay
  comments, and an input measured in such a unit is refused rather than converted.
- A trajectory samples scalar variables only; an array variable named by `fmi:series:` is
  refused, though scalar outputs at `stopTime` still answer arrays as before.
- `fmu` is an input format only.
