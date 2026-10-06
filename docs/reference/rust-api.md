# The Rust client API

This page covers what the `opensysml` crate exposes and why it is blocking. To choose between the clients, see [client libraries](clients.md); for a
task-oriented walkthrough, see the [Rust client guide](../clients/rust.md). The crate's own notes on
binary provisioning and its trust model are in
[client/rust/README.md](../../client/rust/README.md).

The crate is published to crates.io as `opensysml` with each core release:

```toml
[dependencies]
opensysml = "0.9"
```

A consumer developing against a checkout takes it from a path or from git:

```toml
[dependencies]
opensysml = { git = "https://github.com/Open-MBEE/OpenSysML.git", branch = "main" }
```

The minimum supported Rust version is **1.83**, and there is no asynchronous
runtime: every call blocks. A caller that needs concurrency owns that decision —
threads, or its own runtime's blocking pool — rather than inheriting one from a
client library.

## The one-shot functions

```rust
use opensysml::{load, loads, load_with, loads_with, ParseOptions};

let model = loads("package Demo { part def Car; }")?;
let value = model.eval("2 + 2")?;
```

`load(path)` and `loads(content)` connect, parse, and hand back a `Model` that
keeps its connection alive. `load_with` and `loads_with` take `&ParseOptions`,
whose fields are `language` (`Language::Sysml` or `Language::Kerml`) and
`strict_conformance`; both are capability-gated, and the client checks before it
calls.

## Connection

```rust
use opensysml::Connection;

let connection = Connection::connect()?;                 // $OPENSYSML_SERVICE, else private
let private = Connection::private()?;                    // a child of this process
let external = Connection::external("localhost", 50051)?; // a service someone else runs
```

| method | what it does |
| --- | --- |
| `parse_file(&Path, &ParseOptions)` | parses a file the service can read |
| `parse_content(&str, &ParseOptions)` | parses inline content |
| `diagnostics(&model_hash)` | asks the service for a held model's diagnostics |
| `model_by_hash(&hash)` | adopts a model the service already holds |
| `server_info()`, `capabilities()` | what `GetServerInfo` reported, asked once at connect |
| `private_service_pid()` | the child's pid, or `None` for an external service |
| `parse_sources(&[SourceDocument], &SourcesOptions)` | parses files and named inline content as one model |
| `convert(to_format, &ConvertSource, &ConvertOptions)` | converts a file, content or held model; `IdForm` spells derived ids; a SysML v1 model is refused with `InvalidRequest` — it is migrated |
| `migrate(to_format, &MigrateSource, &MigrateOptions)` | migrates a SysML v1 model (`.mdzip`, `.xmi`, `.uml`, as a file or inline bytes) to notation or Turtle, answering a `Migration` whose `report` gives every element's verdict and whose `write(path)` puts the image files beside the model |
| `query(hash, &Query)`, `query_oslc(hash, text)` | selects elements, structured or as OSLC Query 3.0 text |
| `run_document_query`, `render_document` | runs a named document query, renders a named document as Markdown or HTML |
| `execute_action`, `execute_state`, `explore_action`, `explore_state` | runs a behavior once, or every valid order of its choice points |
| `verify_constraint`, `verify_requirement`, `verify_satisfaction`, `validate_instance` | verdicts, with bounds, standings, witnesses and engine verdicts |
| `calc`, `run_analysis`, `explore_analysis`, `run_sweep`, `list_engines` | calculations, analysis cases, parameter sweeps, the registered engines |
| `apply_edits(hash, document, operations)` | one atomic batch of source-preserving edits |
| `call(method, request)` | sends any `opensysml::wire` request message, for a field not yet wrapped |

`Model::render_view(name, RenderViewPorts::Minimal)` returns a typed
`RenderedView`; use `RenderViewPorts::Full` to include every declared port.
Unset geometry, style and canvas are represented as absent optionals. The RPC
requires the `render_view` capability.

A private child is started with `-port 0 -health-port 0 -report-address
-exit-with-parent` and its address read from its first stdout line, so no port is
chosen or probed. One child serves the process, so its parse cache is shared, and
`Drop` releases it deterministically — no garbage collector, no finalizer. `Drop`
does not run for `process::exit`, `abort` or `SIGKILL`, so the guarantee that
matters is the stdin pipe the client holds and never writes to: the child sees end
of file when the kernel closes it, however this process dies. Closing an external
connection never stops that service.

## Model, Symbol, Instance

```rust
let model = connection.parse_file(Path::new("model.sysml"), &ParseOptions::default())?;
model.hash();                          // what the service holds it under
model.diagnostics();                   // in the order the service reported them
model.root();                          // Option<&Symbol>

let car = model.symbol("Demo::Car")?;
for child in car.children()? { }       // one call per level

let value: Value = model.eval("2 + 2")?;
let evaluation = model.evaluate("mass", &EvalOptions { .. })?;   // subject, context
let built = model.instantiate("Demo::Car")?;
for instance in built.instances() {
    instance.feature("wheels");        // Option<&FeatureValue>
}
```

