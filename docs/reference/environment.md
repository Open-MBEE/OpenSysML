# Environment variables

These variables are read by `sysml`, `sysml-lsp` and `sysml-grpc` alike. Each budget turns a
run that would never finish into a reported error instead of a hang.

| Variable | Default | Meaning |
|----------|---------|---------|
| `OPENSYSML_LIBRARY_PATH` | unset (use the bundled standard library) | Directory to load the SysML/KerML standard library from instead of the embedded copy |
| `OPENSYSML_MAX_STEPS` | `10000000` | Evaluation step budget: the number of expression evaluations one run may spend before it is reported as a runaway |
| `OPENSYSML_MAX_ACTION_STEPS` | `1000000` | Token-flow steps one action run may perform |
| `OPENSYSML_MAX_EVENTS` | `1000000` | Events one state machine run may dispatch, and the events one `%advance` drains |
| `OPENSYSML_MAX_DO_STEPS` | `5000000` | Do actions one state machine run may perform, and the ones one `%advance` drains |
| `OPENSYSML_MAX_ELEMENTS` | `1000000` | Collection elements one evaluation may hold — the bound on the memory a run holds rather than on the work it does |
| `OPENSYSML_MAX_CALC_DEPTH` | `10000` (ceiling `25000`) | Nested `calc` invocations one run may hold on the stack, which is what a recursion spends |
| `OPENSYSML_MAX_SWEEP_RUNS` | `1000` | Runs one parameter sweep or sample may make (`-sweep`/`-samples`, `%sweep`/`%samples`, `RunSweep`), each a whole analysis or calc run with the budgets above of its own |
| `OPENSYSML_JOBS` | one per CPU, fewer where the memory available leaves less than 512 MiB per worker | Runs of one check that may go concurrently (`-jobs`, `%jobs`; the gRPC service reads it at startup), each on a worker of its own over the shared model, and how many files of one load are parsed and validated at once. Bounds how many runs go at once, not the work or memory of any one of them: a fleet of `n` workers may hold `n` times `OPENSYSML_MAX_ELEMENTS`. The result of a check does not depend on it |
| `OPENSYSML_CALC_COMPILE` | unset (on) | Set to `0`, `false`, `off` or `no` to run every `calc` on the reference evaluator, instead of compiling a pure scalar body to a closure fast path on its first invocation; results, errors and step counts are the same either way, so this is a bisecting aid |
| `OPENSYSML_SMT` | unset (look for `z3`, then `cvc5`, on `PATH`) | Executable the `smt` and `solve` engines (`-engine smt`, `%engine smt`) and `%check`, `%explain`, `%solve`, `%configure` and `%optimize` drive as their SMT solver, speaking SMT-LIB2 on standard input (experimental); `%optimize` needs `z3` in particular, as `(minimize …)`/`(maximize …)` is a z3 extension cvc5 does not implement |
| `OPENSYSML_SMT_TIMEOUT` | `10s` | How long one solver query may take, as a Go duration (`5s`, `500ms`), after which the verdict is `unknown`; a check's `-check-timeout` (`%check-bounds timeout=`) takes its place for the `smt` engine's queries |
| `OPENSYSML_SMT_CORE_BUDGET` | `30s` | How long `%explain` may spend reducing an unsat core to a minimal one, as a Go duration; past it the solver's own core is reported, said not to be necessarily minimal |
| `OPENSYSML_SMT_MAX_CONFIGURATIONS` | `32` | How many variant selections `%configure … all` may report before saying the enumeration was cut short at the bound |
| `OPENSYSML_TOOLS` | unset (no tools) | Directory of the **tool manifest**: one JSON file per external tool, each registering a `tool:<name>` analysis engine that runs the tool for the `ToolExecution`-annotated actions naming it; see [External tools](#external-tools). A file whose `kind` is `engine` registers an external engine as `OPENSYSML_ENGINES` does |
| `OPENSYSML_ENGINES` | unset (no engines) | Directory of the **engine manifest**: one JSON file per external analysis engine — a model checker, a simulator, a solver — spoken to over its standard input by the protocol on [External engines](external-engines.md), listed by `-engines` and selected by `-engine <name>` like any engine of the build. Read beside `OPENSYSML_TOOLS` under the same rules; see [External engines](#external-engines) |
| `OPENSYSML_TOOL_TIMEOUT` | `10s` | How long one tool process may take, as a Go duration (`5s`, `500ms`), after which the performance fails with a timeout; for an external engine, how long its `describe` and `covers` may take and the grace a `run` has to answer `cancel` before its process is ended. A value that is not a positive duration is the default |
| `OPENSYSML_TOOL_MAX_OUTPUT` | `64M` | How much one external process may write before it is cut off: a tool's one reply and the whole of its standard error, an external engine's one protocol line and the whole of its standard error. Bytes, or bytes with a `K`, `M` or `G` suffix; a value that is not a positive size is the default |
| `OPENSYSML_TOOL_ENV_PASSTHROUGH` | unset | Comma-separated names of this process's environment variables handed to a tool started under a manifest [`invocation`](#external-tools) block, beside `PATH`, `HOME`, `TMPDIR` and `LANG`; nothing else of the environment reaches such a tool. An entry containing `=` or a NUL byte is not an environment variable name and is refused. A tool without the block inherits the whole environment as before |
| `OPENSYSML_TOOL_KEEP` | unset | `1` keeps the per-invocation directory a tool's `{inputFile}` and `{outputDir}` name, for inspection; otherwise it is removed once the reply is read |
| `OPENSYSML_GRPC_INDEX_POOL` | `4` | Whether `sysml-grpc` builds the one shared standard library index ahead of the requests needing it; any positive value prewarms, `0` builds it on the first request instead |
| `OPENSYSML_GRPC_MAX_HELD_OBJECTS` | `10000` | The most objects `sysml-grpc` keeps for one cached model, nested objects counted: `Instantiate` creates them and document queries bind and enumerate them for as long as the model stays cached. An `Instantiate`, query or render whose objects would pass the bound fails whole with `RESOURCE_EXHAUSTED`, leaving none of them, until the model leaves the cache, which releases them together; nothing is evicted behind an id a client holds. Read at startup |
| `OPENSYSML_WEASYPRINT`, `OPENSYSML_PANDOC`, `OPENSYSML_PRINCE` | unset (look on `PATH`) | Executable of the HTML-to-PDF converter `-render-document -doc-form pdf` drives under `-pdf-engine weasyprint` (the default; `pandoc` also needs it), `pandoc` or `prince`; the selected one absent is a typed `tool-missing` error naming its variable |
| `OPENSYSML_MMDC`, `OPENSYSML_MMDC_PUPPETEER` | unset (look for `mmdc` on `PATH`) | Mermaid CLI, which draws a PDF's Mermaid diagrams, and a Puppeteer configuration file for its browser (`--no-sandbox` in a container). Required as soon as a PDF has a Mermaid diagram; a document without one needs neither |
| `OPENSYSML_KATEX`, `OPENSYSML_KATEX_CSS` | unset (look for `katex` on `PATH`, its stylesheet beside it) | KaTeX, which typesets a PDF's formulas, and its stylesheet when it is not installed beside the command. Required as soon as a PDF has a formula |
| `OPENSYSML_DOT` | unset (look for `dot` on `PATH`) | Graphviz, which draws a PDF's diagrams under `-diagram-form dot` — `-Tsvg`, under the engine each block's `// layout:` header names (`dot`, `neato`, `neato -n`), so positioned views are drawn where the model put them. Optional: without it every DOT block stays in the PDF as source under a notice naming this variable |
| `OPENSYSML_PLANTUML_JAR`, `OPENSYSML_JAVA` | unset; unset (look for `java` on `PATH`) | The PlantUML jar that draws a PDF's diagrams under `-diagram-form plantuml` (`java -jar <jar> -tsvg -pipe`) and the Java that runs it. Optional: without the jar or a Java every PlantUML block stays in the PDF as source under a notice naming the variable to set. `go test ./internal/ir/view` also passes its PlantUML goldens through the jar's `-checkonly` when the jar is named |
| `OPENSYSML_GRPC_MAX_HELD_EVENTS` | `100000` | The most trace records `sysml-grpc` keeps for one cached model's population, which is traced from its first `Instantiate` so `DocumentQueries::Events` can read the run. Past the bound the oldest records are dropped; an `Events` query whose interval reaches back to a dropped record fails as a typed `trace-truncated` error (`FAILED_PRECONDITION`) naming the instant history is kept from, so `since` can be bound later, rather than answering an incomplete relation. Read at startup |

The PDF tools are external, not bundled: `scripts/download-doc-pdf-toolchain.sh` fetches pinned
copies of every one but Java and prints the exports above. A tool that is present and fails is a
typed `tool-failed` error carrying its standard error, whichever variable found it; see
[Rendering a document as PDF](cli.md#rendering-a-document-as-pdf).

Every variable above uses the `OPENSYSML_` prefix. The eight that predate it
(`OPENSYSML_LIBRARY_PATH`, the six `OPENSYSML_MAX_*` budgets and
`OPENSYSML_GRPC_INDEX_POOL`) also answer to their legacy `SYSML_`-prefixed
names (`SYSML_LIBRARY_PATH`, `SYSML_MAX_STEPS`, and so on), which remain accepted
indefinitely. When a variable is set under both prefixes and the `OPENSYSML_`
value is non-empty, the `OPENSYSML_` value wins. Setting only the legacy name
prints a one-time deprecation warning to standard error that names the
`OPENSYSML_` form to switch to.

The three `OPENSYSML_SMT*` variables belong to the experimental solving extension
(`%check`/`%explain`) and the `smt` model checker, which need an external z3 or cvc5; `-engines`
reports which solver each of the `smt` and `solve` engines found, or that none was. Installing one is covered in
[1. Install: installing a solver](../guide/01-install.md#installing-a-solver-optional); the
extension follows the design of OpenMBEE's [HMF](https://github.com/hivecore-dev/hmf)
(see [Acknowledgements](../../README.md#acknowledgements)).
`OPENSYSML_SMT` takes an executable name or a path and is consulted before `PATH` is searched
(where `z3` is preferred over `cvc5`); a value that names no executable file is reported rather
than falling back to the search. It may name **any** solver that speaks SMT-LIB2 on standard
input, not only those two. The feature subset a backend must support, what z3 and cvc5 were each
measured to support, and how a backend that lacks a feature is reported are described in
[1. Install: solver compatibility](../guide/01-install.md#solver-compatibility--pointing-the-driver-at-another-solver).
Nothing else in the toolchain reads these variables, and the concrete evaluator needs no solver.

## External tools

An action of an analysis case carrying the `AnalysisTooling::ToolExecution` metadata (its
`toolName` and `uri`), with `ToolVariable` on the parameters the tool knows by other names, is
performed by that tool rather than by its body. A `calc def` or calc usage carrying the same
metadata is computed the same way — its `in` parameters go to the tool, its result parameter
and `out` parameters are bound from the reply, and its body is never evaluated — wherever a
calc is invoked: `sysml -calc`, `%calc`, `EvaluateCalc`, a derived attribute (`attribute x =
toolCalc(a, b)`) and the formulas a rendered document evaluates. The tools a `sysml` or `sysml-grpc` process may
run are the entries of the directory `OPENSYSML_TOOLS` names, read once at startup; each
becomes an engine `tool:<toolName>` that `-engines`, `%engines` and `ListEngines` list with its
status, and a manifest that cannot be read is reported at startup, as a bad run bound is.

**Manifest.** One JSON object per file, `*.json`; other files and subdirectories are ignored.

```json
{
  "toolName": "ModelCenter",
  "version": "14.1",
  "executable": "/opt/modelcenter/bin/mc-batch",
  "variables": ["deltaT", "power", "C_D", "C_F", "mass", "v0", "x0", "a", "v", "x"]
}
```

`toolName` is the name a `ToolExecution` gives, matched exactly, and two files naming the same
tool are refused. `version`, optional, is what the status column shows beside the executable's
path.
`executable` is a path — an absolute one taken as written; a relative one with a directory
part joined to the manifest's directory, followed through its links and refused unless it stays
inside that directory — or a bare name looked up on `PATH`; an executable that is not found
keeps the engine registered and listed as `unavailable: tool 'ModelCenter': executable … not
found`, and a performance naming the tool is refused with that reason. `variables` are the
`ToolVariable` names the tool accepts, non-empty and distinct; a parameter whose variable is
not among them refuses the performance before the process is started. Unknown keys are refused,
a key spelled twice or a member set to `null` anywhere in an entry likewise,
and so is a `kind` other than `tool`, `engine`, `policy` or `sampler` (`tool` when absent); the
last three are [engine entries](#external-engines), which a tool directory may hold too.

A manifest directory is read only from outside every workspace of the invocation — one under
a loaded model's directory is refused with the workspace named, since a model must not be able
to register a program — and a directory or entry writable by anyone but its owner is refused
(`is writable by others (mode 0664); a manifest entry and its directory may be written by their
owner alone`). A fault in one entry registers nothing from its directory.

**Invocation.** Without more, the executable is started with no arguments and the request
below on its standard input. An optional `invocation` block composes the command from the
model's values instead, for a program that takes its inputs as arguments, environment
variables or a file:

```json
{
  "toolName": "Dynamics",
  "executable": "bin/dynamics.py",
  "variables": ["mass", "power", "a", "v"],
  "invocation": {
    "args": ["--mass", "{mass}", "--power", "{power.value}", "--power-unit", "{power.unit}",
             "--inputs", "{inputFile}", "--out", "{outputDir}/result.json"],
    "env": {"OMP_NUM_THREADS": "4", "RUN_URI": "{uri}"},
    "cwd": "work",
    "stdin": "none",
    "inputFile": {"format": "csv", "name": "inputs.csv"}
  }
}
```

| Member | Meaning |
|---|---|
| `args` | Templates, each rendered to exactly one argument of the process: a value holding spaces, quotes or a shell's metacharacters stays one argument, since no shell is involved and nothing is split |
| `env` | Variables added to the process environment, values rendered from templates. The environment is otherwise `PATH`, `HOME`, `TMPDIR` and `LANG` from this process plus the names `OPENSYSML_TOOL_ENV_PASSTHROUGH` lists; the parent environment is not inherited |
| `cwd` | The working directory: an absolute path taken as written, or a manifest-relative one followed through its links and refused unless it stays inside the manifest's directory, as `executable` is |
| `stdin` | What the process reads: `"json"` (the default) the request object below; `"none"` nothing; `"csv"` one header row and one data row — the inputs sent in `variables` order, each measured one followed by a `<name>.unit` column; or `{"template": "..."}`, the template rendered |
| `inputFile` | A file written before the process starts into a directory made for the invocation: `format` `json` (the request object) or `csv` (the CSV form above), `name` a bare file name. `{inputFile}` is its path |

A template is literal text with placeholders: `{mass}` and `{mass.value}` are the variable's
value as the request would carry it (a number, `true`/`false` or the string itself, no quotes),
`{mass.unit}` its unit's short name or empty; `{toolName}` and `{uri}` are the annotation's;
`{inputFile}` is the input file's path and `{outputDir}` the invocation's directory, made empty
for the process to write into; `{{` and `}}` are literal braces. A placeholder naming a variable
not in `variables`, an `{inputFile}` without an `inputFile` member, a malformed placeholder,
and a `cwd` outside the manifest directory are manifest faults; beside the block a `variables`
entry spelled like a reserved placeholder or containing a period, brace or space is refused
too, so `{mass.unit}` always means the unit of `mass` and a CSV header never repeats. A declared variable the performance
did not send fails it before the process starts (`tool 'Dynamics': input not carried: the
invocation names {mass} but the call sent no value for mass`).
The invocation's directory is removed once the reply is read unless `OPENSYSML_TOOL_KEEP=1`.
The reply is read as the `reply` block below says — from standard output by default — and
the timeout, size bounds and divergence report apply unchanged; `-engines` shows the protocol
as `argv+json`, `argv+none`, `argv+csv` or `argv+template` rather than `object`, with the
reply's format appended when it is not `object` (`argv+none/csv`, or the format alone for an
entry with no `invocation`).

**Reply.** Without more, the reply is the protocol's one JSON object on standard output. An
optional `reply` block reads a different reply for a program that answers as a JSON document,
CSV, `key = value` lines or its exit status:

```json
{
  "toolName": "ThermalSolver",
  "executable": "bin/thermal",
  "variables": ["mass", "power", "T_max"],
  "invocation": {
    "args": ["--mass", "{mass}", "--power", "{power.value}", "--out", "{outputDir}/result.csv"]
  },
  "reply": {
    "format": "csv",
    "source": "file:{outputDir}/result.csv",
    "outputs": {"T_max": {"column": "T_max", "row": "last", "unitColumn": "unit"}}
  }
}
```

| Member | Meaning |
|---|---|
| `format` | `"object"` (the default), `"json"`, `"csv"`, `"lines"` or `"exitcode"`: how the reply is read. `object` admits no other member but `source`; the others require `outputs` |
| `source` | `"stdout"` (the default) or `"file:<template>"`, whose template may name `{outputDir}` alone — the path must then be under that directory, a regular file the tool wrote, and an `invocation` must hand `{outputDir}` to the tool. It may not be the invocation's `inputFile`, which the tool did not write, and symbolic links under `{outputDir}` are not followed out of it |
| `outputs` | An object mapping each output's tool variable to the selector finding its value |
| `header` | `csv`: whether the first record names the columns (default `true`) |
| `delimiter` | `csv`: the field separator, one character (default `,`) |
| `regex` | `lines`: an RE2 expression whose named groups are the outputs, each named exactly once; exclusive with `key` and `errorKey` |
| `success` | `exitcode`: the exit statuses that render `true` (default `[0]`) |
| `errorPath`, `errorColumn`, `errorKey` | `json`, `csv`, `lines`: where the tool's own refusal message is; a non-empty value there fails the performance with the tool's message, checked before any output; may not be an output's own selector |

Each member of `outputs` is a selector:

| Member | Formats | Meaning |
|---|---|---|
| `path` | `json`, required | An [RFC 6901](https://datatracker.ietf.org/doc/html/rfc6901) JSON Pointer to the value (`/results/0/T_max`); `~0` spells `~`, `~1` spells `/`, and the empty pointer `""` names the whole document |
| `column` | `csv`, required | The column: a header name, or a zero-based index as an integer |
| `row` | `csv` | Which data records (the header is not one): `"first"`, `"last"` (the default), a zero-based index or `"all"` |
| `key` | `lines` | The key on the left of the first `=` or `:` (default the variable's name); a line with neither is ignored |
| `type` | all but `object` | What the text is read as: `"number"` (the default), `"integer"`, `"real"`, `"boolean"` or `"string"`. Under `exitcode` only `boolean` (the default) and `integer` are admitted |
| `unit` | all but `object`, `exitcode` | The value's unit as a fixed expression (`K`, `km/h`) |
| `unitPath`, `unitColumn` | `json`, `csv` | Where a string holding the unit is read from; exclusive with `unit` |

Only the outputs the performance declares are read; other mapped outputs may be absent
from the reply without fault.

A unit found either way goes through the same conversion as the JSON protocol's `unit`: it
is read as a SysML unit expression and converted to the coherent unit of the parameter's
declared quantity kind. Every failure is one of the typed errors the protocol reports: a
selector finding nothing names the variable and where it was looked
(`tool 'ThermalSolver': missing output: T_max at /results/0/T_max: nothing there`), a value
the type cannot read is `malformed output` naming the variable and place (`T_max in column
T_max, row 3: "n/a" is not a number`), a tool refusal is `tool error`, and a file the tool
did not write, a non-zero exit (except under `exitcode`), oversize output or a timeout fail
as they do today.

Three selector forms answer a **sequence** rather than a single value: `row: "all"` reads every
CSV data record in order (a reply with no data record answers the empty sequence), a `path`
naming a JSON array reads its elements, and under the `object` protocol `"value": [..]` does
the same. Every element is a scalar, and all elements of one answer are the same kind — all
numbers, all Booleans or all Strings (integers and reals are one kind); a nested array, an
object, `null` or a kind mixing is `malformed output` naming the element's index. A
`unitColumn` under `row: "all"` is read on every record and all records must agree; the
shared unit is the sequence's, converted once for every element.

A sequence binds only to a parameter whose multiplicity admits more than one value
(`[0..*]`, `[1..*]`, `[0..n]` with n > 1), and a scalar only to a single-valued one — neither
is ever silently wrapped or truncated, and the multiplicity's bounds are enforced like any
write (a scalar for `Real[0..*]`, a sequence for `Real`, and a fifth value for `Real[2..4]`
are each `malformed output`). `lines` has no sequence form: every match or key there answers
one value.

```bash
$ OPENSYSML_TOOLS=~/tools sysml -engines
engine            kind      protocol  authority  answers          status
check             built-in  -         bounded    outcomes, holds  ready
explore           built-in  -         proved     outcomes         ready
run               built-in  -         observed   evaluate         ready
smt               built-in  -         proved     holds            ready (z3 at /usr/bin/z3)
solve             built-in  -         proved     satisfiable      ready (z3 at /usr/bin/z3)
sweep             built-in  -         observed   sweep            ready
tool:ModelCenter  tool      object    observed   compute          ready (ModelCenter 14.1 at /opt/modelcenter/bin/mc-batch)
tool:ModelCenter 14.1: tool from /home/me/tools/modelcenter.json, runs /opt/modelcenter/bin/mc-batch
```

**Protocol.** Each performance of the annotated action starts the executable once, with no
arguments, writes one JSON object to its standard input and reads one JSON object from its
standard output. The request carries `toolName` and `uri` exactly as the model spells them, and
`inputs` keyed by the `ToolVariable` name of each `in` and `inout` parameter (a parameter
carrying no `ToolVariable` takes no part in the exchange), each a `value` — a JSON number,
`true`/`false` or a string — and, for a quantity, the `unit` by its short name, unconverted
(`s`, `kg`, `km/h` for a value the model wrote as `36 [SI::km / SI::h]`):

```json
{"toolName": "ModelCenter", "uri": "aserv://localhost/Vehicle/Equation1",
 "inputs": {"deltaT": {"value": 1, "unit": "s"}, "mass": {"value": 1500, "unit": "kg"},
            "v0": {"value": 36, "unit": "km/h"}, "C_D": {"value": 0.3}}}
```

The reply is `outputs`, keyed the same way with one entry per `out` and `inout` parameter, or
`error` with a message:

```json
{"outputs": {"a": {"value": 3.0, "unit": "m/s**2"}, "v": {"value": 12.0, "unit": "m/s"}}}
```

```json
{"error": "license server unreachable"}
```

An output `unit` is a SysML unit expression read in the action's scope, then in `SI` (`m/s`,
`SI::km`, `'m⋅s⁻²'`); the value is converted to the coherent unit of the parameter's declared
quantity kind (`36 km/h` bound to a `SpeedValue` is `10.0 [SI::'m/s']`). A unit the model does
not declare, one of another dimension, one on a parameter that is no quantity (a `Real`), or
one on a string or a truth is refused. The process
must exit 0 within `OPENSYSML_TOOL_TIMEOUT` (default `10s`). A non-zero exit (its standard
error is quoted), a reply that is not exactly one JSON object of this shape, a missing output,
an output no parameter receives, a key repeated at any depth, a member not of this shape
(`units` for `unit`), a `null` in place of a member, an `error` beside `outputs`, more
than `OPENSYSML_TOOL_MAX_OUTPUT` (default 64 MiB) on either standard stream, or the timeout is a
typed error that fails the performance, and with it the action, sweep row or analysis case
performing it; no default value is ever invented, and nothing falls back to the action's body.
The body is never run when the metadata is present: with `OPENSYSML_TOOLS` unset or the tool
absent from it, the performance fails with `tool 'ModelCenter' is not registered; set
OPENSYSML_TOOLS`.

A calc computed by a tool follows the same exchange: `inputs` are the `in` and `inout`
parameters carrying `ToolVariable`, `outputs` the `out` and `inout` parameters carrying it,
together with the result parameter — keyed by its `ToolVariable` name when it carries one, else
its declared name, else `result`. The result a `calc def` invocation returns is the value bound
under that key; a calc usage invoked without arguments reports every output it names. The same
typed errors, the same refusal to run the body and the same divergence reporting apply — the
calculation fails as the tool failed, and no value is invented for an output the tool did not
answer.

A tool's answer stands as the value of that performance at strength *observed*: nothing in
OpenSysML knows what the tool should have computed. Two invocations with equal inputs answering
different outputs are reported as a divergence in the run's notes (`%trace` summarizes them), so
an exploration over a non-deterministic tool says its outcome table is not reproducible. See
[Analysis engines](cli.md#analysis-engines) and the design note
[`docs/internals/design/analysis-framework.md`](../internals/design/analysis-framework.md#external-tools).

## External engines

`OPENSYSML_ENGINES` names a second manifest directory, read at startup as `OPENSYSML_TOOLS` is
and under the same rules — `*.json` files only, outside every workspace, writable by their
owner alone, one fault registering nothing from the directory, a name taken by another entry of
either directory refused. Its entries are analysis engines rather than tools: a `kind: engine`
entry registers under its own `name` (`spin-bridge`, not `tool:spin-bridge`) beside `run`,
`explore`, `check`, `sweep` and `solve`, answers the question kinds it lists and is reached by
`-engine <name>`, `-engine all` and — after every built-in engine has refused — `-engine auto`.
The entry's fields, the protocol the engine speaks over its standard input, the model forms it
is handed, what stands of its answer and how each failure is reported are on
[External engines](external-engines.md). The directory may also hold `policy` and `sampler`
entries, which this build parses and lists as `unavailable` naming the stage that serves them.

`OPENSYSML_TOOL_TIMEOUT` is also the deadline for an engine to start and `describe` itself, and
for it to answer `cancel` once a plan's context ends; `OPENSYSML_TOOL_MAX_OUTPUT` bounds each
line it writes and the whole of its standard error, as it does a tool's reply.

The budgets are what turn a run that would never finish into a reported error instead
of a hang. They count different things (expression evaluations, action token
steps, dispatched events, do actions, materialized collection elements),
so raising one says nothing about the others, and each has its own variable.
`OPENSYSML_MAX_SWEEP_RUNS` counts runs rather than work inside a run: a plan whose
ranges would make more runs than it allows is refused before the first one is
made, naming the count the plan asks for and the bound it exceeds.
`OPENSYSML_JOBS` is no budget at all but the width of the fleet: how many of one check's
runs — an exploration's linearizations, a sweep's rows, the engines `-engine all` consults —
may go at once, and how many files of one load are parsed and validated at once. A value that is not a positive integer is refused at startup; `-jobs` and `%jobs`
override it for one invocation or session. See
[Running in parallel](cli.md#running-in-parallel).

A budget bounds **one run** (one `%eval`, one `%instantiate`, one `%calc`, one
action, one state machine), not a whole session, so a long REPL session of small
operations never runs out. A run started inside another, such as an action invoked
from an expression, shares the outer run's budget rather than getting a fresh
one, and so does a run stepped through with `%step`/`%advance`.

The step and event defaults are chosen by how long a runaway takes to report rather
than by memory. Those steps allocate nothing that outlives them (peak RSS is about 34
MB whether a run spends ten thousand steps or fifty million), and the only thing
they make grow is a `%trace`, at 34–83 bytes per entry. At the measured ~13.6M
evaluation steps/s and ~1.9M events/s, each default reports a runaway within about
a second, and a fully traced run at those four ceilings holds about 320 MB.

Collection elements are the exception, and `OPENSYSML_MAX_ELEMENTS` is the budget
that really is about memory: a materialized element is a 104-byte value that lives as
long as the collection holding it, and `1..10000000` creates one per step. Every way of
materializing a sequence is charged against it (a range, a sequence literal,
`->collect` and the other collection operations), so the default bounds the
elements held at once at about 104 MB, in the same range as the figures above:

```
error: evaluation failed: collection element limit exceeded
(1000000 elements; raise OPENSYSML_MAX_ELEMENTS to allow more)
```

Because it bounds memory rather than work, the count is what a statement's evaluation
holds at once: a loop building a ten-element collection a million times never approaches
it, while a single `1..2000000` exceeds it immediately.

`OPENSYSML_MAX_CALC_DEPTH` is about stack rather than work: a recursive calculation
evaluates to its result as long as it terminates within the depth, and one that
does not terminate is reported instead of exhausting the stack:

```
error: calc recursion limit exceeded: calc P::spin nested 10000 deep
(unbounded recursion?; raise OPENSYSML_MAX_CALC_DEPTH to allow more)
```

A nested invocation costs about 10 KB of stack, so this is the one budget with a
ceiling: a value above 25000 is refused, because past that point the goroutine stack
limit (a fatal error rather than a reported one) would be reached before the
budget was. A recursion that needs more depth than that should be rewritten as a loop.

The evaluation step budget:

```
error: execution failed: eval assignment RHS: evaluation step limit exceeded
(10000000 steps; raise OPENSYSML_MAX_STEPS to allow more)
```

A legitimately long run (a numeric integration in an action body, say) needs a
higher ceiling, so raise it for that run:

```bash
OPENSYSML_MAX_STEPS=200000000 sysml descent.sysml
```

Unset or empty means the default. Anything that is not a positive integer is
reported at startup (and at gRPC service construction) rather than silently
ignored:

```bash
$ OPENSYSML_MAX_STEPS=lots sysml model.sysml
sysml: OPENSYSML_MAX_STEPS="lots" is not an integer: set it to a positive number of evaluation steps (default 10000000)
```

The other budgets behave identically, and their errors name the variable that
raises them:

```
execution exceeded max steps (1000000 steps; raise OPENSYSML_MAX_ACTION_STEPS to allow more), possible infinite loop
state machine exceeded max events (1000000 events; raise OPENSYSML_MAX_EVENTS to allow more), possible infinite loop
state machine exceeded max do action steps (5000000 steps; raise OPENSYSML_MAX_DO_STEPS to allow more), possible non-terminating do behavior
```

A long simulation therefore raises the state machine bounds rather than the
evaluation one:

```bash
OPENSYSML_MAX_EVENTS=20000000 OPENSYSML_MAX_DO_STEPS=100000000 sysml descent.sysml
```

## The gRPC service's shared library index

`OPENSYSML_GRPC_INDEX_POOL` is no longer a count: any positive value asks
`sysml-grpc` to build the standard library index before the requests that need
it arrive, and `0` disables this prewarming. The library does not depend on the model
and is immutable once loaded, so the service builds and freezes **one** index and gives
each model a thin overlay on top of it holding that model's own document. A cache
miss adds its document to that overlay instead of loading and expanding the
library again (measured on a 163-line model: about 0.5–0.9 ms rather than
100–128 ms), and 100 cached models cost about 1 MiB in total rather than 1.6 GiB.

A model writes only into its own overlay, so cached models stay independent and
none can see another's document. A request that arrives before prewarming has finished
builds the shared index itself, so an answer never depends on how far prewarming
got, and concurrent requests wait for one build rather than starting several.

```bash
OPENSYSML_GRPC_INDEX_POOL=0 sysml-grpc   # load the library on the first request instead
```

Anything but a non-negative integer is reported at service construction rather
than silently ignored. The legacy `SYSML_GRPC_INDEX_POOL` name remains accepted
for compatibility with deployments that set it.

## Variables the test suite reads

None of the variables below is read by `sysml`, `sysml-lsp` or `sysml-grpc`; they govern `go test`
alone. Each corpus gate in the suite skips, announcing the skip, while the corpus it reads is
absent, and fails instead when its variable is set — so whatever sets one must run the matching
download script first (each is idempotent and refuses to report success over an empty corpus).

| Variable | Download | Gate |
|----------|----------|------|
| `OPENSYSML_REQUIRE_TRAINING_CORPUS` | `./scripts/download-training-examples.sh` → `examples/sysml-v2-training/` | `TestTrainingExamples*` in `tests/corpus` |
| `OPENSYSML_REQUIRE_PILOT_CORPORA` | `./scripts/download-pilot-corpora.sh` → `examples/pilot-corpora/` | `TestPilotCorpora*` in `tests/corpus` |
| `OPENSYSML_REQUIRE_PILOT_LIBRARY_XMI` | `./scripts/download-pilot-library-xmi.sh` → `build/pilot-library-xmi/` | `TestPilotLibraryXMI` in `internal/semantic/identity` |
| `OPENSYSML_REQUIRE_PSSM_SUITE` | `./scripts/download-pssm-suite.sh` → `build/pssm/` | `TestPSSMSuiteMigration` in `tests/corpus`, and the referee's gates in `tools/referee/pssm` |
| `OPENSYSML_REQUIRE_PDF_TOOLCHAIN` | `./scripts/download-doc-pdf-toolchain.sh` → `build/doc-pdf/` (WeasyPrint, pandoc, Mermaid CLI, KaTeX, Graphviz, the PlantUML jar; Java from the host) | `Test*Installed*` in `internal/doc/docpdf`, which draw a real PDF through each tool |

CI sets all five — the PDF one in its `pdf-toolchain` job, with the script's exports set; see [pilot-corpora.md](../project/pilot-corpora.md) for the pin the downloads
share and what the gates measure.
