# The MATLAB client API

This reference covers the `+opensysml` package. See [client libraries](clients.md) to compare
language surfaces, [guide chapter 9](../guide/09-clients.md#from-matlab) for a walkthrough, and
[client/matlab/README.md](../../client/matlab/README.md) for local test and conformance commands.

The package uses Connect-JSON over HTTP, with no generated protobuf code, and runs in **MATLAB
R2019b+** and **GNU Octave 7+**. MATLAB uses `matlab.net.http`; Octave uses `curl`. Add
`client/matlab/` to the path:

```matlab
addpath('client/matlab')
```

## Connections and capabilities

```matlab
conn = opensysml.connect();  % OPENSYSML_SERVICE, otherwise a private child
conn = opensysml.external('localhost:50051');
conn = opensysml.private('binary', '/path/to/sysml-grpc');
conn = opensysml.connect('version', 'v0.0.0', ...
    'capabilities', {'query', 'convert'});
```

`connect` selects `external` when `OPENSYSML_SERVICE` is set and `private` otherwise. `private`
accepts `binary`, `timeout`, `version`, and `capabilities`; `external` accepts `version` and
`capabilities`. When no binary is given, `private` checks `$OPENSYSML_GRPC_BINARY`,
`~/.opensysml/bin/sysml-grpc`, and `PATH`. It starts the process with
`-port 0 -health-port 0 -report-address -exit-with-parent` through Java's `ProcessBuilder`.
Octave built without Java cannot spawn a private service. `conn.close()` stops a private child
but does not stop an external service.

`[version, names] = conn.serverInfo()` asks `GetServerInfo` once and caches the result.
`conn.hasCapability(name)` tests the advertised names; `conn.describe()` returns a service
description; `conn.require(name)` and `conn.requireAll(names)` enforce support. The expected
`version` and `capabilities` options are checked against the service before use.
`opensysml.capabilities()` returns the known capability names as MATLAB camelCase struct fields
whose values are the wire's snake_case strings.

## Parsing and model values

```matlab
model = opensysml.parseFile(conn, 'model.sysml', ...
    'language', 'sysml', 'strict', true);
model = opensysml.parseSource(conn, ...
    'package P { part def B; }', 'name', 'p.sysml');
docs = {opensysml.SourceDocument.file('base.sysml'), ...
    opensysml.SourceDocument.inline('more.sysml', moreSource, 'sysml')};
model = opensysml.parseSources(conn, docs);
```

`SourceDocument.file(path)` represents a file read by the service. `SourceDocument.inline(name,
content, language)` represents inline SysML or KerML source and supplies the diagnostic document
name. `parseSources` accepts a cell array mixing these forms and parses the documents as one
model. `parseFile`, `parseSource`, and `parseSources` accept `language`, `strict`,
`strictConformance`, and `raiseForErrors` name-value options as applicable. `strict` is an alias
for `strictConformance`; specifying both requires the same logical value. The strict-conformance
option requires the `strict_conformance` capability. `raiseForErrors` returns a usable model or
raises a typed model diagnostic.

A `Model` carries `connection`, `hash`, `diagnostics`, `roots`, `root`, `documents`, and
`sourcePath`. Its `errors()` method returns error-severity diagnostics, `ok()` indicates whether
there are any, and `raiseForErrors()` raises with the diagnostics and model in the error details.
`model.edit()` opens an editor. Model operations are also exposed as methods:

| Area | Methods |
| --- | --- |
| Symbols | `symbol`, `get`, `find`, `walk` |
| Values and instances | `evaluate`, `instantiate` |
| Behaviors | `executeAction`, `exploreAction`, `executeState`, `exploreState` |
| Verification | `verifyConstraint`, `verifyRequirement`, `verifySatisfaction`, `satisfied`, `validateInstance` |
| Analysis | `calc`, `runAnalysis`, `exploreAnalysis`, `runSweep` |
| Query and documents | `query`, `runDocumentQuery`, `renderDocument` |
| Notation | `convert`, `toSysml`, `toTurtle`, `toApiJson`, `save` |

`get(fqn)` resolves a qualified id. `find(name)` accepts a short name and returns an empty value
when it finds no result. `walk(depth)` traverses child symbols to the requested depth; its default
is unlimited. `opensysml.symbol(model, id)` returns a `Symbol`, which exposes `id`, `name`,
`kind`, `typeFacts`, `multiplicity`, `specializations`, and the decoded `record`. Its methods
include `children()`, `attributes()`, `parts()`, `getAttr(name)`, `facts()`, and
`attributeFacts()`. Symbol collections use cells. `opensysml.diagnostics(model)` returns
diagnostic records with severity, message, file, line, column and code.

`opensysml.call(conn, method, request)` sends a decoded request to
`/sysml.SysMLService/<method>`; `opensysml.callRaw(conn, method, requestJsonText)` returns
`[status, contentType, bodyText]` without parsing. The conformance runner uses `callRaw` to send
scenario requests exactly as written.

## Values

`opensysml.decodeValue` reads all nineteen Connect-JSON `Value` arms and `opensysml.encodeValue`
writes supported request values. `intValue` uses decimal JSON strings and exact `int64`
accumulation, including `-9223372036854775808`; `realValue` supports `"NaN"`, `"Infinity"`, and
`"-Infinity"`. Other primitive mappings include `boolValue` to `logical`, `stringValue` to
`char`, `complex` to a complex scalar, and `sequence` to a cell array. `null` becomes `[]`.

Structured arms use MATLAB structs: `instanceId` uses `instanceRef`; quantity values use
`magnitude`, `unit`, and `unitTerm`; enums include `literalId`, `enumerationId`, `name`, and
`value`; arrays include `dimensions` and an `elements` cell; vectors include `components`;
measurement references retain unit fields; functions include `calcId` and `self`; sets contain
an `elements` cell; metaobjects retain `elementId` and `metaclassId`; `unset`, `infinity`, and
undetermined values have their named struct wrappers. An unset value, undetermined value, or
function with a self reference cannot be sent back as an input.

Public constructors and utilities include `quantity`, `infinity`, `enumLiteral`, `instanceRef`,
`objectRef`, `elementRef`, `functionRef`, `metaobject`, `parseUint64`, and `isExperimental`.
Format constants are `FORMAT_SYSML`, `FORMAT_TURTLE`, and `FORMAT_API_JSON`. Use cells for
repeated values and request lists: MATLAB's `jsonencode` emits a scalar struct array as one JSON
object, while an empty cell encodes as an empty list.

## Execution, verification, and result classes

The package functions and corresponding `Model` methods cover expression evaluation,
instantiation, single-run action and state execution, behavior exploration, constraint and
requirement verification, satisfaction verification, instance validation, calculations,
analyses, and parameter sweeps. Options are name-value pairs; supported options vary by method
and include `inputs`, `events`, `schedule`, `performer`, `subject`, `arguments`,
`namedArguments`, `engine`, and `question`. Explore schedules return an `Exploration`; a normal
`executeAction`, `executeState`, or `runAnalysis` request answers one run. Exploration entry
points are `opensysml.exploreAction`, `opensysml.exploreState`, and
`opensysml.exploreAnalysis`.

`opensysml.listEngines(conn)` returns `EngineInfo` records. Engine selection is available with
the `engine` option on supported calculation, verification, analysis, and sweep methods; a named
engine requires the `engines` capability. `Standing` records the engine, evidence strength and
bounds associated with an answer.

| Result class | Contents and helpers |
| --- | --- |
| `Verdict` | Outcome fields including `holds`, condition, witness, error, engine and diagnostics; `explain()`, `raiseForError()`, and logical conversion. |
| `Validation` | Verdicts, summary, ordered instance cells in service order, diagnostics and verifications; `valid()`, `violated()`, `undecided()`, `raiseForError()`, and `explain()`. |
| `CalcResult` | `value`, `outputs`, diagnostics and `Standing`; character conversion formats the computed result. |
| `AnalysisResult` | Outputs, verdicts, instances, verifications, evaluations and standing; `selected()`, `satisfied()`, and `explain()`. |
| `SweepTable` | Rows, parameters, sampling metadata, instances, diagnostics and standing; `failures()` selects failed rows. |
| `Exploration` | Outcome cells, `complete`, run and budget counts; `status()`, `ok()`, `explain()`, and `raiseForIncomplete()`. |
| `EngineInfo` | Engine identity, answers, bounds, readiness and protocol details; `explain()` describes it. |
| `Standing` | Engine, strength and bounds, with `reported()`, `reached()`, and `explain()`. |

The result's `diagnostics` and repeated records are cells of decoded records/objects, not MATLAB
object arrays.

## Conversion and saving

```matlab
conversion = opensysml.convert(conn, 'sysml', ...
    'content', sourceText, 'fromFormat', 'sysml');
conversion.write('out.sysml');

turtle = model.toTurtle();
saved = model.save('model.ttl');
```

`opensysml.convert(conn, toFormat, ...)` requires exactly one source:
`filePath`, `content`, or `modelHash`. Optional fields are `fromFormat` and
`tolerateSyntaxErrors`. `Model.convert(format, 'tolerateSyntaxErrors', true)` uses the parsed
model; `toSysml`, `toTurtle`, and `toApiJson` select common output formats. `Model.save(path,
...)` infers format from the path unless `format` is supplied and writes the returned content.
`Conversion` has `content`, `fromFormat`, `toFormat`, `diagnostics`, `experimental`,
`experimentalNotice`, `char`, `length`, and `write(path)`. Experimental RDF conversions can
issue the `opensysml:experimental` warning.

## Query and document APIs

`model.query('oslc.where=...')` preserves the OSLC text-query interface. The structured SysML v2
API query is built from `scope`, `select`, and `where`:

```matlab
q = opensysml.buildQuery([], ...
    'scope', {'Demo::Car'}, ...
    'select', {'name', '@type'}, ...
    'where', struct('@type', 'PrimitiveConstraint', ...
        'property', 'name', 'operator', '=', 'value', 'Car'));
elements = model.query(q);
```

`scope` accepts qualified names or id-reference records. `select` contains property names.
Constraints may be `PrimitiveConstraint` (`=`, `>`, `<`) or a non-empty
`CompositeConstraint` (`and`, `or`) containing constraints. `buildQuery` validates unknown
fields and malformed operators before sending. Query results are cells of element records with
`id`, `type`, and a `containers.Map` of properties.

`model.runDocumentQuery(queryId, 'bindings', bindings)` sends named parameter bindings and
returns a `DocumentQueryResult` with `columns` and `rows`; both are cell collections.
`bindings` may be a scalar struct or `containers.Map`. Primitive values, `quantity` structs,
`elementRef(id)`, and `objectRef(id, path)` are supported. `model.renderDocument(documentId,
'form', 'markdown')` returns Markdown; `'html'` requests HTML and requires both
`render_document` and `render_document_html`.

## Edit builder

`model.edit()` and `opensysml.Editor(model)` create a chainable handle editor. Each builder
returns the editor so operations can be composed. Positional arguments keep Python's order;
optional arguments use lowerCamel MATLAB name-value pairs. The public surface includes:

- Core operations: `setValue`, `rename`, `addMember`, `addMetadata`, `addMetadataPrefix`,
  `addObjective`, `addVerify`, `addSatisfy`, `addRequirementConstraint`,
  `addRequireConstraint`, `addAssumeConstraint`, `addDocumentation`, `addComment`, `addNote`,
  `addImport`, `move`, `deleteElement`, and `apply`.
- Connection and behavior declarations: `addTransition`, `addEntryTransition`, `addFirst`,
  `addThen`, `addAccept`, `addSend`, `addAssign`, `addIf`, `addWhile`, `addLoop`, `addFor`,
  `addTerminate`, `addGuardedThen`, `addElse`, `addConnection`, `addAllocation`, `addFlow`,
  and `addSuccession`.
- Declaration helpers include package, part, attribute, item, port, class, struct, datatype,
  classifier, feature, association, behavior, function, predicate, interaction, metaclass,
  calculation, action, parameter, return, state, constraint, requirement, and assertion forms.

`deleteElement` is the deletion builder name so the handle object's destructor remains
available as `delete`. `opensysml.Body` is another chainable handle for nested action bodies;
it supports sequence items, actions, accept/send, assignment, `if`, `while`, `loop`, `for`,
termination, guarded transitions and `else`. Recursive bodies are limited to 128 levels.
`Editor.operations()` is hidden from normal help but returns the wire operation cell for
inspection. `opensysml.applyEdits(model, operations)` exposes the low-level request wrapper and
accepts a cell of oneof operation structs.

`Editor.apply()` and `applyEdits` require `apply_edits` and operation-specific capabilities such
as `authoring`, `connection_authoring`, `satisfy_authoring`, `transition_authoring`,
`action_body_statement_authoring`, `member_modifiers`, and `edit_documents`. Missing
capabilities are checked before the RPC. A successful call returns `EditResult`, a `Conversion`
subclass with edited `content`, `applied` edit records, `documents` records, and `save(path)`.
Applying an empty editor raises the typed `noOperations` edit failure; applying the same editor
twice raises `opensysml:argument`.

## Errors and diagnostics

Errors use public `opensysml:*` identifiers. MATLAB raises `MException`; Octave raises ordinary
errors with the same identifiers. `opensysml.lastError()` returns a struct with `identifier`,
`message`, `diagnostics`, and `details` for the most recently raised client error.

| Identifier | Raised for |
| --- | --- |
| `opensysml:argument` | Invalid arguments or unsupported name-value options. |
| `opensysml:transport` | Connection errors, timeouts, or non-JSON responses where JSON was required. |
| `opensysml:connect:modelNotFound`, `symbolNotFound`, `modelFileNotFound` | Missing model, symbol, or source file from a Connect status. |
| `opensysml:connect:invalidRequest`, `unavailable`, `serviceTimeout`, `unsupportedOperation`, `service` | Other typed Connect status failures. |
| `opensysml:missingCapability` | The service does not advertise a required capability; details include `capability` and `service`. |
| `opensysml:staleService` | Requested `version` or required capability set does not match the service. |
| `opensysml:diagnostics:model`, `conversion`, `wrongKind`, `execution` | Model, conversion, kind, execution, or verification errors reported by a successful RPC. |
| `opensysml:diagnostics:edit:<failure>` | Typed edit failure. Failure names use lowerCamel, for example `unknownTarget`, `renameReferenced`, and `deleteReferenced`; `lastError().details` retains the wire enum and structured referrers. |
| `opensysml:encode`, `decode`, `unknownArm` | Value encoding/decoding contract violations or unknown wire values. |
| `opensysml:experimental` | Warning for experimental RDF conversion. |

The Connect status and service origin, missing capability, edit failure enum, referring elements,
and diagnostics are exposed in `lastError().details` or `lastError().diagnostics`. `Content-Type`
is checked before decoding; proxy HTML or plain-text errors are transport errors rather than
malformed JSON errors.

## Not available in MATLAB/Octave

- Release download, pinned digests and signature verification. The client never downloads a
  service binary.
- Typed-module generation.
- The separate FMI runner.
- Dataframe and HTML display helpers.
- Process-wide sharing of a private child service.
- `model[fqn]` indexing; use `get` or `find`.

## Conformance runner

```sh
octave --no-gui --eval "addpath('client/matlab'); addpath('client/matlab/conformance'); addpath('client/matlab/conformance/private'); run_conformance('--address','localhost:50051')"
make conformance-matlab
```

Flags: `--binary`/`--address`, `--scenarios`/`--fixtures`, `--run` substring, `--report` file
(`-` is stdout), `--allow-skips`, `-v`. `--binary` needs Java to start a private child; Octave
without Java uses `--address`. The report follows the shared JSON schema, and the exit status is
non-zero on fail or error, or on a skip without `--allow-skips`.
