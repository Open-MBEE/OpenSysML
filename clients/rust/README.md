# OpenSysML Rust client

`opensysml` is a blocking Rust client for the local `sysml-grpc` service. It
is not published to crates.io yet. The crate name still needs to be checked
for availability, and publishing is a maintainer decision.

## Installation

For now, use a path dependency while developing against a checkout:

```toml
[dependencies]
opensysml = { path = "../OpenSysML/clients/rust/opensysml" }
```

The current Git dependency form is:

```toml
[dependencies]
opensysml = { git = "https://github.com/Open-MBEE/OpenSysML.git", branch = "main" }
```

The minimum supported Rust version is **Rust 1.83**.

## Why blocking

The client is blocking by default and has no asynchronous runtime anywhere in
its normal dependency tree. All 15 service RPCs are unary, and the usual
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
3. a download of the release `$OPENSYSML_GRPC_VERSION` asks for, into that cache;
4. `sysml-grpc` on `$PATH`.

A download only happens when `$OPENSYSML_GRPC_VERSION` names a release
(`latest` resolves through the GitHub releases API), so a caller that never asks
for one still resolves a locally built binary from `$PATH`. When a release *is*
asked for, the download precedes `$PATH`, because a binary on `$PATH` is of no
known version and so does not answer for that release. A cached binary that is
another release is replaced with a warning, never used silently; a replacement
that cannot be downloaded leaves the working cache in place, unless the refusal
was about integrity.

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

A download is verified against the digest table the crate ships
([`opensysml/release-digests.json`](opensysml/release-digests.json), a synced copy of
`clients/release-digests.json` embedded with `include_str!`) — a pin resolved
from outside the published artifact would not be a pin. A `.sha256` served
beside the binary that disagrees with a pin is tampering: the download is
refused, and the cache is untouched.

**Known limitation:** unlike the Python, Node and Java clients, this client does
**not** verify the release's sigstore-signed `SHA256SUMS.txt` manifest
([`clients/python/opensysml/signing.py`](../python/opensysml/signing.py) is the
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
name rather than refusing the call. The client checks request-side requirements
for:

* `strict_conformance` when strict parsing is requested;
* `inline_language` for inline KerML content; and
* `evaluate_subject` when a subject symbol is supplied for evaluation.

Decoding a response is never gated on capabilities: if a service sends an
enum, unset value, complex number, array, vector, vector quantity, measurement
reference, function, set, tensor quantity, or feature-value arm, the client
understands that answer. Consumers can inspect `Capabilities::has` or use
`Capabilities::require` when they need to gate their own use of `enum_values`,
`unset_value`, `complex_values`, `structured_values`, `measurement_refs`,
`function_values`, `set_values`, `tensor_values`, `feature_values`, or another
advertised operation.

A `Value::Set` is a `Collections::Set`'s elements: each member once, sent in
the service's canonical order (numbers ascending, then strings, and so on), and
equal to another set holding the same members in any order. Membership is
judged by `Value::same_value`, as the service judges it: `Integer(1)` and
`Real(1.0)` are one member, `Real(1.5)` and a `Complex` of `1.5 + 0.0i` are one
member, exactly across the whole `i64` range, and a `Quantity` is judged by
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
cargo run --manifest-path clients/rust/Cargo.toml -p opensysml-conformance -- \
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
relative to the repository root. The two expected v1 boundary skips are
`v1 API does not cover <RPC>` and
`unrepresentable by the typed API: ParseFile with no source`. Other skips name
the missing capability and fail the run unless `-allow-skips` is supplied.

When a covered RPC answers successfully with a non-empty top-level `error`, the
typed API exposes `Error::Model(message)` and does not retain the rest of that
response. The runner therefore compares a partial reconstruction,
`{"error": message}`. This is intentionally fail-safe: an expectation that
names another response field alongside the top-level error fails rather than
passing. Widening this representation requires the client to carry the whole
response on an in-band error, which is outside the v1 boundary.

## v1 boundary

The current API deliberately does not include generated model-ergonomics types
beyond its existing domain objects, the edit API, RDF conversion, or
verification helpers. The conformance runner consequently skips RPCs that the
typed v1 API does not cover.

## Release procedure

Before a release, `cargo package -p opensysml` must succeed cleanly. `cargo
publish` is a maintainer action; CI never publishes this crate.
