# OpenSysML MATLAB client

The `+opensysml` package calls the `sysml-grpc` service over Connect-JSON,
without protobuf-generated code. It supports **MATLAB R2019b+** and **GNU
Octave 7+**.

For a task-oriented walkthrough, see the
[MATLAB client guide](https://opensysml.org/clients/matlab/).

## Requirements and installation

Add `client/matlab/` to the MATLAB or Octave path:

```matlab
addpath('client/matlab')
```

Provide a running service or a `sysml-grpc` binary; the client does not
download one. MATLAB uses `matlab.net.http`; Octave uses a `curl`
subprocess. The Octave + curl path is exercised by CI. `opensysml.private`
starts a child service through Java's `ProcessBuilder`; Octave builds
without Java should use `opensysml.external` or `OPENSYSML_SERVICE`.

## Connections and capabilities

```matlab
conn = opensysml.connect();  % OPENSYSML_SERVICE, otherwise a private child
conn = opensysml.external('localhost:50051');
conn = opensysml.private('binary', '/path/to/sysml-grpc', 'timeout', 30);
```

`private` accepts `binary`, `timeout`, `version`, and `capabilities`
name-value options; `external` accepts `version` and `capabilities`.
`connect` forwards the applicable requirements to whichever connection it
selects. When no binary is named, `private` checks
`OPENSYSML_GRPC_BINARY`, `~/.opensysml/bin/sysml-grpc`, then `PATH`. The
child starts with `-port 0 -health-port 0 -report-address
-exit-with-parent`; closing the connection closes its stdin and stops the
child. Closing an external connection leaves its service running.

`[version, names] = conn.serverInfo()` fetches and caches the service's
version and capability names. `conn.hasCapability(name)`,
`conn.describe()`, `conn.require(name)`, and `conn.requireAll(names)`
inspect or enforce service support. `opensysml.capabilities()` returns a
struct mapping MATLAB camelCase property names to the wire's
snake_case capability strings. An absent required capability raises
`opensysml:missingCapability`; details are available through
`opensysml.lastError()`.

## Parsing and models

```matlab
model = opensysml.parseFile(conn, 'model.sysml', ...
    'language', 'sysml', 'strict', true);
model = opensysml.parseSource(conn, ...
    'package P { part def B; }', 'name', 'p.sysml');
sources = {opensysml.SourceDocument.file('base.sysml'), ...
    opensysml.SourceDocument.inline('other.sysml', sourceText, 'sysml')};
model = opensysml.parseSources(conn, sources);

model.ok()
model.errors()
car = model.get('Demo::Car');
candidate = model.find('Car');
symbols = model.walk(3);
```

`SourceDocument.file(path)` names a file by its path.
`SourceDocument.inline(name, content, language)` names inline content for
diagnostics; its language is `sysml` or `kerml`. A cell of documents can
mix these forms. `parseSources` parses related documents as one model.
`parseFile`, `parseSource`, and `parseSources` accept `strict`,
`strictConformance`, and `raiseForErrors` options; `strict` is an alias
for `strictConformance`. `strictConformance` requires that capability.
`raiseForErrors` returns a valid model or raises
`opensysml:diagnostics:model` with diagnostics and the parsed model in the
error details.

A `Model` exposes `hash`, `connection`, `diagnostics`, `documents`,
`root`, `roots`, and `sourcePath`. Its methods include `errors`, `ok`,
`raiseForErrors`, `symbol`, `get`, `find`, `walk`, `edit`, `evaluate`,
`instantiate`, `executeAction`, `exploreAction`, `executeState`,
`exploreState`, `verifyConstraint`, `verifyRequirement`,
`verifySatisfaction`, `satisfied`, `validateInstance`, `calc`,
`runAnalysis`, `exploreAnalysis`, `runSweep`, `query`,
`runDocumentQuery`, `renderDocument`, `convert`, `toSysml`,
`toTurtle`, `toApiJson`, and `save`. `get` resolves a qualified id;
`find` accepts a short name and can return an empty value when absent.
`walk` traverses the model's children.

`opensysml.getSymbol(model, id)` returns the `GetSymbol` answer as a plain
record; `model.symbol(id)` and model lookup wrap it in a `Symbol` with
`id`, `name`, `kind`, `typeFacts`, `multiplicity`, `specializations`, and
`record`. It provides `children()`, `attributes()`, `parts()`,
`getAttr(name)`, `facts()`, and `attributeFacts()`. The function is named
`getSymbol` rather than `symbol` so that its file does not collide with
`Symbol.m` on a case-insensitive file system.
`opensysml.diagnostics(model)` returns diagnostic records as a cell array.

## Values

`opensysml.decodeValue` reads all twenty-two `Value` arms;
`opensysml.encodeValue` writes request values.

| Wire arm | MATLAB/Octave value |
| --- | --- |
| `intValue` | Exact `int64`; JSON decimal digits are never routed through a double. |
| `bigIntValue` | `struct('bigInteger', char)`: an Integer beyond `int64`, kept as its decimal digits since no MATLAB number holds it, and sent back as written. A quantity's `bigIntMagnitude` and a vector component decode the same way. |
| `realValue` | `double`, including `"NaN"`, `"Infinity"`, and `"-Infinity"`. |
| `boolValue` / `stringValue` | `logical` / `char`. |
| `instanceId` | `struct('instanceRef', int64)`. |
| `sequence` | Cell array; `null` decodes to `[]`. |
| `unset` / `infinity` | `struct('unset', true)` / `struct('infinity', true)`. |
| `quantity` / `vectorQuantity` / `tensorQuantity` | Magnitude/unit structs or component cells. |
| `enumLiteral` | Struct with `literalId`, `enumerationId`, `name`, and `value`. |
| `complex` | MATLAB/Octave complex number. |
| `array` / `vector` | Dimensioned element cell or numeric component struct. |
| `measurementRef` / `function` / `set` / `metaobject` | Struct wrappers retaining wire ids and members. |
| `undetermined` | Struct with `reason`, `lower`, and `upper`; cannot be sent. |

Constructors and helpers include `quantity`, `infinity`, `enumLiteral`,
`instanceRef`, `objectRef`, `elementRef`, `functionRef`, `metaobject`,
`parseUint64`, and `isExperimental`. Format constants are
`FORMAT_SYSML`, `FORMAT_TURTLE`, and `FORMAT_API_JSON`. JSON lists are
cell arrays: `jsonencode` serializes a one-element struct array as an
object, while `{}` represents an empty list.

## Execution, verification, analysis, and results

Model methods and package functions cover expression evaluation,
instantiation, action/state execution and exploration, verification,
instance validation, calculation, analysis, and parameter sweeps.
Options are MATLAB name-value pairs such as `inputs`, `events`,
`schedule`, `performer`, `subject`, `arguments`, `namedArguments`,
`engine`, and `question`, as supported by each call.

- `Verdict` records a verification or execution outcome and provides
  `explain`, `raiseForError`, and logical conversion.
- `Validation` groups instance verdicts, diagnostics, verification
  records, summary and `valid`, `violated`, `undecided`, and `explain`.
  Its `instances` property is an ordered cell array in service order.
- `Standing` describes the engine, strength and bounds behind a result.
- `CalcResult` carries `value`, `outputs`, diagnostics and standing.
- `AnalysisResult` carries outputs, verdicts, verifications, evaluations,
  instances and `selected`, `satisfied`, and `explain`.
- `SweepTable` carries rows, parameters, sample metadata, instances,
  diagnostics, standing and `failures`.
- `Exploration` carries outcomes, completeness, run budgets and status;
  use `raiseForIncomplete` when incomplete exploration is not acceptable.
- `EngineInfo` describes an engine returned by `opensysml.listEngines`.

Named engines are capability-gated through `engines`; exploration
requires an explore schedule. `opensysml.exploreAction`,
`opensysml.exploreState`, and `opensysml.exploreAnalysis` return an
`Exploration`, while the corresponding `execute*` and `runAnalysis`
functions request a single run.

## Conversion and persistence

`opensysml.convert(conn, toFormat, ...)` takes exactly one of `filePath`,
`content`, or `modelHash`, plus optional `fromFormat` and
`tolerateSyntaxErrors` name-value arguments. `model.convert(format)` and
`model.save(path, ...)` operate on a parsed model. The returned
`Conversion` has `content`, `fromFormat`, `toFormat`, `diagnostics`, and
`write(path)`. `EditResult` also supports `save(path)`.

A SysML v1 model (`.mdzip`, `.xmi`, `.uml`) is migrated, not converted:
`opensysml.convert` refuses it, and `opensysml.migrate(conn, toFormat,
'filePath', path, 'report', true)` — or `'content', bytes, 'fromFormat',
'mdzip'` for an archive carried inline — returns a `Migration` with
`content`, `report` (the `mapped`, `approximated`, `unmapped` and `skipped`
counts, every element's entry with `report`, selected by `byVerdict`),
`results`, the image `files`, `sourcePath`, and `write(path)`, which puts the
images beside the model and refuses to overwrite the v1 model — under any
spelling or link to it — or to write outside the model's directory, through
`..`, an absolute path or a symbolic link. The options are `fromFormat`, `report`,
`results`, `layoutPath` or `layoutContent`, `imageBaseUrl` and `strict`.

## Queries and documents

`model.query('oslc.where=...')` sends an OSLC query. For the standard
SysML v2 API query, use `opensysml.buildQuery` with `scope`, `select`, and
`where` fields or construct its structured payload, then pass it to
`model.query`. Query results are cell arrays of element records, with
property maps.

`model.runDocumentQuery(queryId, 'bindings', bindings)` runs a named
document query and returns `DocumentQueryResult` (`columns` and `rows`).
Bindings may be a struct or `containers.Map`; scalar values and the
`ElementRef`/`ObjectRef` helpers represent supported binding types.
`model.renderDocument(documentId, 'form', 'markdown')` renders Markdown;
`'html'` requires the `render_document_html` capability.

## Authoring edits

`model.edit()` or `opensysml.Editor(model)` creates a chainable editor.
The builder exposes `setValue`, `rename`, `addMember`, metadata,
documentation, comments and notes, requirements and constraints,
transitions and action-body statements, imports, connections, flows,
allocations, succession, `move`, and `deleteElement`, plus declaration
helpers such as `addPartDef`, `addAttribute`, `addAction`, `addCalc`,
`addRequirement`, and `addState`. Positional arguments keep their
documented order; optional arguments use lowerCamel MATLAB name-value
pairs.

```matlab
body = opensysml.Body();
body.addAssign('count', 'count + 1');
thenBody = opensysml.Body();
thenBody.addWhile('count < 3', body);
editor = model.edit();
editor.addMember('Demo::Car', 'attribute', 'mass', ...
    'type', 'ScalarValues::Real', 'doc', 'Vehicle mass');
editor.addIf('Demo::Car::run', 'count < 3', thenBody);
result = editor.apply();
```

`Body` builds nested `if`, `while`, `loop`, and `for` action statements
along with accept, send, assignment, termination, and sequence items.
Nesting is limited to 128 levels. `opensysml.applyEdits(model, operations)`
accepts a cell array of wire operation structs; the editor's hidden
`operations()` method exposes the built list for inspection.
`EditResult` extends `Conversion`, with `applied` and `documents` cells
containing the service's edit records. An empty editor raises the typed
`noOperations` edit failure; applying the same editor twice raises
`opensysml:argument`. Edits require `apply_edits` and the relevant
authoring capabilities; the client checks them before sending the
request.

## Errors

Errors are MATLAB `MException`s with `opensysml:*` identifiers. In
Octave, where exception subclassing is unavailable, the same identifiers
are raised as ordinary errors.

| Identifier | Meaning |
| --- | --- |
| `opensysml:argument`, `opensysml:encode`, `opensysml:decode`, `opensysml:unknownArm` | Invalid options, values or wire arms. |
| `opensysml:transport` | Connection failure, timeout, or non-JSON response. |
| `opensysml:connect:<kind>` | Connect status such as `modelNotFound`, `symbolNotFound`, `modelFileNotFound`, `invalidRequest`, `unavailable`, `serviceTimeout`, or `unsupportedOperation`. |
| `opensysml:missingCapability` | Required service capability is not advertised. |
| `opensysml:staleService` | Service version or required capability set does not match the requested requirements. |
| `opensysml:diagnostics:<kind>` | Model, conversion, migration, execution, verification or edit failure. Edit failures use `opensysml:diagnostics:edit:<failure>`. |
| `opensysml:experimental` | Warning emitted for experimental RDF conversion and for SysML v1 migration. |

`opensysml.lastError()` returns the most recently raised error record:
`identifier`, `message`, `diagnostics`, and structured `details` such as
the Connect code, service, capability, edit failure, and referring
elements. `opensysml.call` sends a decoded request to a named RPC;
`opensysml.callRaw` returns `[status, contentType, body]` without decoding.

## Not available in MATLAB/Octave

- Release download, pinned digests, and signature verification; this
  client never downloads a binary.
- Typed-module generation and the separate FMI runner.
- Dataframe and HTML display helpers.
- Process-wide sharing of a private child service.
- `model[fqn]` indexing; use `get` or `find`.

## Conformance runner and tests

`conformance/run_conformance.m` drives the shared scenarios through
`callRaw`:

```sh
octave --no-gui --eval "addpath('client/matlab'); addpath('client/matlab/conformance'); run_conformance('--binary','bin/sysml-grpc')"
octave --no-gui --eval "addpath('client/matlab'); addpath('client/matlab/conformance'); run_conformance('--address','localhost:50051','--report','-')"
```

Flags include `--binary`/`--address`, `--scenarios`/`--fixtures`,
`--run SUBSTRING`, `--report FILE`, `--allow-skips`, and `-v`.

Run the MATLAB/Octave suite from `client/matlab/tests`:

```sh
octave --no-gui --eval "run_tests"
```

Offline tests run without a service. Live tests use
`OPENSYSML_SERVICE` and skip when it is unset.