`Symbol` exposes `id()`, `name()`, `kind()` and `children()`; `Instance` exposes
`id()`, `type_symbol_id()`, `feature_values()` and `feature(name)`; `FeatureValue`
exposes `name()`, `value()`, `values()`, `materialized()` and `error()`, which is
how a feature the service could not compute is told apart from one it computed as
empty. Every domain type also has `wire()`, returning the protobuf message behind
it, for a caller that needs a field the typed surface does not carry yet — the
protocol layer is `opensysml::wire`, and it is documented as the protocol layer,
not the ergonomic one.

`Model` forwards each of those calls for its own hash, and adds `find`, `get`,
`lookup` (an `Error::SymbolNotFound` naming near matches) and `contains`, `edit()`,
`to_sysml`, `to_turtle`, `to_api_json`, `save(path, ..)` and `satisfied(scope)`. A
model parsed from several documents names them in `documents()`. `Symbol` adds
`type_facts()`, `multiplicity()`, `specializations()`, `metadata()` and `facts()`.

## Results

Each result keeps the response it was read from behind `wire()`. A false verdict is an
answer: `Verdict` carries `holds`, its `Standing`, `Bound`s, `WitnessAssignment`s and
the `FailureReason` when it could not be decided. `Satisfaction` holds one per
satisfaction assertion; `Validation` one per assertion about an object. `CalcResult` and
`AnalysisResult` carry their value, named outputs, verdicts and case evaluations; an
`ActionRun` or `StateRun` its outputs, the states visited and the final clock instant; an
`Exploration` its distinct `Outcome`s and whether it explored every order. A `SweepTable`
is one `SweepRow` per point, and a failed row keeps the outputs and verdicts it reached.
`RunOptions.trace` opts a state run into typed `DocumentEvent` records in
`StateRun.trace`, with `StateRun.trace_dropped` reporting records the service
discarded. The option requires `state_trace` and is refused with exploration.

## Editing

```rust
use opensysml::{Body, MemberOptions};

let mut editor = model.edit();
editor
    .rename("Demo::car", "sedan")
    .set_value("Demo::Car::mass", "1200.0")
    .add_member("Demo", "part", "spare", MemberOptions::new().typed("Car"));
let result = editor.apply()?;          // EditResult: content, documents, applied spans
```

`Editor` collects `set_value`, `rename`, `delete`, `move_to`, `add_member` and the
`add_*` declarations — objectives, verifications, metadata and metadata prefixes,
documentation, comments and notes, satisfy and requirement constraints, transitions,
imports, connections, allocations, flows, successions, calcs, actions, states and
asserts — and `Body` the statements of an action body, nested to any depth. `apply`
requires `apply_edits` and then each operation's own capability, in order, before it
sends anything; `in_document` selects a document of a model parsed from several. A
refusal is `Error::Edit`, whose `EditFailure` names what was wrong and, for a delete,
the elements that still refer to it.

## Values

`Value` is an ordinary enum, so a `match` over it is exhaustive:

```rust
match value {
    Value::Integer(v) => (),
    Value::Real(v) => (),
    Value::Rational(r) => (),          // exact; answered only when no f64 holds it, sent as rational_value always: r.numerator(), r.denominator() as decimal text; r.to_f64() rounds once
    Value::Complex(z) => (),           // z.real, z.imaginary; one value, Display as `1.5 - 2.0i`
    Value::Boolean(v) => (),
    Value::Text(v) => (),
    Value::InstanceRef(id) => (),
    Value::Sequence(values) => (),
    Value::Quantity(q) => (),          // Magnitude::Integer | ::BigInteger | ::Rational | ::Real, unit, unit_term
    Value::Array(a) => (),             // a.dimensions(), a.elements() row-major, a.get(&[i, j])
    Value::Vector(v) => (),            // v.components: Vec<Magnitude>, Integer and Real apart
    Value::VectorQuantity(q) => (),    // q.components(): one Quantity per component; q.unit() when shared
    Value::MeasurementRef(m) => (),    // a bare unit: m.unit, m.unit_term, m.unit_id when it names a declaration
    Value::Function(f) => (),          // a calc held as a value: f.calc_id, f.self_id when read off an object
    Value::Set(s) => (),               // a Collections::Set's elements: each once, unordered
    Value::TensorQuantity(t) => (),    // t.dimensions(), t.components() row-major, t.get(&[i, j, k])
    Value::Metaobject(m) => (),        // x meta KerML::Feature: m.element_id, m.metaclass_id
    Value::EnumLiteral(l) => (),       // literal_id, enumeration_id, name; value: the scalar a `high = 3` literal carries
    Value::Null => (),                 // evaluated, no value
    Value::Unset => (),                // a materialized feature with no value
    Value::Undetermined(u) => (),      // left open by the model: u.reason, u.count_lower, u.count_upper
    Value::Infinity => (),             // the unbounded `*`
}
```

