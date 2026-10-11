# OpenSysML Rust client

`opensysml` is a blocking Rust client for the local `sysml-grpc` service. It
is published to crates.io with each core release, at the core's version.

For a task-oriented walkthrough, see the
[Rust client guide](https://runtime.opensysml.org/clients/rust/).

## Installation

From crates.io:

```toml
[dependencies]
opensysml = "0.9"
```

A crate published from a release tag carries that release's service digests in
its embedded `release-digests.json`, stamped from the release checksum manifest
at publish time, so its built-against default can verify and download the
binary. A crate built from a Git checkout, or asked for another release, still
needs a matching pin or `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`. The Rust client
does not verify the manifest's Sigstore signature itself. See
[docs/project/releasing.md](../../docs/project/releasing.md#releasing-the-rust-client-to-cratesio)
for how the crate is published.

For developing against a checkout, a path dependency still works:

```toml
[dependencies]
opensysml = { path = "../OpenSysML/client/rust/opensysml" }
```

The current Git dependency form is:

```toml
[dependencies]
opensysml = { git = "https://github.com/Open-MBEE/OpenSysML.git", branch = "main" }
```

The minimum supported Rust version is **Rust 1.83**.

## Why blocking

The client is blocking by default and has no asynchronous runtime anywhere in
its normal dependency tree. All 22 service RPCs are unary, and the usual
consumer talks to a local child that answers in milliseconds. Async buys the
average consumer little here, while putting a private `tokio::Runtime` in a
library taxes every consumer. That is why this client does not use `tonic`.

The runtime-free design also has no nested-runtime hazard: the library has no
`Runtime::new` that could panic inside an existing runtime. The
`blocking_calls_work_inside_a_runtime` test calls the client from
`Runtime::block_on` to pin this property. An async surface could be added
later behind a feature flag without changing the default.

## Transport

Protobuf request and response bodies are the default transport. JSON remains
available as the `curl` and debugging affordance. The measured comparison in
[`docs/internals/design/transport-evaluation.md`](../../docs/internals/design/transport-evaluation.md)
was 6.5 ms for protobuf versus 42 ms for JSON on a 468 KB response.

## Service lifecycle

There are two connection modes.

* `Connection::private()` starts one child process per parent process with
  `-port 0 -health-port 0 -report-address -exit-with-parent`. The address is
  read from the child's first stdout line. The child is shared, so its parse
  cache is shared too.
* `Connection::external(host, port)` explicitly connects to an existing
  service. `Connection::connect()` also accepts `$OPENSYSML_SERVICE`. Closing
  an external connection does not stop that service.

`Drop` gives deterministic cleanup rather than relying on Python garbage
collection or JVM finalizers. `Drop` does not run for `std::process::exit`,
`abort`, or `SIGKILL`, so the stronger guarantee is the stdin pipe: the client
holds it, never writes to it, and the child observes EOF when the kernel closes
it as the process dies. The SIGKILL lifecycle test pins this orphan-cleanup
behavior.

## Binary provisioning

Resolution is, in order:

1. `$OPENSYSML_GRPC_BINARY`, the explicit path;
2. `~/.opensysml/bin/sysml-grpc` (`sysml-grpc.exe` on Windows), the cache shared
   with the Python client;
3. a download into that cache of the release `$OPENSYSML_GRPC_VERSION` names,
   or the release this client was built against;
4. `sysml-grpc` on `$PATH`.

`$OPENSYSML_GRPC_VERSION` overrides the built-against default, and `latest`
resolves through the GitHub releases API. An executable cache without release
metadata is treated as a hand-installed binary and kept. When a requested
release cannot be downloaded, a working cache is kept with a warning, or a
binary on `$PATH` is used with a warning; checksum mismatches are never
answered from either.

The crate's behavior depends on how it was built. A release-tag crate receives
its own tag's digests at publish time from the release checksum manifest, so
its built-against default can be verified. A checkout build or a request for
another release still needs a matching embedded pin or
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`. This client does not verify the manifest's
Sigstore signature itself.

The download goes to a temporary file, is verified, and only then atomically
replaces the cache with mode `0700` (POSIX). Requests time out after 15 seconds.
Beside the binary the client writes `sysml-grpc.json` — `version`, `sha256`,
`repo` — the same shape the Python client reads and writes, and re-checks the
recorded digest before reusing a cache, so a hand-swapped binary is not read as
the release it displaced. Without `$HOME` (`$USERPROFILE` on Windows) there is no
cache: resolution says so rather than treating the working directory as a home.

The cache is one path several clients install over, so two things guard it. The
whole check-and-install is done holding `~/.opensysml/bin/sysml-grpc.lock` — the
same advisory lock the Python and Java clients take (`fcntl` on POSIX,
`LockFileEx` on Windows) — so no client pairs one release's bytes with another's
metadata; a lock that cannot be taken across processes is reported and the
install still runs, rather than failing to resolve a binary at all. What the
caller is then handed is not the cache path but a hard link (a copy where the
filesystem has no links) beside it named for its own digest,
`sysml-grpc-<first 16 hex digits>`, which the Python and Java clients name the
same way: a later install replaces the cache, never the file that was verified
and is about to be started.

| Variable | Effect |
|---|---|
| `$OPENSYSML_GRPC_BINARY` | Explicit binary path; nothing is downloaded. |
| `$OPENSYSML_GRPC_VERSION` | Release tag to install, or `latest`. |
| `$OPENSYSML_GITHUB_REPO` | Repository to download from; default `Open-MBEE/OpenSysML`. |
| `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` | `1`, or an `owner/repo` (comma-separated), to accept an unpinned release on same-origin trust. |

### Trust model, and what this client does not verify

A download is verified against the digest table embedded in the crate
([`opensysml/release-digests.json`](opensysml/release-digests.json)); a pin
resolved from outside the published artifact would not be a pin. The committed
copy is synced from `client/release-digests.json`, and the release job adds the
crate's own tag from the release checksum manifest when it packages a release.
A `.sha256` served beside the binary that disagrees with a pin is tampering: the
download is refused, and the cache is untouched.

**Known limitation:** unlike the Python, Node and Java clients, this client does
**not** verify the release's sigstore-signed `SHA256SUMS.txt` manifest
([`client/python/opensysml/signing.py`](../python/opensysml/signing.py) is the
reference). It verifies pins only, so a release the installed crate version pins
no digest for cannot be verified here at all and is refused, naming the gap. The
only way through is `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`, which accepts the
served `.sha256` with a warning — same origin as the binary, so it detects
corruption but not a compromised release. In practice, installing a release
newer than the crate's pins means upgrading the crate.

## Capability negotiation

The service's advertised capability list is the negotiation surface, and the
client checks it before it calls rather than relying on the refusal: a request
that needs a capability the service does not have is refused with
`UNIMPLEMENTED` naming that capability, and checking first turns that into a
local error naming what to install instead of a transport round trip.
Capabilities that only describe how a response is populated omit the fields they
name rather than refusing the call. Every typed call requires the capability of
its RPC (`parse_sources`, `convert`, `migrate`, `query`, `oslc_query`, `document_query`,
`render_document`, `verification`, `engines`, `apply_edits`, ...) and those of
the options it is given: `strict_conformance`, `inline_language`,
`evaluate_subject`, a named engine or question, an exploring schedule, a
performer, and the kind of every value sent as an argument, nested values
included. An editor requires `apply_edits` and then each capability its
operations need, in the order they were added, before anything is sent. A
service older than the client that refuses an RPC with `UNIMPLEMENTED` is read
the same way. `Error::MissingCapability` carries the capability and
`upgrade_remedy(capability)`: the release to install or the binary to build.

Decoding a response is never gated on capabilities: if a service sends an
enum, unset value, complex number, array, vector, vector quantity, measurement
reference, function, set, tensor quantity, metaobject, or feature-value arm,
the client understands that answer. Consumers can inspect `Capabilities::has`
or use `Capabilities::require` when they need to gate their own use of
`enum_values`, `unset_value`, `complex_values`, `structured_values`,
`measurement_refs`, `function_values`, `set_values`, `tensor_values`,
`metaobject_values`, `feature_values`, or another advertised operation.

A `Value::Set` is a `Collections::Set`'s elements: each member once, sent in
the service's canonical order (numbers ascending, then strings, and so on), and
equal to another set holding the same members in any order. Membership is
judged by `Value::same_value`, as the service judges it: `Integer(1)` and
`Real(1.0)` are one member, `Real(1.5)` and a `Complex` of `1.5 + 0.0i` are one
member, exactly across the whole `i64` range and beyond it (an Integer outside
`i64` arrives as `Value::BigInteger`, its exact decimal digits), and a `Quantity` is judged by
magnitude through its `unit_term`, so `1 m` and `100 cm` are one member (one
without a `unit_term` in its unit as written); `==` on `Value` stays
structural. A
`Value::TensorQuantity` is a `Quantities::TensorQuantityValue` of any rank:
its `dimensions()` and its `components()` flattened row-major, each a
`Quantity` with its own unit; `get(&[i, j, k])` takes one coordinate per
dimension. A rank-one tensor stays a `TensorQuantity`, distinct from a
`VectorQuantity`. A malformed set (a member listed twice) or tensor (a
non-positive dimension, or components that do not fill the shape) is an
`Error::Decode`, never a partial value.

A `Value::Metaobject` is an element of the model held as an instance of its
reflective metaclass: what `x meta KerML::Feature`, or the last element of
`x.metadata`, evaluates to. Its `element_id` is the FQN of the element
reflected on and is its identity: two metaobjects are `==` exactly when their
`element_id` is, whatever type each was cast to. Its `metaclass_id` is the FQN
of the element's own metaclass (`SysML::Systems::PartUsage`), not the type it
was cast to. Its features (`declaredName`, `ownedFeature`, ...) are read in the
model, not carried. A metaobject naming no element is an `Error::Decode`.

A `Value::Undetermined` is a model-level answer the model leaves open — an
attribute with no value, a count the multiplicity does not fix — as a
successful answer rather than an error: `reason` says why, `count_lower` and
`count_upper` bound its count as the model spells them. It is read, never
sent. `Value::Infinity` is the unbounded `*`, ordered above every finite
magnitude. An `EnumLiteral` of an enumeration that specializes a scalar type
(`enum def Level :> Integer { high = 3; }`) carries that scalar as `value`,
`None` otherwise.

## Typed surface

`Connection` wraps every service RPC, and `Model` the ones that read a parsed
model:

| Call | RPC | Answer |
|---|---|---|
| `parse_file`, `parse_content`, `parse_sources(&[SourceDocument], &SourcesOptions)` | `ParseFile`, `ParseSources` | `Model`; `documents()` names each document parsed together |
| `convert(to_format, &ConvertSource, &ConvertOptions)`, `Model::{to_sysml, to_turtle, to_api_json, save}` | `Convert` | `Conversion`, with `experimental_notice`; `IdForm` spells derived ids; a SysML v1 model is refused — it is migrated, not converted |
| `migrate(to_format, &MigrateSource, &MigrateOptions)` | `Migrate` | `Migration`: the notation or Turtle a `.mdzip`, `.xmi` or `.uml` model became, its `MigrationReport` (every element mapped, approximated, unmapped or skipped), the image files `write(path)` puts beside the model, and the `experimental_notice` |
| `query(&Query)`, `query_oslc(text)` | `Query` | `Vec<QueryElement>`; `Query` is built with `scope`, `select`, `filter` and `Constraint` |
| `run_document_query(id, bindings)`, `render_document(id, DocumentForm)`, `render_view(name)` | `RunDocumentQuery`, `RenderDocument`, `RenderView` | typed query rows, Markdown or HTML, or `RenderedView` with optional layout data |
| `execute_action`, `execute_state`, `explore_action`, `explore_state`, `explore_analysis` | `ExecuteAction`, `ExecuteState`, `RunAnalysis` | `ActionRun`, `StateRun`, `Exploration` (`Outcome`s, `complete`) |
| `verify_constraint`, `verify_requirement`, `verify_satisfaction`, `satisfied` | `Verify*` | `Verdict`, `Satisfaction`: `Bound`, `Standing`, `WitnessAssignment`s, `VerificationVerdict`s |
| `validate_instance`, `calc`, `run_analysis`, `run_sweep(&[SweepRange], &SweepOptions)` | `ValidateInstance`, `EvaluateCalc`, `RunAnalysis`, `RunSweep` | `Validation`, `CalcResult`, `AnalysisResult`, `SweepTable` of `SweepRow`s |
| `list_engines()` | `ListEngines` | `Vec<EngineInfo>` |
| `Model::edit()` then `Editor::apply()`, or `apply_edits` | `ApplyEdits` | `EditResult`: `content`, `EditedDocument`s, `AppliedEdit` spans |

`Model::find`, `get`, `lookup` and `contains` name symbols by short or
qualified name; `lookup` fails with `Error::SymbolNotFound` listing near names.
`Symbol::type_facts`, `multiplicity`, `specializations` and `facts` read the
resolved type a symbol declares.

A false verdict is an answer, not an error. A call the service could not answer
is `Error::Execution` with its `FailureReason`, or `Error::WrongKind` when it
named an element of another kind; an analysis that failed after establishing
something is `Error::AnalysisRun`, carrying that partial `AnalysisResult`, and a
failed sweep row keeps the outputs and verdicts it made. Every result keeps the
response it was read from behind `wire()`.

`Editor` collects source-preserving edits as `&mut Self` builder calls —
`set_value`, `rename`, `delete`, `move_to`, `add_member` and the `add_*`
declarations (objectives, verifications, metadata, documentation, comments,
satisfy and requirement constraints, transitions, imports, connections,
allocations, flows, successions, calcs, actions, states, asserts) — and sends
them in one atomic request. `Body` builds an action body's statements (`first`,
`then`, `accept`, `send`, `assign`, `if`, `while`, `loop`, `for`) to any depth.
`in_document` picks the document of a model parsed from several. A refused edit
is `Error::Edit`, an `EditError` whose `EditFailure` says what was wrong and
which elements still refer to a deleted one.

```rust
use opensysml::{loads, MemberOptions, VerifyOptions};

let model = loads("package Demo { part def Car { attribute mass = 1200.0; \
                   constraint light { mass < 1500.0 } } part car : Car; }")?;
let verdict = model.verify_constraint(
    "Demo::Car::light",
    &VerifyOptions { subject: Some("Demo::car".into()), ..Default::default() },
)?;
assert!(verdict.holds);

let mut editor = model.edit();
editor.add_member("Demo", "part", "spare", MemberOptions::new().typed("Car"));
let edited = editor.apply()?;
```

A request value is checked before it is sent: a quantity's unit must be reduced
and its scale exact, vectors, arrays and tensors must fill their dimensions, a
function or metaobject must name its element, and nesting is bounded.
`Value::Unset` and `Value::Undetermined` are answers the service gives, never
arguments, and are refused as `Error::UnsupportedValue`.
A value the service holds but cannot send arrives as `Error::UnsupportedValue`
naming it, never as `Value::Null`; on an instance feature the reason is
`FeatureValue::error` and the other features stay readable.

## Conformance runner

The workspace includes `opensysml-conformance`, which runs the language-neutral
scenarios through the typed client API. It uses the committed protobuf
descriptor to decode requests, calls only the public client surface, and reads
responses through domain `wire()` accessors before comparing normalized JSON.
The report has per-outcome totals for passed, failed, skipped, and errored
scenarios, including skipped scenarios.

Run it from the repository root:

```bash
make conformance-rust
```

Or run the binary directly:

```bash
cargo run --manifest-path client/rust/Cargo.toml -p opensysml-conformance -- \
  -binary bin/sysml-grpc \
  -scenarios conformance/scenarios \
  -fixtures conformance/fixtures \
  -report bin/conformance-report-rust.json
```

The runner accepts:

* `-binary PATH` to select the service binary;
* `-run SUBSTRING` to select scenario IDs;
* `-report FILE` or `-report -` for the JSON report;
* `-allow-skips` to allow capability-dependent skips;
* `-v` to print per-scenario timing.

The `-binary` default is `$OPENSYSML_GRPC_BINARY`, then `bin/sysml-grpc`
relative to the repository root. The expected skips are requests the typed API
refuses to build, reported as `unrepresentable by the typed API: <what>`: a parse
or document naming no source, an edit of several documents that does not accept
them, a query in both forms at once, and a malformed value. Other skips name the
missing capability and fail the run unless `-allow-skips` is supplied.

When a call answers successfully with a top-level `error`, the runner compares
the response the typed error or partial result retains.

`Connection::call` sends one method's request message from `opensysml::wire`
and decodes the response without the ergonomic layer, for a field a newer
service adds before this client wraps it.

## Release procedure

Before a release, `cargo package -p opensysml` must succeed cleanly. `cargo
publish` is a maintainer action; CI never publishes this crate.
