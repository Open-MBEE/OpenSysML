# OpenSysML .NET client

`OpenMBEE.OpenSysML` is the C# client for the `sysml-grpc` service, built from a checkout
of this repository and consumed by `ProjectReference`. It is not published to NuGet.

> **Status:** the client connects to a `sysml-grpc` that is already running, or starts a
> private one from a binary installed locally or from a release it downloads and verifies, over
> gRPC or the Connect protocol with protobuf
> or JSON bodies; reports what that service is and can do; and calls any of its methods
> through a raw, schema-checked escape hatch. It loads models from a file, from inline
> notation or from several documents, and reads them: symbols and name lookup, diagnostics,
> expression evaluation and instantiation, with the immutable values those operations
> exchange. It [runs and checks behavior](#running-and-checking-behavior): actions and state
> machines once or over every order of their choice points, and a search of every schedule
> for a violated property. It [verifies and analyzes](#verifying-calculating-and-analyzing)
> models: constraints, requirements, satisfactions and whole objects, calculations, analysis
> cases, parameter sweeps and the engines that answer them; it
> [runs them repeatedly](#running-repeatedly-self-checking-and-recording), self-checks and
> records runs. It [converts and migrates](#converting-migrating-and-saving) models, between
> notation, Turtle and the API's JSON form and from SysML v1, and saves them to disk without
> ever leaving a partial file. It [queries models, runs their document queries and renders
> their documents and views](#querying-documents-and-views), [edits their
> notation](#editing) while keeping everything an edit did not touch, and [exports behaviors'
> graphs, imports data and compares a SysML v1 tool's results with
> runs](#exporting-graphs-importing-data-and-comparing-results). Its [conformance
> runner](#conformance) runs the shared suite through that API, every RPC of the service
> covered. It [downloads releases of the service](#downloading-releases) into the
> [shared cache](#the-shared-cache) every client uses, under the same lock, and verifies them
> against the digests this checkout pins; unlike the [Python](../python/), [Java](../java/) and
> [Node](../node/) clients, and like the [Rust](../rust/) one, it does not verify a release's
> signature, so a release it pins no digest for is refused. Its [generator](#typed-model-classes)
> writes typed C# interfaces and classes over a model's definitions, which read instances through
> the library. A separate, standalone assembly holds [the SysML metamodel as C#
> types](#metamodel-types) and [reads an exported model's JSON](#reading-exported-json) into
> them, without a service.

For a task-oriented walkthrough, see the [.NET client guide](../../docs/clients/dotnet.md); the
[.NET API reference](../../docs/reference/dotnet-api.md) lists the surface by area.

## Requirements

- The .NET SDK that the repository-root [`global.json`](../../global.json) pins (10.0.402),
  with its .NET 10 runtime. Run every `dotnet` command from the repository root so that
  file selects the SDK; `make dotnet-preflight` checks both.
- Python 3, for the preflight and coverage checks the Make targets run.
- For the integration tests, a `sysml-grpc` binary: `make build-grpc` writes `bin/sysml-grpc`.

The library, tests and sample target `net10.0` only. Native AOT and trimming are not
supported: the assembly makes no `IsAotCompatible` or `IsTrimmable` claim.

## Using it from a checkout

Reference the project from your own:

```xml
<ItemGroup>
  <ProjectReference Include="path/to/OpenSysML/client/dotnet/src/OpenMBEE.OpenSysML/OpenMBEE.OpenSysML.csproj" />
</ItemGroup>
```

[`samples/Vehicle`](samples/Vehicle/) is such a consumer, and the program the
[.NET client guide](../../docs/clients/dotnet.md) walks through: a test holds the guide's code to
the sample's walkthrough region and requires its complete run command, including the service-binary
assignment. It prints the referenced assembly's version and target framework
and, when `OPENSYSML_SERVICE` names a running service or `OPENSYSML_GRPC_BINARY` names a binary
to start privately, what that service reports, then parses a vehicle model, checks its
diagnostics, looks the sedan up and evaluates its redefined mass:

```bash
make build-grpc
OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" dotnet run --project client/dotnet/samples/Vehicle -c Release
# OpenMBEE.OpenSysML <version> (.NETCoreApp,Version=v10.0)
# sysml-grpc <version>, <count> capabilities
# Demo::sedan (partUsage): mass = 1800.0
```

## Connecting to a service

```csharp
using OpenMBEE.OpenSysML;

await using var connection = await Connection.OpenAsync(new ConnectionOptions
{
    Address = "localhost:50051",
    RequireCapabilities = [Capabilities.Query],
});
Console.WriteLine(connection.ServerInfo);              // sysml-grpc <version>, <count> capabilities
bool converts = connection.Capabilities.Has(Capabilities.Convert);
```

`Connection.OpenAsync` reaches the service `ConnectionOptions.Address` names, or else the one
the `OPENSYSML_SERVICE` environment variable names, or else starts a private one (see
[Private services](#private-services)). An address is `host:port`, which means
`http://host:port`, a bracketed IPv6 `[::1]:port`, or an `http` or `https` URL with nothing
after its port; user information, a path, a query or a fragment is refused. A service named
this way is never started, stopped or downloaded by the client.

Opening asks the service `GetServerInfo` once. `ServerInfo` keeps the answer: the service's
version, informational only, and its `Capabilities`, which `Capabilities` has a constant for
each of. Decide what to do on capability names, never on the version. A service too old to
have `GetServerInfo` answers it `Unimplemented`; `ServerInfo.Answered` is then false and no
capability is claimed. `ConnectionOptions.Version` requires an exact version, failing with
`StaleServiceException`, and `RequireCapabilities` requires capabilities, failing with
`CapabilityException`; both are checked after that one call, and a failed open closes what
it opened.

`ConnectionOptions` also holds the request timeout (60 seconds by default), headers sent with
every call, the largest request and response messages (512 MiB each by default), a logger
(`NullLogger` by default) and an HTTP handler to borrow. Every option is checked when the
connection opens, which refuses an invalid one with `InvalidRequestException` before it sends
anything. Headers carry authorization and custom metadata only: a header the protocol
controls (content type and encoding, timeouts, host, any `grpc-` or `connect-` header) is
refused, and header values are never logged. A handler given as `HttpMessageHandler` is
borrowed, never disposed, so it can serve several connections; otherwise the connection
makes its own and disposes it.

A service serving an address other machines reach requires a bearer token on every call (see
[Service transports](../../docs/reference/service-transports.md#authentication)).
`ConnectionOptions.Token` gives it, and every call then carries `authorization: Bearer <token>`
over each protocol; when it is null, the `OPENSYSML_GRPC_TOKEN` environment variable
(`ConnectionOptions.TokenVariable`) names it, trimmed, a blank value being none. A token is
what the service accepts, RFC 6750's `b64token`: letters, digits and `-._~+/` with trailing `=`
only; anything else, from the option or the variable, is refused with `InvalidRequestException`
before anything is sent. An `authorization` entry in `Headers` takes the place of the variable,
and is refused beside `Token`, since a call carries one set of credentials. A token is never
logged nor written into a message. Credentials sent over cleartext `http` to a host other than
this machine cross the network unencrypted, which the open logs as a warning; it does not
refuse, since the network between may itself be protected.

```csharp
await using var connection = await Connection.OpenAsync(new ConnectionOptions
{
    Address = "https://sysml.example.org:50051",
    Token = Environment.GetEnvironmentVariable("SYSML_SERVICE_TOKEN"),
});
```

The client never retries a call, hedges it or falls back to another protocol: a call that
failed in transport may still have run.

### Private services

```csharp
await using var connection = await Connection.OpenAsync();   // no service named: start one
```

With no service named and `ConnectionOptions.AutoStart` true, the default, `OpenAsync` starts
`sysml-grpc` as a child of this process: `sysml-grpc -port 0 -health-port 0 -report-address
-exit-with-parent`. The kernel chooses its port, which the child reports as the first line of
its output; the connection dials that loopback address, which no other process is told of. The
child exits at end of file on its input, and only this process holds the other end of that
pipe, so it exits with this process however this process ends, a kill included; no exit
handler is involved. Its output is read for as long as it runs, so a chatty child never
blocks, and its last 20 lines, each cut at 1,024 characters, are kept for error messages. With
`AutoStart` false, a connection that names no service fails with `ServiceStartException`.

The child is started without `OPENSYSML_GRPC_TOKEN` in its environment (on Windows, under any
case of the name), so a token set for a remote service is never one the child demands. It
binds loopback and requires no token; one the connection carries anyway, from `Token` or the
variable, is ignored, as the other clients' private children ignore theirs.

The binary is, in order, the one `ConnectionOptions.BinaryPath` names, the one the
`OPENSYSML_GRPC_BINARY` environment variable names, the release asked for in the shared cache
`~/.opensysml/bin/sysml-grpc` (`sysml-grpc.exe` on Windows, under `%USERPROFILE%`), which is
downloaded and verified there when the cache does not hold it (see
[Downloading releases](#downloading-releases)), and, only when the release this client was
built with cannot be had, the first `sysml-grpc` on `PATH`. A binary named by the option or
the variable that is not an executable file fails the open with `BinaryNotFoundException`
rather than another being used, and a named binary is never downloaded; finding none lists
every place looked at in `SearchedLocations`, and why no release could be had. The binary is
hashed before it is
started, with symbolic links followed before any subsequent `..` component, including in
link targets; a missing directory or regular file cannot be traversed by `..`. A binary found
in the shared cache is not started at that path but from its snapshot under its own digest,
taken with the cache locked (see [The shared cache](#the-shared-cache)); a binary named
explicitly or found on `PATH` is started as it is.
`ConnectionOptions.ExpectedBinarySha256` requires a digest, and a binary without it
fails with `ChecksumMismatchException` and is never executed. A platform that cannot start a
process, such as a browser, fails with `UnsupportedPlatformException`.

The connections of one copy of the client share a private service while they agree on what
it is: the same binary, by its path with every link followed and by the digest of its
contents, and the same `StartupTimeout`, `ShutdownTimeout` and `Version`. Protocol, encoding,
headers, logger, request timeout and HTTP handler belong to each connection and never split
a service. Concurrent opens of one service wait for one start; one that is cancelled gives up
only its own wait, and the start is cancelled when none waits for it any more. A start that
fails stops its child and gives every waiting open the same failure. A connection holds the
service from the moment it starts opening; an open that fails, its handshake included, gives
its hold up, and the last connection to close stops the service: its input is closed, it has
`ShutdownTimeout` (5 seconds by default) to exit, and is then killed with every process it
started. `IsolatedService` gives a connection a service of its own, stopped when it closes.
A copy of the client is its assembly load context: plugins sharing one loaded copy share its
services, and a copy loaded into another context starts its own. Unloading a collectible
context closes the input of every service its copy started, a start still in progress
included, and each exits; the copy registers no process-exit handler, starts no foreground
thread and leaves nothing in the default context referring to it, so once its services are
gone it can be collected. That cleanup is best effort; that a child ends when its input does
is the guarantee.

A private service that exits on its own is not restarted: the connections to it fail from
then on, calls in flight included, with `TransportException` and `Unavailable`, saying how it
ended, and a connection opened afterwards starts another. `Connection.StopSharedServicesAsync`
stops every shared private service of this copy of the client, as a host unloading it does;
their connections fail the same way and are never moved to another service. It tries every
service, including stops already underway, before reporting the ones it could not stop in a
`ServiceStopException` with a read-only `Failures` snapshot, and leaves isolated services to
their connections. Unloading also stops a child that finishes starting after its startup was
cancelled. A shared service logs its life (start, unexpected
exit, a kill after the grace period, a failure to stop) to the logger of the connection that
started it; each line of its output goes to that logger at `Trace`.

`StartupTimeout` (30 seconds by default) bounds the wait for the address. A child that
reports something other than a loopback `host:port`, exits or ends its output first, or
reports nothing in time is stopped, and the open fails with `ServiceStartException` quoting
how it ended and what it last wrote. A binary the operating system cannot execute fails the
open the same way, with its reason, except one refusal: Linux will not execute a file any
process holds open for writing (`ETXTBSY`), and a child another thread of this process is
forking holds a copy of every open descriptor until it executes its own program, so a binary
this process has just written, as the shared cache does, can be busy for a moment. That start
alone is retried, five times over about one and a half seconds, logged at `Debug`, before it
fails. `BinaryPath`, `ExpectedBinarySha256`, `IsolatedService`,
`DownloadVersion` and `GithubRepo` only describe a private service, so they are refused
alongside `Address` or with `AutoStart` false; when `OPENSYSML_SERVICE` names a service instead,
they are unused and a warning says so. `DownloadVersion` and `GithubRepo` describe a release to
download, so they are refused alongside `BinaryPath` too, and when `OPENSYSML_GRPC_BINARY`
names the binary instead, they are unused and a warning says so.

These guarantees hold where the tests that prove them run: Linux, macOS and Windows, the
platforms CI runs the whole suite on (see [Building and testing](#building-and-testing)). On
any other platform the client starts a private service the same way, but nothing proves that
it ends with the process or that an unloaded copy is collected.

### The shared cache

`~/.opensysml/bin` is one cache every OpenSysML client shares, under the names they all use:
the executable `sysml-grpc` (`sysml-grpc.exe` on Windows), its record `sysml-grpc.json` of
the release it is (`{"version": …, "sha256": …, "repo": …}`, written as Python's `json.dump`
writes it), the lock `sysml-grpc.lock` every client holds while it changes them, and
snapshots `sysml-grpc-<first 16 hex digits of the digest>`, the executable linked under its
own digest. This client reads the cache, decides whether to use it, and installs a release
into it only with its lock held.

The lock is three locks, taken in this order and released in reverse:

1. a semaphore of this copy of the client, so its threads wait without polling;
2. `sysml-grpc.lock.dotnet`, a guard file of this client's own beside the lock, held with
   `flock` on POSIX and an unshared open on Windows, which belong to the open file rather than
   the process. It shuts out another copy of the client loaded into another load context of the
   same process before that copy opens the lock file: the lock below belongs to the process, so
   that copy would otherwise take it too, and closing its descriptor would drop it;
3. `sysml-grpc.lock` itself, held as the Python, Java and Rust clients hold it: `lockf` over
   the whole file on a descriptor libc's `open(2)` opened, never a `FileStream`'s, whose
   implicit `flock` conflicts with the other clients' locks on macOS; `LockFileEx` over the
   whole file on Windows.

Descriptors are opened close-on-exec, so no process started while the lock is held, a private
service included, inherits any of it; the guard is never removed. Each lock is attempted
without blocking and retried after pauses doubling from 10 ms to 250 ms, so a wait is
cancelled by the caller's token and sees a release within a quarter second. After two seconds
a warning names the lock and who holds it; after two minutes, as long as the Java client
waits, the open fails with `ServiceStartException`. A holder that ends, however it ends,
releases everything with it.

With the lock held, the executable is hashed and linked under its digest, and that snapshot is
what starts, so a release another client installs over the cache later never changes a
running or starting service. A snapshot already there is used only when it has its digest
and is not a symbolic link; otherwise it is replaced with a warning. A hand-installed
executable that is a symbolic link is followed before its file is linked under its digest.
Where the file system cannot hard link, the
executable is copied there instead, with a warning; where it can be neither linked nor copied,
or the cache cannot be locked because this user may not write there, the executable is started
where it is, with a warning that a release installed over it may be started instead; nothing
can be installed in a cache that cannot be locked, so a release asked for then fails. Old
snapshots are not removed.

A release is installed through the same transaction the cache is read through: the release is
streamed into a file staged in the cache directory (mode 0700 on POSIX), verified against the
digest required of it, and published by an atomic rename, executable first and its record
second. Cancellation is checked immediately before the executable is published,
leaving the previous executable and record unchanged when cancelled; once published, its
record is completed even if the caller cancels.
Bytes without the digest required are never published. A record vouches for the executable
only while the digest it records is the executable's, so an interruption between the two
leaves an executable no record vouches for rather than a torn pair taken for a release; a
record another client was still writing in place, or one with invalid Unicode in a name or
release field, is unusable metadata with a diagnostic reason. What an interrupted .NET
transaction staged, named `<file>.dotnet-<16 hex digits>.tmp`, is removed by the next, which
holds the lock every transaction that stages files holds; other clients' staged files are
left alone.

### Downloading releases

When no binary is named, the private service is started from a release of `sysml-grpc` kept in
the shared cache:

| What | Taken from, in order |
| --- | --- |
| The release | `ConnectionOptions.DownloadVersion`, the `OPENSYSML_GRPC_VERSION` variable, the release this client was built with (`v` and the `Version` in `Directory.Build.props`) |
| The repository | `ConnectionOptions.GithubRepo`, the `OPENSYSML_GITHUB_REPO` variable, `Open-MBEE/OpenSysML` |
| The asset | `sysml-grpc-<goos>-<goarch>` for this machine: `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64` or `windows-amd64` (with `.exe`); any other platform fails with `UnsupportedPlatformException` naming it |

A release is a tag, as `v0.10.1`, or `latest`, which the GitHub releases API resolves once for
each open. A tag or repository that is not one URL path segment (or `owner/repo` of two) is
refused, from an option or a variable, with `InvalidRequestException` before anything is
fetched. The whole decision is made with the cache's lock held:

| The cache holds | What happens |
| --- | --- |
| The release asked for, of the repository asked for, by a record that vouches for it | It is started from its snapshot after checking any committed pin and `ExpectedBinarySha256`; nothing is fetched |
| The binary `ExpectedBinarySha256` requires, and no release was explicitly asked for | It is started after checking any committed pin for its recorded release; nothing is fetched |
| An executable with no record at all, and no release was asked for | It is taken for a build installed by hand, and started as it is |
| Another release, a release of another repository, an executable its record does not vouch for, or nothing | The release asked for is downloaded, with a warning when it replaces something; it replaces the cache only once it has the digest required of it |

A release asked for, by the option or the variable, is that release or a failure. The release
this client was built with gives way: when it cannot be had (the network is down, it is not
published, as for an unreleased checkout, or it is not pinned), a working cache is kept with a
warning, and with nothing cached the first `sysml-grpc` on `PATH` is started, with a warning.
When `latest` cannot be resolved, a release of the repository asked for that the cache holds is
kept, with a warning; anything else fails. An explicit release and `ExpectedBinarySha256`
must both hold: a matching cached digest never overrides a tag or suppresses the lookup of
`latest`. An API response with invalid Unicode is an unavailable release, reported as
`ServiceStartException`; the same cache fallback applies.

**Trust.** A release is downloaded only when this checkout pins the SHA-256 digest of its asset
(`src/OpenMBEE.OpenSysML/release-digests.json`, embedded in the assembly; see
[Version and pinned release digests](#version-and-pinned-release-digests)). The `.sha256` the
release serves beside the binary must agree with the pin, and the bytes downloaded must have it;
`ConnectionOptions.ExpectedBinarySha256` must agree with it too, and is checked before the
binary is fetched. Any disagreement fails with `ChecksumMismatchException`, never answered from
the cache or `PATH`, and leaves the cache as it was. A cached release whose record agrees with
its executable is also checked against any committed pin for that repository, tag and platform,
including on offline fallback and when the cache must be read unlocked. A matching record
or caller-supplied digest never excuses a contradictory pin. A hand-installed executable with
no usable release record cannot be identified as a pinned release.
This client does not verify the signature
on a release's `SHA256SUMS.txt`, as the Python, Java and Node clients do: the .NET Sigstore
library evaluated for it trusts a timestamp without checking its signature and a bundle without
proof of its transparency-log entry, which the other clients' libraries refuse. So a release it pins no digest for fails with `UnpinnedReleaseException`
before anything is fetched, signed or not. The only way past that is same-origin trust:
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=1` (any repository) or `=owner/repo,…` (those exactly), as
every client reads it, which takes the digest the release serves beside the binary, with a
warning. That detects corruption but not a compromised release, and it never excuses a digest
that contradicts a pin or `ExpectedBinarySha256`.

**Transfers.** Releases are fetched from `https://github.com` and the API at
`https://api.github.com`, over HTTPS alone: every redirect is checked, and one to plain HTTP is
not followed. A response that sends nothing for 15 seconds, its headers included, is abandoned;
each transport read has its own idle deadline, so partial headers and compressed-body progress
reset the wait, and local file writes do not consume it. A slow download that keeps sending
is not abandoned. A binary may be 512 MiB and a checksum or API answer
8 MiB, counted after any decompression; a larger declared length is refused before the body is
read. A download is streamed into its staged file and nothing it staged is left behind,
whatever ends it. Cancelling the open cancels it with the caller's token. A download is never
retried.

### Protocols

`ConnectionOptions.Protocol` and `ConnectionOptions.Encoding` choose how a connection speaks to
the service, and `Connection.Protocol` and `Connection.Encoding` report it:

| `Protocol` | `Encoding` | On the wire |
| --- | --- | --- |
| `Grpc` (default) | `Protobuf` (default) | gRPC over HTTP/2 (cleartext h2c for `http`) |
| `Connect` | `Protobuf` | One HTTP/1.1 `POST /sysml.SysMLService/<Method>` per call, `application/proto` |
| `Connect` | `Json` | The same, `application/json`, in the protobuf JSON mapping |

`Grpc` with `Json` is refused when the connection opens, before anything is sent: gRPC carries
protobuf alone. A connection speaks the one protocol it was given; if the service does not
answer it, the call fails rather than trying another. The service's default `connect`
transport answers all three; its `grpc` transport speaks gRPC alone and resets a Connect
request's connection, which the client reports as a `TransportException` with `Unavailable`.
Connect suits a proxy that does not carry HTTP/2, and JSON one that must read the traffic;
protobuf is smaller than JSON and faster to decode.

The gRPC channel advertises only the identity encoding. A message size limit in the
underlying library bounds a compressed frame, not what it inflates to, so accepting
compressed responses would let one exceed `MaxResponseSize`.

Over Connect, the client asks for gzip and reads it itself, so that `MaxResponseSize` bounds
the body after inflation, and the compressed body by the most a deflate encoder can make of
that many bytes; no buffer of the limit's size is made up front. A gzip body must be one gzip
member, as HTTP servers write one, and its CRC-32 and length are checked: the .NET inflater
alone returns a body cut short as if it were whole. A Connect call sends its deadline as
`Connect-Timeout-Ms` and also enforces it itself, because the service does not cut a call
that is already running short. The connection's own HTTP handler follows no redirect and keeps
no cookies; a borrowed `HttpMessageHandler` is used as it is configured.

With `Json`, the size limits measure the JSON text. A response field this client does not
know is ignored, so that a newer service can still be read, and so is an enum value given by
a name the client has no number for: it reads as the enum's zero value. Protobuf keeps such a
value's number, and JSON keeps one the service sends as a number; the service sends names.

### Raw calls

`Connection.CallAsync<TRequest, TResponse>(method, request)` calls any method of the service
with the generated `OpenMBEE.OpenSysML.Wire` messages, under the connection's headers, timeout,
size limits, error mapping and disposal checks. The method is named bare (`GetSymbol`) or
qualified (`sysml.SysMLService/GetSymbol`); a name the schema does not have, or messages it
does not pair with the method, fails with `InvalidRequestException` and sends nothing. It is
the escape hatch below the typed API: it checks no capability and interprets no result, so
a failure the service reports inside an answer comes back as an ordinary response.

### Failures

Everything the client reports derives from `OpenSysMLException`, except cancellation and
use after disposal, which are the standard exceptions:

| What happened | Exception |
| --- | --- |
| An option, address, header or raw method/message pairing is invalid, or `OPENSYSML_GRPC_VERSION`, `OPENSYSML_GITHUB_REPO` or `OPENSYSML_GRPC_TOKEN` names no release, repository or bearer token | `InvalidRequestException`, before anything is sent |
| No service is named and `AutoStart` is false, or the private service did not start | `ServiceStartException` |
| The shared cache stayed locked by another client or process for two minutes | `ServiceStartException` naming the lock |
| A release asked for could not be downloaded or installed, or `latest` could not be resolved | `ServiceStartException`, with the cause as `InnerException` |
| No service is named, no binary was found to start, and the release this client was built with could not be had | `BinaryNotFoundException`, a `ServiceStartException` with `SearchedLocations` and why no release could be had as `InnerException` |
| The binary does not have `ExpectedBinarySha256`; or a release's served checksum, its bytes or `ExpectedBinarySha256` contradict its pin, or the checksum it serves under same-origin trust | `ChecksumMismatchException`, a `ServiceStartException` with `BinaryPath` (the release's URL for a release not downloaded), `ExpectedSha256` and `ActualSha256` |
| A release asked for is one this checkout pins no digest for, and same-origin trust was not granted | `UnpinnedReleaseException`, a `ServiceStartException` with `Repository`, `Version` and `Asset` |
| This platform cannot start a process, or no release is published for it and one was asked for | `UnsupportedPlatformException`, a `ServiceStartException` with `Platform` |
| The private service exited on its own, or was stopped by `StopSharedServicesAsync` | `TransportException` with `Unavailable`, saying which |
| `StopSharedServicesAsync` could not stop some services cleanly | `ServiceStopException`, with one of `Failures` for each |
| The service is not the `Version` asked for | `StaleServiceException` |
| The service lacks a required capability | `CapabilityException`, with `Capability` and `Remedy` |
| The service answered with a status other than OK, `Unavailable` included | `ServiceException`, with `StatusCode`, `ServiceMessage` and `Method` |
| The service wants a bearer token the call did not carry, or refused the one it carried | `AuthenticationException`, a `ServiceException` with `Unauthenticated`, `CredentialsSent` and the `Remedy` its message ends with |
| The service no longer holds the model a call named | `ModelNotFoundException`, a `ServiceException` with `NotFound` |
| The service could not read a file a parse named | `ModelFileNotFoundException`, a `ServiceException` with `NotFound` |
| The service answered, reporting a failure about the model: an expression that did not evaluate, a symbol that is not there, an object that could not be built | `ModelException`, with `Method` and the answer's `Diagnostics`; `SymbolNotFoundException`, with `Name` and `Suggestions`, for a symbol; `ModelErrorsException` from `RequireNoErrors` |
| The service answered that a run failed: no initial node, no such behavior, a performer that could not be made, a run still waiting when its advance ran out | `ExecutionException`, a `ModelException` with the run's `FinalTime` and, for a traced run, its `Trace` up to the failure and `TraceDropped` |
| The service answered a check it refused before any search: the behavior, a property or the performer does not resolve | `CheckException`, a `ModelException` with the `Partial` result the answer carried, including its verdict when supplied |
| The service answered that it could not verify, validate, calculate or sweep at all: an unknown symbol, a symbol of another kind, a subject or argument that does not read, a range no values follow from | `VerificationException`, a `ModelException` with the failure's `Reason` (`FailureReason.WrongKind` for a symbol of another kind) and the `Instances`, `Verifications` and engine `Standing` the answer carried |
| The service answered that an analysis case could not run to its end | `AnalysisException`, a `VerificationException` with the `Partial` run: the outputs computed, the verdicts and evaluations made and the objects named before the failure |
| The service refused a document query or document because the model declares it, or a query it runs, wrongly (`FailedPrecondition`) | `QueryException`, a `ModelException` with the `StatusCode` and `ServiceMessage` the service gave |
| The service answered that it refused a batch of edits: an unknown target, a value that does not parse, a name taken, references a delete would break | `EditException`, a `ModelException` with the `Failure` kind, the `Diagnostics` behind it, and the `Referrers` and `ReferringElements` of a refused rename, delete or move |
| No well-formed answer was read: the endpoint was unreachable or did not speak the protocol, a Connect refusal was not a Connect error, the answer was malformed or over a size limit, or the deadline passed | `TransportException`, with `StatusCode`, `Method` and the cause as `InnerException` |
| The service answered a value of a kind this client does not know | `UnsupportedValueException`, with `Method`, `Path` and the value's `ValueBytes` |
| The caller's token was cancelled | `OperationCanceledException` carrying that token |
| The connection was disposed before or during the call | `ObjectDisposedException` |

A status is the service's only when the service sent it. Over gRPC, that is when the response
carried it and the gRPC library raised it unchanged; a status the library made up for a broken
connection or a malformed answer is a `TransportException`. Over Connect, it is when the body
of a refusal is a Connect error, `{"code": …, "message": …}` with one of the protocol's codes,
which is JSON whatever the request's encoding; the code, not the HTTP status, is the status.
Any other refusal, such as a proxy's HTML page or the plain 404 an HTTP router answers for a
method it does not have, is a `TransportException` whose status is inferred from the HTTP
status as the Connect protocol specifies, quoting up to 200 characters of what was answered
with control characters blanked. So over Connect, a service too old to have `GetServerInfo`
fails to open rather than answering `Unimplemented`, and a proxy's 401 page is a
`TransportException` with `Unauthenticated`, never an `AuthenticationException`.

A deadline is always a `TransportException` with `DeadlineExceeded`, whether the client or the
service saw it pass. When both the caller's token and the deadline fired, the caller's
cancellation is reported, and an answer already received is returned whatever happens to the
token afterwards. An endpoint that speaks only HTTP/1.1 fails a gRPC connection with
`Internal`, and the message says so and names `Protocol.Connect`.
This diagnosis covers both HTTP/2 protocol errors and failed version negotiation,
which runtimes can report differently.

`StatusCode` numbers its values as gRPC does and the Connect protocol names them. Where the
gRPC library's status disagrees with gRPC's own definitions, the client reports gRPC's: an
answer that does not parse is `Internal`, and a connection that could not be established (name
lookup, refusal, TLS) is `Unavailable`. Connect failures are classified the same way: an
answer that does not parse, is cut short or is not HTTP is `Internal`, and a connection that
could not be made or was lost before an answer is `Unavailable`.

`DisposeAsync` refuses new calls, cancels the calls in flight, waits for them and closes the
channel, then gives up its hold on a private service, stopping it if it was the last. It is
idempotent, safe to call concurrently, and never throws: a failure to close or to stop is
logged as a warning.

## Models

```csharp
await using var connection = await Connection.OpenAsync(new ConnectionOptions { Address = "localhost:50051" });
var model = (await connection.LoadAsync("vehicle.sysml")).RequireNoErrors();
var vehicle = await model.LookupAsync("Vehicle");                       // Demo::Vehicle (partDef)
var mass = await model.EvalAsync("mass", new EvalOptions { ContextSymbolId = vehicle.Id });
var car = await model.InstantiateAsync("Demo::Vehicle");
var engine = car.Resolve((Value.InstanceRefValue)car.Root.FeatureValues["engine"].Value!);
```

`Connection.LoadAsync` parses a file the service reads, in its own file system and with the
path as given; `LoadContentAsync` parses notation given inline, in `ParseOptions.Language`
(SysML by default); `ParseSourcesAsync` parses several `SourceDocument`s, files or named
inline notation, as one model, so a name one declares resolves in another. Each returns a
`Model`, which the service caches under its `Hash`. `Connection.ModelByHash` names one parsed
earlier without sending anything. A model is immutable and safe to share between threads: it
keeps its `Roots`, one per document, the `ParseDiagnostics` and their `Errors`, and the
`Documents` they name, and nothing a later call changes. A source with errors still parses
into a model; `HasErrors` says so, and `RequireNoErrors` turns it into a
`ModelErrorsException`.

| Method | Answers |
| --- | --- |
| `SymbolAsync` / `FindSymbolAsync` | The `Symbol` of a qualified name as the service resolves it, through imports and aliases; not found raises `SymbolNotFoundException`, its message the service's own, or answers null |
| `FindAsync` / `LookupAsync` | The symbol of a short name or of its own id, the outermost first; not found answers null or raises `SymbolNotFoundException` naming at most three near names |
| `GetDiagnosticsAsync` | The diagnostics again, under a `DiagnosticsOptions` lint selection |
| `EvalAsync` / `EvaluateAsync` | The `Value` of an expression, in `EvalOptions.ContextSymbolId`'s scope or against an object of `SubjectSymbolId`; `EvaluateAsync` keeps the diagnostics with it |
| `InstantiateAsync` | An `Instantiation`: the object built, every object reachable from it, and `Resolve` for the references among their `FeatureValues` |

A `Symbol` carries everything the service reports of an element: its kind, metadata, child
ids, attributes with their values, `TypeFacts`, `Multiplicity` and every `Specialization`.
Each fact is read whatever the service advertises, and one the service did not report is
empty or null, never invented. `FindAsync` and `LookupAsync` ask the service which elements
carry a name when it advertises `query`, in one or two calls whatever the model's size, and
otherwise walk the model from its roots, one call per symbol; a model named by hash alone has
no roots to walk and needs `query`. Their near names are those Python's `difflib` would
suggest, each named once, comparing Unicode code points and applying its automatic
popular-character heuristic to names of at least 200 code points.

Options a service would silently ignore without its capability are refused first, with
`CapabilityException` and before anything is sent, in the order the service checks them:
`strict_conformance` for `StrictConformance`, `inline_language` for a language named for
inline notation, `parse_sources`, `parse_sources_affected` for `SourcesOptions.BaseModelHash`,
and `evaluate_subject` for a subject. A file's notation comes from its extension, so loading
one with `ParseOptions.Language` set is refused with `InvalidRequestException`.

A model loaded through a connection borrows it: disposing the model does nothing, and the
connection's owner closes it. `OpenSysMLClient.LoadAsync` and `LoadContentAsync` open a
connection of their own, which the model they return owns (`OwnsConnection`) and closes,
once, when it is disposed. Every concurrent disposal waits for that same close; a parse that
fails closes it before the failure is reported.

```csharp
await using var model = await OpenSysMLClient.LoadContentAsync(
    "package P { part def A; }", new ConnectionOptions { Address = "localhost:50051" });
```

## Running and checking behavior

```csharp
var model = (await connection.LoadAsync("behavior.sysml")).RequireNoErrors();
var run = await model.ExecuteActionAsync("Test::addFive", new Dictionary<string, Value> { ["result"] = new Value.IntegerValue(10) });
var declared = await model.ExecuteActionAsync("Test::race", options: new RunOptions { Schedule = "declared" });
var every = await model.ExploreActionAsync("Test::race");             // every outcome, and whether that is all of them
var traced = await model.ExecuteStateAsync("Test::Machine", options: new RunOptions { Trace = true });
var sent = await model.ExecuteActionAsync("Plant::Tank::fill", options: new RunOptions
{
    PerformerSymbolId = "Plant::tank", Events = ["Plant::Refill"], Trace = true,     // posted as %send posts them
});
var tank = (await connection.LoadAsync("tank.sysml")).RequireNoErrors();
var check = await tank.CheckActionAsync("Plant::Tank::fill", options: new CheckOptions
{
    PerformerSymbolId = "Plant::tank", Properties = ["Plant::Tank::low"], Diverge = ["this.level"],
});
```

| Method | Answers |
| --- | --- |
| `ExecuteActionAsync` | An `ActionRun`: the action's `Outputs`, the `PerformerAttributes` of the object it ran on (`this.level`), its `Diagnostics` (choice points, unevaluable guards), the `FinalTime` of its clock, the outputs each `OrderDependent` choice touched, and, when `RunOptions.Trace` asks for it, the run's `Trace` of `DocumentEvent`s (token steps, waits, accepts, sends, decisions, assignments and completion) and how many records the service dropped |
| `ExecuteStateAsync` | A `StateRun`: the `StatesVisited` in the order the run entered them, the `FinalContext`, and, when `RunOptions.Trace` asks for it, the run's `Trace` of `DocumentEvent`s and how many records the service dropped |
| `ExploreActionAsync` / `ExploreStateAsync` | An `Exploration`: each distinct `Outcome` with its linearizations, witness choices and model-draw probability, whether the exploration was `Complete`, and the budgets that cut it short |
| `CheckActionAsync` / `CheckStateAsync` | A `CheckResult`: the verdict (`holds`, `fails`, `undecided`), the report's lines, the search's statistics, each violation, divergence and final outcome with a `CheckWitness` that replays it, the symbolic engine's claim, and the `Standing` of the engine that answered |

`RunOptions` carries what the `sysml` command's run flags do: the `Schedule` (`declared`,
`reverse`, `seed:<n>`, or `explore[:runs=<n>,depth=<d>]` for an exploration), the
`PerformerSymbolId` of the object performing the behavior, the run's `Trace`, the `Events` an
action run posts to its performer in order, as the REPL's `%send` does, and the run's `Advance`,
`Seed`, `Draws` and `ClockStep`. A run and an exploration answer differently, so each call takes
only its own kind of schedule: an exploring schedule passed to `ExecuteActionAsync`, a single-run
schedule passed to an explore call, or a trace or events asked of an exploration is an
`InvalidRequestException` before anything is sent. So are `Events` with no `PerformerSymbolId`
to post them to, and `Events` given to a state run, which takes its events as an argument. An explore
call naming no schedule explores under the service's default budget. `CheckOptions` carries
the `-check-*` flags: the properties held at every stable state, the features allowed to
diverge, the symbolic engine's free inputs and assumptions, whether each witness carries its
file, the bounds, the engine and the performer. A failing property is a `CheckResult`, never
an exception. A value the service would not accept, a malformed schedule or budget, an unknown
draw policy or engine, and a negative bound are the service's to refuse, with
`InvalidArgument`.

Before anything is sent, a call is held to the capabilities its options need, in the order
the service checks them: `schedule` for a schedule and `schedule_explore` for an exploring
one, `action_trace` or `state_trace` for a trace, `performer` for a performer, and the value capabilities of
each input; a check needs `check`, then `performer`, `engines` for an engine and
`schedule_explore` for the `explore` engine. `final_time` and `order_dependence` only say
whether the service fills those fields: they are read whatever it advertises, as 0 or empty
from a service without them.

A trace record, and the row of a document query, is read into the `DocumentValue` family:
an element (with no `Id` when the service names an anonymous one), an object, a string, a
number kept as the `Value.NumberValue` it was sent as, a boolean, `*`, a quantity, or a
verdict, state or event row. An arm this client does not know is an
`UnsupportedValueException`, never an anonymous element, and an answer of the wrong shape (an
exploration answering one run, a run answering an exploration) is a malformed answer, a
`TransportException` with `Internal`.

A check answer containing neither a verdict nor an error is also malformed and raises
`TransportException` with `Internal`. An in-band error remains a `CheckException`, preserving
the partial result even when the service omitted its verdict.

## Verifying, calculating and analyzing

```csharp
var model = (await connection.LoadAsync("verification.sysml")).RequireNoErrors();
var light = await model.VerifyConstraintAsync("Demo::Vehicle::massLight");          // Verdict.Violated, with its Condition
var onSedan = await model.VerifyRequirementAsync("Demo::Vehicle::tiny", new VerifyOptions { SubjectSymbolId = "Demo::sedan" });
var proved = await model.VerifyConstraintAsync("Demo::sq", new VerifyOptions { Question = VerifyOptions.Holds });
var sedan = await model.ValidateInstanceAsync("Demo::sedan");                       // every assertion, and a Summary
var sum = await model.CalcAsync("Demo::add", [new Value.IntegerValue(2), new Value.IntegerValue(3)]);
var analysis = (await connection.LoadAsync("analysis.sysml")).RequireNoErrors();
var sweep = (await connection.LoadAsync("sweep.sysml")).RequireNoErrors();
var cost = await analysis.RunAnalysisAsync("An::CostAnalysis", new AnalysisOptions { SubjectSymbolId = "An::barge" });
var table = await sweep.RunSweepAsync("Sw::Ratio", [new SweepRange("b", new Value.IntegerValue(-1), new Value.IntegerValue(1))],
    new SweepOptions { Arguments = [new Value.RealValue(4)] });                       // the b = 0 row carries its error
var engines = await connection.ListEnginesAsync();
```

| Method | Answers |
| --- | --- |
| `VerifyConstraintAsync` / `VerifyRequirementAsync` | A `Verification`: the `Verdict`, the objects it is about with its `Subject`, and, for a requirement, the `Verifications` its verification cases answered |
| `VerifySatisfactionAsync` / `SatisfiedAsync` | A `Satisfaction`: one verdict per satisfaction assertion the model, or one element of it, states, each against an object of its own subject; `SatisfiedAsync` answers whether every one is decided and holds |
| `ValidateInstanceAsync` | A `Validation`: one verdict per assertion about an object of a part and the objects it holds, each with its `InstancePath`, the `Summary` about the object as a whole, and whether nesting was left `Bounded` |
| `CalcAsync` | A `Calculation`: the `Result` an invocation returned, or each output feature a calc usage named with no arguments computed, in declaration order |
| `RunAnalysisAsync` | An `Analysis`: the case's `Outputs` in declaration order, the verdicts of its objective and assertions, a verification case's `Verifications`, the `Evaluations` a trade study made with the `Selected` one, the objects they name, and with `AnalysisOptions.ToolDryRun` the `ToolDryRun` preview of the external tool the run reached |
| `ExploreAnalysisAsync` | An `Exploration` of the case's distinct outcomes, as for a behavior, with the `Standing` of the engine that answered |
| `RunSweepAsync` | A `Sweep`: one `SweepRow` per run, each with the inputs it bound, its outputs, verdicts, evaluations and wall time, and the error of a run that failed; the swept `Parameters`, and whether the rows were `Sampled` with the `Seed` |
| `Connection.ListEnginesAsync` | Each `EngineInfo`, in name order: what it answers, its authority, its bounds, whether it is ready and served, and where a manifest engine comes from; `ListEnginesOptions.Probe` starts each external engine once to check its handshake, which a listing alone never does |

A verdict of false is an answer, never an exception: `Verdict.Decided` is true and `Violated`
names it. A verdict that could not be evaluated, an unbound feature, a solver that decided
nothing, or a symbol of another kind, is answered too, with its `Error` and `FailureReason`,
and `Decided` false. Every verdict carries the `Standing` of the engine that answered, the
`Question` it answers (`evaluate`, `holds`, `satisfiable`) with its `Status`, the
`Witness` assignment that replays a violated or satisfiable answer, and a solver's `Notes`:
each `VerdictNote` an assumption or omission the answer rests on, as the REPL prints it, of
`Kind` `VerdictNote.AssumedOne` (a feature read as one value while only the default `0..*`
governs it) or `VerdictNote.Uncovered` (an object-holding feature whose own assertions the
question does not state), with the `Subject` feature and the sentence as `Text`. Only a request the
service could not answer at all raises `VerificationException`, whose `Reason` tells a wrong
request (`FailureReason.WrongKind`) from one that could not be evaluated. A sweep's failed run
is that row's `Error`, beside what it computed first, and never fails the table; an analysis
case that failed raises `AnalysisException` keeping its `Partial` run.

`VerifyOptions` carries the `Question`, the `SubjectSymbolId` the verdict is about, the
`Arguments` and `NamedArguments` bound to the element's `in` parameters, and the `Engine`; a
satisfaction is asked about an object of each assertion's own subject, so it takes neither a
subject nor arguments. `CalcAsync` binds its arguments positionally, which is all the service
reads. A result or `out` parameter that states no multiplicity is `[0..*]` (SysML §7.6.3), so
the service answers its one value as a one-element `Value.SequenceValue`; declare it `[1]`
(`return : Real[1]`) to read the value itself. `AnalysisOptions` carries the subject, arguments, schedule and engine, and
`SweepOptions` the subject, the arguments every row binds, `Samples` and `Seed`. A run and an
exploration of a case answer differently, so `RunAnalysisAsync` refuses an exploring schedule
and the `explore` engine, and `ExploreAnalysisAsync` a schedule that runs once and a tool dry
run, with `InvalidRequestException` before anything is sent; an exploration naming neither
explores under the service's default budget. A question, schedule or engine the service does
not know, and arguments with a solver question, are the service's to refuse, with
`InvalidArgument`; a negative sample count and a range no values follow from it refuses in its
answer.

Before anything is sent, a call is held to the capabilities its options need, in the order
the service checks them: `verification`, then `verification_questions` for `holds` or
`satisfiable`, `engines` for an engine and `schedule_explore` for the `explore` engine,
`verification_arguments` for any argument, and the value capabilities of each argument and
range endpoint; a case run checks `schedule` and `schedule_explore` for its schedule first.
`ListEnginesAsync` needs `engines`. `verification_verdicts`, `case_evaluations`, `verdict_notes` and the
`engines` response fields only say whether the service fills them: they are read whatever it
advertises, empty from a service without them. A failure reason this client does not name
is kept as its number.

## Running repeatedly, self-checking and recording

```csharp
var mc = (await connection.LoadAsync("monte_carlo.sysml")).RequireNoErrors();
var runs = await mc.RunActionRunsAsync("MC::Settle", 100, options: new ActionRunsOptions { Seed = 7, Observe = ["t"] });
var t = runs.Distribution("t");                                                     // Min, Max, Mean, Percentiles, Histogram
var sample = await mc.RunAnalysisRunsAsync("MC::Mc", 100, new AnalysisRunsOptions { SubjectSymbolId = "MC::probe", Seed = 7 });
var check = await mc.SelfCheckAsync();                                              // check.Holds, check.Findings
var recording = await mc.RecordRunAsync("MC::Cost", new RecordOptions { SubjectSymbolId = "MC::p" });
var recorded = recording.Model;                                                     // a new model holding Records::Cost_run1
```

| Method | Answers |
| --- | --- |
| `RunActionRunsAsync` | `Runs`: one `SweepRow` per run, in run order, its `Inputs` the run's number as `run` and its `Outputs` what the run observed, a failed run's `Error` beside what it observed first; the `Observables` tabled and each one's `RunDistribution` over the completed runs, numeric (`Min`, `Max`, `Mean`, `Deviation`, `Percentiles`, `Histogram`) or counted by distinct `Values`; the `Lines` `sysml -runs` prints, the `Seed` echoed, and the engine's `Standing` |
| `RunAnalysisRunsAsync` | The same for a `Simulation::MonteCarlo` case, whose runs report its `observed` value, with the case's conclusion over the sample in `Outputs` and `Verdicts`, which settle every row's, or why it has none in `Unconcluded` |
| `SelfCheckAsync` | A `SelfCheck`: the `Status` (`holds`, `fails`, `unresolved`) of the `SysMLValidation` constraints, and those of any rule packages named, applied to every element, every `SelfCheckFinding`, the counts, and the elements each constraint was applied to |
| `RecordRunAsync` | A `Recording`: the `Run`, `Sweep` or `Sample` made, the record `Package`, `Definition` and `Records`, the `Source` generated, the `Documents` they were merged into (with `Content` for a model of one document the records joined), the runs `Skipped` and why, and the recorded `Model` |

The same `Seed` makes the same runs on every platform; random draws without one, unless
`Draws` fixes them as `min`, `max` or `average`, are the service's to refuse with
`InvalidArgument`, as are a run count that is not positive, an unknown draw policy, a feature
observed twice, and, for a recording, both `Ranges` and `Runs`. An action runs with its inputs,
a performer and the features to observe, a Monte Carlo case with its subject and arguments, so
each has its own method and options. A run that fails is that row's error, never an exception;
runs the service could not table, an unknown action or case, a case that is not a Monte Carlo
one, or a feature no completed run produced, raise `RunsException`, a `VerificationException`
keeping the `Partial` table. A violated self-check constraint is a finding; a rule package the
model does not declare raises `ModelException`.

A recording is a new model the service holds, named by `Recording.Model`, which borrows the
connection; the model it was asked of keeps its hash and contents, and nothing is written to
disk. With neither `RecordOptions.Ranges` nor `Runs` the case runs once, with `Ranges` once per
row of the sweep, and with `Runs` and a `Seed` as a Monte Carlo sample; `RecordInto` names the
package the records go into. The request always accepts the documents of a model of several,
since every document the records went into is returned. A recording not made raises
`RecordException` with its `FailureReason`, the run, sweep or sample made, whether that run
itself reported the failure (`RunFailed`), and, when the records were generated but would not
analyze cleanly, their `GeneratedSource`. Before anything is sent the calls need `runs`,
`self_check` or `record_run`; then `performer` for a performer and the value capabilities of
each input, `engines` and `schedule_explore` for an engine, and the value capabilities of each
argument and range endpoint, in the order the service checks them.

## Converting, migrating and saving

```csharp
var model = (await connection.LoadAsync("vehicle.sysml")).RequireNoErrors();
var turtle = await model.ToTurtleAsync();                                 // turtle.Experimental, turtle.ExperimentalNotice
await model.SaveAsync("vehicle.json");                                    // the format the extension names: api-json
var back = await connection.ConvertAsync(ConvertSource.Inline(turtle.Content), ConversionFormat.SysML,
    new ConvertOptions { FromFormat = ConversionFormat.Turtle });
var migration = await connection.MigrateAsync(MigrateSource.FromFile(Path.GetFullPath("Model.mdzip")), ConversionFormat.SysML,
    new MigrateOptions { Report = true });
await migration.WriteAsync("Model.sysml");                                // and its images, beside it
```

| Method | Answers |
| --- | --- |
| `Connection.ConvertAsync` | A `Conversion` of a `ConvertSource` (a file the service reads, inline content, or a model by hash): the `Content` in the target format, the `FromFormat` and `ToFormat` the service used, canonically named, the `Diagnostics` of tolerated syntax errors and of the semantic check when asked for, whether the RDF mapping it went through is `Experimental`, with the service's `ExperimentalNotice`, and for `api-json` the `LibraryReferences` it refers to |
| `Model.ConvertAsync`, `ToNotationAsync`, `ToTurtleAsync`, `ToApiJsonAsync` | The same for the source the model was parsed from, which a file edited since does not change |
| `Model.SaveAsync` | The `Conversion` written to a file, in the format its extension names (`.sysml`, `.kerml`, `.ttl`, `.turtle`, `.json`) unless one is given |
| `Connection.MigrateAsync` | A `Migration` of a SysML v1 `MigrateSource` (an `.xmi`, `.uml` or `.mdzip` file, or its bytes): the `Content`, its `Report` (the `Summary` and the `Mapped`, `Approximated`, `Unmapped` and `Skipped` counts always, every `MigrationEntry` and the report `Text` with `MigrateOptions.Report`), the tool's stored `Results` with `MigrateOptions.Results`, and the `Files`, such as images, the model refers to |

Formats are named as `sysml -convert` names them; `ConversionFormat` holds the canonical
names, and the service also takes its aliases (`text`, `turtle`, `rdf`, `json`). Which
directions and options apply is the service's to judge, as is a SysML v1 source given to
`ConvertAsync` or a v2 one to `MigrateAsync`: each is refused with `InvalidArgument`, naming
the other call. `ConvertOptions` carries the source's `FromFormat` (inline content must name
it), `TolerateSyntaxErrors`, the `IdForm` of derived ids in a graph, the `Documents` of a
multi-document model to write, the `Compact` API JSON form with `OmitDerived` and
`KeepDerived`, `IncludeCheckDiagnostics`, and `IncludeLibraryStubs`. An `api-json` conversion
lists the standard-library elements it refers to as `LibraryReferences` (each a
`LibraryReference` with its `Id`, `QualifiedName` and metaclass `Type`, and, for an inherited
unnamed member, which has no qualified name, its positional `LibraryPath` `owner/Metaclass[i]`) rather than writing them
as roots of the element array; `IncludeLibraryStubs` writes them as roots too, as conversions
before 0.11.0 did, and the service refuses it for any other format. A service without
`convert_library_references` leaves the list empty. `MigrateOptions` carries the v1 `FromFormat`,
`Report`, `Results`, an MTIP layout as `LayoutPath` or `LayoutContent`, an `ImageBaseUrl` and
`Strict`. A conversion the service answered but could not make, a source that would not parse
or a mapping refusal, raises `ConversionException` with its diagnostics and formats; a v1
model that could not be read raises `MigrationException`. An element with no v2 form is a
report entry, never an exception. A file the service cannot read is a
`ModelFileNotFoundException`, a model it no longer holds a `ModelNotFoundException`. Before
anything is sent the calls need `convert`, then `convert_documents`, `convert_library_stubs`,
`convert_compact` and `convert_check_diagnostics` for the options that use them, in that order,
or `migrate`.

`MigrateSource.FromFile` requires a fully-qualified path in the client's path syntax, rejecting
relative, invalid, and (on Windows) drive-relative or root-relative paths with `ArgumentException`.
It normalizes the path once, then sends that exact path to the service and keeps it as
`Migration.SourcePath` for source protection. The service's working directory is never inferred
from the client's. Use `Path.GetFullPath` when deliberately naming a local file; for a service on
another host, use an absolute path shared with the client or carry the document inline.
`ConvertSource.FromFile` and migration layout paths still resolve relative to the service.

Nothing is written to disk until `Conversion.SaveAsync`, `Model.SaveAsync` or
`Migration.WriteAsync` is called, and those never replace an existing file unless
`SaveOptions.Overwrite` asks. Each file is written whole to a temporary file beside it,
flushed, and moved into place, so a failure or cancellation leaves no partial or temporary
file and an existing destination as it was. A migration's `Files` are written below the
model's directory, creating their directories, and the model last. Every destination is
judged first, and `UnwritableException` refuses, with nothing written: an existing file
without an overwrite, a directory, a symbolic link, a missing directory, the v1 model the
migration came from (even with an overwrite), and a migration file whose path is absolute,
names a drive or a share, holds `.`, `..` or a backslash, is not a portable file name (a
control character, a Windows-invalid character such as `?`, `*`, `|`, `"`, `<` or `>`, a trailing
dot or space, a Windows device name), lands on another file (compared without case on Windows
and macOS, with Unicode normalization equivalence on macOS), or passes through a symbolic link
or junction. The same comparisons protect the v1 source and detect file/directory collisions.
A failed write removes the files and directories it created; several files are not published
atomically, so a file an overwrite already replaced stays replaced, and a directory replaced
by a link while the write is under way is not guarded against.

## Querying, documents and views

```csharp
var model = (await connection.LoadAsync("query.sysml")).RequireNoErrors();
var parts = await model.QueryAsync(new Query
{
    Scope = ["Demo::vehicle"],
    Select = ["name", "owner"],
    Where = QueryConstraint.And(QueryConstraint.Equal("@type", "PartUsage"), QueryConstraint.Less("multiplicityUpper", "5")),
});                                                                       // Demo::vehicle::wheels (PartUsage)
var byName = await model.QueryAsync(new Query { Where = QueryConstraint.InstanceOf("Usage"), OrderBy = ["name"] });
var same = await model.QueryOslcAsync("oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name");

var observatory = (await connection.LoadAsync("document.sysml")).RequireNoErrors();
var table = await observatory.RunDocumentQueryAsync("Observatory::SubsystemTable",
    [new DocumentBinding("root", DocumentValue.ElementRef("Observatory::telescope"))]);
var report = await observatory.RenderDocumentAsync("Observatory::MassReport", DocumentForm.Html);
var asciidoc = await observatory.RenderDocumentAsync("Observatory::MassReport", DocumentForm.AsciiDoc);

var plant = await connection.LoadAsync("layout.sysml");
var view = await plant.RenderViewAsync("PlantViews::placedView",
    new RenderViewOptions { Ports = RenderViewPorts.Full, Show = [ViewNotation.EdgeLabels], Depth = 2 });
var tree = await plant.RenderViewAsync("#tree:Plant::Loop", new RenderViewOptions { Compartments = RenderViewCompartments.Most });
```

| Method | Answers |
| --- | --- |
| `QueryAsync` | A `QueryResult` of a SysML v2 API & Services `Query`: the `Elements` in its `Scope` (the whole model when empty, nested elements included) that its `Where` constraint matches, in the order its `OrderBy` sorts them or else in declaration order, each a `QueryElement` with its `Id`, its metamodel `Type` and the `Properties` its `Select` asked for, a property the element does not have absent |
| `QueryOslcAsync` | The same for OSLC Query 3.0 parameter text |
| `RunDocumentQueryAsync` | A `DocumentQueryResult` of a document query the model declares, with its entry parameters bound: the projected `Columns` and the `Rows`, each a `DocumentRow` with the `Element` it selects and a cell of `DocumentValue`s per column |
| `RenderDocumentAsync` | A `RenderedDocument` the model declares, byte-for-byte what `sysml -render-document` writes, in Markdown, `DocumentForm.Html` or `DocumentForm.AsciiDoc`, with the `FileName` a rendered set writes it to, null from a service that names none |
| `RenderViewAsync` | A `RenderedView` of a declared view or a `#<kind>:<qualified name>` pseudo-view: every field the view renderer produced |

A `Query` is an immutable record; `Query.All` considers every element, and a `with`
expression derives another. Its `OrderBy` names the properties the matches are sorted by, in
order of precedence, each ascending, as the standard's `orderBy` does: an ordered property
such as `multiplicityLower` compares as a number, any other byte-wise as text, an element
without the property first, and elements equal under every one keep declaration order. A
`QueryConstraint` is a `Comparison` of one property (`Equal` to or `In` any of its values,
`Greater`, `GreaterOrEqual`, `Less` or `LessOrEqual` than exactly one, or `InstanceOf` one of
the metaclasses named, which compares `@type` so that a `PartUsage` is an instance of `Usage`;
each optionally `Inverted`) or a `Composite` combining at least two constraints by `And` or
`Or`, as the standard combines them. A constraint is complete by construction: one without an
operator or an operand, a composite of fewer than two constraints (write a single constraint on
its own), and `InstanceOf` on another property or with an empty name cannot be built, each an
`ArgumentException`, and a structured query and OSLC text are two calls, never one request.
What the service still judges, a property it does not know, a scope element the model does not
have, an order on a property that has none, a metaclass the library does not declare, it
refuses with `InvalidArgument` rather than answering empty.

A `DocumentBinding` binds one entry parameter to its values, in order; a parameter several
bindings name takes all their values. A value is a `DocumentValue`: an element by qualified
name (`DocumentValue.ElementRef`), an object the service holds by path or id
(`DocumentValue.ObjectRef`), a string, a number, a boolean or a quantity. An element or object
a query answered can be bound again as it was answered. Infinity and the rows of `Verdicts`,
`States` and `Events` are only ever answered, so a binding refuses them with
`ArgumentException`, as it does an element with no name and an object with neither a path nor
an id. The objects are those the service holds for the model, which `InstantiateAsync`
creates. A row's element and cells read every arm, the verdict, state and event rows included.

A `RenderedView` keeps its `Nodes` flattened, parents before children, each naming its
`Parent`; `Roots`, `ChildrenOf` and `Node` walk them. A node has its `Ports`, an edge its end
ports, `Route` and `Label`, a tabular rendering its `Columns` and `Rows`, and the view its
`Notes`, its `Notices`, the optional notation it has `Shown`, and, round a graph-shaped
rendering, its `Frame`: the view's name and view definition and the lines of its exposes,
filters and annotations compartments (null for a frameless rendering). An item box drawn on a
connection line names the end the item goes toward as its `Arrow` (`right` or `left`), and a
state whose substates run in parallel is `Parallel`. What a view leaves unstated
stays absent: a node no layout places has a null `Geometry`, one no style colours a null
`Style`, a view stating no canvas a null `Canvas`, and an element the service located nowhere
a null `Origin`, never zeros. A layout that only folds or hides a node is `Unplaced`, and one
that states no size says so with `HasSize`. `RenderViewOptions` chooses the `Ports`, the
optional notation to `Show` as `ViewNotation` names it, a positive `Depth` overriding the
view's own, and how a tree folds owned members into `Compartments` of their owner's box:
`Positioned` (the default) folds only what a Layout leaves unpositioned, `Most` every owned
usage no Layout, nested definition or package keeps as a node, `Ports` the port usages. A
folded member is a node whose `Compartment` names the compartment (`attributes`, `parts`,
`ports`) of its `Parent`'s box, and `ChildrenOf` lists it among the boxes nested there. A node or
port the owner has through typing, subclassification or subsetting rather than declares is
`Inherited`, which the notation marks with `^`; its `Name` stays bare. A service older than
`Compartments` draws every member as a node whatever it asks, since there is no capability to
tell it by.

A model the service no longer holds is a `ModelNotFoundException`; a query, document or view
the model does not declare, or an object it does not hold, is a `ServiceException` with
`NotFound`, and a name declaring something else one with `InvalidArgument`. A document query
or document the model declares wrongly, an operation the engine does not support or a
property no element has, is a `QueryException`. A malformed answer, an element with no id or
type, a row selecting no element, a node whose parent is not drawn before it, a node repeating
another's id, a compartment row with no box to sit in, is a `TransportException`. Before
anything is sent the calls need `query`, then `query_order_by` for a non-empty `OrderBy`
(a service without it would ignore the ordering), and `oslc_query` for OSLC text; `document_query`, then `big_int_values` and `rational_values` for a bound Integer
beyond `long` or exact Rational (one a `double` holds exactly is sent as that `double` to a
service without `rational_values`); `render_document`, then `render_document_html` for HTML or `render_document_asciidoc` for AsciiDoc;
or `render_view`.

## Editing

```csharp
var model = (await connection.LoadAsync("editable.sysml")).RequireNoErrors();
var result = await model.Edit()
    .SetValue("Demo::SC::unitMass", "1050.0[SI::kg]")
    .AddMember("Demo", "part def", "Engine", m => m with { IsAbstract = true, Doc = "Turns fuel into torque." })
    .AddIf("Demo::Run", "x == 0", new Editor.Body().AddAssign("x", "1"), new Editor.Body().AddAssign("x", "2"))
    .ApplyAsync();                                                       // result.Content, result.Applied
await result.SaveAsync("editable.sysml", new SaveOptions { Overwrite = true });

var engines = await connection.ParseSourcesAsync([SourceDocument.FromFile("library.sysml"), SourceDocument.FromFile("user.sysml")]);
var renamed = await engines.ApplyEditsAsync([new Edit.Rename("EngineLibrary::Engine", "Motor")]);
foreach (var document in renamed.Documents) { /* each document the rename rewrote */ }
```

| Method | Answers |
| --- | --- |
| `Model.Edit` | An `Editor` collecting edits in the order they are added, with a method for each kind and the shorthands of action-body sequencing, applied as one batch by `ApplyAsync` |
| `Model.ApplyEditsAsync` | An `EditResult` of a batch of `Edit`s: the edited notation of a single-document model as `Content`, every document the edits rewrote as an `EditedDocument` in `Documents`, and what each edit changed as an `AppliedEdit` (its `OperationIndex`, the bytes' `Offset` and `Length`, `OldText`, `NewText` and `Document`) |

The edits apply in order to the notation the model was parsed from, all of them or none, and
the answer is byte-identical to that notation outside what they changed, comments and layout
kept. The model itself is unchanged: load the edited notation to read the edited model. An
`Edit` is an immutable record, one nested record per kind the service has: `SetValue`,
`Rename`, `AddMember`, `Delete`, `Move`, `AddConnection`, `AddSatisfy`,
`AddRequirementConstraint`, `AddTransition`, `AddImport`, `AddDocumentation`, `AddVerify`,
`AddMetadata`, `AddSequence`, `AddMetadataPrefix`, `AddComment` and `AddNote`. Each names its
target or owner as the model reports elements (`Demo::Vehicle`), an owner left empty naming
the document root, and writes notation (a type, a value, an expression) as text that the
service judges. Its optional parts are null when not written, and a `with` expression derives
another; an optional part set to empty text is refused before anything is sent, since the
service reads empty as absent. No type outside the library derives from `Edit`.

`AddMember` writes a declaration of any kind with its `Type`, `Multiplicity`, `Value` (or a
`BodyExpression` written in braces, a constraint's condition or a calculation's result),
`Specializes`, `Redefines`, `IsAbstract`, `IsDefault`, `Direction`, `MetadataPrefixes` and
`Doc`; an empty `Kind` writes a directed parameter, as `Editor.AddParameter` does.
`AddSequence` writes an action body's `first` and `then` members, guarded successions and
`else`, and the `accept`, `send`, `assign`, `if`, `while`, `loop`, `for` and `terminate`
statements, whose `Body` and `ElseBody` nest further items; its factories write each form, and
an `Editor.Body` collects a nested body's items, writing its first statement without `then`. A
body is read when it is passed, so changing it afterwards changes nothing built from it. The
service reads items nested at most `Edit.AddSequence.MaxDepth` (128) levels below their edit,
and one nested deeper cannot be built.

An editor sends the edits collected when `ApplyAsync` is called. Once it succeeds the editor
is spent, and an edit added while it is being applied or after is refused with
`InvalidOperationException` rather than lost; a failed apply leaves it to apply again. An
empty batch is sent too, and refused as `EditFailure.NoOperations`.

A model of several documents is edited as one: the edits target the declarations of
`EditOptions.InDocument` (the first document when null), and a rename or cascade delete
follows references into every other document, rewriting those too. Its edited notation is in
`Documents` alone and `Content` is null, so it is never mistaken for the whole model, and
`EditResult.SaveAsync` refuses it; write each document with `EditedDocument.SaveAsync`
instead. A model loaded from one file or inline notation has one document, one parsed from
several has as many as it was given; for a model named by hash alone, whose documents this
client does not know, an answer naming documents but no notation reads as one of several. A
service without `edit_documents` edits a model of one document alone, answers its `Content`
without `Documents`, and names no document an edit changed. `EditOptions.AcceptDocuments` is
true by default, since this client reads `Documents`; false asks the service to refuse a model
of several documents with `FailedPrecondition` rather than edit it.

A batch the service refused, nothing of it applied, raises `EditException` with its `Failure`
kind (`UnknownTarget`, `InvalidValue`, `MemberNameTaken`, `DeleteReferenced`, ...), the
`Diagnostics` behind it, and, for a rename, delete or move the model's references block, the
`Referrers` with the document declaring each and the service's `ReferringElements` spelling.
A kind this release does not name keeps its number over protobuf. A model the service no longer
holds is a `ModelNotFoundException`; a document the model does not have is a
`ServiceException` with `InvalidArgument`. A malformed answer, a refusal beside edited
notation, an applied edit of no edit in the batch, notation in `content` for a model of
several documents, or documents other than the one document of a single-document model, is a
`TransportException` with `Internal`. Before anything is sent the batch needs `apply_edits`,
then the capabilities its edits need, judged over the whole batch in the service's order:
`authoring` for any edit but a value or a rename, the dedicated capability of a connection,
satisfy, requirement constraint, transition, verification, metadata, prefix, sequence,
action-body statement or import, then `member_modifiers`, `implicit_parameters`,
`constraint_body_authoring` and `state_action_authoring` for the member forms that use them,
then `documentation_authoring` and `comment_authoring`; and last `edit_documents` to name a
document. `EditResult.SaveAsync` and `EditedDocument.SaveAsync` write as
[`Conversion.SaveAsync`](#converting-migrating-and-saving) does: whole, through a temporary
file, and never over an existing file unless `SaveOptions.Overwrite` asks.

## Exporting graphs, importing data and comparing results

```csharp
var model = (await connection.LoadAsync("behavior.sysml")).RequireNoErrors();
var graphs = await model.ExportGraphsAsync("Test::race");             // graphs.Content, the graphs:1 JSON

var fleet = (await connection.LoadAsync("import.sysml")).RequireNoErrors();
var imported = await fleet.ImportDataAsync(
[
    new DataImport("fleet.csv", await File.ReadAllTextAsync("fleet.csv")),
    new DataImport("fleet.txt", "{\"id\":\"car\",\"n\":3}\n")
    {
        Format = "jsonl",
        Map = "{\"element\":{\"path\":\"/id\",\"prefix\":\"V::\"},\"features\":{\"count\":{\"path\":\"/n\"}}}",
    },
]);                                                                    // imported.Model, imported.Content
await imported.SaveAsync("import.sysml", new SaveOptions { Overwrite = true });

var migrated = (await connection.LoadAsync("Probe.sysml")).RequireNoErrors();
var comparison = await migrated.CompareResultsAsync(await File.ReadAllTextAsync("Probe.results.json"), new CompareOptions { Seed = 7 });
Console.WriteLine(comparison);                                         // as sysml -compare-results prints it
```

| Method | Answers |
| --- | --- |
| `Model.ExportGraphsAsync` | The `Graphs` of an action or state machine, definition or usage: its `Content`, the canonical `graphs:<version>` JSON of its lowered graph and of every behavior it performs, the `Version` and the `Subject` as resolved |
| `Model.ImportDataAsync` | An `ImportResult` of `DataImport`s: an `ImportedFile` per file with the `Values` it set on how many `Elements` and each `ImportedValue` assignment, the edited notation as `Content` or `Documents`, and the imported `Model` |
| `Model.CompareResultsAsync` | A `Comparison` of a results index: a `ConfigurationComparison` per configuration with its `Status`, its runs as `SweepRow`s, each `ObservableComparison` of the tool's and the runs' `ComparedDistribution` with their `StatisticDifference` and the `CaseStatistics` a Monte Carlo case declares, and its `Lines` as `sysml -compare-results` prints them; and the `Standing` of the engine that ran them |

A graph's form is the text the service wrote, the same an external engine is sent: the client
reads nothing out of it and hands it on unchanged. A subject no element is named by is a
`ServiceException` with `NotFound`; one that is no action or state machine, or that several
elements are named by, is `InvalidArgument`; a model the service no longer holds is a
`ModelNotFoundException`. An export needs `export_graphs`.

Each data file is read as its name's extension says (`.csv`, `.tsv`, `.json`, `.jsonl`) or as
`DataImport.Format` names, by its own header or by the JSON mapping `DataImport.Map` holds (the
file `sysml -import-map` names), and the files are applied in order, one edit each, all of them
or none: a feature the element declares has its value set, one it inherits is redefined in the
element's body. The service parses the imported model and holds it under a hash of its own,
which `ImportResult.Model` names as a new handle borrowing the connection; the model imported
into is left as it was, and nothing is written to disk. An import that set no value answers
that model under its own hash and no notation. `ImportOptions.DryRun` reports the assignments
alone, with no model, notation or documents. A model of several documents is imported into as
one, its edited notation in `Documents` alone and `Content` null, so `ImportResult.SaveAsync`
refuses it and each `EditedDocument.SaveAsync` writes one; `ImportOptions.AcceptDocuments` is
true by default, and false asks the service to refuse such a model with `FailedPrecondition`.
A row naming no element or feature, a value that does not read, or values that make the model
invalid refuse the whole import with a `ModelException` keeping the diagnostics behind it, and
no model is made. An empty list of files and an import form other than `values` are refused by
the service with `InvalidArgument`; a format, mapping or form set to empty text is refused
before anything is sent, since the service reads empty as absent. A malformed answer, a refusal
beside what it set, other files than one per file in order, a dry run's model, notation in
`content` for a model of several documents, or documents other than the one document of a
single-document model, is a `TransportException` with `Internal`. An import needs
`import_data`; a lost answer does not prove the import was not made.

The results are the JSON `sysml -migration-results` writes beside a migrated model, compared
against that model. Every configuration they index is run, or those `CompareOptions.Actions`
names, as many times as `Runs` or its own `numberOfRuns` says, under the `Seed`, `Draws` and
`ClockStep` given or its own; `Observe` pairs stored observables with the features of the runs
that answer them. A configuration that cannot be run is `unresolved`, saying why, one whose runs
failed `fails` with their errors, and one not every observable of which compared `undecided`: a
comparison, never an exception. A distribution, difference or case statistic the answer has
none of is null, never zeros, and a pooled tool distribution knows its count and mean alone.
Results the service cannot read, a negative number of runs, an unknown draw policy, a clock
step no clock ticks by and an unknown engine are refused with `InvalidArgument`; results
nothing of which could be compared raise `CompareException` with the failure's `Reason` and
the `Partial` comparison the answer reported beside it. A comparison needs `compare_results`,
then `engines` to name an engine and `schedule_explore` for the explore engine, checked in
that order before anything is sent.

## Values

`Value` is an immutable value the service evaluates, or one sent to it: one nested sealed
class per arm of the schema's `Value`, `IntegerValue` to `InfinityValue`, which `Kind`, a
`ValueKind`, names. No type outside the library can derive from it. A switch over the arms
still needs a default case, since a release adds an arm whenever the service does. A
model's evaluations, attributes and feature values are these values, and so are the inputs,
arguments and sweep ranges a call sends.

```csharp
var speed = new Value.QuantityValue(new Quantity(
    new Value.RealValue(5.4), "km/h",
    new UnitTerm(1000, 3600, [new UnitFactor("SI::m", 1), new UnitFactor("SI::s", -1)])));
var third = new Value.RationalValue(new Rational(2, 6));      // 1/3
var members = new Value.SetValue([new Value.IntegerValue(3), new Value.IntegerValue(1)]);
Console.WriteLine($"{speed} {third} {members}");              // 5.4 [km/h] 1/3 {3, 1}
```

Numbers keep the schema's kinds apart and never pass through a `double`: `IntegerValue`
within `long`, `BigIntegerValue` beyond it (`Value.FromInteger` chooses), `RationalValue`
holding an exact `Rational` in lowest terms over a positive denominator, and `RealValue`,
which keeps NaN, the infinities and a negative zero exactly. These four are
`Value.NumberValue`, which a `Quantity`'s magnitude and a `VectorValue`'s components are;
`ToDouble` gives the nearest `double`, ties to even. A `Quantity` keeps its unit as written
and the `UnitTerm` it reduces to. `InfinityValue` is the model's unbounded `*`, not a Real.

Equality is by content and never the model's numeric comparison: `IntegerValue(1)` is not
`RealValue(1.0)`. Reals compare as `double.Equals` does, so NaN equals NaN and `0.0` equals
`-0.0`, while each is kept and sent as given. A `SetValue` equals one holding the same members
in any order, and hashes alike; it keeps the order it was given, which for an answer is the
service's canonical order, for reading and display only. Its membership is the same equality,
so `1` and `1.0` are two members here, although the service equates them and refuses a set
holding both. An `EnumLiteralValue` is its `LiteralId` and a `MetaobjectValue` its
`ElementId`, whatever else describes them; a `FunctionValue` is its `CalcId` and `SelfId`.
Collections are `ValueList<T>`, compared element by element in order, and
`ValueDictionary<TValue>`, compared entry by entry whatever order it was built in and
enumerated in ordinal order of its names; both copy what they are built from, and `default`
is empty. `ToString` writes a value as a reader would, in any culture: `4.0`, `1/3`,
`"text"`, `5.4 [km/h]`, `(1, 2)`, `{1, 2}`, `<undetermined>`, `*`.

Reading an answer checks what the schema makes canonical and never repairs it: an Integer
beyond `long` in canonical decimal digits, a Rational in lowest terms that no `double` holds
(the service sends one a `double` holds as a Real), dimensions the elements fill, a vector
of numbers. A malformed value is a `TransportException` with `Internal` naming where in the
answer it is, as `result.sequence[1].set[0]`, and a value of a kind the client does not know
is an `UnsupportedValueException`, never a null. Every arm is read whatever the service
advertises. Over Connect JSON a value of an unknown kind arrives with no kind at all, since
the parser drops fields it does not know.

Sending refuses what no service accepts as an argument, before anything is sent, with
`InvalidRequestException`: `UnsetValue` and `UndeterminedValue`, which are answers only; a
null standing for a value the service could not send; a function closing over an instance,
which lives only within the response that held it; and a quantity in a named unit without its
reduction. A kind the service gates needs its capability wherever the service looks for it:
in a sequence, set or array element or a vector component at any depth, and for an Integer
beyond `long` or a Rational also in a quantity's magnitude and an enumeration literal's
scalar. A missing one is a `CapabilityException` naming the first the service would refuse.
To a service without `rational_values`, a Rational a `double` holds exactly is sent as that
`double`, which is the same number; any other still needs the capability.

## Typed model classes

[`src/OpenMBEE.OpenSysML.Generate`](src/OpenMBEE.OpenSysML.Generate/) writes one C# file of typed
interfaces and classes over the definitions of your SysML model, which you commit beside your code
and read instances through. It is a console project of this checkout, run from the repository root;
it is not a Roslyn source generator, an MSBuild task or a packaged `dotnet` tool, and nothing starts a
service while you build.

```bash
dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Generate -- model.sysml -o ModelTypes.cs --namespace Acme.Model
dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Generate -- lib/ top.sysml -o ModelTypes.cs --namespace Acme.Model --check
make dotnet-generate SOURCES='lib/ top.sysml' OUTPUT=ModelTypes.cs NAMESPACE=Acme.Model [CHECK=1]
```

The model is one or more `.sysml` or `.kerml` files, parsed as one model in the order given; a
directory contributes every model file beneath it in path order, hidden directories skipped. The
service is the default any connection chooses (`$OPENSYSML_SERVICE`, else a private one), or the one
`--address` names or `--binary` starts. Generation needs the `type_facts` capability and stops with
`CapabilityException` before parsing anything without it: every property would otherwise be read as
a `Value`, indistinguishable from an untyped feature. A model with errors is refused with its
diagnostics. `--check` writes nothing and exits 1 unless the file is what the model generates now;
misuse and filesystem failures exit 2 with a diagnostic. An output path must name a file, not a
directory.

```csharp
using Acme.Model.Demo;

var vehicle = Vehicle.FromInstance(await model.InstantiateAsync("Demo::Vehicle"));
Console.WriteLine($"{vehicle.Mass} {vehicle.Engine.Power}");   // IVehicle, IEngine
```

Each definition gets an interface, `IVehicle`, declaring a read-only property per feature it owns,
and a class, `Vehicle`, implementing it. All declarations of a package across the input documents
contribute their children, including nested packages. Packages are namespaces under `--namespace`; a definition
nested in another element is named after its owners, `Vehicle_Wheel`. The interface extends the
interfaces of every generated definition it specializes, in declared order, and the class inherits
the class of the first, so diamond ancestry is one interface. Enumerations get no class: a feature
they type holds a literal. `Vehicle.FromInstance(instance, instantiation)` (or the instantiation
alone) returns an `IVehicle` of the most specific generated class that describes the instance's
type, a usage being as the definition it is typed by, so `is ICar` holds of a car; an instance of
an unrelated generated definition is refused with `InstanceTypeException`, and one no generated
class describes is viewed as a `Vehicle`. `Vehicle.Unchecked` views any instance.

| Feature type | Property type |
| --- | --- |
| `Boolean`, `String` | `bool`, `string` |
| `Integer`, `Natural` | `System.Numerics.BigInteger`, exactly |
| `Rational` | `Rational`, exactly |
| `Real`, `Complex` | `double`, `System.Numerics.Complex` |
| a quantity whose value names a unit | `Quantity` |
| an enumeration | `Value.EnumLiteralValue` |
| a generated definition | its interface, resolved through the instantiation |
| anything else (`Number`, an untyped feature, a library type with no class) | `Value`, its documentation saying why |

Multiplicity `1` (or none stated) reads one value, `0..1` a nullable one, and more a
`ValueList<T>`. An upper bound the service cannot determine statically also produces a list, never
an assumption that only one value is possible. With an upper bound of one, an unknown lower bound
produces a nullable property. Inherited facts are followed without a depth limit: when several
features are redefined, their ranges are intersected and their most specific compatible type is
used; incompatible facts are refused naming their providers.
A feature's identity is its qualified name, never its property's name: an inherited
property is read, in each class, from the feature that redefines it there, under whatever name, and
a redefinition narrowing its type under the same name hides the inherited property with `new`.
What cannot be generated soundly is refused, every problem named by the SysML elements behind it:
two features of one definition that neither redefines the other under one property name (`mass` and
`Mass` too), two redefinitions of one feature meeting in a specialization, a redefinition the
redefined property cannot read (`Real` as `String`, one value as many), two elements that would
generate one C# name, a name that would hide a member every view has (`Instance`, `ToString`, …),
a definition specializing itself, and a reference the documents do not resolve. C# keywords,
including contextual type names such as `record` and `required`, are written with `@`. Unicode
formatting characters, which C# ignores in identifiers, are removed before naming and collision
checks; equivalent names are refused rather than silently colliding in the compiler. Nothing is
renamed with a suffix.

Properties read through `OpenMBEE.OpenSysML.Typed`: `TypedFeatures` reads a feature by name for
each multiplicity, `FeatureDecoders` decodes a value, and the file's `SysmlTypes.Registry`, a
`TypedRegistry`, creates views. A value of another type or number is a `TypeMismatchException`, and
a feature whose evaluation failed or that was never materialized a `FeatureValueException`.
A required property rejects both null and `Unset`, including a property whose type is `Value`.

The file is deterministic: declarations in ordinal order of qualified names, nothing of the machine,
the time or the service. `SysmlTypes.GeneratorVersion` is the emission schema, which a change to the
output for an unchanged model (or to what the standard library contributes) bumps, and
`SysmlTypes.ModelHash` stamps the documents: a SHA-256 digest of each document's name (a file's
name, or a directory's name and the path within it) and exact bytes, length-delimited and in ordinal
order of the names, so a converted line ending changes it. A model is generated from its documents
alone: a reference outside them and the standard library is refused rather than stamped.

## Metamodel types

[`src/OpenMBEE.OpenSysML.Metamodel`](src/OpenMBEE.OpenSysML.Metamodel/) is the SysML v2 metamodel
as C# types, and a reader of exported model JSON into them, in an assembly of its own: it references
neither this library nor any package at run time, so a project that reads exported model JSON needs a
`ProjectReference` to it alone, and no service.

- One interface per metaclass, `IPartUsage` for `PartUsage`, extending the interfaces of the
  metaclass's superclasses, so `element is IFeature` and `elements.OfType<IPartUsage>()` select
  exactly the elements whose metaclass specializes `Feature` or `PartUsage`. Each declares its
  metaclass's own properties, read-only and PascalCased (`ownedFeature` is `OwnedFeature`): a
  single value as a nullable (`string?`, `bool?`, `int?`, `double?`, an enum or another
  metaclass's interface), a many-valued one as an `IReadOnlyList<T>`.
- One sealed class per concrete metaclass, in `OpenMBEE.OpenSysML.Metamodel.Elements`, implementing
  every property it inherits explicitly. An abstract metaclass such as `Element` has an internal one,
  for the elements a document types as the abstract metaclass itself or as a type the reader maps to
  it. Only the reader creates them, so a project reads elements through the interfaces.
- One enum per enumeration, its members the literals PascalCased (`inout` is
  `FeatureDirectionKind.Inout`); `EnumerationInfo` maps members and literals both ways.
- `Metaclasses` describes them: a `MetaclassInfo` per metaclass (`Metaclasses.PartUsage`, or by
  name or generated type), with its parents, linearized ancestors and declared and inherited
  `MetaProperty`s, each found by JSON key or C# name and carrying its range, multiplicity,
  derivation, default literal, redefinitions, subsets and opposite; and `MetamodelVersion`,
  `SourceTag` and `SourceCommit`, the metamodel release the types were generated from.

All metadata collections are immutable snapshots, including the registry lists, ancestry,
properties, association links and enumeration literals; none exposes a writable backing array.

A property the document does not supply throws `NotSuppliedException`, a `MetamodelException`,
rather than reading as a default the metamodel states; `IsDerived` tells a derived property, which an
export of owned properties omits, from an owned one. A property supplied as null reads as null, or
as empty when many-valued. `MetamodelException` derives from `Exception`, not
`OpenSysMLException`: the assembly does not depend on the client library.

### Reading exported JSON

`MetamodelGraph.ReadJson` reads a JSON export, the SysML v2 API's element array as `ToApiJsonAsync`
or `sysml -convert api-json` writes it, or sysml-toolkit's `full-json`, into the generated types.
[`samples/MetamodelReader`](samples/MetamodelReader/) references the metamodel project alone and reads
the file it is given:

```csharp
var graph = MetamodelGraph.ReadJson("vehicle.json");   // a path, never JSON text
foreach (var definition in graph.All<IPartDefinition>())
{
    Console.WriteLine($"{definition.DeclaredName}: {string.Join(", ", definition.OwnedFeature.Select(f => f.DeclaredName))}");
}
```

```bash
dotnet run --project client/dotnet/samples/MetamodelReader -c Release -- client/python/tests/fixtures/metamodel/bigger.full.json
# OpenMBEE.OpenSysML.Metamodel <version> (.NETCoreApp,Version=v10.0)
# 87 elements, 1 root namespace(s), metamodel 20250201
# part def Base owns mass
# part def Vehicle owns mass2, wheel, p, cargo, a, c, s
# part wheel : (none)
# part v : Vehicle
```

- **Sources.** A file path, a `Stream` (read to its end and left open), a parsed `JsonDocument`
  (left undisposed) or a sequence of `JsonElement`s, each with or without `ReadOptions`. A document is
  an element object, an array of them, a DataVersion (`identity` and `payload`, a null payload holding
  no element), an array of DataVersions, or a Commit whose `change` array holds them.
- **Complete and immutable.** Every element is created, every value converted and every reference
  resolved before `ReadJson` returns; the graph keeps none of the source's JSON, which the caller may
  dispose, and any number of threads may read it.
- **The graph.** `Elements` in document order, `this[id]`, `TryGet` and `Contains` by `@id`,
  `All<T>()` selecting by metamodel ancestry (`All<IFeature>()` holds every part usage), and `Roots`,
  the namespaces the document records no owner for and no element's `ownedRelationship` or
  `ownedRelatedElement` refers to. An element is one object however it is reached.
- **Types.** `@type` is matched with a `sysml:` prefix or the SysML namespace IRI, unversioned or
  versioned, removed, and kept as written in `JsonType`. A type the metamodel lacks is read as the first
  metaclass along `ReadOptions.Supertypes`, and as `Element` without one; a type naming an abstract
  metaclass is read as that metaclass.
- **Keys.** A key the element's metaclass has no property for, an exporter's `sysx:` annotations
  among them, is not converted. With `ReadOptions.KeepExtensions` every element keeps those members, as
  written, in `Extensions`, a dictionary of `JsonElement`s copied out of the source, so they outlive
  it; without it, reading `Extensions` throws. The keys of a type the metamodel lacks beyond those of
  the metaclass it is read as are kept the same way. `GetValue(jsonKey)` reads a property untyped, and
  throws `UnknownJsonKeyException` for such a key, as `MetaclassInfo.GetProperty` does.
- **Failures.** A document that is not an element graph fails the read with
  `MalformedDocumentException`: not JSON, an item that is no object, an element without a string `@id`
  or `@type`, two elements with one `@id`, invalid UTF-8, an undecodable Unicode property name,
  `@id` or `@type`, a key twice in any object (including ignored extensions and envelope metadata),
  or supertypes that form a cycle. These checks apply to every source overload.
  A value that does not fit its property, of the wrong JSON kind, a literal its enumeration lacks, an
  integer beyond 32 bits, a real that is not finite, an undecodable Unicode string (including a
  reference target) or a null in an array, fails only reads of that
  property, every time, with `MalformedValueException`. So does a reference to an element outside the
  property's metaclass, unless `ReadOptions.CheckRanges` is off: then the typed property, which cannot
  hold it, still throws, and `GetValue` returns it as written. Only range checks are off. A reference
  to an `@id` the document does not include, such as a standard-library element, or any `@ref`, fails
  only reads of its property with `UnresolvedReferenceException`.

OpenSysML's `api-json` export carries the owned properties it writes and some derived ones
(`ownedFeature`, `owner`, `qualifiedName`, `ownedMember`, `type` on a typed usage, …), but no
`feature`, `inheritedFeature` or `definition`, so those reads throw `NotSuppliedException` with
`IsDerived` until the engine serves derived properties; sysml-toolkit's `full-json` carries derived
properties as well. Both write some derived values outside their range: sysml-toolkit a requirement
definition's `result` as the definition itself, OpenSysML a constructor expression's `function` as
the attribute definition it constructs; read with `CheckRanges` off to reach them through
`GetValue`. OpenSysML also types some standard-library usages with metaclass names the metamodel
lacks (`BooleanUsage`, `StructureUsage`, …), read as `Element` unless `Supertypes` maps them. The
compact form ([`api-json-compact/1`](../../docs/reference/api-json-compact.md)) is not read: ask for
the element array.

`Generated.cs` is written by [`tools/gen/csmetamodel`](../../tools/gen/csmetamodel/) from the
metamodel table [`internal/translate/rdf/ontology`](../../internal/translate/rdf/ontology/) holds,
the same table the Python client's metamodel classes are generated from:

```bash
make dotnet-metamodel          # regenerate after the table changes
make dotnet-metamodel-check    # fail when the committed file is stale, as CI does
```

The generator refuses a table the C# types could not carry soundly, naming the properties or
metaclasses behind each problem: a property a subclass redeclares without redefining it, or redefines
with a wider range or another multiplicity; one inherited from unrelated metaclasses; two keys with
one C# name; a name every element already has (`Id`, `JsonType`, `MetaclassInfo`, `GetValue`,
`Extensions`, `ToString`, …); type names that would collide with the runtime or its framework types; and a table
without `Element` or `Namespace`, which the reader names. Metaclass names cannot
hide inherited registry members, and primitive and enumeration ranges require datatype properties,
while metaclass ranges require object properties. A redefinition that keeps a property's name narrows
its type in the subclass's interface (`new`), and every interface of the chain reads the most specific value.
The output is deterministic: declarations in ordinal order, nothing of the machine or the time.

## Coverage

Every RPC of the service and the public methods that call it, as the
[parity inventory](#parity-inventory) records them, in schema order; a test fails when this table
and the inventory disagree. An RPC the schema adds is listed `deferred` until a public method
calls it. Every field of the messages these RPCs exchange is classified in the inventory too, and
the [conformance runner](#conformance) runs every RPC's scenarios over each protocol, skipping only
the reviewed requests the API cannot express.

| RPC | Status | Called by |
| --- | --- | --- |
| `GetServerInfo` | implemented | `Connection.OpenAsync` |
| `ParseFile` | implemented | `Connection.LoadAsync`, `Connection.LoadContentAsync` |
| `ParseSources` | implemented | `Connection.ParseSourcesAsync` |
| `GetSymbol` | implemented | `Model.SymbolAsync`, `Model.FindSymbolAsync`, `Model.FindAsync`, `Model.LookupAsync` |
| `GetDiagnostics` | implemented | `Model.GetDiagnosticsAsync` |
| `Evaluate` | implemented | `Model.EvalAsync`, `Model.EvaluateAsync` |
| `Instantiate` | implemented | `Model.InstantiateAsync` |
| `ExecuteAction` | implemented | `Model.ExecuteActionAsync`, `Model.ExploreActionAsync` |
| `ExecuteState` | implemented | `Model.ExecuteStateAsync`, `Model.ExploreStateAsync` |
| `Convert` | implemented | `Connection.ConvertAsync`, `Model.ConvertAsync`, `Model.ToNotationAsync`, `Model.ToTurtleAsync`, `Model.ToApiJsonAsync`, `Model.SaveAsync` |
| `Migrate` | implemented | `Connection.MigrateAsync` |
| `ApplyEdits` | implemented | `Model.Edit().ApplyAsync`, `Model.ApplyEditsAsync` |
| `VerifyConstraint` | implemented | `Model.VerifyConstraintAsync` |
| `VerifyRequirement` | implemented | `Model.VerifyRequirementAsync` |
| `VerifySatisfaction` | implemented | `Model.VerifySatisfactionAsync`, `Model.SatisfiedAsync` |
| `ValidateInstance` | implemented | `Model.ValidateInstanceAsync` |
| `EvaluateCalc` | implemented | `Model.CalcAsync` |
| `RunAnalysis` | implemented | `Model.RunAnalysisAsync`, `Model.ExploreAnalysisAsync` |
| `CheckBehavior` | implemented | `Model.CheckActionAsync`, `Model.CheckStateAsync` |
| `RunSweep` | implemented | `Model.RunSweepAsync` |
| `RunRuns` | implemented | `Model.RunActionRunsAsync`, `Model.RunAnalysisRunsAsync` |
| `SelfCheck` | implemented | `Model.SelfCheckAsync` |
| `RecordRun` | implemented | `Model.RecordRunAsync` |
| `ListEngines` | implemented | `Connection.ListEnginesAsync` |
| `Query` | implemented | `Model.QueryAsync`, `Model.QueryOslcAsync` |
| `RunDocumentQuery` | implemented | `Model.RunDocumentQueryAsync` |
| `RenderDocument` | implemented | `Model.RenderDocumentAsync` |
| `RenderView` | implemented | `Model.RenderViewAsync` |
| `ExportGraphs` | implemented | `Model.ExportGraphsAsync` |
| `ImportData` | implemented | `Model.ImportDataAsync` |
| `CompareResults` | implemented | `Model.CompareResultsAsync` |

## Building and testing

```bash
make dotnet-test       # preflight, locked restore, build, format check, tests, samples
make dotnet-coverage   # the tests under coverlet: Cobertura and JUnit in bin/dotnet/coverage
make conformance-dotnet conformance-dotnet-check   # the conformance suite; see Conformance
```

`make dotnet-test` runs these commands, which also work on their own:

```bash
dotnet restore client/dotnet/OpenSysML.slnx --locked-mode
dotnet format client/dotnet/OpenSysML.slnx --verify-no-changes --no-restore
dotnet build client/dotnet/OpenSysML.slnx -c Release --no-restore -warnaserror
dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build --filter 'Category=Unit'
```

`dotnet test` runs on Microsoft.Testing.Platform, which `global.json` selects; the VSTest
form `dotnet test <solution>` is not supported. Every test declares a `Category` trait,
`Unit`, `Integration` or `Interop`, and a test enforces that, so a filter never silently
drops one. `Unit` tests need only the checkout and the runtime; `Integration` tests need the
service binary, or buf for the one that holds the [protocol code](#protocol-code) to the
schema; `Interop` tests need the other clients' toolchains.

Integration tests run against the binary `OPENSYSML_GRPC_BINARY` names, which they start
themselves on ports the kernel chooses and reach by address: once serving its default
`connect` transport, reached over gRPC, Connect with protobuf and Connect with JSON, and once
its grpc-go-only `grpc` transport, reached over gRPC, where Connect must be refused. Others
start it as a private service, shared, isolated, killed and stopped. Unit tests of behavior
every protocol shares run over all three wires too. They skip without a binary; set
`OPENSYSML_REQUIRE_SERVICE=1` to turn that skip into a failure, as CI does. The one test that
needs a solver's answer (the notes a `holds` verdict carries) asks the service whether its
`solve` engine is ready, and skips when it is not, with the engine's reason; set
`OPENSYSML_REQUIRE_SMT=1`, as CI does with z3 installed, to make that a failure too. `make
dotnet-test` also runs the Vehicle sample against such a service, and with the binary alone,
which it starts privately, under the same rule; the MetamodelReader sample it runs over the Python
client's sysml-toolkit fixture with neither named, since it needs no service:

```bash
OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" OPENSYSML_REQUIRE_SERVICE=1 make dotnet-test
```

The generator's tests, in [`test/OpenMBEE.OpenSysML.Generate.Tests`](test/OpenMBEE.OpenSysML.Generate.Tests/),
plan and emit synthetic models and compile what they emit against the library with warnings and
missing documentation as errors; against the service they run the command, regenerate every
committed golden over each protocol, and require `--check` to fail on a changed document inside a
named directory without writing. The goldens, generated from the shared Vehicle model, the Python
client's multi-file model and [`test/testdata/generate/cases`](test/testdata/generate/cases/) as
[`goldens.json`](test/testdata/generate/goldens.json) records, are compiled by
[`test/OpenMBEE.OpenSysML.Generate.Consumer`](test/OpenMBEE.OpenSysML.Generate.Consumer/), which
references the library alone under its build policy and reads live instances through them. `make
dotnet-generate-goldens` regenerates them and `make dotnet-generate-check` checks them.

The metamodel's tests, in [`test/OpenMBEE.OpenSysML.Metamodel.Tests`](test/OpenMBEE.OpenSysML.Metamodel.Tests/),
reference the metamodel project alone. They hold every interface's ancestry against the committed
metaclass hierarchy (`sysml-metaclasses.ttl`), select one element of every concrete metaclass by
every interface, read every inherited property of every element class through the interface
declaring it, and prove that no client, protobuf, gRPC, logging or Sigstore assembly or package
enters the metamodel's dependency graph. The generator's output for a synthetic table holding what
the metamodel itself lacks, same-named redefinitions and a diamond under a property-declaring
ancestor, is committed as [`test/testdata/metamodel/synthetic.golden.cs`](test/testdata/metamodel/synthetic.golden.cs);
the generator's Go tests keep it current, and these tests compile it with the runtime, warnings and
missing documentation as errors, and read it.
They also prove that public metadata lists refuse mutation and copy the arrays used to initialize
them, keeping enumeration round trips and property and ancestry lookups consistent.

The reader's tests, in the same project, port the Python client's reader and toolkit tests: every
source and envelope, type normalization and supertypes, abstract and unknown types, references
resolved, unresolved and cyclic, ranges with checks on and off, absent, null and malformed values,
the documents refused, extension keys kept as written and outliving every source, and the
sysml-toolkit `full-json` fixture the Python client reads, in place.
They read a value for every property of every metaclass, select by every interface over an element of
every metaclass, and read a graph after disposing its source. `MetamodelExportTests`, among the
library's integration tests, reads what a real service exports as `api-json` over every protocol.

Unit tests drive every way a private service starts and stops against
[`test/FakeService`](test/FakeService/), a stand-in the solution builds and the tests copy,
under the binary's name, into a directory of their own with a script beside it: it reports,
misreports or withholds an address, exits, lingers or floods its output as the script says.
It serves nothing; the in-process fake HTTP handler answers the connection.

Two probes, which the solution also builds, prove what only a process of its own can show,
and the tests run them against the real binary. [`test/OpenMBEE.OpenSysML.OrphanProbe`](test/OpenMBEE.OpenSysML.OrphanProbe/)
opens a private service, starts unrelated processes beside it, eight of them at the very
moment the service starts in the repeated race rounds, and is killed alone: the service must
end and the other processes run on. Its leak control hands a duplicate of the service's input
to one of them, and the test must then see the service survive, and end once that process
goes, so the check is known to catch an orphan. [`test/OpenMBEE.OpenSysML.LoadContextProbe`](test/OpenMBEE.OpenSysML.LoadContextProbe/)
loads three copies of the client, with their package dependencies, into collectible load
contexts and drives each by reflection, never naming a client type in the default context:
each copy must start a service of its own, which closing or unloading that copy alone stops,
including one still starting; every copy must be collected; and the probe must then exit by
returning from `Main`. Every process id a probe reports is killed when its test ends, whatever
the outcome. Race tests release opens, closes, stops of every service, cancelled waits and a
service's death from one barrier, round after round, and require every service to be stopped
exactly once and nothing to remain registered.
Each race round has a thirty-second deadline and passes its cancellation token to the
operations. Terminal assertions wait for background service-death cleanup. Connection attempts
and resolved child process ids belong to a disposable test scope: partial acquisition or a
failed assertion still releases successful connections, and a ten-second cleanup deadline
falls back to killing and awaiting only the scope's owned processes while reporting the
timeout as a failure.

A third probe, [`test/OpenMBEE.OpenSysML.CacheProbe`](test/OpenMBEE.OpenSysML.CacheProbe/),
holds and tries the shared cache's lock from a process of its own: through the client, or the
cross-client lock alone with its own system calls, as another client would, so that a change
to how the client locks cannot also change what checks it. It also loads two copies of the
client into collectible load contexts, which must shut each other out and be collected, and
runs a cache transaction that stops at a chosen step to be killed, after which the lock must
be free and nothing torn trusted. Interop tests run holders that take the lock with the
Python, Java and Rust clients' exact calls, in [`test/interop`](test/interop/): each client
must shut the other out both ways, wait for and see the other's release, and take the lock of
a killed holder of the other, and the Python client must read and write the cache's record
byte for byte as this one does. A failed holder acquisition stops and awaits its process
before reporting the failure, including when it times out or receives unexpected output.
They need Python, a JDK (`JAVA_HOME` or `PATH`, with
`javac`) and Cargo, which builds the Rust holder from its locked manifest into
`bin/dotnet/interop`; each test whose toolchain is missing skips naming it, and with
`OPENSYSML_REQUIRE_INTEROP=1`, as CI sets, fails instead:

```bash
OPENSYSML_REQUIRE_INTEROP=1 OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" OPENSYSML_REQUIRE_SERVICE=1 make dotnet-test
```

No test reaches the network. Downloads are tested against a release host and releases API of
the tests' own, [`ReleaseServer`](test/OpenMBEE.OpenSysML.Tests/ReleaseServer.cs), which speaks
HTTP/1.1 on a loopback port the kernel chooses and misbehaves on purpose: a checksum that
contradicts its pin, bytes that contradict both, a body that stalls before or after its
headers, never ends, trickles, claims a length it does not have or arrives gzipped to many times
its size, and redirects that loop, say not where, or leave for plain HTTP. Each runs against a
cache of the test's own, through the same resolution an open makes, and must leave a working
cache as it was and nothing staged. The signed-release fixtures the Python client's verifier is
tested against are served as a release nothing pins, valid, valid only historically, by another
identity, over a changed manifest, and unsigned, and every one must be refused as unpinned
without its signature being fetched: this client verifies pins alone, so no signature passes as
verified. The real binary is published there too, under a pin of its digest, and a private
service must start from its download, over each protocol, and a second open must fetch nothing.

CI runs the whole suite on Linux against the service its build job makes, then both
conformance targets, and, in the full tier, natively on macOS and Windows against a
`sysml-grpc` built on that runner, with the same minimum test count. Every one of those jobs
installs the Python, Java and Rust toolchains and requires the interop tests.
The Python gates use ASCII success markers so preflight, sample and coverage reports
also work with legacy Windows console encodings.

## Protocol code

`src/OpenMBEE.OpenSysML/Wire/Sysml.cs` and `SysmlGrpc.cs` are the message classes and the
gRPC client stub buf generates from [`api/proto/sysml.proto`](../../api/proto/sysml.proto)
into the `OpenMBEE.OpenSysML.Wire` namespace. They are committed, so a checkout builds with
neither buf nor network access. Do not edit them; regenerate them after a schema change:

```bash
make proto-csharp      # also part of make proto
```

CI regenerates them with the pinned buf before building and fails on any difference. A test,
`WireSchemaTests.TheCommittedStubsAreTheSchema`, catches the same drift locally: it builds
the schema with buf and compares its `sysml.proto` with the descriptor compiled into
`Sysml.cs` on every message, field (name, number, type, label, type name, oneof and proto3
`optional`), oneof, reserved number and name, enum value and RPC, and names the first
declaration where they part. Options are not compared, since managed mode rewrites the C#
namespace in what only the C# plugins see. It runs `$BUF`, read as the Makefile reads it,
else `buf` on `PATH`, and skips without either; set `OPENSYSML_REQUIRE_BUF=1` to turn that
skip into a failure, as the Linux CI jobs, which install buf, do. The native macOS and Windows
jobs have no buf and let it skip: the comparison does not depend on the platform. To run it
alone with the pinned buf:

```bash
BUF='go run github.com/bufbuild/buf/cmd/buf@v1.57.2' OPENSYSML_REQUIRE_BUF=1 \
  dotnet test --project client/dotnet/test/OpenMBEE.OpenSysML.Tests -c Release --filter 'FullyQualifiedName~WireSchemaTests'
```

[`api/proto/buf.gen.csharp.yaml`](../../api/proto/buf.gen.csharp.yaml) pins the generators
to the runtime packages `Directory.Packages.props` pins: `protocolbuffers/csharp` v36.2 for
Google.Protobuf 3.36.2 and `grpc/csharp` v1.84.0, client stub only, for Grpc.Net.Client
2.84.0. The generated types are public and mutable protocol code, not the client's API:
they are left out of the public API baseline and of coverage by their exact paths, in
[`.editorconfig`](.editorconfig) and the Makefile's `DOTNET_GENERATED`, as the
[metamodel types](#metamodel-types)' `Generated.cs` is, and a test fails when either names
anything else. Handwritten code refers to them as `Wire.X` rather than
importing the namespace, whose `Array` would otherwise shadow `System.Array`.

## Parity inventory

[`test/testdata/parity.json`](test/testdata/parity.json) records what the client must cover,
and the tests hold it to the schema and the [conformance scenarios](../../conformance/):

- `rpcs`: one row per RPC, in schema order, with its request and response messages, the
  public methods that will call it, its status (`deferred` until a public method calls it),
  every scenario of the RPC, and the capabilities the service requires of a request, each
  with the request field and the condition that make it required, in the order the service
  checks them. `valueFields` lists the request fields whose `Value`s are subject to the
  value gates.
- `values` and `edits`: every arm of `Value` and of `EditOperation`, with the capability a
  request holding the arm needs and, for a `Value`, the one under which the service sends it,
  and once implemented the test that proves the arm. Every `Value` and `EditOperation` arm is
  implemented; `sendGateOrder` and `gateOrder` are the orders the service checks the value and
  edit capabilities in, which tests read from the service's source along with the predicates
  behind each edit capability.
- `population`: response fields the service fills only under a capability.
- `fields`: every field of every message an RPC reaches, `deferred`, or once mapped `typed`,
  `raw-only` or `unsupported` with its reason and the test that proves it. Each message lists
  its fields in groups of one classification, `{"status", "reason", "test", "fields": [...]}`
  with a reason and test only where the classification has them; a field named twice, a group
  naming no field, and two groups of one classification, which are one group, are refused.

The checks compare sets, never counts: an RPC, field, arm or scenario the schema or the
suite adds, renames or removes fails the tests until the inventory accounts for it, and so
does a scenario that requires a capability no gate or governed field of its RPC names.
The schema they read is the descriptor compiled into the committed protocol code, which is
itself held to `api/proto/sysml.proto` (see [Protocol code](#protocol-code)), so the
inventory cannot pass against a stale copy of the schema.
The scenarios are read with the conformance runner's own reader, described next.

## Conformance runner core

[`src/OpenMBEE.OpenSysML.Conformance`](src/OpenMBEE.OpenSysML.Conformance/) is the part of a
.NET runner for the [conformance suite](../../conformance/) that does not depend on how a
call is made: it reads the scenarios, builds each request, judges each answer and writes the
reports, by the rules of the suite's README and as the reference runner,
[`tools/cmd/conformance`](../../tools/cmd/conformance/), applies them.

- **Reading.** Every `*.json` of `conformance/scenarios`, in the order of the file names'
  bytes (`1-…`, `10-…`, `2-…`), then of declaration. Reading is strict: a member the format
  does not define or states twice, a value of the wrong kind, a scenario without an id or an
  RPC, a model naming no fixture or both forms, and an id declared twice are refused.
- **Requests.** `${model_hash}`, `${fixture:…}` and `${fixture_base64:…}` are resolved where
  a whole string is one of them, a fixture outside `conformance/fixtures` is refused, and the
  protobuf-JSON is decoded into the RPC's message by the generated schema. A 64-bit integer
  keeps every digit, including decimal and exponent spellings: they are normalized exactly,
  range-checked and passed to Google.Protobuf as text rather than through a `double`.
  An invalid fixture path is a scenario error, not an abort of the remaining suite.
- **Judging.** Capability gating and without-capability expectations; each model parsed once
  per run; a deadline per parse and call, a call outliving it answering `DEADLINE_EXCEEDED`;
  the runner bounds its wait even if an adapter ignores cancellation, observes late failures
  and never caches a late parse hash. Caller cancellation is thrown, not reported as an outcome.
  Statuses compare by canonical name; responses are normalized by descriptor (version, model hash and
  absolute paths replaced, runtime instance ids labelled `@1`, `@2`, … in order of first
  appearance, enums by name, integral fields never compared as Reals) and compared: only
  what is named, every named list at its exact length, an unset field equal to its default,
  finite Reals within a relative 1e-9, integers by their exact digits for every whole-number
  spelling. NaN and infinities match only their explicit text spellings, never a finite number;
  overflowing or nonzero underflowing expectations are rejected. Mismatch messages spell
  values as the reference does.
- **Reports.** The JSON `-report` and JUnit `-junit` shapes the reference writes, member for
  member, with outcomes `pass`, `fail`, `skip` and `error`. JUnit replaces XML-invalid
  characters with U+FFFD in text and attributes; JSON retains the original strings.

How a scenario reaches the service is an adapter's: an `IScenarioClient` answers through the
client's public API and renders the result as the RPC's response message, or reports that the
API cannot express the request, which is a skip with that reason. The next section describes
the one that runs the suite.

[`test/OpenMBEE.OpenSysML.Conformance.Tests`](test/OpenMBEE.OpenSysML.Conformance.Tests/)
holds every comparison rule to a case that must hold and one that must not, ports the
reference runner's suite checks (every RPC covered, both kinds of failure pinned, every
fixture read), and pins a whole run of a small suite against a scripted adapter as goldens
of its JSON report, JUnit report and log. After an intended change to any of them, rewrite
the goldens and review the difference:

```bash
OPENSYSML_UPDATE_GOLDENS=1 dotnet test --project client/dotnet/test/OpenMBEE.OpenSysML.Conformance.Tests -c Release
```

## Conformance

The same project is the runner: it runs the suite **through the public API**, so a report says
what the client gave its caller, not only what the service put on the wire.

```bash
make build-grpc
make conformance-dotnet        # every protocol, then strict_conformance and oslc_query withheld;
                               # bin/conformance-report-dotnet.json and .xml
make conformance-dotnet-check  # the same run beside the reference runner's, compared scenario by scenario
dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Conformance -c Release -- \
  -binary bin/sysml-grpc -run '^evaluate/' -protocols connect-json -v
```

| Flag | Meaning |
| --- | --- |
| `-binary <path>` | The `sysml-grpc` to start, once per configuration, on ports the kernel chooses |
| `-service <host:port>` | A service already running, instead; it cannot withhold capabilities or choose a transport |
| `-dir <path>`, `-expected-skips <path>` | The suite and the reviewed skips; by default the checkout's `conformance/` and [`test/testdata/expected-skips.json`](test/testdata/expected-skips.json) |
| `-run <regexp>` | Only the scenarios whose id matches |
| `-protocols <list>` | Of `grpc`, `connect`, `connect-json`; all three by default |
| `-transport <name>` | What a started service serves: `connect` (default) or `grpc`, which only `-protocols grpc` can reach |
| `-withhold-capabilities <list>` | Also run a configuration whose service withholds these, through its test-only switch |
| `-report <path>`, `-junit <path>` | The reference runner's JSON (`-` for stdout) and JUnit reports |
| `-allow-skips` | Accept every skip, not only the reviewed ones |
| `-mutate <name>` | Corrupt every answer, to show the comparison is not vacuous |
| `-v` | Print each normalized response, as the reference runner does |

A flag the runner does not know, one given twice, a value missing, and a contradictory line
are usage errors (exit 2). A following option is never consumed as a value. Value-taking flags
also accept `-flag=value`; use this form for a literal value starting with `-`, such as
`-run=-v`, while `-report -` still selects stdout. A run that passes exits 0, and any other
exits 1, including a JSON or JUnit destination that cannot be written: the diagnostic names
the report format, destination and filesystem error.

Each RPC the API covers has an adapter, grouped by area beside the methods it calls
(`ConnectionArea`, `ModelArea`, `ExecutionArea`, `VerificationArea`, `SamplingArea`, `ConversionArea`,
`QueryDocsArea`, `EditingArea`, `GraphDataArea`). An adapter reads the
decoded request as the arguments of the typed method, so the request the client sends is the
one the scenario states, and renders what the method returned, with
[`Rendering`](src/OpenMBEE.OpenSysML.Conformance/Rendering.cs) reading only public types, back
into the response. The values a request sends are read into public `Value`s by public
constructors, so the typed call writes them itself. ExecuteAction and ExecuteState call the
explore method when the schedule explores, and RunAnalysis when its schedule or engine does,
and the run method otherwise; RunRuns calls the action or the Monte Carlo case method by which
the request names. ApplyEdits reads each operation as the `Edit` of its arm, an empty field
an optional part left null. A failure the service reported in band is rendered as the
response's `error`, failure reason and diagnostics, with what the exception kept of the answer:
a run's clock and trace, a check's or an analysis's partial result, a verification's objects,
verification verdicts and engine standing, the table runs made, the run, sweep or sample a
recording made, with its own failure where it reported one, a conversion's or migration's
formats, notice and diagnostics, a refused edit's kind, diagnostics and referrers, a refused
import's diagnostics, and a comparison's kind, source and engine standing; a refusal keeps the service's status and message, as does a
document query or document the model declares wrongly, which the API raises as a
`QueryException`; a
capability the client refuses for before sending is `UNIMPLEMENTED`, which the service would
answer; a transport failure is its status. Nothing is sent raw: a test reads the runner's IL
and fails on any call to `Connection.CallAsync` or the generated stub. A request setting a field
the adapter cannot send, or one the API has no way to express, is skipped for a stated reason;
so is one the API itself refuses with `InvalidRequestException`, for that exception's message.

Skips are accounted exactly. `expected-skips.json` lists the RPCs no adapter covers, which
must be the inventory's `deferred` rows and today are none, and the scenarios whose request the API cannot
express, each with the reason its adapter gives. The run fails for any other skip, a listed
skip given another reason, and a listed skip that ran, whether or not `-allow-skips` is
passed; in a configuration withholding capabilities, a skip is accounted only by a withheld
capability, the service must advertise exactly the default set without them, and each must
have its without-capability expectation executed, or, when every such scenario is a listed
skip, the run says so. Per protocol, of 194 scenarios:

| Configuration | ran | passed | skipped as uncovered | skipped as unexpressible |
| --- | --: | --: | --: | --: |
| default | 188 | 188 | 0 | 6 |
| `strict_conformance` and `oslc_query` withheld | 187 | 187 | 0 | 6 |

The unexpressible requests send what the public types refuse to hold or send:
`parse/naming_no_source_is_invalid` names no source, which the API always does; three
EvaluateCalc scenarios send a measurement reference without its reduction, a vector with a
string component, and a function closing over an object of another call; and two Query
scenarios send a comparison with no operator, and a structured query together with OSLC text,
which `QueryConstraint` and the two query calls cannot express. Withheld, the fallbacks of
`strict_conformance` and `oslc_query` run and are refused before sending.

`-mutate` shows the comparison catches what each corruption breaks, on every protocol, and a
test asserts it fails the run:

| `-mutate` | What it does to every answer | Scenarios that fail, per protocol |
| --- | --- | --: |
| `perturb-reals` | Moves each real by a millionth | 28 |
| `truncate-lists` | Drops the last entry of each list | 74 |
| `rewrite-strings` | Replaces each non-empty string but no map key | 104 |

`make conformance-dotnet-check` also runs the reference runner against the same binary, both
with `-v`, and `dotnet-gates.py conformance-check` holds the two runs together: both reports
have the reference's members in its order and totals that add up, the JUnit report states the
same results, including each testcase's pass, failure, error or skip disposition, and every
scenario the .NET run did not skip as reviewed has the reference's
outcome, status, missing capabilities and normalized response. Only timing, including the
wall time each row of a sweep, of runs or of a comparison reports and the time a recording states in the
records it generates, with the recorded model's hash that follows from it, which compares only
as present, and what answered are not compared, and the service's capabilities compare as a
set, which is how the client holds them.

## Build policy

[`Directory.Build.props`](Directory.Build.props) holds the policy every project shares:
nullable reference types, deterministic builds, warnings as errors, the recommended .NET
analyzers, and generated XML documentation, with a public member that has no XML comment
failing the build. The library and the metamodel each track their handwritten public API in
`PublicAPI.Shipped.txt` and `PublicAPI.Unshipped.txt`; a public symbol missing from them fails the
build.

Package versions are pinned once, in [`Directory.Packages.props`](Directory.Packages.props),
and each project's `packages.lock.json` records the resolved graph: a locked restore
fails on any change to either. [`nuget.config`](nuget.config) restores from nuget.org
alone.

## Version and pinned release digests

The client's version is the `Version` in `Directory.Build.props`, the SemVer spelling of
the core version it is released with. `python3 client/python/scripts/check_version.py
--tag v<version> --dotnet` checks it, and the Python suite fails when it disagrees with
the Python client's version. It also names the release a private service is downloaded from
when none is asked for: `v` and that version.

Nothing publishes the client: its release is the checkout of the core `v<version>` tag. On the
release branch, `python3 client/dotnet/scripts/dotnet-gates.py promote-api` moves every entry of
`PublicAPI.Unshipped.txt` into `PublicAPI.Shipped.txt`, and the release workflow refuses a tag
whose public API is still unshipped (`promote-api --check`) or whose `Version` is not the tag's.
[Releasing OpenSysML](../../docs/project/releasing.md#releasing-the-net-client-as-the-tags-checkout)
describes the procedure.

`src/OpenMBEE.OpenSysML/release-digests.json` is a copy of
[`client/release-digests.json`](../release-digests.json), embedded in the assembly: the digests
a downloaded release must have, and the only ones this client trusts (see
[Downloading releases](#downloading-releases)). Do not edit it:
`python3 scripts/sync-release-digests.py` rewrites it, and its `--check` and the tests fail
when it drifts. A checkout whose version is not yet released pins nothing for it, so its
default download is refused and gives way to the cache or `PATH`; ask for a pinned release
with `DownloadVersion` or `OPENSYSML_GRPC_VERSION` to download one.
