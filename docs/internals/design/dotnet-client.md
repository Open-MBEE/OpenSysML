# The .NET client

`OpenMBEE.OpenSysML`, in `client/dotnet`, is the C# client for `sysml-grpc`: one asynchronous,
typed method per thing the service does, over any of the three wires, starting a private service
when none is named. This note records how it is built and the decisions a maintainer would
otherwise have to reconstruct from the code; the [README](../../../client/dotnet/README.md) and
[DEVELOPING](../../../client/dotnet/DEVELOPING.md) state every rule in full.

**Status.** Experimental: both assemblies are `[Experimental("OSML0001")]`, which a consuming
project suppresses to use them, and a release may change their API. The client is consumed from a
checkout by `ProjectReference` and published to no registry.

## Shape

- **Connection and model.** A `Connection` is one service, safe to share between threads; it asks
  `GetServerInfo` once and keeps the answer. A `Model` names a parse by its hash and holds nothing a
  later call changes. Every RPC has a typed method on one of the two; `Connection.CallAsync` reaches
  any RPC with the generated messages, for a field the typed surface does not cover.
- **Results and inputs.** What the service answers with is immutable and has internal constructors,
  so a release can add to it. What a caller passes has public constructors: options, edits, values
  (every arm of `Value`, `DocumentValue` and `ElementPropertyValue`), document bindings, query
  constraints, and the instances the generated typed classes read.
- **Failures.** Everything derives from `OpenSysMLException`, except cancellation, disposal and an
  argument refused before anything is sent, which are .NET's own. A failed call is a
  `CallException`: a `ServiceException` for a status the service sent, a `TransportException` for one
  this client gave a call that read no well-formed answer. A failure the service reports inside an
  answer is a `ModelException`, and a capability the service lacks a `CapabilityException`, raised
  before sending where the advertised list shows it.
- **Layout.** The public types sit in `OpenMBEE.OpenSysML`. Their machinery is in four internal
  namespaces: `Transport` (the two transports and what they share), `Lifecycle` (private services),
  `Provisioning` (finding, downloading and caching binaries) and `Mapping` (reading and writing the
  wire messages).

## Transports

gRPC, through `Grpc.Net.Client`, is the default. The Connect protocol, with protobuf or JSON bodies
over HTTP/1.1, is there for proxies that do not carry HTTP/2. No ConnectRPC library exists for
.NET, so `ConnectTransport` implements unary Connect itself: the request and error bodies, a gzip
answer inflated within a bound and checked against its CRC and length, and the deadline, sent as
`Connect-Timeout-Ms` and enforced locally because the service keeps working past it.

A status is the service's only when the service sent it. Over Connect that is a well-formed Connect
error body. Over gRPC the library raises statuses it made up and statuses it read alike, so
`GrpcTransport` observes each response's own `grpc-status` through a `DelegatingHandler` that finds
its call through an `AsyncLocal`. That depends on `Grpc.Net.Client` flowing the caller's execution
context into its handler, which it does today; `TransportContractTests` fails if it stops.

## Private services

With no address named, `Connection.OpenAsync` starts `sysml-grpc` as a child on a port the kernel
chooses (`-port 0 -report-address`) that exits with this process however it ends
(`-exit-with-parent`). Connections whose binary, contents, limits and required version agree share
one child, and the last to close stops it, killing it and what it started when it does not exit in
time. A child that exits on its own is not restarted; the next open starts another.

## Provisioning and trust

The binary is the one named, else the shared cache `~/.opensysml/bin`, into which the release this
client was built with, or the one asked for, is downloaded, else `sysml-grpc` on `PATH`. The cache is
shared with the Python, Java and Rust clients under their lock: `lockf` on a descriptor libc opened
on POSIX and `LockFileEx` on Windows, never a `FileStream`'s, whose implicit `flock` would conflict
on macOS. An install is staged and published under the lock, so a client killed midway leaves the
cache as it was. The tests hold the lock against a process making the Python client's calls, which
stands for the Java and Rust clients' identical lock, as it does in the Rust client's own test.

A download is trusted only by digest. `release-digests.json`, a byte-for-byte copy of
`client/release-digests.json` embedded in the assembly, pins each release's assets, and a release
it pins nothing for is refused (`UnpinnedReleaseException`) unless the caller grants same-origin
trust. The Python, Java and Node clients can also accept a release by its Sigstore signature; this
client, like the Rust one, cannot. The only Sigstore implementation for .NET is a community
prerelease (`Sigstore` 1.1.0-alpha), and the trust path should not rest on one. The cost is that a
checkout cannot download a release made after it without same-origin trust, and that the checkout
of a release tag does not pin that release, whose digests exist only once its binaries are built;
the pin pull request each tag opens adds them on `develop`.

## Publishing

Nothing publishes the client: a release is the checkout of the core `v<version>` tag, whose
`Version` must be the tag's and whose public API `promote-api` has moved into
`PublicAPI.Shipped.txt`. Publishing to NuGet would take:

- **Packages.** `OpenMBEE.OpenSysML` and `OpenMBEE.OpenSysML.Metamodel` as packages, and the
  generator as a .NET tool.
- **Versions.** The core version in its SemVer spelling, as now, with a prerelease suffix while
  the assemblies are experimental.
- **A job on the tag.** It would pack and push with an API key held by the release context, and
  publish symbols (`.snupkg`) with Source Link over the deterministic build the client already has.
- **Package metadata.** The license, README and project URL.

None of it is needed to use the client from a checkout, so it waits for the API to be declared stable.

## Testing

- **Fake service.** An in-process `HttpMessageHandler` speaks all three wires, so request and
  response mapping is tested byte for byte over each. A test whose call is refused before sending
  runs on one.
- **Live tests.** These run against a real `sysml-grpc`, which CI requires rather than skips.
- **Conformance.** The conformance runner drives the shared scenarios through the typed API alone,
  as `BannedSymbols.txt` enforces. Its report must agree with the Go reference runner's on the same
  service, and mutated answers prove the comparison is not blind.
- **Lifecycle probes.** Separate processes test what one process cannot: orphaned children, copies of
  the client in collectible load contexts, and a cache transaction killed at each step.

## Open questions

- **Unwrapped RPCs.** Whether an RPC the client does not wrap fails its CI, as now, or is skipped as
  the other clients skip it.
- **Operating systems.** Whether the .NET jobs keep running on Linux, macOS and Windows, where every
  other client's run on Linux.
- **Transports.** Whether a client needs both gRPC and Connect, given that the second is
  implemented by hand here.
- **Shared cache.** The clients share `~/.opensysml/bin` by a format none of them writes down and
  an OS lock this client takes only through native calls; this provisioning requires intentional
  design.