Every variant but `Unset` and `Undetermined` — answers the service gives, never
arguments — can be sent, nested to a bounded depth. A request value is checked before it
is sent: a quantity's unit must be reduced and its scale exact, an array, vector or tensor
must fill its dimensions, and a function or metaobject must name its element; anything
else is `Error::UnsupportedValue`, and nothing is sent.

## Errors

`Error` is one enum over every way a call can fail, and its variants keep the
distinction the conformance suite draws: `Error::Service { status, message }` is a
refused call, `Error::Model(message)` a successful call whose answer reports a
model failure.

| variant | what happened |
| --- | --- |
| `Service { status: Status, message }` | the service refused the RPC, with a canonical gRPC status |
| `Model(String)` | the call succeeded and the answer reports a model failure |
| `MissingCapability { capability, remedy }` | the service advertises no such capability; the remedy names how to get one that does |
| `ServiceStart(String)` | a private service would not start, or would not stop cleanly |
| `BinaryNotFound { looked_in }` | no `sysml-grpc` resolved, naming everywhere that was looked |
| `UnsupportedPlatform { os, arch }` | no release asset is published for this platform |
| `BinaryDownload(String)` | a release could not be downloaded or installed |
| `ChecksumMismatch(String)` | a download's digest is not the one expected of it |
| `UnpinnedRelease(String)` | this crate version pins no digest for that release, so it cannot verify it |
| `Transport(String)` | the HTTP transport failed before a response was decoded |
| `Decode(String)` | a successful response was not valid protobuf |
| `InvalidRequest(String)`, `UnsupportedValue(String)` | a request or argument the client refused to send |
| `SymbolNotFound { name, suggestions }` | a lookup found nothing, naming the closest names |
| `ModelErrors { message, diagnostics }` | a strict parse found errors |
| `Conversion { message, diagnostics }` | the service could not write the model in that format |
| `Migration { message }` | the service could not read the SysML v1 model it was asked to migrate |
| `Unwritable(String)` | `Migration::write` refused its destination: it names the v1 model, or an image would land outside the model's directory |
| `Execution { message, reason, diagnostics, trace, trace_dropped }` | a run, verification, calculation or analysis could not be answered; a failed traced state run keeps its partial trace and discarded count |
| `WrongKind { message, diagnostics }` | the call named an element of another kind |
| `AnalysisRun { message, result }` | an analysis failed, keeping what it established |
| `Edit(Box<EditError>)` | the service refused an edit; nothing was written |
| `Query(String)` | a query the SysML v2 query model cannot express |
| `Io(std::io::Error)` | local filesystem or process IO failed |

## Capability negotiation

The advertised capability names are the negotiation surface, never the version
string, and the client checks request-side requirements before it calls:
`strict_conformance` for a strict parse, `inline_language` for inline KerML,
`evaluate_subject` when an evaluation names a subject, and for every other call the
capability of its RPC, of each option it is given and of the kind of every argument
value, nested ones included. That turns a round trip that
would come back `UNIMPLEMENTED` into a local `Error::MissingCapability` naming what
to install, as `upgrade_remedy(capability)` spells it. `Capabilities::has` and `Capabilities::require` are public, for a caller
gating its own use of something like `feature_values`. Decoding is never gated: an
enum, unset value or feature-value arm from a service is understood whatever the
handshake said.

## The service binary

Resolution is `$OPENSYSML_GRPC_BINARY`, then `~/.opensysml/bin/sysml-grpc` (the
cache the Python, Node and Java clients share, in the same
`sysml-grpc.json`/hard-link format), then a download of the release
`$OPENSYSML_GRPC_VERSION` names, then `$PATH`. No version means no download.

**One known gap, stated rather than papered over:** unlike the Python, Node and
Java clients, this one does **not** verify a release's sigstore-signed
`SHA256SUMS.txt`. It verifies the digests the crate itself pins, so a release newer
than the installed crate's pins cannot be verified here and is refused, naming the
gap; the only way through is `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`, which accepts
the checksum served beside the binary — same origin, so it detects a corrupted
transfer and nothing about a compromised release. In practice, installing a newer
release means upgrading the crate.

## Conformance

`make conformance-rust` runs the language-neutral scenarios through the typed API —
public surface only, responses read through the domain accessors — and writes the
report shape `tools/cmd/conformance` writes. The runner takes `-binary`, `-run`,
`-report FILE` (or `-report -`), `-allow-skips` and `-v`. Every RPC is driven through
its typed method. The expected skips are requests the typed API refuses to build — a
parse naming no source, an edit of several documents that does not accept them, a
query in both forms at once, a malformed value — and any other skip names the
capability it lacked and fails the run unless skips are allowed.
