# The Rust client API

This page covers what the `opensysml` crate exposes, why it is blocking, and where its v1
stops. To choose between the clients, see [client libraries](clients.md); for a
task-oriented walkthrough, see [guide chapter 9](../guide/09-clients.md#from-rust). The crate's own notes on
binary provisioning and its trust model are in
[clients/rust/README.md](../../clients/rust/README.md).

Nothing is published to crates.io yet, so a consumer takes it from a path or from
git:

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

## Values

`Value` is an ordinary enum, so a `match` over it is exhaustive:

```rust
match value {
    Value::Integer(v) => (),
    Value::Real(v) => (),
    Value::Complex(z) => (),           // z.real, z.imaginary; one value, Display as `1.5 - 2.0i`
    Value::Boolean(v) => (),
    Value::Text(v) => (),
    Value::InstanceRef(id) => (),
    Value::Sequence(values) => (),
    Value::Quantity(q) => (),          // Magnitude::Integer | ::Real, unit, unit_term
    Value::Array(a) => (),             // a.dimensions(), a.elements() row-major, a.get(&[i, j])
    Value::Vector(v) => (),            // v.components: Vec<Magnitude>, Integer and Real apart
    Value::VectorQuantity(q) => (),    // q.components(): one Quantity per component; q.unit() when shared
    Value::MeasurementRef(m) => (),    // a bare unit: m.unit, m.unit_term, m.unit_id when it names a declaration
    Value::Function(f) => (),          // a calc held as a value: f.calc_id, f.self_id when read off an object
    Value::EnumLiteral(l) => (),       // literal_id, enumeration_id, name
    Value::Null => (),                 // evaluated, no value
    Value::Unset => (),                // a materialized feature with no value
}
```

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
| `Io(std::io::Error)` | local filesystem or process IO failed |

## Capability negotiation

The advertised capability names are the negotiation surface, never the version
string, and the client checks request-side requirements before it calls:
`strict_conformance` for a strict parse, `inline_language` for inline KerML,
`evaluate_subject` when an evaluation names a subject. That turns a round trip that
would come back `UNIMPLEMENTED` into a local `Error::MissingCapability` naming what
to install. `Capabilities::has` and `Capabilities::require` are public, for a caller
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

## What v1 does not do

Deliberately out of scope: generated model-ergonomics types beyond the domain
objects above, the edit API, RDF conversion and the verification helpers. The
service still serves them, but this crate has no generic RPC escape hatch:
`Connection` exposes only the calls above, and `opensysml::wire` gives the message
types for reading a field off an answer, not a way to make a call the typed surface
omits. Reach those RPCs from the Go or Python client until a v2 wraps them here.

## Conformance

`make conformance-rust` runs the language-neutral scenarios through the typed API —
public surface only, responses read through the domain accessors — and writes the
report shape `cmd/conformance` writes. The runner takes `-binary`, `-run`,
`-report FILE` (or `-report -`), `-allow-skips` and `-v`. The two expected v1
boundary skips are an RPC the typed API does not cover and a `ParseFile` naming no
source; any other skip names the capability it lacked and fails the run unless
skips are allowed. Where a covered RPC answers successfully with a top-level error,
the typed API keeps only `Error::Model(message)`, so the runner compares
`{"error": message}` — an expectation naming another field alongside that error
fails rather than passes, which is the fail-safe direction.
