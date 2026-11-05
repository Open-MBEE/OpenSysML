# C# / .NET client: implementation plan

Add the repository-only `OpenMBEE.OpenSysML` client under `client/dotnet/`, followed by
typed model code generation and a standalone metamodel JSON reader. This is a private,
uncommitted planning document, not a user-facing specification or an instruction to start
implementation.

**Repository baseline:** `a662c2bce8882349feeac0c54a29952fde1cc143`, reviewed 2026-10-21.
The baseline has **31 service RPCs, 23 Value arms, and 17 EditOperation arms**. Counts are
observations, not permanent constants. A schema change must fail the classification tests
until the inventory and implementation status are explicitly reconciled.

Read [AGENTS.md](../../../AGENTS.md), [CONTRIBUTING.md](../../../CONTRIBUTING.md), and
[architecture](../../../docs/internals/architecture.md) before implementing. Do not modify the Go
service or its protocol to accommodate a client shortcut.

## 1. Scope and decision register

### 1.1 Settled product decisions

| Topic | Contract |
| --- | --- |
| Distribution | Checkout and `ProjectReference` only. Every project has `IsPackable=false`. No NuGet publication, package token, tag-time package stamping, or release artifact job. |
| Main assembly | `OpenMBEE.OpenSysML`; generated protocol namespace `OpenMBEE.OpenSysML.Wire`. |
| API | Async-first `Task<T> XxxAsync(..., CancellationToken cancellationToken = default)`, with immutable options and results. Connections and owned models implement `IAsyncDisposable`. No synchronous mirrors. |
| Scope | All 31 current RPCs through the typed facade and shared conformance scenarios. Typed generation and the metamodel reader are independent subsequent deliverables, not prerequisites for announcing service parity. |
| Protocols | gRPC/protobuf by default; Connect/protobuf and Connect/JSON explicitly selectable. gRPC plus JSON fails before opening a channel. Never silently downgrade or switch protocols. |
| Targets | `net10.0` only for the library, metamodel and tests (maintainer decision, 2026-10-21; see 1.2). No net8 target, runtime, sample or SDK check. Tests must execute on net10, not merely compile. |
| Dependencies | `Google.Protobuf`, `Grpc.Net.Client`, `Microsoft.Extensions.Logging.Abstractions`; additional production dependencies require a recorded reason. No `Grpc.Tools`; buf owns protocol generation. |
| Tests | xUnit v3 on Microsoft.Testing.Platform, public API analyzers, and coverlet.MTP, proven together by the decisions package (1.3) and again in the scaffold. |
| Signing | **Pins-only** (decided in 1.2): the evaluated .NET Sigstore library fails the trust contract. Refuse unverified new releases and state the limitation. Never implement Sigstore cryptography. |
| Metamodel | Separate `OpenMBEE.OpenSysML.Metamodel` assembly with no reference to the main client. `MetamodelException : Exception`; do not introduce a shared abstractions assembly just for exception inheritance. |
| AOT/trimming | Initially unsupported; do not set `IsAotCompatible` or `IsTrimmable` as an unproved claim. A successful publish-and-run spike may enable a separately gated support contract before API freeze. |
| Logging | Optional `ILogger`, default `NullLogger`. Warning paths remain explicit and tested, including cache reuse, unpinned downloads, and shutdown failure. Never log credentials or full authorization headers. |

The .NET 8 support decision was taken before API freeze: net8 is removed (it reaches end of
support on 2026-11-10). The consumer compatibility contract, test/runtime matrix, samples,
docs and SDK checks below all assume `net10.0` alone; reintroducing a second target is a new
product decision, not a build-file edit.

### 1.2 Blocking decisions and evidence

The `decisions` work package recorded these results on 2026-10-21 against the baseline commit.
Every row is resolved; the evidence, exact versions, commands and failed candidates are in 1.3.

| Decision | Status | Chosen policy |
| --- | --- | --- |
| Target support | **Decided (maintainer):** remove net8 before API freeze | `net10.0` only. No net8 runtime in CI, no net8 consumer/probe, no two-TFM assertions. |
| SDK and packages | **Proven** on macOS arm64 (1.3.1) | SDK 10.0.402 pinned in root `global.json` with the Microsoft.Testing.Platform runner; exact package/plugin pins in 1.3.1. VSTest-mode `dotnet test` is unsupported with these pins and is not used. |
| Sigstore | **Rejected candidate → pins-only** (1.3.5) | Sigstore 1.1.1 accepts unsigned RFC 3161 timestamps and bundles without an inclusion proof. Ship pins-only: refuse unpinned new releases unless explicitly opted out; signature-specific tests are classified unavailable, never passed. Re-evaluate only on a library release that fixes both. |
| OS locking | **Proven** on Linux (arm64 container, x64 CI), macOS (arm64 local and CI), Windows (x64 CI) (1.3.3) | POSIX: libc `lockf(F_LOCK)` on a descriptor opened with libc `open(2)`, never on a .NET `FileStream`/`File.OpenHandle` handle. Windows: `LockFileEx` exclusive over the whole range. Cross-ALC guard: libc `flock(LOCK_EX)` on POSIX, `FileShare.None` on Windows. |
| Process inheritance | **Proven** on Linux, macOS, Windows (1.3.4) | `Process.Start` with redirected stdin plus `-exit-with-parent` is sufficient; no handle-inheritance workaround is needed. The leak control proves the detector. |
| gRPC errors/JSON | **Measured** on macOS arm64 against both server transports (1.3.2) | Policy corrections recorded in 3.3, 3.4 and 3.5: identity-only gRPC compression, caller-token cancellation mapping without `ThrowOperationCanceledOnCancellation`, client-enforced Connect deadlines, unknown-enum JSON limitation. |
| C# quality gate | **Decided (maintainer):** accept the exclusion | Exclude `client/dotnet/**` from the generic Sonar scan with a reason comment; C# is gated by recommended analyzers as errors, PublicAPI tracking, nullable, `dotnet format` and coverlet Cobertura reports. |
| Optional AOT support | **Spike passed** on osx-arm64 and linux-arm64 only (1.3.6) | Still unsupported: no `IsAotCompatible`/`IsTrimmable` claim. A future support contract needs a CI publish-and-run gate for every claimed RID; the spike removes no blocker for ordinary delivery. |

Keep decisions and command results in this document while it remains private. Before
implementation PRs depend on them, preserve the relevant contract in the client README or
tests, and tool/package pins in build files. Scratch spike code is not a deliverable.

### 1.3 Decision evidence

Environments: local macOS 26.6.2 arm64 (Python 3.14.2, Temurin 21.0.12.1, cargo 1.94.0,
Go 1.26.1); Linux arm64 in `mcr.microsoft.com/dotnet/sdk@sha256:0d99ebf1ccad15037b5c98c9fe8979d6b8dbac23812feae2277a0b277b5c5ffa`
(Ubuntu 24.04, Python 3.12.3, OpenJDK 21.0.12.1, cargo 1.75.0); and one disposable GitHub
Actions run on `windows-latest` (Windows 10.0.26100 x64, Git Bash), `ubuntu-latest`
(Ubuntu 24.04 x64) and `macos-latest` (macOS 26.6.2 arm64), each with setup-dotnet 10.0.402,
Temurin 21.0.12.1, Python 3.12, cargo 1.99.0 and `sysml-grpc` built from the baseline commit.
The run's branch and logs were deleted afterwards. The service was `make build-grpc` output
(v0.10.1-1816-ga662c2bce) locally and a `CGO_ENABLED=0` cross-build for the container.
Timings below are single observations, not performance claims.

#### 1.3.1 SDK, packages and generator

| Item | Pin | Notes |
| --- | --- | --- |
| SDK | 10.0.402 (runtime 10.0.13) | Latest .NET 10 SDK on 2026-10-13. `global.json`: `"sdk": {"version": "10.0.402", "rollForward": "latestPatch", "allowPrerelease": false}`, `"test": {"runner": "Microsoft.Testing.Platform"}` |
| buf | v1.57.2 | The repository's existing `BUF_VERSION`, via `go run`; no second buf pin |
| C# messages | `buf.build/protocolbuffers/csharp:v36.2` | Pairs with Google.Protobuf 3.36.2 (protobuf vN.M ↔ 3.N.M) |
| C# gRPC client | `buf.build/grpc/csharp:v1.84.0`, `opt: no_server` | Pairs with Grpc.Net.Client 2.84.0 |
| Google.Protobuf | 3.36.2 | |
| Grpc.Net.Client | 2.84.0 | Brings Grpc.Core.Api 2.84.0 |
| Microsoft.Extensions.Logging.Abstractions | 10.0.13 | |
| Microsoft.CodeAnalysis.PublicApiAnalyzers | 5.6.0 | `PrivateAssets="all"` |
| xunit.v3 | 4.0.2 | Brings xunit.v3.mtp-v2 4.0.2; test projects are `OutputType=Exe` |
| coverlet.MTP | 10.1.0 | MIT; the MTP extension, not the VSTest collector |

Central package management (`ManagePackageVersionsCentrally`, transitive pinning) with
`RestorePackagesWithLockFile` and an `.slnx` solution. The separate C# buf template enables
managed mode with only `csharp_namespace = OpenMBEE.OpenSysML.Wire` overridden and the other
language options disabled, so Go output is untouched. It generated `Wire/Sysml.cs` (2.56 MB)
and `Wire/SysmlGrpc.cs` (161 KB); the descriptor exposes 31 methods.

Commands, all passing from the scratch root (exit 0 unless stated):

```bash
dotnet --version                                                     # 10.0.402
dotnet restore client/dotnet/OpenSysML.slnx --locked-mode
dotnet format client/dotnet/OpenSysML.slnx --verify-no-changes --no-restore
dotnet build client/dotnet/OpenSysML.slnx -c Release --no-restore -warnaserror  # 0 warnings
dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build --filter 'Category=Unit'   # 1 of 3
dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build --minimum-expected-tests 3 # 3 of 3, incl. uncategorized
dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build --results-directory <dir> \
  --coverlet --coverlet-output-format cobertura --report-xunit-junit \
  --coverlet-exclude-by-file '**/Wire/Sysml.cs' --coverlet-exclude-by-file '**/Wire/SysmlGrpc.cs'
```

Findings the scaffold and wire packages must carry:

- **VSTest mode is gone for these pins.** With xunit.v3 4.x, `dotnet test <sln>` under the
  .NET 10 SDK fails: "Testing with VSTest target is no longer supported by
  Microsoft.Testing.Platform on .NET 10 SDK and later". The MTP runner must be opted in through
  `global.json` and the solution passed as `--solution`; `--filter` keeps VSTest filter syntax
  under xUnit's MTP host. Failed candidate: xunit.runner.visualstudio 4.0.1 +
  Microsoft.NET.Test.Sdk 18.11.0 + coverlet.collector 10.1.0.
- **Generated code and the API analyzer.** The generated files carry `// <auto-generated>`,
  yet PublicApiAnalyzers reports 11,160 RS0016 and 6,544 RS0041 against them; nothing else
  fires. Exact-path `.editorconfig` severity (`[src/OpenMBEE.OpenSysML/Wire/{Sysml,SysmlGrpc}.cs]`,
  RS0016/RS0041 `none`, `generated_code = true`) removes them with no project-wide suppression.
- **Coverage counts generated code** unless the two files are excluded by path, as above;
  the excluded report then lists only handwritten files.
- **Name collision.** Generated `OpenMBEE.OpenSysML.Wire.Array` shadows `System.Array` in any
  file that imports the wire namespace (CS0104). Handwritten code aliases rather than imports it.
- **Negative controls observed failing:** a changed package pin fails `--locked-mode` restore;
  an extra space fails `dotnet format --verify-no-changes` (exit 2, WHITESPACE); an undeclared
  public method fails the build with RS0016.

#### 1.3.2 gRPC, Connect and JSON behavior

Grpc.Net.Client 2.84.0 against `-transport connect` (h2c mux) and `-transport grpc` (grpc-go):

| Probe | Observation | Policy consequence |
| --- | --- | --- |
| h2c gRPC to both transports | `GetServerInfo` succeeds on both (72 capabilities) | Plain `http://` channels; no TLS or handler tweaks for h2c |
| Connect protobuf/JSON over HTTP/1.1 to `connect` | 200 `application/proto` / `application/json` | As specified in 3.4 |
| Connect to `grpc` (grpc-only) | `HttpRequestException(HttpRequestError.Unknown)` ← `IOException` ← `SocketException(ConnectionReset)` | Explicit refusal is a TransportException; the cause alone does not identify a grpc-only server |
| Service error over gRPC | `RpcException(NotFound, "model not found: nope")`, no trailers, no debug exception | ServiceException from the status code, never from text |
| Service error over Connect | HTTP 404, `application/json` `{"code":"not_found","message":"model not found: nope"}` for both request encodings | Error envelope is always JSON, even for protobuf requests |
| Unknown Connect method | HTTP 404 `text/plain` "404 page not found" | Non-Connect body: HTTP-status fallback (404 → unimplemented) |
| Refused port / DNS failure | `RpcException(Unavailable, "Error connecting to subchannel.")`, `DebugException` `SocketException(ConnectionRefused / HostNotFound)` | TransportException(Unavailable) with that cause |
| gRPC to an HTTP/1.1-only server | `RpcException(Internal, "Error starting gRPC call…")`, `DebugException` `HttpRequestException(HttpProtocolError)` ← `HttpProtocolException(PROTOCOL_ERROR 0x1)` | The Connect remedy applies only to this cause chain, which is `Internal`, not `Unavailable` |
| Pre-canceled / mid-flight caller token (default options) | `RpcException(Cancelled, "Call canceled by the client.")`, `DebugException` `OperationCanceledException` | Map to `OperationCanceledException(callerToken)` when the caller's token is canceled |
| Deadline (default options) | `RpcException(DeadlineExceeded)` | TransportException(DeadlineExceeded) |
| `ThrowOperationCanceledOnCancellation = true` | Deadline expiry also surfaces as `TaskCanceledException` | **Do not set it**: it makes deadline and caller cancellation indistinguishable |
| Default receive limit, grpc-go server | 13,520,122-byte Query response: `ResourceExhausted` | Override to 512 MiB as planned |
| Default receive limit, connect server | Same response **succeeds** under 4 MiB, and under 3 MiB; fails at 1 MiB | Connect-go gzips the response, and the limit bounds only the compressed frame |
| `CompressionProviders = []` | 4 MiB and 1 MiB limits fail; 512 MiB succeeds | Advertise identity only on gRPC unless a decompressed-size-bounded provider is implemented |
| Connect mid-flight caller cancel | `TaskCanceledException` ← `IOException` ← `SocketException(OperationAborted)`, caller token canceled | OCE carrying the caller's token |
| `Connect-Timeout-Ms: 150` on an ~800 ms parse | HTTP 200 with the full result | The server does not cut CPU-bound handlers short; the client enforces its own deadline |
| Large Query (60,001 elements) | Protobuf 13,520,122 B, JSON 15,220,165 B; Connect totals ~440 ms (proto) vs ~600–640 ms (JSON) including decode | Observation only |
| 8 concurrent Query calls on one connection | gRPC 1,437 ms, Connect proto 1,470 ms, Connect JSON 2,353 ms; all results equal | No head-of-line or pool failure; observation only |
| Child stdin EOF | Both transports exit 0 | Orderly shutdown as in 3.6 |

JSON (`JsonParser` 3.36.2, `Verdict.failure_reason`):

| Input | `JsonParser.Default` | `IgnoreUnknownFields(true)` |
| --- | --- | --- |
| Known enum name | Parsed | Parsed |
| Unknown numeric enum (99) | Preserved as 99, re-formatted as `99` | Same |
| Unknown enum name | `InvalidProtocolBufferException` | **Field dropped**: reads as 0/UNSPECIFIED |
| Unknown field | `InvalidProtocolBufferException` | Ignored |

Binary protobuf preserves an unknown enum number (99) and an unknown field byte-for-byte.

#### 1.3.3 Cache lock interoperability

Probes mirrored each client's exact call: Python `fcntl.lockf(LOCK_EX)` / `msvcrt.locking`
(`binary.py`), Java `FileChannel.tryLock()` polled every 25 ms (`BinaryDownloader`), Rust
`rustix::fs::fcntl_lock` 1.1.4 / std `File::lock` (`binary.rs`). Every holder held while every
contender tried a non-blocking lock (must be BUSY), then retried after release (must succeed).

| .NET candidate | Linux | macOS | Windows |
| --- | --- | --- | --- |
| libc `lockf` on a libc-`open(2)` descriptor | **Interoperates both ways** | **Interoperates both ways** | n/a |
| libc `lockf` on a `File.OpenHandle` descriptor | Interoperates | **Fails**: the open itself throws while another client holds the lock, and a blocking `F_LOCK` deadlocks against the process's own implicit `flock` | n/a |
| `FileStream.Lock(0, 1)` | Interoperates, but non-blocking and process-scoped | `PlatformNotSupportedException` | Interoperates |
| libc `flock` / `FileShare.None` | **No exclusion** against any other client | Excludes only by accident of BSD flock/fcntl interaction | `FileShare.None` reports BUSY via sharing violation, not a lock |
| `LockFileEx` exclusive, whole range | n/a | n/a | **Interoperates both ways** |

Root cause of the macOS failure: on Unix, .NET opens every writable file with an implicit
`flock(LOCK_SH|LOCK_NB)` for `FileShare.Read/ReadWrite` (`LOCK_EX` for `None`), and on macOS
BSD `flock` and `fcntl` locks conflict. The proven open is: create the file if absent with
`FileMode.CreateNew` (O_EXCL fails before any flock is attempted), then call libc
`open(path, O_RDWR | O_CLOEXEC)`. That call passes only the two fixed arguments, so it is
valid under the Apple arm64 variadic ABI. `O_CLOEXEC` is 0x80000 on Linux and 0x1000000 on
macOS. Wrap the result in `SafeFileHandle(fd, ownsHandle: true)`.

Other observations, identical on every applicable platform:

- Blocking waits: .NET `lockf` waited ~2,430 ms for a 2,500 ms Python holder, and Python waited
  ~2,430 ms for .NET (Windows: 2,418 ms and 3,000 ms, the latter Python's 1 s retry granularity).
- Crash recovery: Python is BUSY before and acquires immediately after SIGKILL/TerminateProcess
  of a .NET holder.
- Descriptor pitfall (Linux, macOS): opening and closing a second descriptor to the lock file
  while holding it lets Python acquire. The lock must own its only descriptor.
- `DOTNET_SYSTEM_IO_DISABLEFILELOCKING=1` silently removes `FileShare.None` exclusion on POSIX,
  so the POSIX guard must be an explicit libc `flock(LOCK_EX)`, not a sharing mode.
- Cross-ALC (two collectible contexts loading the same assembly, distinct static state):
  `flock` and `FileShare.None` exclude on POSIX; `lockf` and `FileStream.Lock` do not, since
  they are process-scoped. On Windows `LockFileEx`, `FileStream.Lock` and `FileShare.None` all
  exclude, since they are handle-scoped.
- The negative controls are platform-specific: substituting `flock` fails the matrix on
  Linux, and substituting a `FileStream`-opened descriptor fails it on macOS.

#### 1.3.4 Child lifetime and handle inheritance

A .NET parent started `sysml-grpc -port 0 -health-port 0 -report-address -exit-with-parent`
with redirected stdin/stdout/stderr, then unrelated redirected siblings (`sleep 300` /
`ping.exe -n 300`), sequentially or with eight siblings started on other threads at the same
moment as the child. The harness then killed only the parent (`Process.Kill(false)`).

| Variant | Linux (arm64, x64) | macOS (arm64 local, CI) | Windows x64 |
| --- | --- | --- | --- |
| Sequential | Child exits in 2–4 ms; sibling alive | 1–8 ms; alive | 373 ms; alive |
| Concurrent, 8 siblings (3 local, 5 CI runs) | 2–6 ms; 9/9 alive | 1–45 ms; 9/9 alive | 5–6 ms; 9/9 alive |
| Leak control (stdin write end made inheritable: `dup` / `SetHandleInformation`) | Child survives 10 s → detector fails as intended | Same | Same |

#### 1.3.5 Sigstore

Candidate: `Sigstore` 1.1.1 (MIT; deps Tuf 1.1.1, BouncyCastle.Cryptography 2.7.0; net10.0
only; one maintainer; not endorsed by the sigstore organization; its release run passes
sigstore-conformance v0.0.29). The cross-check was the repository's `opensysml.signing` on
sigstore 4.5.1.

- **Passes by configuration:** the real v0.10.1 release (CircleCI-signed; the certificate
  expired on 10-16 but was valid at integration), every committed fixture (valid
  historically-expired, tampered manifest, tampered signature, other identity, invalid at
  integration, unreadable), wrong project/issuer/definition, non-UUID or suffixed definition,
  untrusted instance, and a one-byte tamper of the real manifest. The SAN policy needs an
  anchored pattern (`^…/pipeline-definitions/<uuid>\z`), because matching is unanchored.
- **Disqualifying:** RFC 3161 timestamp tokens are trusted without verifying their CMS
  signature. A forged token embedding the public TSA certificate verifies against the
  production root and supplies the time used for certificate validity. The inclusion proof
  is optional: with the checkpoint and root hash removed, the entry still verifies. Together
  they accept a bundle with no transparency evidence and an attacker-chosen time. Python
  sigstore rejects every such case. Guarding these in our code would mean patching the
  library's security checks, which this plan forbids.
- Also observed: release nightlies are signed by the GitHub Actions nightly workflow, not
  CircleCI, so both existing clients (correctly) reject them under the signer policy. Gaps in
  `make_signed_release_fixture.py` that the .NET suite must construct itself: wrong project
  only, wrong issuer only, not yet valid, non-UUID definition, untrusted instance.

#### 1.3.6 NativeAOT spike

A `PublishAot=true` consumer of the scratch library made a gRPC call, ParseFile+Query, Connect
protobuf and JSON Query round trips, a full descriptor walk with every request parser, and
oneof JSON formatting. `dotnet publish -r osx-arm64` and `-r linux-arm64` (clang + zlib1g-dev
in the container) produced 0 warnings, and every path passed at runtime against the service.
Not run: linux-x64, osx-x64, win-x64, or any trimmed non-AOT publish. These results are
recorded only as input to a future support decision.

## 2. Authoritative parity inventory

### 2.1 Reading and mapping rules

The schema and scenarios, not a language client's README, define the service contract:

- [Service schema](../../../api/proto/sysml.proto): request/response fields, oneofs, presence, comments.
- [Conformance contract](../../../conformance/README.md) and [scenarios](../../../conformance/scenarios/).
- [Reference runner](../../../tools/cmd/conformance/main.go) and its neighboring tests/renderers.
- [Java model facade](../../../client/java/opensysml-client/src/main/java/org/openmbee/opensysml/Model.java)
  and [converters](../../../client/java/opensysml-client/src/main/java/org/openmbee/opensysml/internal/Protos.java).
- [Rust client](../../../client/rust/opensysml/src/) and [Python client](../../../client/python/opensysml/)
  for independent evidence where behavior is unclear.

In the table below, method names return `Task<Result>` and take the standard final
`CancellationToken`. `C` means `Connection`, `M` means `Model`. Request/response types are
the exact types named by each RPC descriptor; options are the handwritten public inputs.
The gates column names RPC/important option gates, not an exhaustive field ledger.

During `wire`, create `testdata/parity.json`: one row per RPC containing its full name,
wire request/response types, public method/owner, options/result type, scenario IDs,
delivery package, and status (`deferred` or `implemented`). Also classify every request
and response field by descriptor path as typed, intentionally raw-only, or unsupported,
with a reason and test. Request/response nested types are traversed once with cycle detection.
Raw-only fields must not be described as typed parity. Before `release`, every current
service field needs a typed mapping or a reviewed, documented limitation; none may disappear
merely because a scenario does not assert it.

### 2.2 All current RPCs

| RPC | Typed method | Options/input -> result | Capability gates | Scenario group | Package |
| --- | --- | --- | --- | --- | --- |
| GetServerInfo | C.OpenAsync; cached ServerInfo/Capabilities | ConnectionOptions -> Connection | Bootstrap, RequireCapabilities checked afterward | 01-server-info | 04: grpc |
| ParseFile | C.LoadAsync / LoadContentAsync | ParseOptions -> Model | inline_language, strict_conformance | 02-parse | 07: model |
| ParseSources | C.ParseSourcesAsync | SourceDocument[], SourcesOptions -> Model | strict_conformance, parse_sources_affected when requested | 02-parse | 07: model |
| GetSymbol | M.SymbolAsync / FindSymbolAsync / LookupAsync | symbol ID -> Symbol | Response-population fields decoded regardless of advertisement | 03-symbol | 07: model |
| GetDiagnostics | M.GetDiagnosticsAsync | DiagnosticsOptions -> diagnostics | diagnostic_codes is response-population | 02-parse | 07: model |
| Evaluate | M.EvalAsync / EvaluateAsync | expression, EvalOptions -> Value/evaluation | evaluate_subject; nested values | 04-evaluate | 07: model |
| Instantiate | M.InstantiateAsync | symbol ID -> Instantiation | feature_values is response-population; no request values | 05-instantiate | 07: model |
| ExecuteAction | M.ExecuteActionAsync / ExploreActionAsync | RunOptions -> ActionRun/Exploration | schedule, schedule_explore, performer | 06-behavior | 12: execution |
| ExecuteState | M.ExecuteStateAsync / ExploreStateAsync | RunOptions -> StateRun/Exploration | schedule, schedule_explore, performer, state_trace | 06-behavior | 12: execution |
| Convert | C.ConvertAsync; M.To*Async | ConvertSource, ConvertOptions -> Conversion | convert, convert_documents, convert_compact | 07-convert | 15: conversion |
| Migrate | C.MigrateAsync | MigrateSource, MigrateOptions -> Migration | migrate | 16-migrate | 15: conversion |
| ApplyEdits | M.Edit().ApplyAsync | Edit[], EditOptions -> EditResult | apply_edits plus per-operation gates; edit_documents | 08-apply-edits | 17: editing |
| VerifyConstraint | M.VerifyConstraintAsync | symbol ID, VerifyOptions -> Verification | verification, verification_questions, verification_arguments, engines when selected | 09-verify | 13: verification |
| VerifyRequirement | M.VerifyRequirementAsync | symbol ID, VerifyOptions -> Verification | verification, verification_questions, verification_arguments, engines when selected | 09-verify | 13: verification |
| VerifySatisfaction | M.VerifySatisfactionAsync / SatisfiedAsync | symbol/scope, VerifyOptions -> Satisfaction | verification; applicable option gates | 09-verify | 13: verification |
| ValidateInstance | M.ValidateInstanceAsync | symbol ID, ValidateOptions -> Validation | verification; engines when selected | 09-verify | 13: verification |
| EvaluateCalc | M.CalcAsync | symbol ID, positional arguments, CalcOptions -> Calculation | verification; nested values | 10-evaluate-calc | 13: verification |
| RunAnalysis | M.RunAnalysisAsync / ExploreAnalysisAsync | symbol ID, AnalysisOptions -> Analysis/Exploration | verification, schedule, schedule_explore, engines | 13-run-analysis | 13: verification |
| CheckBehavior | M.CheckBehaviorAsync | CheckOptions -> CheckResult | check, performer when selected; nested input values | 19-check-behavior | 12: execution |
| RunSweep | M.RunSweepAsync | symbol ID, SweepRange list, SweepOptions -> Sweep | verification; nested values; engines when selected | 14-run-sweep | 13: verification |
| RunRuns | M.RunRunsAsync | RunsOptions -> Runs | runs, performer, engines when selected; nested values | 20-runs-record-selfcheck | 14: sampling |
| SelfCheck | M.SelfCheckAsync | rule package names -> SelfCheck | self_check | 20-runs-record-selfcheck | 14: sampling |
| RecordRun | M.RecordRunAsync | analysis ID, RecordOptions -> Recording | record_run; underlying run/sweep/sample option gates | 20-runs-record-selfcheck | 14: sampling |
| ListEngines | C.ListEnginesAsync | ListEnginesOptions(Probe=false) -> EngineInfo[] | engines | 15-list-engines | 13: verification |
| Query | M.QueryAsync / QueryOslcAsync | Query or OSLC text -> QueryResult | query, oslc_query for OSLC | 11-query | 16: query-docs |
| RunDocumentQuery | M.RunDocumentQueryAsync | query ID, typed bindings -> DocumentQueryResult | document_query; rational_values in bindings | 12-document | 16: query-docs |
| RenderDocument | M.RenderDocumentAsync | document ID, DocumentForm -> RenderedDocument | render_document | 12-document | 16: query-docs |
| RenderView | M.RenderViewAsync | view name, RenderViewOptions -> RenderedView | render_view; current option gates in field ledger | 17-render-view | 16: query-docs |
| ExportGraphs | M.ExportGraphsAsync | behavior subject -> Graphs | export_graphs | 18-export-graphs | 18: graph-data |
| ImportData | M.ImportDataAsync | DataImport[], ImportOptions -> ImportResult | import_data; document acceptance | 21-import-compare | 18: graph-data |
| CompareResults | M.CompareResultsAsync | results JSON, CompareOptions -> Comparison | compare_results; applicable option/value gates | 21-import-compare | 18: graph-data |

Freeze all conditional gate predicates in `wire` by reading the current service/client
implementation and testing capabilities withheld. CheckBehavior's RPC capability is `check`,
not a name inferred from its RPC spelling.

### 2.3 Drift guards

`ParityInventoryTests` must compare descriptor sets, not just counts:

1. Every RPC has exactly one inventory row with matching wire request/response types.
2. Every Value kind and EditOperation arm has exactly one mapping/test classification.
3. Every descriptor field has a classification; removed/renamed fields leave no stale rows.
4. Every scenario ID exists, and every scenario is either executed or exactly allowlisted
   with a stable reason. Every implemented RPC executes its expressible scenarios.
5. Every advertised default-service capability has a public constant. Unknown capability
   names from a newer server remain readable; the local drift gate must not reject an
   external server merely for being newer.
6. Adding a synthetic RPC, field, arm, or scenario to the test inventory must fail the
   appropriate check. A newly advertised capability alone is not a substitute for RPC coverage.

For each owning package, deferred inventory entries become implemented only in the same
change as converters, request gating, public API adapters, tests, and README coverage.

## 3. Architecture and API contracts

### 3.1 Layout and build ownership

```text
global.json                              # repository-root SDK pin; commands run at repo root
client/dotnet/
  OpenSysML.slnx
  Directory.Build.props                  # single client Version, TFMs, common build policy
  Directory.Packages.props               # centrally pinned packages
  README.md
  src/
    OpenMBEE.OpenSysML/
      Wire/Sysml.cs, Wire/SysmlGrpc.cs     # committed buf output
      Internal/                          # transport, converters, lifecycle, binary/cache helpers
      Model.*.cs                         # partials owned by service-area packages
      release-digests.json               # EmbeddedResource; synced, never manually edited
      PublicAPI.Shipped.txt
      PublicAPI.Unshipped.txt
    OpenMBEE.OpenSysML.Conformance/
    OpenMBEE.OpenSysML.Generate/
    OpenMBEE.OpenSysML.Metamodel/
  test/
    OpenMBEE.OpenSysML.Tests/
    OpenMBEE.OpenSysML.Conformance.Tests/
    OpenMBEE.OpenSysML.Metamodel.Tests/
    OpenMBEE.OpenSysML.OrphanProbe/
    OpenMBEE.OpenSysML.LoadContextProbe/
    testdata/parity.json
    testdata/expected-skips.json
  samples/Vehicle/
```

Only create projects when their work package needs them; no empty placeholder projects.
The main library, metamodel, and tests target `net10.0` alone (1.2). Console tools and
probes target net10 as well.

Root `global.json` pins an exact .NET 10 SDK with explicit roll-forward policy. Do not rely
on SDK discovery from a nested solution path. The same `global.json` opts `dotnet test` into
the Microsoft.Testing.Platform runner (1.3.1). CI installs that SDK, checks root-invoked
`dotnet --version` and `dotnet --list-runtimes`, and fails if the approved SDK or net10
runtime differs.

Build policy: nullable enabled, deterministic output, warnings as errors, recommended
analyzers, documentation generation, CS1591 errors for handwritten public APIs,
`ContinuousIntegrationBuild` only in CI, and committed lock files restored with
`--locked-mode`. Generated code is marked generated and excluded from handwritten API/docs
analyzers by exact paths, not by disabling warnings across the project. Prove generated code
still compiles without blanket suppression of runtime compatibility warnings.

### 3.2 Layers and the two wire seams

Dependencies flow from facade -> converters/transport/lifecycle -> generated wire/runtime.
Domain objects never implement HTTP or gRPC calls themselves.

- `ITransport.UnaryAsync<TRequest,TResponse>` receives a validated method descriptor,
  request, absolute deadline, and caller token. It returns a wire response or mapped
  transport/service failure.
- Maintain one descriptor-derived method registry linking full RPC names, CLR request and
  response types, parsers, and unary gRPC methods. Generate it from the same schema using a
  small tool only if the runtime descriptors cannot supply it cleanly. Never hand-maintain
  a second 31-case dispatch table disconnected from descriptor tests.
- gRPC uses the channel's `CallInvoker` and typed unary methods from that registry; the
  generated client stub remains public protocol code, but the facade does not return a
  configured stub/channel. Connect uses the same registry and message parsers.
- `Connection.CallAsync<TRequest,TResponse>(string method, TRequest request, CancellationToken)`
  is the deliberate low-level escape hatch. It validates method/type pairing locally and
  uses the selected transport, headers, timeout, error mapping, and disposed-state checks.
  It bypasses **typed capability and model-result interpretation**; document that explicitly.
  It does not bypass server-side validation. Invalid pairings raise `InvalidRequestException`
  without sending a request.
- Public wire messages and generated stubs are mutable and explicitly outside the immutable
  domain contract. `result.Wire`, where supplied on RPC result wrappers, returns a deep clone
  on every access. A private retained clone is never exposed. Domain snapshots are fully
  decoded and cannot change through a wire mutation.
- `PublicApiTests` inspect handwritten facade signatures only, allowing wire types precisely
  in `Wire` getters and `CallAsync`. They separately verify generated namespace boundaries.
  Generated APIs are not duplicated in the handwritten shipped/unshipped API baseline.
- Options defensively copy collections at construction/open. No user-supplied mutable
  dictionary or wire message may become shared internal state by aliasing.

### 3.3 Values and immutable data

Use sealed domain records and the minimum collection wrappers that preserve semantics:

| Shape | Equality, hashing, and ordering contract |
| --- | --- |
| ValueList<T> | Defensive immutable copy; sequence equality/hash; default equals empty; IReadOnlyList<T> |
| ValueMap<TKey,TValue> | Defensive copy; key/value equality independent of insertion order; ordinal string keys; deterministic rendering by ordinal key |
| Set value | Unique members and order-independent equality/hash; retain deterministic service order for rendering, not equality |
| Numeric values | int64, BigInteger, double, normalized Rational, ComplexValue; never convert exact numbers through double |
| Semantic references | Function identity follows calc/self IDs; metaobject identity follows element ID, as specified by the schema; descriptive fields do not redefine identity |

If all public map needs can use an existing structurally equal immutable abstraction, reuse
it instead of adding a wrapper. Do not expose mutable arrays or collection backing storage.

The 23 Value arms are Integer, BigInteger, Real, Rational, Complex, Boolean, Text,
InstanceRef, EnumLiteral, Sequence, Set, Quantity, Array, Vector, VectorQuantity,
TensorQuantity, MeasurementRef, Function, Metaobject, Null, Unset, Undetermined, Infinity.
Quantity magnitudes are closed over Integer/BigInteger/Rational/Real. Array dimensions and
row-major elements must agree; index rank/bounds are checked. Preserve NaN, signed zero,
and real infinities separately from the nonnumeric Infinity marker.

Use nested sealed derived records with constructor/copy-constructor accessibility that
prevents external derivation. `ValueKind` is a discriminator, **not a compiler exhaustiveness
guarantee**. Reflection tests enumerate all concrete variants and require one discriminator,
encoder, decoder, and display test each; an external consumer compile test rejects derivation.

Canonicalize caller-created rationals (positive denominator, lowest terms, nonzero
denominator). Inbound rational/big-integer validation follows the schema's canonical wire
rules, including the rational-versus-exact-double distinction. Reject malformed responses
with a typed protocol/transport error rather than silently repairing service corruption.
Define equality/hash for NaN and signed zero consistently using the chosen .NET numeric
semantics and pin them with tests.

Always decode known response fields even when their population capability is absent.
Unknown numeric enum values must remain representable; Google.Protobuf preserves them in
both binary and JSON (1.3.2). Connect/JSON parses with `IgnoreUnknownFields(true)`, so unknown
fields are ignored and an unknown **named** enum value is dropped and reads as the zero value.
Document that limitation, and never claim such values are preserved; the strict parser would
instead reject the whole response from a newer server.
An absent/unrecognized Value oneof must not become Null: raise `UnsupportedValueException`
identifying response decoding; retain original wire bytes/message on the enclosing error
where available. Do not claim unknown variants are losslessly wrapped.

Validate sendability and capability gates at every nested argument depth before network I/O.
Unset and Undetermined are response-only. Use the schema/current client gate predicates,
including structured, measurement, function, metaobject, big-integer, rational, set and tensor
cases where applicable; do not assume every response-population capability is a request gate.

### 3.4 Connection options and transport policy

Use immutable `ConnectionOptions` with Address, Protocol, Encoding, RequestTimeout,
StartupTimeout, ShutdownTimeout, Headers, RequireCapabilities, Version, Logger,
HttpMessageHandler, BinaryPath, ExpectedBinarySha256, AutoStart, IsolatedService,
DownloadVersion, GithubRepo, and explicit message-size options.

- Defaults: gRPC/protobuf; request timeout 60 s, startup timeout 30 s, shutdown grace 5 s.
  Timeouts and size limits must be positive; reject invalid inputs before starting work.
  `AutoStart=true` from the first public API. Until lifecycle lands, no address produces a
  documented `ServiceStartException` explaining the incomplete subset; do not flip defaults
  between phases.
- Address resolution: explicit option -> `OPENSYSML_SERVICE` -> private child when AutoStart.
  Explicit/remote addresses never trigger downloads or child creation. `AutoStart=false`
  without an address fails locally.
- Bare host:port becomes `http://host:port`; accept explicit http/https and IPv6. Reject
  user-info, fragments, query strings, and non-root base paths unless a separately tested
  proxy-prefix contract is added. Never relax TLS certificate validation.
- A connection owns exactly one channel/HttpClient. Different connections may share a child,
  not transport settings or cancellation tokens. Set `PooledConnectionLifetime` to 5 minutes.
- The connection owns created handlers. An injected handler is borrowed: create its client
  with handler disposal disabled, dispose the client/channel, and leave the handler alive.
  Tests cover both transports and ensure one connection cannot dispose another's handler.
- User headers are copied and accept auth/custom metadata. Reject attempts to override
  protocol-controlled content type, encoding, timeout, Host, or gRPC/Connect status headers.
  Define case-insensitive reserved-header matching; redact sensitive values from logs.
- gRPC uses HTTP/2/h2c and native status/deadline handling. Connect unary uses HTTP/1.1,
  POST `/sysml.SysMLService/<Method>`, `Connect-Protocol-Version: 1`,
  `application/proto` or `application/json`, `Connect-Timeout-Ms`, and gzip responses.
  The service does not cut a CPU-bound handler short at `Connect-Timeout-Ms` (1.3.2), so the
  client enforces the same deadline locally and never waits on the server to honor it.
- Both transports default to 512 MiB serialized request and decoded response limits,
  explicitly overriding gRPC's smaller receive default. Bound compressed and decompressed
  response reads; test just-below/at/above limits with smaller configured limits.
  Grpc.Net.Client's `MaxReceiveMessageSize` bounds only the compressed frame (1.3.2), so the
  gRPC channel sets `CompressionProviders` to empty (identity only) unless a provider that
  bounds decompressed size is implemented and tested.
  Download binary limits are separate. Do not preallocate buffers of the maximum size.
- No automatic retries, hedging, replay after disconnect, or protocol fallback. A lost
  response does not prove the service did not execute an edit/record/import request.

### 3.5 Errors, cancellation, and capability validation

All library-specific failures derive from `OpenSysMLException`; caller cancellation and
standard disposed-object errors retain normal .NET types. Metamodel exceptions are separate.

| Condition | Result |
| --- | --- |
| Invalid local options, invalid raw method/type pairing | InvalidRequestException, before send |
| Missing request capability | CapabilityException(Capability, Remedy), before send |
| Caller token canceled when handling cancellation/deadline | Unwrapped OperationCanceledException carrying that token |
| Local request timeout or remote DeadlineExceeded, without caller cancellation | TransportException(StatusCode.DeadlineExceeded) |
| DNS/refused socket/TLS/HTTP negotiation, I/O disconnect, invalid/truncated response, size limit | TransportException with underlying cause and canonical status where available |
| Well-formed non-OK service status, including service Unavailable | ServiceException(StatusCode), with model/file-not-found specializations where applicable |
| Successful RPC with model error/failure fields | ModelException subtype preserving failure reason, diagnostics and any partial result |
| False decided verdict | Ordinary result with Decided=true, never an exception |
| New call after disposal begins | ObjectDisposedException |

Caller cancellation wins if both cancellation sources are observed fired when mapping a
cancellation/deadline result. A completed successful response is not retroactively changed
to cancellation. Tests synchronize races rather than depending on sleeps.

Use gRPC status plus structured channel/debug/inner-exception evidence measured in the
decision spike (1.3.2). Keep `ThrowOperationCanceledOnCancellation` off: with it, a deadline
also surfaces as `TaskCanceledException`. Instead, map `RpcException(Cancelled)` to
`OperationCanceledException(callerToken)` when the caller's token is canceled.
Inspect the complete cause chain, not just one inner exception. Do not claim
a service was never reached based on an IOException. Where origin cannot be distinguished,
preserve the canonical status and state uncertainty; never retry. Connect uses its canonical
error envelope (JSON even for protobuf requests) or the specification's HTTP-status fallback
for non-Connect bodies, retaining a bounded sanitized diagnostic excerpt. HTTP/1.1-only gRPC
failure surfaces as `Internal` whose debug chain holds `HttpProtocolException`
(PROTOCOL_ERROR); name Protocol.Connect as a remedy only for that proven chain, never for
`Unavailable` or a connection reset.

Exception families retain the original planned payloads:
ServiceException -> ModelNotFoundException/ModelFileNotFoundException;
ModelException -> SymbolNotFoundException(Name, Suggestions), ModelErrorsException(Diagnostics),
ConversionException, MigrationException, AnalysisException(Partial),
EditException(Failure, Referrers), ExecutionException(Reason, Diagnostics, Trace, TraceDropped),
WrongKindException, QueryException; plus TransportException, CapabilityException,
UnsupportedValueException, ServiceStartException -> StaleServiceException/BinaryNotFoundException/
UnsupportedPlatformException, ChecksumMismatchException, UnpinnedReleaseException,
UnsignedReleaseException, ManifestSignatureException, UnwritableException.
Add new operation-specific subclasses only when a distinct actionable payload requires one.

Request validation order is stable: option shape -> RPC gate -> option gates in declared
order -> nested values in deterministic traversal order -> send. Edit ordering is refined
in its package. RequireCapabilities and an optional exact expected service Version are
checked after one GetServerInfo at open; version is not a capability proxy.

### 3.6 Ownership, registry, and lifecycle state machine

An ordinary Model borrows its Connection and never stops it. One-shot
`OpenSysMLClient.LoadAsync/LoadContentAsync` returns an owning model; disposing it closes its
private connection exactly once. Parse failures after connection acquisition release that
connection. Domain handles are immutable; edits/imports/recordings return new model handles,
never mutate an old model's hash or cached data.

Registry scope is the ALC containing the client assembly, not the caller's ALC. If a plugin
shares the library loaded in the default ALC, it shares that registry. Tests must load
distinct library copies into collectible ALCs to prove separate scope.

There is one shared child **per service identity per library ALC**, not one global child
regardless of incompatible options. Identity comprises canonical executable/digest identity
and immutable startup configuration, including the expected service version. Protocol,
encoding, headers, logger, per-call timeout, and handler are connection-local and do not
split a child. Download/cache selection and pin verification finish before publishing its
service identity. IsolatedService always opts out. Document this refinement consistently.

State transitions:

```text
registry entry: absent -> starting -> ready -> stopping -> absent
                               \-> failed -> absent
connection: opening -> open -> disposing -> disposed
```

- Concurrent opens for the same identity await one startup task; never hold a monitor across
  await. One canceled waiter releases its reservation without canceling other waiters.
  Cancel startup when no waiters/owners remain. Failed startup drains output, stops the child,
  removes the entry, and gives all remaining waiters the same actionable cause.
- A connection claims one lease only after startup and its own handshake checks succeed.
  Cancellation or handshake failure rolls back that reservation. Closing releases once.
- First disposal rejects new calls, cancels this connection's active calls, drains them and
  closes its transport, then releases the child lease. Other connections remain usable.
- Last release closes stdin for orderly child exit; after the grace period terminate only
  the owned PID/process tree, await exit and drain pumps. Never stop an external service.
- `StopSharedServicesAsync` invalidates current leases and stops all entries in this registry.
  Existing connections subsequently fail, never silently attach to a replacement child.
  A later explicit OpenAsync may create a fresh entry after stopping completes.
- Unexpected child death faults its entry and active calls; later use fails clearly and is
  not automatically restarted. A new explicit open may start a new child.
- DisposeAsync is idempotent and nonthrowing, including races. Shutdown failures are logged
  with operation/PID/cause and tested; do not swallow failures silently. An explicit
  StopSharedServicesAsync surfaces a typed shutdown failure after attempting all entries.
- Keep unload/process-exit handlers bounded. Unregister static ProcessExit handlers on
  orderly teardown, and retain no default-ALC references to collectible client state.
  Unload cleanup is best-effort; kernel EOF is the orphan guarantee.

The child argv is `-port 0 -health-port 0 -report-address -exit-with-parent`. Parse only
the first stdout line as its address; continuously drain both stdout/stderr thereafter
into bounded logging/tails. Never choose/probe/retry a port. Startup malformed output,
early exit, timeout, and cancellation must leave no child or blocked pump.

Local resolution: BinaryPath -> OPENSYSML_GRPC_BINARY -> `~/.opensysml/bin/sysml-grpc[.exe]`
-> PATH. An explicitly selected missing binary fails rather than silently choosing another.
ExpectedBinarySha256 is checked before exec. List searched locations on failure.
Before downloads land, missing local binary reports the unavailable download subset.
Browser spawning is unsupported and fails explicitly; browser transport compatibility is
not part of this release's support claim.

### 3.7 Cache transaction, downloads, and trust

Use the exact shared paths/metadata format of the
[Python binary implementation](../../../client/python/opensysml/binary.py),
[Java binary implementation](../../../client/java/opensysml-client/src/main/java/org/openmbee/opensysml/internal/),
and [Rust binary implementation](../../../client/rust/opensysml/src/binary.rs):
`sysml-grpc.json` with version/sha256/repo, the executable, and digest-named executable
`sysml-grpc-<16 hex>`. Read their actual lock path/range before coding; do not invent a
parallel cache layout or lock.

Use the cross-client primitive proven in `decisions` (1.3.3): on POSIX, libc `lockf(F_LOCK)`
on a descriptor from libc `open(path, O_RDWR | O_CLOEXEC)` (file created beforehand with
`FileMode.CreateNew`); on Windows, an exclusive whole-range `LockFileEx`. Never take the
cross-client lock through a `FileStream`/`File.OpenHandle` handle on POSIX: .NET's implicit
`flock` on that handle conflicts with the other clients' fcntl locks on macOS and deadlocks
a blocking `F_LOCK`. FileShare.None/flock is **not** a substitute for the shared POSIX fcntl
lock, and `FileStream.Lock` is unsupported on macOS and non-blocking elsewhere.

fcntl locks are process-scoped. An ALC-local SemaphoreSlim is insufficient for multiple
copies of this library in one process. Serialize .NET cache transactions across ALCs with
a separate persistent `.dotnet` guard file beside the existing shared lock, held with libc
`flock(LOCK_EX)` on a natively opened descriptor on POSIX. Do not use `FileShare.None`, which
`DOTNET_SYSTEM_IO_DISABLEFILELOCKING=1` silently disables. On Windows hold it with
`FileShare.None`; `LockFileEx` is already handle-scoped. Acquire guard first, then
the existing cross-client lock; release in reverse order. The guard is not the inter-client
lock and is never unlinked while in use. Prove same-process/distinct-ALC exclusion and
crash recovery. Do not open another descriptor to the fcntl lock file while holding it:
closing any descriptor to that file can release process locks.

Hold the lock across recheck, verified replacement, metadata update and digest snapshot
creation. Use temp files in the cache filesystem, atomic per-file moves, and crash-consistent
recovery: metadata is advisory and its digest must match the executable before reuse.
Never trust a torn executable/metadata pair after interruption. Prefer a hard link for the
digest snapshot; use supported platform APIs established by the SDK spike, or copy with a
warning. Never assume a particular File.CreateHardLink API exists on a TFM.

Version precedence: DownloadVersion -> OPENSYSML_GRPC_VERSION -> built-against release.
Resolve `latest` once through the GitHub API. Repo option -> OPENSYSML_GITHUB_REPO ->
Open-MBEE/OpenSysML. Supported assets: linux amd64/arm64, darwin amd64/arm64, windows amd64.
Other pairs fail explicitly. Downloads have 15 s **idle**, not total, read timeout;
512 MiB binary, 8 MiB metadata/bundle limits; bounded reads including missing/false
Content-Length and compressed bodies. Validate every redirect; HTTPS must never downgrade
to HTTP. HTTP is permitted only for an injected loopback test fixture endpoint.

| Cache/trust observation | Action |
| --- | --- |
| Metadata matches requested repo/version and executable digest, without contradicting a committed asset pin | Reuse verified cache/snapshot |
| Cache is another repo/version | Warn and replace only after new bytes verify |
| Executable without metadata | Preserve as hand-installed; still enforce explicit expected digest/version |
| Network/unavailable release or missing verification material, with a working cache | Warn and keep it only if caller's explicit expected version/digest constraints still hold |
| Candidate contradicts a pin or signature, including wrong signer | Fail; no success-shaped fallback to cache |
| No usable cache and no accepted verification path | Fail with the applicable typed trust/download error |

Trust order: matching repository/version/asset committed pin -> verified SHA256SUMS manifest
and bundle from an allowed signer -> refuse. Signed-manifest support may be absent only
under the recorded pins-only decision. An explicit digest is enforced even when another
verification path succeeds. A digest-only cache shortcut applies only when no release was
explicitly requested; an explicit tag or `latest` and a required digest must both hold.
Recorded cached releases are checked against available pins before reuse, including offline
fallback and unlocked-cache reads. The sole unpinned opt-out is
`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=1|owner/repo,...`, with the existing same-origin rules
and a warning; an opt-out must not excuse a contradictory supplied pin/signature.

Signer policy must match
[Java SignedManifest](../../../client/java/opensysml-client/src/main/java/org/openmbee/opensysml/internal/SignedManifest.java)
and [Python signing](../../../client/python/opensysml/signing.py):
issuer `https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62`;
URI SAN naming a valid pipeline-definition UUID under
`https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d/pipeline-definitions/`.
Do not accidentally require one undocumented definition or accept another project.
Verify certificate validity at authenticated signing/log time: an expired-today certificate
can be valid historically; a certificate invalid at that authenticated time must fail.
Trusted-root, transparency-log, artifact-binding and identity checks belong to the library,
not a custom verifier. Use the
[existing fixture generator](../../../client/python/scripts/make_signed_release_fixture.py).

### 3.8 File-writing contract

Model/Conversion/EditResult SaveAsync and Migration.WriteAsync do not overwrite by default.
An explicit overwrite option uses temp-write/flush/atomic-replace on the same filesystem.
Cancellation/failure removes owned temp files and preserves an existing destination.
Migration never overwrites its v1 source even with overwrite enabled.
`MigrateSource.FromFile` requires a valid, fully-qualified path in the client's path syntax,
normalizes it once, and uses that same path for the service request and source guard. It never
infers the service's working directory from the client's. For another host, use a shared
absolute path or carry the document inline.

Before writing migration assets, reject absolute paths, drive/UNC paths, `..`, invalid or
duplicate normalized destinations, and symlink/reparse-point escapes. Resolve containment
using platform path semantics; a lexical StartsWith check is insufficient. Stage assets and
content before publishing; do not claim atomic multi-file publication. Document and test
rollback of files created by a failed operation without deleting pre-existing user files.
Prefer failing safely on an unsafe destination rather than silently renaming an asset.
Path comparisons include Unicode normalization equivalence on macOS, but preserve Unicode
distinctions on Linux and Windows. Asset names reject Windows-invalid characters on every
platform, not just characters invalid on the host.

## 4. Execution protocol and validation gates

### 4.1 Work-package rules

Each package below has an outcome, prerequisites, read-first references, owned surface,
test contract, gate/negative control, non-goals and handoff. Execute prerequisites, not
numeric order. Use one focused PR per package or split it further **before** implementation
if its contract cannot be reviewed independently. Do not merge placeholder types/stubs.

Branch from origin/develop; draft PRs target develop. Use Conventional Commits, changelog
fragments, and the required co-author trailer when committing. Inspect and preserve the
[PR template](../../../.github/pull_request_template.md). Private package labels in this plan must
not appear in public docs, PR prose, changelogs or diagnostics.

Tests precede or accompany behavior. Every package updates README coverage, inventory,
handwritten public API baseline, sample where relevant, and its changelog fragment.
The integration owner merges shared-file changes; no autonomous branch rewrites another's
API baseline or renames shared contracts.

**Marking a package done (mandatory).** When a package's handoff evidence (8.1) is complete,
the same change to this document must update all three markers, and must not update them
before that evidence exists:

1. Append ` — ✅ DONE (YYYY-MM-DD)` to the package heading in section 5.
2. Make the package's first-line status bullet `**Status: ✅ DONE (YYYY-MM-DD).**`, followed
   by its handoff summary: negative controls observed, cleanup, and what was not run and why.
3. Set its row in the section 6 dependency table to `✅ DONE (YYYY-MM-DD)`.

A package in progress shows `🚧 IN PROGRESS` in the table only; one that is blocked shows
`⛔ BLOCKED` with the reason in its status bullet. Never mark a package done with
unresolved failures, skipped required gates or pending decisions. A partially delivered
package stays open and states the remaining scope.

### 4.2 Gates and exact commands

Commands run from repository root. The following are **future implementation commands**,
not validation evidence for this document revision.

**G0: prerequisites (once per environment, then fail fast in CI).**
Install approved SDK/runtimes/tool pins, restore locked dependencies, and `make build-grpc`
to produce `bin/sysml-grpc` (Windows: the equivalent native .exe). Do not claim a service
test passed because a binary/runtime was absent. Early transport tests start a fixture
service with an explicit address using the reference harness pattern; that fixture helper
is not a second production lifecycle implementation.

```bash
dotnet --version
dotnet --list-runtimes
dotnet restore client/dotnet/OpenSysML.slnx --locked-mode
make build-grpc
```

**G1: fast local/build gate (every implementation package).**

```bash
dotnet format client/dotnet/OpenSysML.slnx --verify-no-changes --no-restore
dotnet build client/dotnet/OpenSysML.slnx -c Release --no-restore -warnaserror
dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build --filter 'Category=Unit'
```

All test cases declare a category. The full gate below must prove that category assignment
does not exclude uncategorized tests. These are Microsoft.Testing.Platform commands (opted in
by root `global.json`; the VSTest-mode `dotnet test <sln>` form fails with the pinned xUnit).
For focused work use `--project <test project> --filter 'FullyQualifiedName~ClassName'`;
the package handoff records the actual selector and result.

**G2: required service integration (service packages).**

```bash
OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" OPENSYSML_REQUIRE_SERVICE=1 \
  dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build \
  --minimum-expected-tests <current total>
```

Test projects must report net10 executions; `--minimum-expected-tests` keeps a filter or
discovery fault from passing as an empty run. Every
transport-independent live-service test is a theory over grpc/connect/connect-json.
Protocol-specific tests name that restriction. Also start a grpc-only service and require
gRPC success plus explicit Connect refusal (a connection reset, 1.3.2).

**G3: shared conformance (from suite-adapters).**

```bash
make conformance-dotnet
make conformance
```

The .NET target runs all three protocols, the exact allowed skips, and a service with
strict_conformance/oslc_query withheld; writes JSON and JUnit under bin; and exits nonzero
for mismatches, unexpected skips, stale allowlists, or missing implemented coverage.
Add `make conformance-dotnet-check` to validate report schemas/totals and compare matched
scenario outcomes/statuses/normalized observations with the reference runner. Exclude
timing, client identity and reviewed unexpressible-request skips, never response fields.
Reference verbose responses supply observations without changing the reference schema.

**G4: generated protocol and repository plumbing.**

Run protocol regeneration/diff from `wire` onward; scaffold runs the remaining commands.
Packages must not invoke a not-yet-created target or replace it with a success-shaped no-op.

```bash
make proto-csharp
git diff --exit-code -- client/dotnet/src/OpenMBEE.OpenSysML/Wire
python3 scripts/sync-release-digests.py --check
python3 scripts/changelog.py check
bash scripts/ci-changed-areas-test.sh
```

The wire package also regenerates existing Go output before/after introducing the separate
template and proves no unrelated descriptor/stub changes. A changed tool runs its own tests.

**G5: platform/lifecycle/cache.** G2 on native Linux, macOS and Windows runners; lifecycle,
orphan, ALC and lock interop tests are mandatory there. CI installs Python, Java and Rust
lock-holder prerequisites. No "Java when available" skip in the required matrix. A developer
without a prerequisite may get a named skip unless OPENSYSML_REQUIRE_INTEROP=1; CI sets it.
Use shell-native environment syntax on Windows, not the POSIX prefixes above.

**G6: final delivery.**

```bash
make dotnet-test
make dotnet-coverage
make conformance-dotnet-check
make docs-check
python3 scripts/check-doc-ids.py
go build ./...
go vet ./...
gofmt -l .
go test ./...
```

`dotnet-test` composes G1/G2/G4, runs API/inventory guards, and asserts the net10 results.
`dotnet-coverage` uses the scaffold-proven collector and emits Cobertura plus test reports;
no invented percentage threshold. Fail missing/empty coverage and classify generated code
exclusions explicitly. Keep required platform jobs separate in the aggregate.
Run the documented MkDocs site build in the docs environment as well.

Go changes also run the smallest owning package tests immediately, before the final Go gate.
For self-model/RDF changes, provision the training and pilot corpora first, then run
`make self-model` and:

```bash
OPENSYSML_REQUIRE_TRAINING_CORPUS=1 OPENSYSML_REQUIRE_PILOT_CORPORA=1 \
  go test -count=1 ./tests/corpus -run TestCorpusRoundTrip
```

Adjudicate movements before any baseline regeneration. Never weaken/skip an existing gate.
Handoffs distinguish passed, failed, and not run, with exact commands, TFMs, OSes and reasons.

## 5. Bounded work packages

Address packages by their stable two-digit ID, for example: **"Implement package 07"** means
`model`. The names remain shorthand in narrative references and the dependency diagram.
IDs identify scope, not execution order: satisfy the prerequisite IDs in section 6 first.
Do not renumber existing packages when inserting or splitting work; assign new IDs after 25.

Completion status is tracked in the Status column of the section 6 dependency table and in
each package's heading; see "Marking a package done" in 4.1. A package without the ✅ DONE
marker is not complete, whatever its branch state.

### Package 01: decisions — ✅ DONE (2026-10-21)

- **Status: ✅ DONE (2026-10-21).** Every 1.2 row is resolved, with evidence in 1.3. Negative
  controls observed failing: a foreign lock held (BUSY on every platform), a tampered manifest
  (rejected by Sigstore 1.1.1 and Python), an inheritable stdin handle (child orphaned on all
  three OSes), plus a changed pin, a format violation and an undeclared public API (1.3.1).
  Scratch probes, the scratch SDK/JDK and the disposable fork branch/run were deleted. Not
  run: AOT on x64 RIDs, and transport probes on Linux/Windows; the latter are covered by G2/G3.
- **Outcome/prerequisites:** settle section 1.2 against the baseline; no code prerequisites.
- **Read first:** signing/fixture references in 3.7, binary implementations, transport
  evaluation in [the design note](../../../docs/internals/design/transport-evaluation.md).
- **Owned surface:** this decision register and disposable scratch probes outside the tree.
- **Tests:** verify real release plus tamper/wrong-project/wrong-issuer/invalid-at-signing-time
  fixtures and valid historically expired fixture; both-direction Python/Java/Rust locks;
  sibling-parent-kill on three OSes; h2c against both server transports; protobuf/JSON large
  Query response and eight concurrent calls; unknown field/numeric/named enum JSON behavior;
  compiler/plugin pair; SDK/xUnit/coverage; optional AOT consumer publish/run.
- **Gate/negative control:** every blocking row has commands/results and a chosen policy;
  hold a foreign lock, tamper a manifest and break a pipe inheritance case to show tests fail.
  Transport timings are observations, not performance claims without repeatable methodology.
- **Non-goals/handoff:** no production implementation or guessed dependency pins. Record
  exact versions, support decisions and failed candidates; delete owned scratch probes.

### Package 02: scaffold — ✅ DONE (2026-10-21)

- **Status: ✅ DONE (2026-10-21).** Built on baseline `a662c2bce`, uncommitted on
  `feat/dotnet-client`; no schema change. Delivered: root `global.json` (SDK 10.0.402,
  latestPatch, MTP runner); `client/dotnet/` with `OpenSysML.slnx`, `Directory.Build.props`
  (Version 0.10.1, net10.0, nullable, deterministic, warnings as errors, Recommended analyzers,
  `EnforceCodeStyleInBuild`, CS1591 as error, lock files, CI-only `ContinuousIntegrationBuild`
  and `RestoreLockedMode`), `Directory.Packages.props` (every 1.3.1 pin, transitive pinning),
  `nuget.config` (nuget.org only), `.editorconfig`, three `packages.lock.json`; the library
  with **no public API** (empty `PublicAPI.*.txt`; the analyzer package adds them itself, so
  explicit `AdditionalFiles` would duplicate them), the synced embedded `release-digests.json`
  and an internal `ReleaseDigests` reader (validating, ordinal, frozen) as the digest plumbing
  downloads consumes; the test project (43 tests: `Unit`/`Integration` categories enforced by
  `TestCategoryTests`, `ServiceGate` with a pure `Decide()` because xUnit v3 records
  `Assert.Skip` in the test context even when caught, a real `sysml-grpc -version` integration
  test); `samples/Vehicle` reading assembly metadata via `Assembly.Load`;
  `client/dotnet/scripts/dotnet-gates.py` (exact-SDK/net10-runtime preflight; Cobertura check:
  nonempty, only `client/dotnet/src/`, no generated path) with 18 self-tests; Make
  `dotnet-preflight/build/test/coverage` (`DOTNET_MIN_TESTS := 43`, `DOTNET_GENERATED` the single
  list of Wire paths excluded from coverage); `check_version.py --dotnet` plus 15 pytest cases;
  digest copy in `sync-release-digests.py`; `dotnet` CI area (`client/dotnet/`, `global.json`;
  `Directory.Build.props` also triggers python) with routing cases; PR job `dotnet-test`
  (setup-dotnet into `runner.temp` so only the pin exists, service required, coverage, reports
  uploaded) in the aggregate; CircleCI `dotnet-executor`
  (`sdk@sha256:0d99eb…`, = `10.0.402-noble`) and `dotnet-test` in build-test only; Sonar
  excludes `client/dotnet/**` with its reason; `.gitignore`; README; changelog fragment.
  Passed (macOS arm64, net10.0, runtime 10.0.13): G0, G1 (format, 0-warning build, Unit 42/42),
  G2 43/43 with `--minimum-expected-tests 43`, `make dotnet-test`, `make dotnet-coverage`
  (53/53 lines of handwritten code), G4 minus wire's `proto-csharp`, routing test, doc-ids,
  pytest 98/98, `go test ./tests/hygiene/... ./examples/`. Also passed on linux/arm64 in the
  pinned image with `CI=true` (the CircleCI steps, service built `CGO_ENABLED=0`).
  Negative controls observed failing, then restored: undocumented public type (CS1591+RS0016),
  undeclared public type (RS0016), unused using (IDE0005), whitespace (format exit 2), changed
  pin (NU1004), wrong SDK in global.json (`dotnet --version` 155; `make dotnet-preflight`
  fails), absent net10 runtime and wrong SDK (gate self-tests), required service without a
  binary (fails on macOS and Linux), `--minimum-expected-tests 44` (exit 9), version drift
  (2 pytest failures + CLI exit 1), digest drift (sync exit 1 + 2 C# failures), uncategorized
  test (guard fails; the Unit filter alone silently dropped it), `global.json` unclaimed by the
  dotnet area (routing test fails). Cleanup: scratch venv/tarball/actionlint/backups deleted;
  no processes left. Not run: hosted GitHub Actions and CircleCI (no PR opened; the
  `setup-dotnet` step is unexercised), linux-x64, Windows; `make proto-csharp` (wire's).
  Downstream: raise `DOTNET_MIN_TESTS` with every added test; wire adds the exact-path
  `generated_code` `.editorconfig` section and real `Wire/` files under `DOTNET_GENERATED`;
  the Homebrew Python here has a protobuf runtime older than the committed gencode, so run the
  Python suite in a venv.
- **Outcome/prerequisites:** root-invoked, net10, CI-gated checkout builds; decisions.
- **Read first:** version checker in [Python scripts](../../../client/python/scripts/), current
  [CI routing](../../../scripts/ci-changed-areas.sh), [PR workflow](../../../.github/workflows/pr.yml).
- **Owned surface:** root global.json, solution/build/package/lock files, main/test projects,
  minimal Vehicle ProjectReference build consumer, .gitignore, initial README, version/digest plumbing,
  CI Linux skeleton and shared Make targets.
- **Tests:** root SDK selection; net10 test executions; xUnit skips fail when service
  required; nonempty coverlet.MTP Cobertura output excluding generated files by path;
  handwritten API analyzer/documentation policy;
  ProjectReference consumer; version checker --dotnet and its Python tests.
- **Gate/negative control:** G0/G1/G4; an absent net10 runtime and a wrong SDK must fail preflight;
  version/digest drift and a missing public XML comment must fail their own tests.
- **Non-goals/handoff:** no transport implementation, empty future projects, or publication.
  Deliver tool/version pins and runnable gates. The initial consumer reads assembly metadata
  without depending on an unimplemented API; grpc adds server info and model adds the vehicle
  parse. Never merge stub methods just to compile a future sample.

### Package 03: wire — ✅ DONE (2026-10-21)

- **Status: ✅ DONE (2026-10-21).** Built on baseline `a662c2bce` plus the scaffold commit
  `bdb44e4fd`, uncommitted on `feat/dotnet-client`; no schema change. Delivered: separate
  `api/proto/buf.gen.csharp.yaml` (managed mode, only `csharp_namespace =
  OpenMBEE.OpenSysML.Wire`, remote `protocolbuffers/csharp:v36.2` and `grpc/csharp:v1.84.0`
  `no_server`); `make proto-csharp` (in `proto` and `.PHONY`); committed `Wire/Sysml.cs`
  (2,561,680 B) and `Wire/SysmlGrpc.cs` (161,429 B), `linguist-generated`; library references
  Google.Protobuf and Grpc.Net.Client (Logging.Abstractions lifted to the central 10.0.13 by
  transitive pinning; three lock files updated); exact-path `.editorconfig` section
  (`generated_code`, RS0016/RS0041 `none`), no other suppression; internal
  `Internal/RpcMethods.cs` built from `SysMLService.Descriptor` (no hand table): `All` in schema
  order, `Find` (bare `ParseFile` or `sysml.SysMLService/ParseFile` only, ordinal),
  `TryResolve<TReq,TResp>` returning a mismatch/unknown reason instead of throwing (grpc maps it
  to `InvalidRequestException`), `RpcMethod.QualifiedName`/types/parsers, and one shared
  `RpcMethod<TReq,TResp>` per method (gRPC caches by `Method` object) with contextual buffer
  marshallers; `client/dotnet/test/testdata/parity.json` (86,543 B): 31 rows (types, public
  methods, input/result, area, `deferred`, server-ordered gates with field and predicate,
  `valueFields`, all 194 scenario IDs), 23 Value arms (send: ungated/response-only/capability;
  receive capability; `sendGateOrder`), 17 EditOperation arms (gates with predicates,
  `gateOrder`), 47 response-population field entries, and all 995 fields of the 175 reachable
  messages as `deferred`. Gates were read from `internal/frontend/grpc` (`requireCapability`,
  `requireValueCapabilities`, `schedulePolicy`, `engineSelection`, `verificationQuestion`,
  `checkArgs`, `requests*Authoring`); notable: RunAnalysis checks schedule before
  verification, RecordRun needs only record_run (+engine), RunDocumentQuery big-int/rational
  binding gates run after model lookup, ParseSources' inline_language is per document.
  Tests (50 new, all `Unit`; 93 total): `ParityInventoryTests` (committed inventory clean;
  synthetic added/renamed RPC, changed request type, added/renamed field, added message,
  added/renamed Value arm, added edit arm, new request Value field; stale/unlisted/unknown
  scenario; scenario capability not covered; unknown capability, undeclared gate field,
  conditional gate without field; arm gate order; duplicate/reordered rows; implemented without
  or with phantom tests; field classification rules; population entries; unknown, repeated and
  missing JSON members), `RpcMethodsTests` (descriptor order, agreement with the generated
  stub's private `Method` fields, spellings, null, shared typed instance incl. concurrent,
  mismatch messages, every method resolves, contextual marshaller round trip, unknown fields
  preserved), `PublicApiBoundaryTests` (handwritten exports in baseline, no Wire symbol in
  baseline, every public Wire type generated, csharp_namespace, Wire dir holds exactly the two
  auto-generated files, no handwritten `namespace`/`using OpenMBEE.OpenSysML.Wire`, Makefile
  `DOTNET_GENERATED` and the `.editorconfig` section name exactly those files). Capability names
  are read from the service's `const CapabilityX` declarations; scenarios from
  `conformance/scenarios`. Also fixed `dotnet-gates.py coverage`: it read Cobertura `filename`
  alone, which broke once coverlet factored two files' common directory into `<sources>`; it
  now resolves filenames against the sources (+4 self-tests, 22 total). `DOTNET_MIN_TESTS := 93`;
  PR `dotnet-test` job installs digest-pinned buf and diffs regenerated `Wire/`; README
  "Protocol code" and "Parity inventory" sections; changelog fragment.
  Passed (macOS arm64, net10.0, runtime 10.0.13): G0; G1 (format, 0-warning build, Unit 92/92);
  G2 93/93 with `--minimum-expected-tests 93`; `make dotnet-test`; `make dotnet-coverage`
  (114/115 handwritten lines); G4: `make proto-csharp` from deleted files byte-identical
  (sha256), `make proto-buf` leaves `api/proto` unchanged, sync-release-digests `--check`,
  `changelog.py check`, `ci-changed-areas-test.sh`, `check-doc-ids.py`; `go test
  ./tests/hygiene/...`; workflow YAML parses. Negative controls observed failing, then
  restored: a real `sysml.proto` change (new RPC, request field, Value arm, regenerated) fails
  the committed-inventory test with exactly those four problems while the registry picks up the
  RPC unaided; removing the `.editorconfig` section (11,160 RS0016 + 6,544 RS0041, as the
  spike); importing the Wire namespace in handwritten code; dropping a scenario from a row; the
  old coverage-gate resolution (3 self-test failures). Cleanup: scratch descriptor image,
  generator and backups under `/tmp/wire03` deleted; no processes started. Not run: hosted
  GitHub Actions/CircleCI (the new buf step is unexercised; CircleCI has no drift step, as for
  the other clients' stubs), Linux/Windows, whole `make proto` (Python/TypeScript generators
  need grpcio-tools/npm), and live withheld-capability calls (no transport yet; gates are
  cross-checked statically against every scenario's `requires_capabilities`).
  Downstream: grpc calls `RpcMethods.TryResolve` in `CallAsync` and throws
  `InvalidRequestException(problem)` before send, dispatches `RpcMethod<,>.Grpc` on the
  channel's `CallInvoker`; connect posts to `/` + `QualifiedName` with the same parsers. Each
  service package flips its rows/arms to `implemented` with `tests`, and its fields to
  `typed`/`raw-only`/`unsupported` with reason and test, in the same change; raise
  `DOTNET_MIN_TESTS` with every added test.
- **Outcome/prerequisites:** reproducible protocol classes/registry and exhaustive inventory;
  scaffold.
- **Read first:** schema, [buf template](../../../api/proto/buf.gen.yaml), reference runner descriptors,
  Java/Rust mappings; reconcile every conditional gate in section 2.
- **Owned surface:** separate buf.gen.csharp.yaml, committed Wire output, method registry,
  parity.json and inventory/API boundary tests; proto-csharp integration.
- **Tests:** all 31 RPC descriptors and field paths; 23/17 arm classification; method/type
  mismatch rejection; generated mutable types excluded only from handwritten API checks.
- **Gate/negative control:** G1/G4; repeat generation is byte-identical; synthetic added/
  renamed RPC, field and arm fail classification. Existing Go stubs remain byte-identical.
- **Non-goals/handoff:** no handwritten edits to generated files, no production RPC bodies.
  Supply complete typed method/options/result and gate contracts to later package owners.

### Package 04: grpc — ✅ DONE (2026-10-21)

- **Status: ✅ DONE (2026-10-21).** Built on baseline `a662c2bce` plus scaffold `bdb44e4fd` and
  wire `7c1e0554d`, uncommitted on `feat/dotnet-client`; no schema change. Delivered public API
  (all in `PublicAPI.Unshipped.txt`, 180 lines, generated by the RS0016 code fix):
  `Connection : IAsyncDisposable` with `static OpenAsync(ConnectionOptions? options = null,
  CancellationToken = default)` (one overload: RS0026 refuses two with optional parameters, so
  null means `ConnectionOptions.Default`), `Address` (`Uri`), `Protocol`, `ServerInfo`,
  `Capabilities`, `CallAsync<TRequest,TResponse>(string, TRequest, CancellationToken)`,
  `DisposeAsync`; sealed `ConnectionOptions` (init-only: `Address`, `Protocol`, `AutoStart`=true,
  `RequestTimeout`=60 s, `Headers`, `RequireCapabilities`, `Version`, `Logger`=NullLogger,
  `HttpMessageHandler`, `MaxRequestSize`/`MaxResponseSize`=512 MiB; `ServiceVariable`,
  `DefaultMaxMessageSize`, `Default`, `DefaultRequestTimeout`, `MaxRequestTimeout`; collections
  copied at init, everything else validated at open as `InvalidRequestException`); `Protocol`
  { `Grpc` }; `StatusCode` (gRPC numbering, test-pinned to `Grpc.Core.StatusCode`); sealed
  `ServerInfo` (`Version`, `Capabilities`, `Answered`; value equality); sealed `Capabilities`
  (73 constants generated from the service's `const CapabilityX` declarations with their Go
  docs, `Names`, `Has`, `Require`, `None`, `UpgradeRemedy`; set equality); exceptions
  `OpenSysMLException`, `ServiceException(StatusCode, ServiceMessage, Method)`,
  `TransportException(StatusCode, Method)`, `CapabilityException(Capability, Remedy)`,
  `InvalidRequestException`, `ServiceStartException`, `StaleServiceException(Address, Reason,
  Remedy, ServiceVersion)`. Internal: `ITransport` (`Protocol`, `UnaryAsync(method, request,
  absolute UTC deadline, token)`), `GrpcTransport` (one `GrpcChannel`, identity-only
  `CompressionProviders`, no service config/retries, `ThrowOperationCanceledOnCancellation`
  off, owned `SocketsHttpHandler` with 5-minute pooled lifetime or a borrowed handler behind a
  non-disposing `HttpMessageInvoker`), `RequestHeaders`, `ServiceAddress`. Status provenance:
  a `DelegatingHandler` records each call's `HttpResponseMessage` through an `AsyncLocal`
  observation; a status is the service's only when the response's own `grpc-status` (headers
  or trailers) equals it and the library attached no `DebugException`, otherwise transport.
  DeadlineExceeded is always `TransportException`; caller cancellation wins at both the
  transport and connection layers; disposal cancels in-flight calls as
  `ObjectDisposedException`. Two status corrections to gRPC's own table, decided on structured
  evidence only: an `InvalidProtocolBufferException` in the cause is `Internal` (the library
  says Unavailable), and an `HttpRequestException` whose `HttpRequestError` is name
  resolution, connection, TLS or proxy tunnel is `Unavailable` (the library says Internal for a
  failed TLS handshake). The library's own statuses are kept for a non-numeric `grpc-status`
  (Internal) and a missing one (Cancelled; still a `TransportException`, never an OCE without
  the caller's token). A service answering GetServerInfo `Unimplemented` gives
  `Answered=false`, `Capabilities.None` and a warning, as the Python and Java clients do. No
  service named: `ServiceStartException` saying the private service is not available yet
  (AutoStart true) or that AutoStart is false. Sample: prints identity and, with
  `OPENSYSML_SERVICE` set, `ServerInfo`; `dotnet-gates.py sample` (+9 self-tests, 31 total)
  runs it with no service and against a `-port 0 -report-address -exit-with-parent` service it
  starts and stops, under the `OPENSYSML_REQUIRE_SERVICE` rule, replacing the inline Make check.
  Inventory: GetServerInfo row `implemented` with three tests; `ServerInfoResponse.version` and
  `.capabilities` `typed`. Tests: 170 new (263 total, `DOTNET_MIN_TESTS := 263`): 248 `Unit`
  (fake gRPC handler framing real responses: every non-OK status, unknown status 99, twelve
  malformed shapes, size limits at/above with small limits both directions, headers on the
  wire, identity-only `grpc-accept-encoding`, `grpc-timeout`; real loopback sockets for refused
  port, `.invalid` DNS, non-TLS endpoint over https, HTTP/1.1-only endpoint; cancellation
  pre/mid-flight, deadline, both race orders, completed-then-cancelled; disposal idempotence,
  concurrency, in-flight cancellation, close-failure logging; borrowed versus owned handler;
  options/headers/address theories; capability constants equal to the service's declared set)
  and 15 `Integration` (each over the service's `connect` and `grpc` transports: server info vs
  `-version` and the 01-server-info scenario's capability list, env-named address, raw call
  and live NotFound, eight concurrent calls, Version/RequireCapabilities, a limit one byte under
  the measured uncompressed answer, external service alive after closing connections).
  The suite caught one real bug during development: `ConnectionOptions.Default` was
  initialized before the static default timeout it read, giving it a zero timeout.
  Passed (macOS arm64, net10.0, runtime 10.0.13, gRPC against both server transports): G0
  (`--locked-mode` restore after updating the three lock files for the direct
  Logging.Abstractions reference); G1 (format, 0-warning build, Unit 248/248); G2 263/263 with
  `--minimum-expected-tests 263`; `make dotnet-test` (incl. sample against a live service);
  `make dotnet-coverage` (521/531 handwritten lines; uncovered are the dispose/enter race,
  defensive `CancelAsync`/channel-construction catches and an impossible GetServerInfo
  resolution failure); G4 (`make proto-csharp` no drift, sync-release-digests `--check`,
  `changelog.py check`, `ci-changed-areas-test.sh`, `check-doc-ids.py`); `go test
  ./tests/hygiene/...`. Negative controls observed failing, then restored: dropping status
  provenance (23 failures: malformed shapes, network, HTTP/1.1, size, unanswered-Unimplemented);
  default compression providers (the wire-header test, and live on the `connect` transport the
  one-byte-under limit succeeds); connection-level caller-wins removed (1); borrowed handler
  owned (4); a removed capability constant (RS0017 fails the build; with warnings allowed, the
  constants test fails); version/capability checks skipped (6, incl. live); required service
  without a binary (15/15 integration fail; unrequired, 15 skip); the static-init bug above.
  Cleanup: scratch mutation script/backups and generated constant scratch deleted; fixtures stop
  every service they start (stdin EOF, then kill); `pgrep` shows no `sysml-grpc` left.
  Not run: Linux/Windows and hosted CI (the PR and CircleCI jobs run the new sample gate with the
  binary they already provide), G3 (suite-adapters), G5, AOT. Limitations: gRPC only; no
  private service; `ModelNotFoundException`/`ModelFileNotFoundException` deferred to model
  (Java derives them from message text, which section 3.5 forbids; model must find structured
  evidence per RPC). Downstream: connect adds `Protocol.Connect`, an `Encoding` option and its
  `ITransport`, reuses `RequestHeaders` (already reserving `connect-*`) and adds the
  Protocol.Connect remedy to the HTTP/2-protocol-error branch of `GrpcTransport.Map`; its
  integration tests add a client-protocol dimension to `ServiceConnectionTests`. lifecycle-local
  replaces the AutoStart branch of `Connection.ResolveAddress` and adds the lifecycle options.
  `CallAsync` throws argument, method and disposal refusals synchronously.
- **Outcome/prerequisites:** external gRPC connection, server info, capability negotiation,
  raw calls and typed errors; wire.
- **Read first:** section 3 transport/error contracts, measured spike, current service startup.
- **Owned surface:** Connection/options, ITransport/GrpcTransport, exceptions/Capabilities,
  server info converters, API baseline, explicit-address integration fixture.
- **Tests:** every canonical status; service versus network Unavailable; timeout/caller race;
  refused DNS/socket/TLS, HTTP/1.1-only endpoint; header/size boundaries; borrowed handler;
  disposed call; RequireCapabilities/version mismatch; external service never stopped.
- **Gate/negative control:** G1/G2 with gRPC-selected tests; wrong method types send zero
  requests; fake handler malformed frames/trailers and a refused endpoint must fail.
- **Non-goals/handoff:** no private service/downloads/retries. Freeze common transport
  and error contracts; no-argument open explicitly reports not-yet-delivered lifecycle.

### Package 05: connect — ✅ DONE (2026-10-21)

- **Status: ✅ DONE (2026-10-21).** Built on baseline `a662c2bce` plus scaffold `bdb44e4fd`, wire
  `7c1e0554d` and grpc `469eed9f7` on `feat/dotnet-client`; no schema change. Delivered public API
  (7 lines added to `PublicAPI.Unshipped.txt`): `Protocol.Connect = 1`; enum `MessageEncoding`
  { `Protobuf`, `Json` } (not `Encoding`, which would be ambiguous with `System.Text.Encoding` in
  any consumer file importing `System.Text`); `ConnectionOptions.Encoding` (default `Protobuf`;
  undefined value and gRPC+JSON refused in `Validate` as `InvalidRequestException`, before a
  channel exists); `Connection.Encoding`. Internal: `ConnectTransport` (one `HttpMessageInvoker`,
  never `HttpClient`, whose 100 s default timeout would override long `RequestTimeout`s; owned
  `SocketsHttpHandler` with 5-minute pooled lifetime, `AutomaticDecompression = None`,
  `AllowAutoRedirect = false`, `UseCookies = false`; borrowed handler not disposed); HTTP/1.1 POST
  `/sysml.SysMLService/<Method>`, `application/proto|json`, `connect-protocol-version: 1`,
  `connect-timeout-ms` (ceil, clamped to 1..9,999,999,999), `accept-encoding: gzip`, user headers
  lower-cased (content headers such as `Content-Language` routed to content headers); local
  deadline via a linked timer `CancellationTokenSource`, a deadline already past sends nothing;
  `JsonBodies` (Google.Protobuf `JsonFormatter.Settings.Default`; parser `IgnoreUnknownFields(true)`;
  responses decoded with strict UTF-8); `ResponseBody` (streamed bounded read: `MaxResponseSize`
  after inflation, compressed body bounded by zlib's conservative `deflateBound` + 18-byte gzip
  wrapper, Content-Length declared over the bound refused unread, Content-Length mismatch
  refused, initial buffer ≤ min(declared, 1 MiB); **gzip trailer verified** (CRC-32 + ISIZE of the
  last 8 raw bytes) because a probe showed .NET `GZipStream` silently returns a half-truncated
  body (99,933 of 200,000 bytes) and ignores a missing trailer or trailing bytes, so multi-member
  gzip is refused, documented); `Causes` (cause-chain helpers now shared with `GrpcTransport`).
  Refusal provenance: a non-200 is a `ServiceException` only when its body is a Connect error
  (JSON object, `code` one of the 16 codes, `message` string/null/absent, `details` array/null/
  absent, whole body valid UTF-8, `application/json` with at most `charset=utf-8`), the code
  winning over the HTTP status; `deadline_exceeded` is `TransportException(DeadlineExceeded)` as
  over gRPC. Anything else (incl. unrecognized/`ok`/null/numeric codes, `{}`, non-JSON, over-limit
  or corrupt error bodies, unknown codings, 3xx) is a `TransportException` with the spec's
  HTTP-status inference and a ≤200-char control-blanked excerpt; this follows the Connect spec and
  connect-go 1.20.0 (`connectValidateUnaryResponseContentType`, `httpToCode`), not Java's
  every-non-2xx-is-the-service's rule. Consequence documented: over Connect a service too old for
  `GetServerInfo` (router's plain 404) fails to open instead of `Answered = false`. A 200 with a
  wrong type is `Internal` for another `application/*` and `Unknown` otherwise, as connect-go.
  Cause classification: name resolution/connection/TLS/proxy → `Unavailable`; invalid response,
  HTTP protocol error, version negotiation, header limits → `Internal`; `ResponseEnded` is
  `Unavailable` before headers and `Internal` while reading the body; other socket/IO →
  `Unavailable`; decode failures `Internal` and never re-labelled as cancellation. `GrpcTransport`'s
  HTTP/2-protocol-error branch now names `Protocol.Connect` (only that proven chain). JSON
  forward-compatibility limitation recorded in README, `MessageEncoding.Json` and `JsonBodies`
  docs and pinned by a test: unknown fields ignored, unknown enum *name* reads as zero, unknown
  enum *number* kept. Tests: 284 new (547 total, `DOTNET_MIN_TESTS := 547`; 514 `Unit`, 33
  `Integration`). `FakeGrpcHandler` became `FakeServiceHandler(ClientWire)` speaking all three
  wires (with `GrpcResponses`/`ConnectResponses` builders and an independent Connect code↔HTTP
  table); cancellation, disposal, raw-call, open/version/capability/logging, every-status,
  network-vs-service Unavailable, size limits at/over, owned/borrowed disposal, refused port, DNS,
  non-TLS https and silent-endpoint deadline are theories over gRPC, Connect+proto and
  Connect+JSON; new `ConnectTransportTests` (exact request incl. JSON body bytes, max timeout
  digits, gzip read, empty gzip-labelled body, gzip bomb vs decompressed limit, 17 malformed 200
  shapes, 29 non-envelope refusal shapes, excerpt bounds, envelope with details/charset/gzip/
  code-over-HTTP, over-limit refusal body falls back, stale deadline sends nothing, HTTP-status
  table, cause table, cancellation-vs-decode), `JsonBodiesTests` (int64/uint64 strings, bytes,
  oneof defaults, presence, names, enums, unknowns, −0/NaN/±∞, trailing/empty/array/null bodies,
  invalid UTF-8), `ResponseBodyTests` (incompressible body at limit fits the bound at all four
  levels, declared-over refused unread, endless empty-stored-block gzip cut at the bound, identity
  endless body cut at the limit, prefix read), Connect real-socket tests (HTTP 400 fallback,
  307 not followed and target never contacted, non-HTTP endpoint, closed before answer, cut-short
  body). Integration: every live theory over (`connect`,gRPC), (`connect`,Connect+proto),
  (`connect`,Connect+JSON), (`grpc`,gRPC); live proof the service gzips Connect answers and the
  client reads them equal to gRPC's; the over-limit test sees gzip on both Connect answers;
  the gRPC-only transport refuses Connect with `Unavailable` after exactly one HTTP/1.1 request
  (no fallback). Found and fixed during development: `JsonDocument` checks string UTF-8 lazily,
  so an envelope with invalid UTF-8 escaped as `Unknown` (now validated whole first); the raw
  TCP test server closed with unread request bytes, making the kernel send RST before the client
  read the answer (now half-closes and drains). Passed (macOS arm64, net10.0, runtime 10.0.13,
  SDK 10.0.402): G0 (locked restore unchanged, no package change); G1 (format, 0-warning build,
  Unit 514/514); G2 547/547 with `OPENSYSML_REQUIRE_SERVICE=1`, 0 skipped, four consecutive runs
  plus eight runs of the network tests; `make dotnet-test` (incl. sample against a live service
  and 31 gate self-tests); `make dotnet-coverage` (821/844 handwritten lines; `ConnectTransport`
  200/200, `JsonBodies` 7/7, `Causes` 7/7, `ResponseBody` 84/97, the rest `Stream` boilerplate);
  G4 (`make proto-csharp` no Wire drift, sync-release-digests `--check`, `changelog.py check`,
  `ci-changed-areas-test.sh`, `check-doc-ids.py`); `go test ./tests/hygiene/...`. An independent
  code review of the staged change found no defects and no vacuous tests. Negative controls
  observed failing, then restored (scratch script, `.bak05` backups, none left): gzip trailer
  check removed (5); any non-200 treated as the service's (22); `Protocol` ignored, always gRPC
  (198 — the common tests really run over Connect); silent gRPC fallback on Connect failure (11,
  incl. the live grpc-only test); redirects followed (2); client deadline removed (4, after
  adding 10 s/20 s caller backstops so it fails rather than hangs, as it first did); envelope
  UTF-8 check removed (2); decompressed bound removed (4, incl. live); gRPC+JSON allowed (2);
  200 content type unchecked (7). Cleanup: probe console and scratch scripts under the session
  folder only; every fixture service stopped (stdin EOF, then kill); `pgrep` shows no
  `sysml-grpc`. Not run: Linux/Windows and hosted CI (no new CI wiring needed: the existing
  `dotnet-test` job runs the suite with the service binary), G3 (suite-adapters), G5, AOT;
  `make proto` as a whole (Python/TypeScript generators). Limitations: no grpc-web or browser
  entry; JSON cannot preserve an unknown enum name; multi-member gzip refused. Downstream:
  suite-adapters runs scenarios over three wires via `ClientWire`/`ClientWires.To(address)`;
  model must find structured not-found evidence per RPC over both protocols (the Connect error
  envelope carries the same `not_found` code and message text as gRPC); lifecycle-local keeps
  `Protocol`/`Encoding` connection-local (they do not split a child's identity).
- **Outcome/prerequisites:** Connect protobuf and JSON behind the same Connection; grpc.
- **Read first:** [Java ConnectTransport](../../../client/java/opensysml-client/src/main/java/org/openmbee/opensysml/internal/ConnectTransport.java),
  [wire JSON rules](../../../docs/reference/wire-contract.md), response-compression tests in Python.
- **Owned surface:** ConnectTransport/JsonBodies and protocol-specific tests/README.
- **Tests:** exact URL/headers/bodies, gzip, timeout header, HTTP fallback/error details,
  truncated/invalid bodies, bounded decompression, JSON int64/bytes/oneof/defaults/enums;
  all common error/handler tests and grpc+JSON local refusal.
- **Gate/negative control:** G1/G2; bad protobuf/HTML 502 cannot return a success-shaped
  result; no implicit gRPC fallback; all three configurations execute common tests.
- **Non-goals/handoff:** no grpc-web or supported browser entry. Record JSON forward-
  compatibility limitations established by the parser spike.

### Package 06: values — ✅ DONE (2026-10-21)

- **Status: ✅ DONE (2026-10-21).** Built on baseline `a662c2bce` plus scaffold `bdb44e4fd`, wire
  `7c1e0554d`, grpc `469eed9f7` and connect `92db5132a` on `feat/dotnet-client`, committed as
  `dd9a5cca1`; no schema change. Delivered public API (313 lines added to
  `PublicAPI.Unshipped.txt` by the RS0016 code fix, `dotnet format analyzers --diagnostics RS0016 --severity warn`): abstract **class**
  `Value : IEquatable<Value>` (private constructor; abstract `Kind`, `Equals(Value?)`,
  `GetHashCode`, `ToString`; sealed `Equals(object)`, `==`/`!=`, `FromInteger(BigInteger)`
  choosing the arm at the int64 boundary); abstract `Value.NumberValue` (`private protected`
  constructor, `ToDouble()` nearest ties-to-even) over `IntegerValue`, `BigIntegerValue`,
  `RationalValue`, `RealValue`; sealed nested `ComplexValue`, `BooleanValue`, `TextValue`,
  `InstanceRefValue`, `EnumLiteralValue(LiteralId, EnumerationId, Name, Scalar?)`,
  `SequenceValue`, `SetValue` (`Contains`, `Count`), `QuantityValue`, `ArrayValue`/
  `TensorQuantityValue` (`Dimensions`, `Rank`, `Get(params ReadOnlySpan<long>)`),
  `VectorValue` (components `ValueList<NumberValue>`), `VectorQuantityValue` (≥1, shared `Unit`),
  `MeasurementRefValue(Unit, Reduction, UnitId?)`, `FunctionValue(CalcId, SelfId?)` (0 refused),
  `MetaobjectValue(ElementId, MetaclassId)`, `NullValue(Unsupported)`/`IsUnsupported`,
  `UnsetValue`, `UndeterminedValue(Reason, CountLower, CountUpper)`, `InfinityValue`; enum
  `ValueKind` (23 members in the plan's order); sealed class `Rational` (canonical
  BigInteger terms, `IsInteger`, `ToDouble`, `TryGetExactDouble`, `IComparable`, all
  comparison operators, `n/d` text); sealed records `Quantity(Magnitude: NumberValue, Unit,
  Reduction?)`/`IsExact`, `UnitTerm(ScaleNumerator, ScaleDenominator, Factors)`/`One`,
  `UnitFactor(UnitId, Exponent)` (get-only properties, so `with` cannot bypass validation);
  readonly structs `ValueList<T>` and `ValueDictionary<TValue>` (default = empty, defensive
  copies, null refused, `==`); `UnsupportedValueException(Method, Path, ValueBytes)`.
  **Deviations, with evidence:** (1) the hierarchy is classes, not records: a non-sealed record's
  copy constructor must be public or protected (CS8878, probed), which an external record could
  chain to; (2) arm types carry a `Value` suffix because `Integer`, `Boolean`, `Set`,
  `Function` fail CA1716/CA1720 as type names (probed), and `ValueKind.Integer` carries the one
  targeted `[SuppressMessage("Naming","CA1720")]` with a justification; (3) `ValueMap<TKey,TValue>`
  is `ValueDictionary<TValue>`: every schema map is string-keyed, and CA1710 requires the
  `Dictionary` suffix; (4) set membership is structural `Value` equality (NaN = NaN, +0 = −0),
  not Java/Rust's model `sameValue`, because the latter is neither reflexive (NaN) nor
  transitive (Rational↔Real at double precision) and so cannot back .NET equality/hashing; the
  service never sends model-equal members, and a client set holding `1` and `1.0` is refused by
  the service (documented in README and `SetValue` remarks). Equality: content; Real/Complex by
  `double.Equals`; enum literal by `LiteralId`, metaobject by `ElementId`, function by
  `CalcId`+`SelfId`; display culture-invariant (`4.0`, `1E+20`, `-0.0`, `1/3`, `"a \"b\""`,
  `5.4 [km/h]`, `(1, 2)`, `{1, 2}`, `Array(2, 2)[…]`, `<unset>`, `<undetermined>`, `*`).
  Internal `ValueWire`: `FromWire(Wire.Value|IReadOnlyList|IReadOnlyDictionary, method, path)`
  checks canonical big-int digits (ASCII, no sign/leading-zero tricks, beyond int64), canonical
  lowest-terms rationals that no double holds, shapes, vector numerics, required sub-messages,
  `true`-only unset/infinity; malformed → `TransportException(Internal)` naming the path
  (`result.sequence[1].set[0]`); absent/unknown arm → `UnsupportedValueException` with the
  message's bytes, never Null; depth bound `MaxDepth = 100`. `ToWire(value, path)` refuses
  Unset, Undetermined, unsupported Null, a function with `SelfId`, a named-unit quantity
  without reduction, unusable unit scales/empty factor ids, and depth > 100, as
  `InvalidRequestException` (the option-shape step). `Fit(wire, capabilities)` first rewrites
  double-exact Rationals as Reals for a service without `rational_values` (Go
  `RationalsAsReals`, including enum-literal scalars), then requires `RequiredCapabilities` in
  `SendGateOrder` (the service's `requireValueCapabilities` order), with the service's exact
  predicates: kinds through sequence/set/array elements and vector components only; big-int and
  rational also through quantity/vector-quantity/tensor magnitudes and enum-literal scalars;
  traversal iterative (100,000-deep proven). Enum literals stay send-ungated, as the service.
  Inventory: all 23 Value arms `implemented` with `ValueArmTests.<Kind>Arm`; all fields of the
  18 messages reachable from `sysml.Value` `typed` (`MultiplicityInfo` via Undetermined). Tests:
  177 new (724 total, `DOTNET_MIN_TESTS := 724`; 679 `Unit`, 45 `Integration`): `ValueArmTests`
  (one per kind: twin equality/hash, inequality, exact wire, decode, binary round trip,
  display; all four quantity magnitudes), `ValueInventoryTests` (one sealed class/arm test/
  implemented arm per kind; constructor accessibility; every schema oneof arm decoded to its
  inventory kind, re-encoded byte-equal and gated exactly as the inventory says; value-message
  fields typed; gate predicates and order parsed from `service.go`; `NestedValues`/
  `ValueHolds*`/`RationalsAsReals`/`ValueCarriesStructured` cases parsed from `convert.go`;
  Roslyn compile of an external consumer: positive control clean, `: Value` CS1729 (CS0122
  with all metadata imported), `: Value.NumberValue` CS0122, `record : Value` CS8864, every
  arm CS0509), `ValueReadTests` (int64 edges and 14 non-canonical spellings incl. Arabic-Indic
  digits; 16 rational spellings incl. 2^-1074 vs 2^-1075; shapes, overflow; missing parts; set
  duplicates; unknown arm bytes, empty, nested path, JSON drop; depth; immutability after read;
  repeated/map paths), `ValueWriteTests` (13 unsendable shapes alone and nested; 14 gated kinds
  at three depths; ungated kinds to a service advertising nothing; full order walk; enum-literal
  scalar rule; magnitude gates; rational fitting at seven positions; no double for 1/3; deep
  traversal; depth bound; fresh messages; bit-exact reals incl. NaN payload; map/list order
  and paths), `ValueSemanticsTests`, `ValueCollectionTests`, `RationalTests` (≈455 quotients
  against IEEE division, ties-to-even, overflow boundary at max+½ulp, subnormal rounding),
  `ServiceValueTests` (Integration, theories over gRPC, Connect+proto, Connect+JSON: 21 kinds
  sent through a live identity calc come back equal — incl. big integers, rationals, complex,
  infinity, tensor/vector quantities, measurement ref, function, metaobject, enum literal; a
  set read from the model and sent back reaches the calc as its members; Unset, Undetermined,
  EnumLiteral read from Instantiate/Evaluate; the service itself refuses the function-with-self
  and unset arguments the client refuses). `PublicApiBoundaryTests` now spells generic types
  as the baseline does (`ValueList<T>`); it failed on the first public generic. Test-only
  dependency `Microsoft.CodeAnalysis.CSharp` 5.0.0 (central pin with its reason; lock file adds
  it, Common 5.0.0 and Analyzers 3.11.0) for the consumer compile test. README "Values" section,
  Failures row, status line and inventory bullet; changelog fragment
  `dotnet-client-values.added.md`. Passed (macOS arm64, net10.0, runtime 10.0.13, SDK
  10.0.402): G0 (`--locked-mode` restore after the lock update); G1 (format, `--no-incremental`
  0-warning build, Unit 679/679); G2 724/724 with `OPENSYSML_REQUIRE_SERVICE=1` and
  `--minimum-expected-tests 724`, three consecutive runs, 0 skipped; `make dotnet-test` (sample
  live, 31 gate self-tests); `make dotnet-coverage` (1545/1570 handwritten lines; the value
  files 724/726, the two misses unreachable closed-hierarchy defaults); G4 (`make
  proto-csharp` no Wire drift, sync-release-digests `--check`, `changelog.py check`,
  `ci-changed-areas-test.sh`, `check-doc-ids.py`); `go test ./tests/hygiene/...` (parity.json
  92 KB, under the fixture cap). Negative controls observed failing, then restored (scripted,
  `.bak06` backups with mtimes touched on restore, none left): set decoder removed (4 incl. the
  schema-arm classification), set encoder removed (19), `Value` and `NumberValue` constructors
  made protected (build fails on RS0016/CS1591; with docs and warnings non-fatal, the compile and
  accessibility tests fail), `ValueList` aliasing an array (1), two gates swapped (the
  service-source test), all gates through literal scalars (1), rational fitting removed (3),
  non-canonical big integers accepted (10), double-exact rationals accepted (5), NaN ≠ NaN (2),
  unknown arm read as Null (5), order-sensitive set equality (2), one inventory arm deferred (1).
  A first control script restored files with old mtimes, so one control ran against a stale
  mutated build; the fixed script reran it isolated. Cleanup: scratch probes, temp files and
  backups deleted; fixtures stop their services; `pgrep` shows no `sysml-grpc`. Not run:
  Linux/Windows and hosted CI (no CI change needed: `dotnet-test` runs the suite with the
  service), G3/G5, AOT, and a direct run of the README snippet (a scratch app outside the tree
  was not permitted; its calls and printed text are asserted by `QuantityArm`, `RationalArm` and
  the set display tests). Limitations: no public method takes or returns a value yet; model
  `sameValue` equivalence (units, Integer≡Real) is not offered; over JSON an unknown arm cannot
  be kept. Downstream: decode response values with `ValueWire.FromWire(…, method, path)`; for a
  request, call `ToWire` for every value at the option-shape step, then `Fit` each at the
  nested-values step after the RPC and option gates, iterating maps in `ValueDictionary` order
  (ordinal); `DocumentValue` (query-docs) and the `AttributeInfo`/`FeatureValue`/request
  `valueFields` owners keep their fields `deferred` until they map them; model may type
  `SymbolInfo.multiplicity` with its own `Multiplicity`, its `MultiplicityInfo` fields already
  `typed`; raise `DOTNET_MIN_TESTS` with every added test.
- **Outcome/prerequisites:** complete immutable domain Value model and wire converters; wire.
- **Read first:** schema Value/Quantity/Rational comments, Java Value/Protos and Rust values.
- **Owned surface:** Value, collection wrappers, Rational/numeric types, value converters.
- **Tests:** each arm encode/decode/display/equality/hash; int64 edges, huge integers,
  normalized/noncanonical rationals, NaN/signed zero/infinities; shape/index failures;
  map insertion order and set permutations; deep gate traversal; immutable copies.
- **Gate/negative control:** G1/G4; external derivation compilation fails; remove an arm
  converter and classification fails; mutate input/wire clones and domain state is unchanged.
- **Non-goals/handoff:** no live-model facade. Deliver stable value APIs and exact gate rules
  before execution/editing branches begin.

### Package 07: model — ✅ DONE (2026-10-22)

- **Status: ✅ DONE (2026-10-22).** Built on baseline `a662c2bce` plus scaffold `bdb44e4fd`, wire
  `7c1e0554d`, grpc `469eed9f7`, connect `92db5132a` and values `dd9a5cca1` on `feat/dotnet-client`,
  left **uncommitted** at the maintainer's request; no schema change. Delivered public API (248 lines
  added to `PublicAPI.Unshipped.txt` by the RS0016 code fix): `Connection.LoadAsync(path,
  ParseOptions?)`, `LoadContentAsync(content, ParseOptions?)`, `ParseSourcesAsync(IEnumerable<SourceDocument>,
  SourcesOptions?)`, synchronous `ModelByHash(hash)` (sends nothing; refuses blank and a disposed
  connection); sealed `Model : IAsyncDisposable` (`Connection`, `Hash`, `Roots`, `Root`,
  `ParseDiagnostics`, `Errors`, `HasErrors`, `Documents`, `BaseModelHash`, `Affected`,
  `OwnsConnection`, `RequireNoErrors()`, `GetDiagnosticsAsync(DiagnosticsOptions?)`,
  `SymbolAsync`/`FindSymbolAsync` (service resolution), `FindAsync`/`LookupAsync` (short or own-id
  name, outermost first; Query when advertised, else a walk from the roots; a by-hash model without
  `query` is `CapabilityException`), `EvalAsync`/`EvaluateAsync(EvalOptions?)`, `InstantiateAsync(symbolId)`);
  static `OpenSysMLClient.LoadAsync`/`LoadContentAsync(…, ConnectionOptions?, ParseOptions?)` returning
  an owning model (internal seam `LoadOwnedAsync`); options `ParseOptions` (`Language`,
  `StrictConformance`), `SourcesOptions` (`StrictConformance`, `BaseModelHash`), `EvalOptions`
  (`ContextSymbolId`, `SubjectSymbolId`), `DiagnosticsOptions` (copied lists), enum `SourceLanguage`;
  sealed record `SourceDocument` (`FromFile`, `Inline(content, name?, language?)`, private ctor,
  get-only properties); sealed records `Symbol`, `AttributeInfo`, `TypeFacts`, `Multiplicity`,
  `Specialization`, `Diagnostic` (+ `SeverityName` kept raw) with enum `DiagnosticSeverity`
  (`Unknown` first), `SourceSpan`, `Instance`, `FeatureValue`, `Instantiation` (`FindInstance`,
  `Resolve`), `Evaluation`; exceptions `ModelException(Message, Method, Diagnostics)` →
  `ModelErrorsException`, `SymbolNotFoundException(Name, Suggestions)`; `ServiceException` →
  `ModelNotFoundException`, `ModelFileNotFoundException`. Internal: `ModelWire` (reads every field
  whatever is advertised, values via `ValueWire` with response paths such as
  `instance.feature_values["n"].value`), `NearNames` (difflib Ratcliff/Obershelp, cutoff 0.6, ties by
  name descending, repeats once as in Java), `Connection.UnaryAsync`/`ModelCallAsync`/`ThrowIfDisposed`.
  **Decisions, with evidence:** (1) not-found is structured per RPC, never by text: the service's
  source shows each model RPC (GetSymbol, GetDiagnostics, Evaluate, Instantiate, Query) answers
  NotFound only for a cache miss, and a parse only from `fileInput`; `ModelServiceSourceTests`
  pins both by reading `service.go`/`query.go`; (2) `SymbolAttribute` became `AttributeInfo` (CA1711
  refuses the `Attribute` suffix); (3) empty-string facts stay empty (as `UndeterminedValue` counts),
  null only for absent messages (`TypeFacts?`, `Multiplicity?`, `Span?`, `Value?`); (4) disposal is
  checked before options or capabilities, so a disposed connection is always
  `ObjectDisposedException`; (5) a file with `ParseOptions.Language`, an undefined language, an empty
  `BaseModelHash` or an empty `EvalOptions` symbol is `InvalidRequestException` (each would otherwise
  be silently ignored), while an empty or duplicate document list is left to the service, so those
  scenarios stay expressible; (6) answers with neither result nor error (no model hash, symbol,
  value or instance) are `TransportException(Internal)`; (7) `FindAsync`/`LookupAsync` call Query
  internally, as Java and Python do; the Query row and `QueryRequest`/`Query`/`Constraint` fields stay
  `deferred` for query-docs; (8) `Diagnostic`/`DiagnosticSeverity` collide with
  `Microsoft.CodeAnalysis` in a file importing both (the Roslyn consumer test now qualifies it).
  Inventory: ParseFile, ParseSources, GetSymbol (methods + `Model.FindAsync`), GetDiagnostics,
  Evaluate, Instantiate `implemented` with unit and live tests; every field of their requests,
  responses, `SourceDocument`, `SymbolInfo`, `Specialization`, `TypeInfo`, `AttributeInfo`,
  `Instance`, `FeatureValue`, `Diagnostic`, `Span` `typed`; `ParseFileRequest.content_hash`
  `unsupported` (deprecated, ignored, never sent). `ParityInventoryTests.AnImplementedRowNamesTestsTheSuiteHas`
  now sets `Tests = []` explicitly instead of relying on Evaluate being deferred. Tests: 158 new (882
  total, `DOTNET_MIN_TESTS := 882`; 807 `Unit`, 75 `Integration`): `ModelParseTests`,
  `ModelSymbolTests`, `ModelEvaluateTests`, `ModelOwnershipTests` (theories over the three wires on
  `FakeServiceHandler`, with `FakeModels`/`FakeModelTree` answering GetSymbol and Query by
  name/id/owner: exact requests, every response field, gate order and zero requests when withheld,
  in-band vs malformed vs not-found, homonym depth, errors-model bounded walk, owner cycle, 32
  concurrent reads, one-shot ownership and failure release for in-band/refused/capability/cancel),
  `NearNamesTests` (expectations computed by Python difflib), `ModelServiceSourceTests`,
  `ServiceModelTests` (Integration, three wires on the `connect` transport: inline/file/strict parse,
  syntax error and `RequireNoErrors`, unreadable file both RPCs, two documents as one model with
  `Affected`, duplicate/empty refused, symbol facts and lookup with suggestions, ModelNotFound for
  five RPCs, lint selection, scope/subject evaluation, instantiation with resolved engine and cyclic
  error, one-shot ownership with concurrent reads, and a service started with
  `OPENSYSML_TEST_WITHHOLD_CAPABILITIES` where a counting borrowed handler proves every withheld
  option sends nothing and lookup walks without Query). `ServiceProcess`/`ServiceFixture` gained the
  withhold switch. Sample: parses an inline vehicle, `LookupAsync("Vehicle")`, evaluates its mass and
  prints `Demo::Vehicle (partDef): mass = 1500.0`, which `dotnet-gates.py sample` now requires (+1
  self-test, 32 total). README "Models" section, status, sample output and Failures rows; changelog
  fragment `dotnet-client-models.added.md`. Passed (macOS arm64, net10.0, runtime 10.0.13, SDK
  10.0.402): G1 (format exit 0, 0-warning build, Unit 807/807); G2 882/882 with
  `OPENSYSML_REQUIRE_SERVICE=1` and `--minimum-expected-tests 882`, three consecutive runs, 0 skipped;
  `make dotnet-test` (sample live, 32 gate self-tests); `make dotnet-coverage` (2091/2117 handwritten
  lines; new code fully covered except the unreachable method-resolution failure in `UnaryAsync`);
  G4 (`make proto-csharp` no Wire drift, sync-release-digests `--check`, `changelog.py check`,
  `ci-changed-areas-test.sh`, `check-doc-ids.py`); `go test ./tests/hygiene/...` (parity.json
  104 KB). Negative controls observed failing, then restored (scripted in the session folder, mtimes
  touched, restored build clean): strict gate removed (5, incl. live withheld), one-shot failure not
  closing (12), NotFound left plain (18), ParseSources gate order swapped (1), homonyms unsorted (3),
  repeated near names (1), warnings counted as errors (4), a symbol field dropped (3), walk without
  the by-hash Query check (6, incl. live), disposal not checked first (1), sample gate ignoring the
  model line (1 self-test). Development caught: the malformed-path error named `instances[0]` because
  reachable objects were decoded before the root (now root first); two live assumptions were wrong
  and replaced by schema guarantees (`invalid syntax (((` fails as trailing text with no
  diagnostics; `affected` lists every document when the service cannot relate the models).
  Cleanup: no temp files left; every fixture service stopped; `pgrep` shows no `sysml-grpc`. Not
  run: Linux/Windows and hosted CI (no CI change needed), G3 (suite-adapters), G5, AOT. Limitations:
  `Documents` names an unnamed inline document `""`; `ModelFileNotFoundException` carries no path of
  its own (the service message names it); no `Wire` getters on results. Downstream: service-area
  packages call `Connection.ModelCallAsync` for model RPCs (NotFound → `ModelNotFoundException`),
  `Connection.ThrowIfDisposed()` first, decode diagnostics with `ModelWire.Diagnostics` and objects
  with `ModelWire.Instance`; query-docs owns the public Query surface; the private `Model.QueryAsync` and `Selecting` build the same request, and
  lookup must keep working through it; suite-adapters render `Symbol`/`Instantiation`/`Diagnostic`, and treat ParseFile with
  no source as an unexpressible request; raise `DOTNET_MIN_TESTS` with every added test.
- **Correctness follow-up (2026-10-22):** owning-model disposal delegates every call to the
  connection's existing idempotent close, so every concurrent caller waits for its completion;
  borrowed-model disposal remains a no-op. `ConcurrentModelDisposalsAllWaitForTheConnectionToDrain`
  holds an active call until cancellation has reached its handler, proves both disposals remain
  incomplete, and then releases the call, over all three wires. Near-name scoring and tie ordering
  use Unicode code points, preserve isolated surrogates, and implement `difflib`'s automatic
  popular-character threshold against the complete second sequence, including extension of a
  match through popular characters. Added Unicode/cutoff/tie-order and 199/200/201-character
  boundary cases: 20 new tests, 902 total (`DOTNET_MIN_TESTS := 902`). Validation passed:
  focused ownership/name tests 50/50; `make build-grpc`; `make dotnet-test` with
  `OPENSYSML_GRPC_BINARY` naming that binary and `OPENSYSML_REQUIRE_SERVICE=1` (0-warning build,
  formatting clean, 902/902 tests, 0 skipped, live Vehicle sample, 32/32 gate-script tests);
  3,000 deterministic randomized comparisons against Python's `difflib`, each checking both
  the ratio and the complete suggestion list. README and a fixed changelog fragment describe
  the corrected behavior. No public API change; per-row matching-buffer allocation remains
  unchanged and is a separate optimization. Repository-wide checks: `go build ./...` and
  `go vet ./...` passed; `gofmt -l .` was empty; `go test ./...` failed outside the .NET
  surface in command lifecycle/path assertions, PDF fake-tool output, analysis manifest
  path assertions (`/var` versus `/private/var` on macOS), JSON-RPC MIME error wording,
  optimization output, generated-C compilation/locale behavior, and a codegen test timeout.
  No Go source was changed.
- **Outcome/prerequisites:** parse/symbol/diagnostic/evaluate/instantiate facade; grpc, values.
- **Read first:** scenario groups 02-05, Java model/symbol/instance facades and converters.
- **Owned surface:** Model core, Symbol/TypeFacts/Multiplicity/Specialization/metadata,
  SourceDocument/ParseOptions/SourcesOptions/DiagnosticsOptions, Instantiation/Instance.
  Instantiate takes only the symbol ID and standard cancellation token, matching the wire.
- **Tests:** file/inline/multidoc and affected-document parse fields; lookup suggestions,
  RequireNoErrors, subject/context, all symbol/result fields; capability preflight; ownership
  rollback; model hash immutability; concurrent reads. ModelByHash requires a live connection.
- **Gate/negative control:** G1/G2 for available protocols; withheld option capability causes
  zero requests; one-shot parse failure releases connection. Full three-protocol evidence
  is required once connect lands, not a reason to block independent domain work.
- **Non-goals/handoff:** no execution/editing. One-shot helpers take explicit address until
  lifecycle is available, while keeping their final ownership semantics.

### Package 08: lifecycle-local — ✅ DONE (2026-10-22)

- **Status: ✅ DONE (2026-10-22).** Built on baseline `a662c2bce` plus scaffold `bdb44e4fd`, wire
  `7c1e0554d`, grpc `469eed9f7`, connect `92db5132a`, values `dd9a5cca1` and model `70a1341f2` on
  `feat/dotnet-client`, left **uncommitted**; no schema change, `parity.json` unchanged (no RPC or
  field is involved). Delivered public API (28 lines added to `PublicAPI.Unshipped.txt` by the RS0016
  code fix): `ConnectionOptions.BinaryVariable` (`OPENSYSML_GRPC_BINARY`), `BinaryPath`,
  `ExpectedBinarySha256`, `IsolatedService`, `StartupTimeout` (`DefaultStartupTimeout` 30 s),
  `ShutdownTimeout` (`DefaultShutdownTimeout` 5 s); `static Connection.StopSharedServicesAsync(token)
  -> Task<int>`; `BinaryNotFoundException(message, searchedLocations)` (`SearchedLocations`),
  `ChecksumMismatchException(binaryPath, expectedSha256, actualSha256, remedy)`,
  `UnsupportedPlatformException(platform, message)` (all three `: ServiceStartException`) and
  `ServiceStopException(failures) : OpenSysMLException` (`Failures`; inner is the one failure or an
  `AggregateException`). `AutoStart` stays true: with no service named, `OpenAsync` now starts a
  private service. Internal: `BinaryResolver` (option → env → `~/.opensysml/bin/sysml-grpc[.exe]`, the
  home read as `HOME`/`USERPROFILE` through the injected environment, an unset home skipping it →
  first on `PATH`; an explicitly named binary that is not executable is `BinaryNotFoundException`,
  never a fallback; managed realpath `Canonical` following every link component, 40 hops; SHA-256
  always, for identity, then `ExpectedBinarySha256`, before exec; browser/wasi/ios/tvos refused);
  `PrivateService : IServiceProcess` (argv exactly `-port 0 -health-port 0 -report-address
  -exit-with-parent`; two `LongRunning` pump threads reading raw bytes, the first stdout line is the
  address, then a 20-line tail, 1,024 chars/line with `…`, each line at `Trace`; the address must be
  a loopback `host:port`; timeout/early exit/EOF-but-alive/garbage/cancel all stop the child and
  drain before failing, quoting how it ended and its tail; stop = close stdin → `ShutdownTimeout` →
  `Kill(entireProcessTree)` → 10 s → typed failure; `Exited` task); `ServiceIdentity` (canonical
  path, SHA-256, startup and shutdown timeouts, `Version`); `ServiceRegistry` (static `Shared` per
  load context; `Starting → Ready → Stopping → gone`, `Starting → Failed → gone`; holders counted from
  the start of an open; no lock across await; one `Task.Run` start per entry on its own token; a
  cancelled waiter releases only its reservation, the last one cancels the start and waits for it to
  be undone; a failed start is forgotten and gives every waiter the same exception; an exited entry
  is never joined; death ends every lease with a reason, logs pid/ending/tail and is not restarted;
  `StopAllAsync` ends shared leases, stops all, waits for stops already under way, aggregates
  failures, leaves isolated ones; an acquirer meeting a stopping entry waits for `Gone` and starts
  afresh; `CloseAllInputs` on `Unloading` of a collectible context only; `Lease` with `Address`,
  `ProcessId`, `Ended`, `EndReason`, idempotent `ReleaseAsync`; `Entries()` snapshot). `Connection`
  holds the lease from before the handshake (a failed handshake or capability/version check releases
  it), links `lease.Ended` into every call and maps a call failing after the service ended, or any
  call after it, to `TransportException(Unavailable, reason)`; caller cancellation and disposal keep
  precedence and other mappings are unchanged; disposal releases after closing the transport.
  `BinaryPath`/`ExpectedBinarySha256`/`IsolatedService` are refused with `Address` or `AutoStart=false`
  and logged as unused when `OPENSYSML_SERVICE` names the service; `StaleServiceException` names the
  binary as the remedy for a private service. **Decisions, with evidence:** (1) identity includes the
  timeouts as immutable startup configuration, so connections never disagree about how their service
  is started or stopped; (2) on Unix, .NET's `File.Exists` is true of a dangling link and of a link
  loop (probed), so executability is judged on the resolved path; (3) no ProcessExit handler: kernel
  EOF on stdin is the orphan guarantee, observed when a mutated test host exited leaving children
  that then exited on their own; (4) the .NET runtime keeps its own duplicate of fd 1 (seen with
  `lsof`), so the test stand-in closes every descriptor with fd 1's `fstat` identity to really end
  its output (fixture only). Tests: 104 new (1006 total, `DOTNET_MIN_TESTS := 1006`; 916 `Unit`, 90
  `Integration`): `BinaryResolverTests` (15: order, PATH skipping, explicit-missing by option and
  variable, directory/dangling/loop/no-exec-bit/unreadable, searched-locations text, digest match in
  either case and mismatch payload, canonical through file/dir/relative links, platform refusal);
  `PrivateServiceTests` (20, against `test/FakeService`: exact argv, six non-addresses incl.
  non-loopback, early exit with stderr, EOF-but-lingering killed, timeout, cancel carries the token,
  4 MiB stderr before the address plus flooding after, endless line bounded, overlong first line,
  ignoring stdin killed with its tree, external kill observed, concurrent idempotent stop, input
  close, non-executable file, pre-cancelled); `ServiceRegistryTests` (19, in-memory `FakeLauncher`:
  eight concurrent acquirers, every identity part, isolated, one waiter gives up, all give up, a start
  finishing after its last waiter left, failed start shared, death, exited entry not joined, stop all
  incl. a start in progress, a stop already under way and failure aggregation, last-release failure
  logged, double release, unload); `PrivateConnectionTests` (22: real children with the fake HTTP
  handler, over all three wires where shared: open/close, sharing across wires and per-connection
  options, identity splits, stale version and missing capability release the hold, missing binary
  and digest mismatch run nothing, killed starting child leaves no hold, cancelled open, in-flight
  call on disposal, death fails in-flight and later calls promptly then reopens, transport failure
  after the end, stop all, env-named service warns, AutoStart off, eight concurrent opens);
  `ServiceLifecycleTests` (13) and `SharedServiceTests` (2, non-parallel collection) against the real
  binary: private open per wire with parse/lookup/eval, eight concurrent opens per wire, isolated and
  differently configured, kill per wire, stop/reopen, stale version, own digest, then the shared
  registry with a one-shot load, `StopSharedServicesAsync` and the binary variable; plus 13 new
  `ConnectionOptionsTests` cases (option validation and contradictions, default open now reporting
  the binary search). `test/FakeService` is a console stand-in reading `fake-service.json` beside a
  renamed apphost copy, referenced with `ReferenceOutputAssembly=false` so coverage never sees it,
  with its own lock file. Sample: connects when `OPENSYSML_SERVICE` or `OPENSYSML_GRPC_BINARY` is
  set; `dotnet-gates.py sample` runs it a third time with the binary alone (+2 self-tests, 34).
  README "Private services" section, Failures rows, status, sample, testing; changelog fragment
  `dotnet-client-private-service.added.md`. Passed (macOS arm64, net10.0, runtime 10.0.13, SDK
  10.0.402): G1 (format, 0-warning build, Unit 916/916 with no binary); G2 1006/1006 with
  `OPENSYSML_REQUIRE_SERVICE=1` and `--minimum-expected-tests 1006`, three consecutive runs, 0 skipped
  (the Unit suite also five consecutive runs); `make dotnet-test` (locked restore, private sample run,
  34 gate self-tests); `make dotnet-coverage` (2626/2691 handwritten lines; uncovered are defensive
  catches: kill failure, kill-wait timeout, undrained pumps, transport construction failure,
  callback exceptions, the collectible-context unload hook); G4 (`make proto-csharp` no drift,
  sync-release-digests `--check`, `changelog.py check`, `ci-changed-areas-test.sh`,
  `check-doc-ids.py`); `go test ./tests/hygiene/...`; `go build`/`go vet`/`gofmt -l` clean (no Go
  change). Negative controls observed failing, then restored byte-identical (scripted in the session
  folder): explicit binary falls back (5), digest unchecked (3, incl. live), stderr undrained (4),
  lines unbounded (2), loopback not required (2), failed start not forgotten (5), a cancelled waiter
  cancels the start for all (1), identity without digest/version (5, incl. live), isolated shared
  (6), last release does not stop (24), calls ignore the service's end (3; this control first
  survived because the in-flight call still failed correctly at its 60 s deadline, so the test now
  requires failure within 10 s), exited entry joined (1), failed handshake keeps its hold (19),
  sample gate without the private run (2 self-tests). Cleanup: every test disposes its stand-in copy
  and kills nothing it did not start; no `opensysml-*` temp directories and no `sysml-grpc` or
  `FakeService` process left (`pgrep`); the scratch probe test and files were deleted. Not run:
  Linux/Windows and hosted CI (the existing `dotnet-test` jobs build and run the new tests; native
  macOS/Windows jobs are lifecycle-safety's), G3 (suite-adapters), G5 (lifecycle-safety), AOT; the
  Windows branches (`USERPROFILE`, `.exe`, `CloseHandle` in the stand-in; four Unix-only resolver
  tests skip there by name) are unexercised. Limitations: no download (a missing binary says so);
  the binary is hashed and then executed by path, so a swap between the two is not detected (the
  downloads package's digest-named snapshot closes this for cached releases); identity paths are
  case-sensitive, so one file reached through differently cased paths on a case-insensitive file
  system starts two services; a shared service logs to the logger of the connection that started it.
  **Downstream (stable contract):** `ServiceIdentity.Of(ResolvedBinary, options)` and
  `ServiceRegistry.AcquireAsync(identity, isolated, logger, token) -> Lease` with
  `ReleaseAsync(logger)`; downloads replace `BinaryResolver.Find`'s not-found branch (keeping explicit
  binaries authoritative and resolving before an identity is published) and add `DownloadVersion`/
  `GithubRepo`; lifecycle-safety proves the per-load-context scope and the `Unloading` hook with real
  collectible copies, runs the lifecycle tests natively on macOS and Windows, and can build its probes
  on `FakeChild`/`FakeScript` and `ServiceRegistry.Entries()`; raise `DOTNET_MIN_TESTS` with every
  added test.
- **Local lifecycle regression contract:** ten additional cases cover absolute and relative
  paths and link targets with a link followed by `..`, refusal to traverse missing directories
  or regular files, aggregation of existing and newly initiated stop failures, read-only copied
  failure collections, and late startup during unload for shared and isolated services. Path
  canonicalization preserves parent components until preceding links have been followed.
  Unload marks starting entries as stopping under the registry lock before cancelling them.
  The public API signatures are unchanged; `DOTNET_MIN_TESTS` is now 1016 (926 Unit, 90
  Integration). Collectible-context and native-platform proof remain lifecycle-safety's scope.
  Verified on macOS arm64, SDK 10.0.402: all ten new cases failed before the fixes; the full
  resolver/registry selection passed 44/44 after them. `dotnet format
  client/dotnet/OpenSysML.slnx --verify-no-changes --no-restore` passed, and `dotnet build
  client/dotnet/OpenSysML.slnx -c Release --no-restore -warnaserror` passed with zero warnings.
  With `OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc"` and `OPENSYSML_REQUIRE_SERVICE=1`,
  `dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build
  --minimum-expected-tests 1016 --results-directory "$PWD/bin/dotnet/review-fixes"
  --report-xunit-junit --coverlet --coverlet-output-format cobertura --coverlet-exclude-by-file
  '**/src/OpenMBEE.OpenSysML/Wire/Sysml.cs' --coverlet-exclude-by-file
  '**/src/OpenMBEE.OpenSysML/Wire/SysmlGrpc.cs'` passed 1016/1016, zero skipped. The coverage
  gate passed at 2642/2708 handwritten lines; the sample's identity-only, named-service and
  private-service runs passed, as did the 34 Python gate tests, release-digest synchronization,
  changelog validation, documentation-label check and `git diff --check`. `go build ./...`,
  `go vet ./...` and `gofmt -l .` passed (no formatting output). `go test ./...` failed in
  unchanged CLI, PDF, analysis, JSON-RPC, REPL and code-generation packages, including a
  ten-minute timeout in `tests/codegen`; no Go source was changed. No Linux/Windows execution
  or collectible-load-context proof is claimed by this follow-up.
- **Outcome/prerequisites:** no-argument open safely resolves local binaries and shares
  private service identities; grpc and process/locking decision evidence.
- **Read first:** section 3.6, existing client lifecycle tests and service parent-exit flags.
- **Owned surface:** BinaryResolver, PrivateService, ServiceRegistry, lease/state tests.
- **Tests:** exact argv/first line; startup timeout/cancel/garbage/early exit; bounded pumps;
  eight concurrent opens, incompatible identities, canceled waiter, failed handshake,
  isolated child, last release, active-call disposal, child death and explicit stop/reopen.
- **Gate/negative control:** G1/G2/G5 lifecycle subset; missing/incorrect explicit binary
  fails before exec; chatty fixture cannot deadlock; kill startup child and assert no lease.
- **Non-goals/handoff:** no downloads or automatic restarts. Preserve AutoStart default.
  Hand off stable registry identity/lease API, including cleanup logging.

### Package 09: lifecycle-safety — ✅ DONE (2026-10-22)

- **Status: ✅ DONE (2026-10-22).** Implementation, local validation and native lifecycle CI
  verification complete on Linux x64, Windows x64 and macOS arm64. All three .NET jobs passed
  for `da57104d5` in workflow `39842113694`, including Windows process-inheritance and collectible
  load-context tests. Implemented on `feat/dotnet-client` from lifecycle-local baseline
  `f24bf7b95`, committed through `da57104d5`; no schema change, `parity.json` and the public API
  baseline unchanged. The initial CI failures and their fixes are recorded below.
  Delivered: library seams only (`InternalsVisibleTo` for the two probes;
  `PrivateService.Input`, the child's stdin stream, read only by the leak control).
  `test/OpenMBEE.OpenSysML.OrphanProbe` (modes `serve`, `race` = 8 siblings released with the
  service's `Process.Start` from one barrier through a wrapping launcher, `leak` = inheritable
  `dup`/`DuplicateHandle` of the stdin write end alive while one sibling starts, `sibling`; exits on
  its own stdin EOF; siblings live 2 min at most). `test/OpenMBEE.OpenSysML.LoadContextProbe`
  (three collectible `CopyContext`s resolving the client and its packages through
  `AssemblyDependencyResolver`, an `Agent` reached by type name and reflection; modes `dispose`,
  `unload`, `unload-starting`; reports per-step `running` vectors, collection after ≤30 s of
  `GC.Collect`, `libraryInDefault`, then returns from `Main`). Tests (+26, `DOTNET_MIN_TESTS :=
  1042`; 931 Unit, 111 Integration): `OrphanTests` (9: serve, 3 race rounds × 8 siblings, leak
  control must see survival ≥3 s then the end within 10 s of killing the holder; per wire),
  `LoadContextTests` (7: dispose and unload per wire, unload-starting against `FakeService` with
  exit markers), `LifecycleRaceTests` (10: 200 barrier rounds of the state machine with 4 holders,
  an isolated holder, a cancelled waiter, two stop-alls and a death, every service stopped exactly
  once; close/close-twice/stop-all races against stand-in children and the real binary; opens
  racing the last close get working services); `ProbeRun` kills the probe and every reported PID
  on disposal. Finding fixed in the probe, not the library: the third copy was never collected
  because the host loaded it in the frame that later ran the collection loop; isolating it in a
  non-inlined method fixed it (5/5 reruns). CI: `dotnet-native` matrix (`macos-latest`,
  `windows-latest`; full tier; `core.autocrlf false`; native `CGO_ENABLED=0` service; locked
  restore, `-warnaserror` build, whole suite with `OPENSYSML_REQUIRE_SERVICE=1` and the Makefile's
  minimum; reports uploaded), added to `build-and-test`; `actionlint` v1.7.7 clean. README
  registry-scope, unload and platform-evidence paragraphs and the probe/race description; changelog
  fragment `dotnet-client-lifecycle-safety.added.md`. Passed on macOS arm64 (SDK 10.0.402, runtime
  10.0.13): G1 (format, 0-warning build, Unit 931/931); G2 1042/1042 three consecutive runs, 0
  skipped; `make dotnet-test` (incl. sample and 34 gate self-tests) and `make dotnet-coverage`
  (2644/2709 handwritten lines); G4 (`make proto-csharp` no drift, digests `--check`, `changelog.py
  check`, `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff --check`); `go build`, `go vet`,
  `gofmt -l` (empty), `go test ./tests/hygiene/...`. Linux arm64 (the 1.3 SDK image by digest,
  `--init`, user `app`, service cross-built from this tree): Unit 931/931, G2 1042/1042 twice; as
  root, the existing `ABinaryThatCannotBeReadIsAStartFailureNamingIt` fails because root reads a
  mode-000 file (environment, unchanged). Negative controls, each caught then restored
  byte-identically (scripted, digest-checked): a `ProcessExit` handler root (7/7 load-context tests
  fail, copies never collected); no `Unloading` hook (4 fail: unload and unload-starting); a
  foreground thread in a collectible copy (7/7 fail: probe never exits); argv without
  `-exit-with-parent` (9/9 orphan tests fail); stop-all re-stopping a stopping entry (state-machine
  race fails); non-idempotent `DisposeAsync` (5 race tests fail). The built-in leak control is the
  orphan detector's own control. Cleanup: no `sysml-grpc`, probe, sibling or `FakeService` process
  (`pgrep`, `/proc` in the container) and no `opensysml-*` temp directory after any run, including
  the deliberately failing ones. Not run: Windows anywhere, hosted CI, G3, G6's full `go test ./...`
  (no Go source changed), the Python/Java/Rust lock-holder prerequisites (no lock interop test
  exists until cache), and a non-ARM Linux or macOS x64 host.
- **Race-test hardening (2026-10-22):** terminal assertions now await the registry's background
  death cleanup, with a deterministic held-stop test proving the wait. `Race.RunAsync` uses a
  thirty-second linked deadline for barrier entry and participating operations, reports unfinished
  parties on timeout, and observes late failures without disposing a barrier still in use.
  `LifecycleRun` owns every connection attempt and resolved child PID; partial acquisition and
  failed assertions release successful connections in asynchronous scope cleanup, with a
  ten-second orderly-cleanup deadline followed by owned-PID kill/wait fallback. Late-resolved
  children are killed too, and cleanup timeouts remain failures. Added 16 Unit cases: six race-runner
  contracts, three cleanup cases on all three wires, and the held-stop case. Current total:
  1058 (947 Unit, 111 Integration), with `DOTNET_MIN_TESTS := 1058`. On macOS arm64, SDK
  10.0.402/runtime 10.0.13, `OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc"
  OPENSYSML_REQUIRE_SERVICE=1 make dotnet-test` passed: locked restore, zero-warning build,
  formatting, 1058/1058 with no skips, sample and 34 gate self-tests. The focused 42 lifecycle,
  orphan and load-context cases passed three consecutive runs, with no skips. `go build ./...`,
  `go vet ./...`, `gofmt -l .` (empty), `python3 scripts/changelog.py check`,
  `python3 scripts/check-doc-ids.py` and `git diff --check` passed. The required-service
  `make dotnet-coverage` rerun passed 1058/1058 with no skips and 2643/2709 handwritten lines
  covered; its initial run failed one Connect-protobuf open with an HTTP 401 authentication
  response from a local endpoint, whose cause was not established. `go test ./...` ran and
  failed in eight untouched Go packages: `cmd/sysml`, `cmd/sysml-grpc`, `internal/doc/docpdf`,
  `internal/exec/analysis`, `internal/frontend/jsonrpc`, `internal/frontend/repl`,
  `internal/translate/codegen` and `tests/codegen`. Failures included macOS temporary-path
  canonicalization differences, an occupied-port shutdown timeout, MIME error-string differences,
  a solver expectation, locale-sensitive output and generated C compilation errors; no Go source
  was changed. Windows and hosted native CI remain unverified by this hardening run.
- **Hosted CI portability follow-up (2026-10-22):**
  [PR #1](https://github.com/richetl/OpenSysML/pull/1), workflow `39836880348`, head `88b37d738`:
  the native macOS arm64 job passed 1058/1058 with zero skips. Linux x64 failed only
  `OverGrpcAnEndpointThatSpeaksOnlyHttp11IsATransportInternalThatSaysSo`; Windows failed
  Python preflight because cp1252 could not encode the success checkmark, before any .NET
  build or test ran. Both causes were reproduced before fixes. The gRPC mapper now recognizes
  `HttpRequestError.VersionNegotiationError`, reports `Internal` regardless of the gRPC library's
  assigned status, preserves the cause, and supplies the same Connect guidance as an HTTP/2
  protocol error. The socket test accepts either typed runtime failure while retaining its exact
  status and diagnostic assertions. Every Python gate's success path now prints ASCII `OK`;
  three new self-tests use a strict cp1252 writer for preflight, coverage and both sample outcomes.
  Two new gRPC mapping cases bring the current total to 1060 (949 Unit, 111 Integration),
  with `DOTNET_MIN_TESTS := 1060`; Python self-tests total 37. Local macOS arm64,
  SDK 10.0.402/runtime 10.0.13: required-service `make dotnet-test` passed 1060/1060 with zero
  skips, sample and 37 self-tests; `make dotnet-coverage` with `PYTHONIOENCODING=cp1252`
  passed 1060/1060 and measured 2650/2715 handwritten lines. The focused transport/network
  selection passed 43/43 on both macOS and an unprivileged Linux arm64 SDK container, using
  read-only source and isolated build outputs. Real cp1252 preflight, formatting, zero-warning
  build, changelog/doc-label/whitespace checks, `go build ./...`, `go vet ./...` and empty
  `gofmt -l .` passed. Windows execution and hosted CI confirmation were pending at this point;
  the successful hosted run below completes that verification.
- **Hosted native-platform verification (2026-10-22):**
  [PR #1](https://github.com/richetl/OpenSysML/pull/1), commit `da57104d5c027389e137f3b72429ccb4a5352f5a`,
  [workflow `39842113694`](https://github.com/richetl/OpenSysML/actions/runs/39842113694):
  all three .NET jobs completed successfully with SDK 10.0.402/runtime 10.0.13, native service
  binaries, `OPENSYSML_REQUIRE_SERVICE=1` and the 1060-test minimum.
  - [Linux x64](https://github.com/richetl/OpenSysML/actions/runs/39842113694/job/120049014185):
    `make dotnet-test` passed 1060/1060 with zero skips, including the sample and 37 Python gate
    self-tests; `make dotnet-coverage` also passed 1060/1060 with zero skips and measured
    2649/2715 handwritten lines.
  - [Windows x64](https://github.com/richetl/OpenSysML/actions/runs/39842113694/job/120046662348):
    preflight, locked restore, warning-free build and the required-service suite passed:
    1060 discovered, 1052 passed, zero failed, eight existing binary-resolver cases skipped.
    Six skips concern link creation, which may require privileges Windows runners lack; two
    concern Unix execute-bit/execute-only modes, which Windows does not provide.
  - [macOS arm64](https://github.com/richetl/OpenSysML/actions/runs/39842113694/job/120046661882):
    preflight, locked restore, warning-free build and the required-service suite passed
    1060/1060 with zero skips.
  No orphan, collectible-load-context or lifecycle race test was skipped on any platform.
  This closes the pending native-platform lifecycle verification and confirms both portability
  fixes on hosted runners.
- **Outcome/prerequisites:** verified ownership/orphan/collectibility across supported OSes;
  lifecycle-local.
- **Read first:** section 3.6 and process-inheritance spike evidence.
- **Owned surface:** OrphanProbe/LoadContextProbe, lifecycle stress tests, native macOS/
  Windows CI jobs and aggregate results.
- **Tests:** kill probe parent while unrelated sibling remains alive; await owned service
  disappearance; load three actual library copies, distinct children, dispose/unload,
  weak-reference collection, no foreground threads/static handler roots; concurrent shutdown.
- **Gate/negative control:** G5 on all three OSes with repeated synchronized race cases;
  deliberately retain stdin in a controlled fixture and ensure the orphan detector fails.
  Every test finally cleans up its own resolved PIDs even on assertion failure.
- **Non-goals/handoff:** no browser promise. Document actual registry scope and native
  platform evidence; an untested platform is not supported by assertion.

### Package 10: suite-core — ✅ DONE (2026-10-22)

- **Status: ✅ DONE (2026-10-22).** Implemented on `feat/dotnet-client` from lifecycle-safety
  baseline `da57104d5`, left uncommitted in the working tree for the maintainer to commit; no schema change, `parity.json` and the
  handwritten public API baseline unchanged (no RPC changes status; every core type is
  `internal` to a separate non-packable assembly). Delivered `src/OpenMBEE.OpenSysML.Conformance`
  (library; references the client for the generated descriptors and `StatusCode`; IVT to both
  test projects): `ScenarioSuite` (strict reader: `*.json` in UTF-8 byte order of names, then
  declaration; refuses unknown/repeated members including inside expected `response` trees,
  wrong kinds, missing id/rpc, neither/both model forms, ids declared twice naming both files,
  non-strict JSON; null members read as absent), `Scenario`/`ScenarioModel`/`Expectation`,
  `FixtureDirectory` (containment by full path), `ScenarioRequests` (whole-string
  `${model_hash}`, `${fixture:…}`, `${fixture_base64:…}`; protobuf-JSON decode by
  `Wire.SysMLService` descriptors), `Normalizer` (descriptor-driven: presence-aware set fields,
  negative zero set, int/uint/real kinds kept distinct, enums by name, bytes as base64,
  `${version}`/`${model_hash}`/`${path}`, first-appearance `@n` labels over field number, UTF-8
  key order and list order), `Checks` (every reference rule and mismatch text, Go `%v`/`%q`
  spellings via `Spelling`), `StatusNames`, the `IScenarioClient` adapter seam
  (`ScenarioAnswer.Answered/Refused/Unexpressible`, `ModelParse`), `ScenarioRunner` (capability
  gating and fallbacks, once-per-run parse cache, per-parse/call deadline → `DEADLINE_EXCEEDED`,
  outer cancellation thrown, injectable `TimeProvider`, reference log lines and verbose
  observation), `Report` records with computed `Totals`, `ReportJson` (reference `-report`
  members and order) and `JUnitReport` (reference `-junit`). Deliberate differences from the
  Go runner, each a reference defect rather than contract: a single fixture and the same
  fixture as a one-document list are separate parse-cache keys (they are different RPCs); bytes
  normalize to protobuf-JSON base64 (Go prints a byte slice); 64-bit integer literals are passed
  to the parser as text because Google.Protobuf 3.36.2 reads JSON numbers through a double
  (observed: `9007199254740993` → `…992`, `uint64` max refused), guarded by a test that the
  schema imports no well-known type; `-0` integer expectations compare as `0`. The parity
  inventory's `Scenario.All` now reads through `ScenarioSuite` (Tests → Conformance project
  reference), removing the second scenario reader. Tests (+188, all Unit; `DOTNET_MIN_TESTS :=
  1248`; 1137 Unit, 111 Integration): `ChecksTests` (pass/fail pairs for named-only comparison,
  relative 1e-9 Reals including a sub-1e-9-absolute case, exact and digit-wise integers incl.
  2^53+1 and `uint64` max, whole-number spellings, defaults of every kind, exact named list
  lengths and order, shape mismatches, wildcards over lists and maps incl. a partial entry,
  contains/contains_all/counts/min_counts, non_empty/absent, enum names, reference scalar
  spelling, stable failure order), `NormalizerTests`, `SpellingTests` (values Go printed),
  `StatusNamesTests`, `ScenarioSuiteTests` (plus ports of the reference suite checks over the
  committed suite: RPCs exist, every RPC covered, both failure kinds, every fixture read, load
  order equals an independent read), `ScenarioRequestsTests`, `ScenarioRunnerTests` (scripted
  adapter), `ReportTests` (JSON/JUnit shapes, a ported JUnit case, and goldens of a two-
  configuration, three-protocol run of `testdata/suite` — report JSON, JUnit and log, files
  named `1-`, `10-`, `2-` — rewritten only with `OPENSYSML_UPDATE_GOLDENS=1`), and a per-project
  category test. MTP enforces `--minimum-expected-tests` on the solution aggregate (observed:
  1129 vs 1130 exits 9). README "Conformance runner core" section; changelog fragment
  `dotnet-client-conformance-core.added.md`. Passed on macOS arm64 (SDK 10.0.402, runtime
  10.0.13): G1 (format, 0-warning `-warnaserror` build, Unit 1137/1137); required-service
  `OPENSYSML_GRPC_BINARY="$PWD/bin/sysml-grpc" OPENSYSML_REQUIRE_SERVICE=1 make dotnet-test`
  1248/1248, 0 skipped, sample and 37 gate self-tests; `make dotnet-coverage` 1248/1248, two
  Cobertura reports, core 1001/1014 lines (uncovered: unreachable default arms, 64-bit map and
  repeated quoting arms no current field reaches, non-string map keys), library 2649/2715
  unchanged; G4 (`make proto-csharp` no Wire drift, digests `--check`, `changelog.py check`,
  `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff --check`); `go build`, `go vet`,
  `gofmt -l` (empty). Linux arm64 (the 1.3 SDK image by digest, `--init`, user `app`, copy of
  the working tree): locked restore, format, 0-warning build, Unit 1137/1137, conformance
  188/188 with byte-identical goldens. Negative controls, each observed failing then restored
  byte-identically (scripted, digest-checked): list length as prefix match and ignored
  (`ANamedListMustHaveExactlyTheNamedEntries`); absolute tolerance (2 tolerance tests), 1e-6
  tolerance, tolerance applied to integers (3 integer tests); default never and always matched
  (`ADefaultExpectationMatchesAnUnsetField`); map entries in insertion order, a fresh label per
  occurrence, ids unlabelled, defaults emitted, no `${path}`, enums by number (each failed its
  normalizer test, and the goldens where they reach them); file order culture-aware or unsorted
  (ordering tests and goldens). Cleanup: no `opensysml-*` temp directory, no `sysml-grpc`/probe
  process, no container of these runs left. Not run: G3 (no adapters or conformance Make
  targets until suite-adapters), G5/Windows/hosted CI (no platform-specific code; pending on
  the next push), G6's `go test ./...` and `make docs-check` (no Go or site change). Handoff to suite-adapters:
  implement `IScenarioClient` per protocol over the public API; the CLI flags, configurations
  and withheld-capability set validation, fallback-execution check, expected-skips allowlist,
  `-mutate`, Make targets and report differential remain that package's.
- **Conformance-core review hardening (2026-10-22):** five correctness fixes, with 85 additional
  Unit cases (53 of the initial regression cases failed before the production fixes). `JsonInteger`
  normalizes signed/unsigned decimal and exponent spellings without floating-point conversion,
  bounds exponent expansion before allocation, preserves the protobuf parser's quoted-number syntax,
  and range-checks requests. Comparisons share that exact normalization, recognize zero exactly,
  reject overflow/underflow and never apply Real tolerance to NaN or infinities; special Reals
  still match their explicit text spellings. These intentionally correct numeric defects inherited
  from the Go comparator rather than preserving them as contract. Both parse and call waits now
  enforce their linked deadline even when an adapter ignores cancellation, observe late faults
  and never cache late parse hashes. Caller cancellation is checked before filtering, skipping,
  parsing or calling and after completion, and propagates with the caller's token. Invalid fixture
  paths become contextual scenario errors so later scenarios still run. JUnit replaces XML-invalid
  text/attribute characters with U+FFFD while preserving valid control characters and surrogate
  pairs; JSON keeps the original valid strings, including NUL.
  Current totals: 273 conformance cases; 1333 solution cases (1222 Unit, 111 Integration),
  `DOTNET_MIN_TESTS := 1333`. On macOS arm64, SDK 10.0.402/runtime 10.0.13:
  required-service `make dotnet-test` passed 1333/1333, zero skips, including locked restore,
  zero-warning build, formatting, sample and 37 gate self-tests. `make dotnet-coverage` passed
  1333/1333 with two accepted reports; the union of source lines across them is core 1061/1074
  and client library 2649/2715. The conformance selection passed 273/273 three consecutive
  runs with `OPENSYSML_UPDATE_GOLDENS=0`, retaining the existing report/log goldens unchanged.
  A Linux arm64 SDK container built the copied working-tree sources and passed all 273
  conformance cases with zero skips. `go build ./...`, `go vet ./...`, empty `gofmt -l .`,
  changelog/doc-label checks and `git diff --check` passed. No wire schema, parity inventory
  or public client API change. Full Go tests were not rerun for these C# changes; the previously
  recorded local failures remain outside their scope. Windows/hosted CI confirmation of this
  uncommitted conformance-core implementation remains pending.
- **Outcome/prerequisites:** faithful scenario loading, comparison, normalization and report
  engine independent of a complete facade; scaffold, wire.
- **Read first:** conformance README and reference runner loading/check/report tests.
- **Owned surface:** Conformance project/core/tests; placeholder expansion, JSON/JUnit schema.
- **Tests:** every comparison rule has pass/fail pairs, including relative 1e-9 real
  tolerance, exact named-list lengths, defaults, wildcard paths, canonical enum/status,
  contains/counts and deterministic first-appearance runtime-ID normalization.
- **Gate/negative control:** G1; fixture ordering and normalized report goldens; deliberately
  corrupt list length, real tolerance and default handling and require their tests to fail.
- **Non-goals/handoff:** no raw-wire scenario execution disguised as public facade coverage.
  Core supports adapters without requiring private-child lifecycle.

### Package 11: suite-adapters — ✅ DONE (2026-10-22)

- **Status: ✅ DONE (2026-10-22).** Built on `feat/dotnet-client` at suite-core `1f270f03a` (baseline
  `a662c2bce` plus every earlier package), left **uncommitted** for the maintainer; no schema change,
  `parity.json` unchanged (no RPC changes status). `OpenMBEE.OpenSysML.Conformance` is now an `Exe`
  (both test projects still reference it). Delivered: `ApiScenarioClient : IScenarioClient` over one
  `Connection` (adapters by bare RPC; a request field outside the adapter's `Fields` is unexpressible as
  `the public API has no way to send <field>`; `ServiceException` → `Refused(status, ServiceMessage)`,
  `CapabilityException` → `Refused(Unimplemented)` since the client refuses before sending what the
  service would, `TransportException` → `Refused(its status)` as the reference treats transport errors;
  `ParseAsync` via `LoadContentAsync`/`ParseSourcesAsync` with fixture-named inline documents);
  `RpcAdapter.Of<TRequest>(method, fields, call)`, `ApiAdapters.All` (frozen, built from area lists),
  `ApiAdapters.Uncovered`/`ModelOf`, `UnexpressibleRequestException`; areas `ConnectionArea`
  (GetServerInfo from the kept `ServerInfo`, capabilities in code-point order, `Answered=false` →
  `UNIMPLEMENTED`) and `ModelArea` (ParseFile, ParseSources, GetSymbol, GetDiagnostics, Evaluate,
  Instantiate; in-band `ModelException` rendered as `error` + diagnostics; empty wire strings → null
  options); `Rendering` (public types only: Diagnostic with raw severity, Symbol, Instance, every Value arm,
  Quantity, UnitTerm; undetermined `count` only when a bound is set); `Mutation`/`Mutations` +
  `MutatingScenarioClient` (only set fields; map values mutated, never keys or entries); `ExpectedSkips`
  (strict reader of `test/testdata/expected-skips.json`: `uncovered_rpcs` = the 24 deferred RPCs,
  `unexpressible` = `parse/naming_no_source_is_invalid`; `Problems` against suite, schema, adapters);
  `SkipAccounting` (`Skips`: unlisted, changed-reason and capability skips not explained by a withheld
  capability fail, `-allow-skips` accepts any skip but never a listed skip that ran; `Fallbacks`: each
  withheld capability needs an executed fallback, or a note when every fallback is a listed skip, which is
  `oslc_query` until Query lands; `WithheldSet`: exact set comparison); `RunArguments`/`RunOptions`
  (`-binary`, `-service`, `-dir`, `-expected-skips`, `-run`, `-protocols`, `-transport`,
  `-withhold-capabilities`, `-report` (`-` = stdout), `-junit`, `-allow-skips`, `-mutate`, `-v`, `-h`;
  `-dir`/`-expected-skips` default to the checkout found upward; unknown, repeated, valueless or
  contradictory flags exit 2); `ConformanceRun` (configurations default + `without-…`, one started
  service per configuration with `-cache-size` scenarios+16 as the reference, one connection per protocol,
  reports, exit 0/1); `Program`. `ServiceProcess` moved from the tests (cache size, bounded output tail;
  `ServiceFixture` uses it). `Normalizer.IsSet` and `ScenarioSuite.Strict/Members/Kind` became internal for
  reuse. Client fix found by the suite: `Model.SymbolAsync` dropped the service's in-band text; it now
  throws `SymbolNotFoundException` with it through new public ctor `(string name, string message, string
  method)` (PublicAPI.Unshipped +1); the unit test that pinned the client-composed text was wrong against
  `ModelException`'s documented contract and Java's behavior and was corrected. Make:
  `conformance-dotnet`, `conformance-dotnet-check` (reference + .NET runs with `-v`, then
  `dotnet-gates.py conformance-check`: report member order and totals for both, JUnit totals/suites/cases,
  per-scenario outcome/status/missing capabilities/without-capability and normalized response, reviewed
  skips excluded, capabilities compared as a set, timing and `service` ignored), `CONFORMANCE_WITHHELD`
  shared with `conformance`/`conformance-mcp`, `DOTNET_CONFORMANCE_SERVICE` override,
  `DOTNET_MIN_TESTS := 1468`. CI: PR `dotnet-test` runs both targets (setup-go for the reference) and
  uploads reports; CircleCI `dotnet-test` runs both and stores reports `when: always`. Results per
  protocol: default 51 run/51 pass, 142 uncovered, 1 unexpressible; withheld 50/50, strict fallback
  `UNIMPLEMENTED` on all three protocols. Tests (+135; 1468 = 1347 Unit + 121 Integration; conformance
  project 324): `ConformanceRenderingTests` (samples set every field reachable from SymbolInfo, Instance,
  Diagnostic, Value; client decode then render is identity, every concrete Value type), `ConformanceAdapterTests`
  (three wires: typed call sends exactly the scenario request and the whole answer comes back; in-band
  failures; refusals; capability refusals send nothing; malformed answer; 11 unexpressible reasons with
  zero requests; ServerInfo; ParseAsync; adapter `Fields` = inventory `typed` request fields; adapters =
  implemented rows and list = deferred rows; IL scan proves no `Connection.CallAsync`/stub call, with a
  positive control), `ServiceConformanceTests` (live: full run with exact skips and fallbacks, each
  mutation fails every protocol, grpc-only transport, `-service`, missing and stale list entries,
  uncovered adapter, adapter type fault, the built executable via apphost with `-report -`),
  `ConformanceRunnerProcessTests`, `MutationsTests`, `SkipAccountingTests`, `RunArgumentsTests`;
  gate self-tests 37 → 52. Passed (macOS arm64, SDK 10.0.402, runtime 10.0.13): G1 (format, 0-warning
  build, Unit 1347/1347); G2 `OPENSYSML_GRPC_BINARY=… OPENSYSML_REQUIRE_SERVICE=1 make dotnet-test`
  1468/1468, 0 skipped, sample, 52 gate tests; `make dotnet-coverage` (new runner files 882/927 lines; the
  uncovered are unreachable default arms, the float perturb arm, the withheld-set early return and moved
  `ServiceProcess` error paths); G3 `make conformance-dotnet` and `make conformance-dotnet-check` ("303
  results agree; 858 reviewed skips excluded"), reference `go run -C tools ./cmd/conformance -binary …
  -withhold-capabilities strict_conformance,oslc_query -v` passed; G4 (`make proto-csharp` no Wire drift,
  digests `--check`, `changelog.py check`, `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff
  --check`); `go build`, `go vet`, empty `gofmt -l`, `go test ./tests/hygiene/...`. Linux arm64 (SDK image
  by digest, `--init`, user `app`, copy of the tree, `CGO_ENABLED=0` linux/arm64 service): locked restore,
  format, 0-warning build, conformance tests 324/324, new main-project and model tests 154/154 live, runner
  exit 0 on all six runs. Negative controls (scripted, each restored byte-identically by digest, then a
  clean rebuild): client-composed symbol message (12 failing, incl. the live suite), FeatureValue
  `order_dependent` not rendered (4), undetermined count always rendered (1), unsendable-field guard off
  (1), capability refusal skipped instead (4), listed skip that ran accepted (2), skipped fallback counted
  as executed (1), GetSymbol sent through `CallAsync` (IL test), truncation only at the top level (1),
  withheld set unchecked (1); `-mutate perturb-reals/truncate-lists/rewrite-strings` failed 13/16/24
  scenarios per protocol, exit 1; a perturbed response or status in the differential's inputs exited 1.
  An independent review found only CircleCI artifacts not stored on failure (fixed). Cleanup: scratch under
  `/tmp/p11` and the session folder only; no `sysml-grpc` process or `opensysml-*` temp directory left.
  Not run / failed outside scope: `go test ./...` fails locally in `cmd/sysml`, `cmd/sysml-grpc` (occupied
  port), `internal/doc/docpdf`, `internal/exec/analysis` (`/var` vs `/private/var`),
  `internal/frontend/jsonrpc`, `internal/frontend/repl`, `internal/translate/codegen`, `tests/codegen`, as
  recorded before, and `tools/cmd/conformance`'s `TestStartServiceReportsAProcessThatExits` (`/bin/false`
  absent on macOS); no Go source changed. Windows and hosted CI pending the next push; `make docs-check`
  (no site change). Handoff: each service-area package adds an `XxxArea` with `RpcAdapter.Of` whose
  `Fields` equal its request's `typed` inventory fields, registers it in `ApiAdapters.All`, renders new
  result types in `Rendering` (extend `CompleteWire` so the reachable-field test stays complete), removes
  its RPC from `uncovered_rpcs` in the same change that flips its inventory row, lists any unexpressible
  scenario with its exact reason, and raises `DOTNET_MIN_TESTS`. query-docs must make the `oslc_query`
  fallback execute, after which the withheld configuration's note disappears.
- **Conformance runner validation fixes (2026-10-23):** value-taking flags refuse a following
  option instead of consuming it and silently running no scenarios; `-flag=value` preserves literal
  dash-prefixed values and embedded equals signs, while switches refuse attached values and duplicate
  flags are detected across both spellings. JSON and JUnit destination failures retain their
  filesystem cause, identify the format and full destination, and exit 1 rather than terminate with
  an unhandled exception. The differential checks each JUnit testcase's disposition against JSON,
  including missing, incorrect, duplicate and conflicting outcome elements; the stand-in reports now
  emit the dispositions their results declare. Added 30 .NET cases and four Python gate tests:
  current solution total 1498 (1373 Unit, 125 Integration), conformance project 346;
  `DOTNET_MIN_TESTS := 1498`. Before the production fixes, all 30 new .NET cases failed and the
  JUnit negative controls were accepted. On macOS arm64, SDK 10.0.402/runtime 10.0.13,
  required-service `make dotnet-test` passed 1498/1498 with zero skips, formatting and the
  zero-warning build passed, and all 56 Python gate tests passed. `make conformance-dotnet-check`
  passed with the unchanged 303 agreeing results and 858 reviewed skips. Process-level negative
  controls verified missing values exit 2 before any run, both invalid report destinations exit 1,
  and removing all 858 JUnit skip markers fails the differential with exit 1.
- **Outcome/prerequisites:** scenarios run through public APIs with exact skip accounting;
  suite-core, model, connect.
- **Read first:** Java conformance rendering and Rust conformance adapters, reference CLI.
- **Owned surface:** adapter files by RPC area, domain response rendering, expected-skips,
  conformance Make targets, JSON/JUnit artifacts and report differential.
- **Tests:** decode scenario requests using Wire but invoke typed methods and render domain
  data; no raw CallAsync/Wire-response shortcut; exact unexpressible-request skip IDs/reasons;
  implemented RPCs never skip expressible scenarios; missing claimed coverage fails.
- **Gate/negative control:** G1/G2/G3; perturb-reals/truncate-lists/rewrite-strings each
  cause at least one mismatch and nonzero exit per applicable protocol. Withhold strict/
  OSLC capabilities and execute fallback expectations, not capability skips.
- **Non-goals/handoff:** no claim that initial adapters cover undelivered RPCs.
  Flags: -binary, -service, -run, -protocols, -report, -junit, -allow-skips, -mutate, -v,
  -transport, -withhold-capabilities. Missing/unknown options fail explicitly.

### Package 12: execution — ✅ DONE (2026-10-23)

- **Status: ✅ DONE (2026-10-23).** Built on `feat/dotnet-client` at suite-adapters `5c5b3eb24`, left
  **uncommitted** for the maintainer; no schema or Go change. Delivered (PublicAPI.Unshipped +405 lines):
  `Model.ExecuteActionAsync`/`ExploreActionAsync(actionId, inputs?, RunOptions?)`,
  `ExecuteStateAsync`/`ExploreStateAsync(stateMachineId, events?, RunOptions?)`,
  `CheckActionAsync(actionId, inputs?, CheckOptions?)`/`CheckStateAsync(stateMachineId, CheckOptions?)`
  (the inventory's `Model.CheckBehaviorAsync` became these two: the request names exactly one behavior and
  inputs bind only to an action, so one method would need a discriminator; parity row methods updated);
  `RunOptions` (Schedule, PerformerSymbolId, Trace, Advance, Seed `ulong?`, Draws, ClockStep, `Explores`,
  const `Explore`), `ActionRun`, `StateRun`, `Exploration` (incl. `Unenumerated`, which Java drops),
  `Outcome`, `ProbabilityRange`, `CheckOptions`, `CheckResult` (+ `CheckSearch`, `CheckViolation`,
  `CheckDivergence`, `CheckDivergentValue`, `CheckFinal`, `CheckWitness`), `Standing`/`Bound` (shared:
  verification must reuse them), `ExecutionException` (FinalTime, Trace, TraceDropped; no `Reason`, since no
  execution response carries a failure reason) and `CheckException(Partial)` (the service answers verdict
  `undecided` beside the error; Java's adapter fabricates it, here it is decoded). Shared read-side
  `DocumentValue` family (Element with nullable Id for the service's kindless anonymous values, Object,
  Text, Number wrapping `Value.NumberValue`, Boolean, Infinity, Quantity, Verdict, State, Event) with
  `DocumentObject`/`DocumentVerdict`/`DocumentState`/`DocumentEvent` (the object a record or row is about is
  `Subject`: CA1720 refuses `Object`); an unknown arm is `UnsupportedValueException`, `element_type` on a
  non-element arm or a non-element where the schema holds one is malformed. Gates in the service's order
  after shape: schedule, schedule_explore (`explore`/`explore:` prefix), state_trace, performer, input value
  gates; check, performer, engines (engine set), schedule_explore (engine trims to `explore`), values.
  Local refusals: empty option strings, execute↔explore schedule mismatch, trace on an action or an
  exploration; numeric bounds, budgets and draw policies stay the service's to refuse (as every other
  client), so `check_behavior/a_negative_depth_bound_is_invalid` stays expressible. Answers of the wrong
  shape (exploration to a run, run fields or no status under explore) are `TransportException(Internal)`.
  A derived `StateRun.FinalState` was removed after review: the last state entered is not where a parallel
  machine rests (`dontWalk+green`), and the schema carries no field to read it from. `ValueWire` exposes
  its quantity/big-integer/rational readers internally. Conformance: `ExecutionArea` (run vs explore by the
  request's schedule; neither behavior is `CheckActionAsync("")`, which sends the same request),
  `RequestValues` (request `Value`s read through public constructors; non-canonical spellings
  unexpressible), rendering of every new type, and `InvalidRequestException` → unexpressible with its
  message. `expected-skips.json`: 21 uncovered RPCs, no new unexpressible scenario. Inventory: three rows
  implemented; every field of the execution/check requests and responses, Outcome, ExplorationStatus,
  ProbabilityRange, Check*, Bound and the five Document* messages typed with tests. Tests +113 (1611 =
  1465 Unit + 146 Integration): `ModelExecutionTests`, `ModelCheckTests`, `DocumentValueTests`,
  `ConformanceExecutionRenderingTests` (samples set every reachable field; read→render identity),
  `ConformanceAdapterTests` rows, live `ServiceExecutionTests` (21 over three wires, incl. a service
  withholding schedule/schedule_explore/performer/state_trace/check); `DOTNET_MIN_TESTS := 1611`. Passed on
  macOS arm64 (SDK 10.0.402, runtime 10.0.13): required-service `make dotnet-test` 1611/1611, 0 skipped,
  format, 0-warning build, sample, 56 gate self-tests; `make dotnet-coverage` (new files 849/854 lines; the
  rest an unreachable default arm and the defensive request-reader catch); G3 `make conformance-dotnet`
  (per protocol default 70 run/70 pass, 123 uncovered, 1 unexpressible; withheld 69/69),
  `make conformance-dotnet-check` ("417 results agree; 744 reviewed skips excluded"), `make conformance`
  (192/193 per protocol, its 1 listed skip); G4 (`make proto-csharp` no Wire drift, digests `--check`,
  `changelog.py check`, `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff --check`); `go build`,
  `go vet`, empty `gofmt -l`, `go test ./tests/hygiene/...`. Linux arm64 (SDK image by digest, `--init`,
  user `app`, copy of the tree, `CGO_ENABLED=0` linux/arm64 service): locked restore, format, 0-warning
  build, 1611/1611 required-service, 0 skipped. `-mutate` per protocol: perturb-reals 16,
  truncate-lists 20, rewrite-strings 31 failures, exit 1. Negative controls (scripted, each restored by
  digest): trace dropped from a run (7 failures) and from a failed run (6), counterexample witness dropped
  (4) or not rendered (4), explore schedule accepted by a run (3), check gate skipped (3), exploration
  accepted as a run (3), unknown document arm read as anonymous (1), check refusal as a plain
  ModelException (6); live runner with the trace dropped fails `execute_state/a_requested_trace_reports_transitions`
  and with the witness dropped the divergence check scenario on all three protocols, exit 1;
  `--minimum-expected-tests 1612` exits 9. Cleanup: scratch under `/tmp/p12*` removed, no container,
  `sysml-grpc` process or `opensysml-*` temp directory left. Not run: Windows and hosted CI (pending the next
  push); `go test ./...` and `make docs-check` (no Go or site change). Handoff: verification reuses `Standing`
  and `Bound`; query-docs adds `DocumentValue` writing (bindings) and the row types on top of the read side
  here; `parity.json` is 125 KiB and projects to ~207 KiB once every field is classified, over the
  repository's 200 KiB fixture cap, so a later package must make the field format more compact rather than
  allowlist it.
- **Response validation:** `CheckActionAsync` and `CheckStateAsync` reject an answer with neither
  a verdict nor an error as `TransportException(Internal)`. An in-band error still raises
  `CheckException` with its partial result unchanged, even when the verdict is absent.
  `ModelCheckTests` covers both paths through both public methods on all three wires (+6 tests;
  `DOTNET_MIN_TESTS := 1617`). Required-service `make dotnet-test` passed on macOS arm64:
  1617/1617 tests, zero skipped, zero-warning Release build, format, sample and 56 gate self-tests.
  `go build ./...`, `go vet ./...`, empty `gofmt -l .`, `changelog.py check`,
  `check-doc-ids.py` and `git diff --check` passed. `go test ./...` failed in `cmd/sysml`,
  `cmd/sysml-grpc`, `internal/doc/docpdf`, `internal/exec/analysis`, `internal/frontend/jsonrpc`,
  `internal/frontend/repl`, `internal/translate/codegen` and `tests/codegen`; failures include
  macOS temporary-path aliases and a ten-minute timeout in `TestCompiledStepBudgetMatchesInterpreter`.
  No Go source changed.
- **Outcome/prerequisites:** action/state execution, exploration and CheckBehavior; model,
  values, suite-adapters.
- **Read first:** groups 06/19, current Java behavior-check facade, execution proto messages.
- **Owned surface:** Model.Execution.cs, execution/check options/results/converters and
  its conformance adapter; ownership excludes shared Model core.
- **Tests:** schedules/performer/trace gates; execute versus explore refusal; complete and
  bounded exploration outcomes; final time; trace/dropped data; check properties, divergence,
  witness/replay fields; unbound/wrong-kind/missing-symbol/step-budget typed failures.
- **Gate/negative control:** G1/G2/G3 on groups 06/19 plus inventory. Deliberate trace/
  counterexample omission must fail rendering checks; no robustness case may hang.
- **Non-goals/handoff:** no local execution engine. Update all covered field classifications,
  including CheckBehavior option gates resolved in wire.

### Package 13: verification — ✅ DONE (2026-10-23)

- **Status: ✅ DONE (2026-10-23).** Built on `feat/dotnet-client` at execution `ae2a6b914`, left
  **uncommitted** for the maintainer; no schema or Go change. Delivered (PublicAPI.Unshipped +393 lines,
  appended as this package's sorted block; the old `Exploration` constructor line replaced):
  `Model.VerifyConstraintAsync`/`VerifyRequirementAsync(symbolId, VerifyOptions?)` -> `Verification`;
  `VerifySatisfactionAsync(scope?, VerifyOptions?)` -> `Satisfaction` and `SatisfiedAsync` -> `bool`;
  `ValidateInstanceAsync(symbolId, ValidateOptions?)` -> `Validation`; `CalcAsync(symbolId, positional
  arguments?, CalcOptions?)` -> `Calculation` (positional only, as the schema); `RunAnalysisAsync`/
  `ExploreAnalysisAsync(symbolId, AnalysisOptions?)` -> `Analysis`/`Exploration`; `RunSweepAsync(symbolId,
  SweepRange list, SweepOptions?)` -> `Sweep`; `Connection.ListEnginesAsync(ListEnginesOptions?)` ->
  `ValueList<EngineInfo>`. Records: `Verdict` (all 17 fields, `Decided`/`Violated`), `WitnessAssignment`,
  `VerificationVerdict`, `CalcOutput` (ordered; a valueless output is malformed), `CaseEvaluation`,
  `ToolDryRun`(+`File`/`Input`/`Output`), `SweepRow` (`ElapsedMicroseconds`, `Failed`, `Holds`), `EngineInfo`,
  `FailureReason` (unknown numbers kept). Reuses `Standing`/`Bound` (adds `Standing.None`); `Exploration`
  gains `Standing` (behavior explorations report `Standing.None`), since an analysis exploration names its
  engine. Exceptions: `VerificationException(Reason, Instances, Verifications, Standing)` for a request the
  service could not answer, `AnalysisException : VerificationException` with `Partial`. The planned
  `WrongKindException` is not a subclass: a wrong-kind analysis failure can carry a partial run and C# has
  single inheritance, so the kind is `Reason == FailureReason.WrongKind`; a wrong-kind *verdict* (verdict-level
  error) is an undecided verdict returned, not raised (Java's behavior; lossless for rendering). A false or
  undecided verdict and a failed sweep row are never exceptions. Malformed (`TransportException(Internal)`):
  no verdict/summary/result and no error; an error beside verdict(s), summary, bounded, result/outputs,
  rows/parameters/sampled/seed, or outcomes/exploration; an analysis run/explore answered in the other shape.
  Gates in the service's order: verification -> verification_questions (holds/satisfiable only; any other
  question is sent for the service to judge) -> engines -> schedule_explore (engine `explore`, trimmed) ->
  verification_arguments -> value gates (positional, named, then range start/end/step); a case run checks
  schedule/schedule_explore first. Local refusals: empty option strings, a subject or arguments on a
  satisfaction, run vs explore schedule/engine mismatch, a tool dry run under explore. Conformance:
  `VerificationArea` (RunAnalysis explores when `AnalysisOptions.Explores`), rendering of every new type,
  `RequestValues.List`; `expected-skips.json`: 13 uncovered RPCs and three new unexpressible EvaluateCalc
  scenarios (measurement ref without reduction, non-numeric vector component, function closing over an
  object). `dotnet-gates.py conformance-check` leaves out exactly `RunSweep` `rows.*.elapsed_micros` (the only
  wall-time field of these responses) with 3 new gate tests (59 total). Inventory: eight rows implemented,
  187 fields typed with tests. Tests +160 (1777): `ModelVerificationTests` (34), `ModelAnalysisTests` (40),
  `ConformanceVerificationRenderingTests` (5; samples set every reachable field; read->render identity),
  `ConformanceAdapterTests` rows (+57), live `ServiceVerificationTests` (24 over three wires, incl. a service
  withholding verification, verification_questions, verification_arguments and engines);
  `DOTNET_MIN_TESTS := 1777`. Passed on macOS arm64 (SDK 10.0.402, runtime 10.0.13): required-service
  `make dotnet-test` 1777/1777, 0 skipped, format, 0-warning build, sample, 59 gate self-tests;
  `make dotnet-coverage` (new files 891/891 lines); G3 `make conformance-dotnet` (per protocol default
  116 run/116 pass, 74 uncovered, 4 unexpressible; withheld 115/115), `make conformance-dotnet-check`
  ("693 results agree; 468 reviewed skips excluded"), `make conformance` (194/194; 192/193 with its listed
  skip); G4 (`make proto-csharp` no Wire drift, digests `--check`, `changelog.py check`,
  `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff --check`); `go build`, `go vet`, empty
  `gofmt -l`, `go test ./tests/hygiene/...`; `make docs-check` and the strict `make docs` site build (after
  `make docs-landing-assets`, mkdocs in a throwaway venv). `-mutate` per protocol: perturb-reals 26,
  truncate-lists 37, rewrite-strings 58 failures, exit 1. Negative controls (scripted, each restored by
  digest): witness dropped (20 failures), a failed analysis's partial erased (6), a false verdict raised
  (18, live included), a failed row's outputs dropped (7), a failed row raised (9, live included), engines
  checked before verification (3), rendering forgetting case evaluations (7), an analysis exploration's
  standing dropped (6); live runner with violated verdicts dropped fails two RunAnalysis scenarios on all
  three protocols, exit 1; `--minimum-expected-tests 1778` exits 9. Not passed: `go test ./...` fails in
  `cmd/sysml`, `cmd/sysml-grpc`, `internal/doc/docpdf`, `internal/exec/analysis`, `internal/frontend/jsonrpc`,
  `internal/frontend/repl`, `internal/translate/codegen`, `tests/codegen` (the macOS temporary-path aliases,
  fake external tools and ten-minute timeout package 12 recorded) and `tests/wasm` under full-suite load,
  which passes alone (57 s); no Go source changed. Not run: Linux container and Windows (G5). Cleanup:
  scratch under `/tmp/p13` removed, no `sysml-grpc` process left. Handoff: `parity.json` is 154 KiB and
  projects to ~213 KiB once the remaining 492 deferred fields are typed, still over the 200 KiB cap, so the
  field format must be compacted before the packages that type them; sampling reuses `SweepRow` (its
  `RunRuns` rows need the same `elapsed_micros` exclusion added to `TIMING_FIELDS`), `CalcOutput`, `Verdict`,
  `CaseEvaluation`, `Standing` and `VerificationException`.
- **Outcome/prerequisites:** all verify/validate/calc/analysis/sweep/engine methods; model,
  values, suite-adapters.
- **Read first:** groups 09/10/13/14/15 and Java verification/options/partial-result handling.
- **Owned surface:** Model.Verification.cs, Model.Analysis.cs, related options/results/
  converters, ListEngines and area adapter.
- **Tests:** false decided versus error-undecided; questions, arguments, subjects, engines,
  Standing/Bound/WitnessAssignment/ProbabilityRange; case evaluations, AnalysisException.Partial;
  positional/named args only where the request schema supports them (EvaluateCalc is
  positional-only); exploration; sweep ranges/sampling/seed and row-level partial failures;
  ListEngines probe=false never starts an external engine, probe=true reports its handshake.
- **Gate/negative control:** G1/G2/G3 on owning groups; false verdict must not throw; erase
  a partial output or witness and conformance/unit rendering must fail.
- **Non-goals/handoff:** no reimplementation of solving/analysis. Return every declared
  field and preserve failed-row results rather than replacing a sweep with an exception.

### Package 14: sampling — ✅ DONE (2026-10-23)

- **Status: ✅ DONE (2026-10-23).** Built on `feat/dotnet-client` at verification `84cf522f0`, left
  **uncommitted** for the maintainer; no schema or Go change. Delivered (PublicAPI.Unshipped +231 lines,
  appended as this package's sorted block): `Model.RunActionRunsAsync(actionId, runs, inputs?,
  ActionRunsOptions?)` and `RunAnalysisRunsAsync(caseId, runs, AnalysisRunsOptions?)` -> `Runs` (the
  inventory's `Model.RunRunsAsync`/`RunsOptions` became these two, as CheckBehavior did: an action takes inputs,
  a performer and observables, a Monte Carlo case a subject and arguments, and the service refuses each with the
  other, so one method would need a discriminator; parity row methods updated); `SelfCheckAsync(rulePackages?)`
  -> `SelfCheck`; `RecordRunAsync(caseId, RecordOptions?)` -> `Recording`. Records: `Runs` (rows reuse
  `SweepRow`, `Distribution(name)`, `Failures`, `Resolve`), `RunDistribution`, `RunPercentile`,
  `RunHistogramBin`, `RunCountedValue`, `SelfCheck` (status constants, `Holds`), `SelfCheckFinding`,
  `SelfCheckApplication`, `EditedDocument` (shared: editing and conversion must reuse it), `Recording` (exactly
  one of `Run`/`Sweep`/`Sample`; `Model` is a **new** handle on the recorded hash borrowing the connection, the
  original model unchanged, nothing written to disk). Exceptions: `RunsException : VerificationException` with
  `Partial`, because the service answers an error *beside* a full table (a feature no completed run produced;
  Java raises and drops the table); `RecordException : VerificationException` with the run/sweep/sample made,
  `RunFailed` (the runs reported the failure themselves), `Package`/`Definition`/`Records`/`GeneratedSource`
  (records generated but refused), `Skipped`/`SkipReasons`; a self-check that cannot start is a plain
  `ModelException`. A failed run stays a row and a violated self-check constraint a finding. `accept_documents` is
  always sent (the recording is read whole); a request clearing it is unexpressible. Gates in the service's
  order: runs -> performer -> input value gates -> engines/schedule_explore (action); runs -> engines ->
  argument values (case); self_check; record_run -> engines -> argument and range values. Local refusals: empty
  option strings and unsendable values only; run counts, seeds, draw policies, duplicate observables and
  ranges-with-runs stay the service's to refuse, so `run_runs/an_unseeded_random_run_is_refused` stays
  expressible. Malformed (`TransportException(Internal)`): runs with neither rows nor an error; a distribution
  entry without its value; a self-check error beside any answer field, or no status; a recording error beside
  content/documents/model_hash, a runs failure that is not the recording's, a runs failure beside a recording, a
  recording with no hash or no runs, runs of a kind the request did not ask for or of two kinds, an exploration
  as the recorded run. `VerificationWire` gained `Rows`/`At` so nested reads name their path
  (`sample.distributions[0].mean`). Conformance: `SamplingArea` (RunRuns by which behavior the request names;
  subject/arguments with an action, or inputs/performer/observe with a case, are unexpressible), rendering of
  every new type including a `RecordException` back in band with the runs' own failure;
  `expected-skips.json`: 10 uncovered RPCs, no new unexpressible scenario. `dotnet-gates.py conformance-check`
  also leaves out `RunRuns`/`RecordRun` row `elapsed_micros`, replaces only the `runAt = "<UTC>"` time inside a
  recording's `source`/`content`/`documents.*.content`, and compares the recorded `model_hash` only as present,
  absent or the scenario model's own (+4 gate tests, 63 total). Inventory: three rows implemented; every field of
  the RunRuns/SelfCheck/RecordRun requests and responses, RunDistribution/Percentile/HistogramBin/CountedValue,
  SelfCheckFinding/Application and EditedDocument typed with tests. Tests +94 (1871 = 1680 Unit + 191
  Integration): `ModelRunsTests` (31), `ConformanceSamplingRenderingTests` (5; samples set every reachable
  field; read->render identity), `ConformanceAdapterTests` rows (+37), live `ServiceSamplingTests` (21 over three
  wires: seeded replay identical and a different seed different, Monte Carlo conclusion matching the action's
  draws, recordings of a run, sweep, sample and into an existing package queried on the new model and absent from
  the original, a service withholding runs/self_check/record_run); `DOTNET_MIN_TESTS := 1871`. Passed on macOS
  arm64 (SDK 10.0.402, runtime 10.0.13): required-service `make dotnet-test` 1871/1871, 0 skipped, format,
  0-warning build, sample, 63 gate self-tests; `make dotnet-coverage` (new files 504/504 lines; `Rendering.cs`
  559/564, the five pre-existing unreachable default arms); G3 `make conformance-dotnet` (per protocol default
  121 run/121 pass, 69 uncovered, 4 unexpressible; withheld 120/120), `make conformance-dotnet-check` ("723 results
  agree; 438 reviewed skips excluded"), `make conformance` (194/194; 192/193 with its listed skip); G4
  (`make proto-csharp` no Wire drift, digests `--check`, `changelog.py check`, `ci-changed-areas-test.sh`,
  `check-doc-ids.py`, `git diff --check`); `go build`, `go vet`, empty `gofmt -l`, `go test ./tests/hygiene/...`;
  `make docs-check` and, after `make docs-landing-assets`, the strict `make docs` site build, both with
  `PYTHON` pointing at a throwaway Python 3.14 venv of `docs-requirements.txt` (installed as the Makefile does:
  `--only-binary` finds no `watchdog` wheel for 3.14 on macOS; the Homebrew `mkdocs` formula is not importable
  by other interpreters). `-mutate` per protocol: perturb-reals 26, truncate-lists 39, rewrite-strings 62 failures,
  exit 1. Negative controls (scripted, each restored by digest): a distribution dropped when read (20 failures,
  live included), the printed lines not rendered (15), a failed run raised instead of kept as a row (15, live
  included), the runs failure raised without its table (9, live included), the recording answered as the original
  model (15, live included), a runs failure not marked as the runs' (9, live included), value gates checked
  before the performer (3); live runner with distributions and recorded documents not rendered fails
  `run_runs/a_seeded_action_is_run_repeatedly_and_its_observable_distributed` and
  `record_run/a_run_is_recorded_into_the_default_package` on all three protocols, exit 1; each of the four new
  gate normalizations removed or widened fails its gate self-test; `--minimum-expected-tests 1872` exits 9.
  Cleanup: probe and control scripts, the docs venv and `$TMPDIR/p14*` removed; no `sysml-grpc` process or
  `opensysml-*` temp directory left. Not run: Linux container and Windows (G5) and hosted CI (pending the next
  push); `go test ./...` (no Go change; packages 12/13 recorded its pre-existing macOS failures). Handoff:
  `parity.json` is 164 KiB with 395 deferred fields and projects to ~203 KiB once they are typed, still over the
  200 KiB cap, so the field format must be compacted before the remaining packages finish; editing and
  conversion reuse `EditedDocument`; import/edit results should follow `Recording.Model` (a new handle, the
  original untouched); `RecordException`/`RunsException` show the pattern for an in-band failure beside a
  partial answer.
- **Outcome/prerequisites:** RunRuns, SelfCheck and RecordRun; verification, suite-adapters.
- **Read first:** group 20, schema run/record/check messages and Java corresponding methods.
- **Owned surface:** Model.Runs.cs, options/results/converters and area adapter.
- **Tests:** action versus analysis selection, positive run count, seeded draws, observables/
  distributions/conclusions, self-check packages/findings/applications/counts; recording
  single/sweep/sample modes, returned source/documents/model hash and diagnostics.
- **Gate/negative control:** G1/G2/G3 group 20; seeded replay is deterministic where the
  service promises it; record/import-style changes never mutate the original Model.
- **Non-goals/handoff:** no new sampler or evaluation algorithm; no hidden write to disk
  when the service only returns edited content.

### Package 15: conversion — ✅ DONE (2026-10-24)

- **Status: ✅ DONE (2026-10-24).** Built on `feat/dotnet-client` at sampling `cc40b75ca`, left **uncommitted** for the
  maintainer; no schema or Go change. Delivered (PublicAPI.Unshipped +172 lines, appended as this package's sorted
  block): `Connection.ConvertAsync(ConvertSource, toFormat, ConvertOptions?)` -> `Conversion` (`ConvertSource.FromFile`/
  `Inline`/`FromModelHash`, the schema's whole oneof); `Model.ConvertAsync`, `ToNotationAsync`, `ToTurtleAsync`,
  `ToApiJsonAsync` and `SaveAsync(path, toFormat?, ConvertOptions?, SaveOptions?)` (format from the extension by
  `ConversionFormat.OfPath`, an unknown one `InvalidRequestException` before sending); `Connection.MigrateAsync(
  MigrateSource, toFormat, MigrateOptions?)` -> `Migration` (`MigrateSource.FromFile`/`Inline(bytes)`, copied).
  `ConvertOptions` (FromFormat, TolerateSyntaxErrors, `IdForm?` enum, Documents, Compact, OmitDerived, KeepDerived,
  IncludeCheckDiagnostics), `MigrateOptions` (FromFormat, Report, Results, LayoutPath|LayoutContent, ImageBaseUrl,
  Strict), `ConversionFormat` constants, `Conversion` (+`SaveAsync`), `Migration` (+`WriteAsync`, `SourcePath` the
  client-side full path of a file source), `MigrationReport` (+`Total`, `ByVerdict`), `MigrationEntry`, `MigrationFile`
  (bytes as `ReadOnlyMemory<byte>`, value equality), `SaveOptions { Overwrite }`. Exceptions: `ConversionException :
  ModelException` and `MigrationException : ModelException` keep FromFormat/ToFormat/Experimental/ExperimentalNotice
  (the service sets them beside an in-band error, so rendering is lossless); `UnwritableException(Path) :
  OpenSysMLException` for every refusal decided before writing. The plan's "wrong v1/v2 API routing" is the
  service's to refuse (Java and Python refuse locally): the two routing scenarios, the empty target and inline content
  without a form stay expressible and run; tests prove the request is sent and the `InvalidArgument` naming the other
  verb surfaces as `ServiceException`. Local refusals: empty option strings, an undefined `IdForm`, both layouts.
  Gates: convert -> convert_documents -> convert_compact (compact, omit_derived or keep_derived) ->
  convert_check_diagnostics; migrate. NotFound is `ModelFileNotFoundException` for a file or layout path,
  `ModelNotFoundException` for a model hash (`ParseCallAsync` renamed `FileCallAsync`). Malformed
  (`TransportException(Internal)`): an error beside content (Convert) or content/report/results/files (Migrate), a
  success without its formats or (Migrate) its report, an experimental notice on an answer not marked experimental.
  Safe-write helper `Internal/SafeWriter` (section 3.8; editing reuses it): every destination judged first, then
  temp-in-same-directory, flush to disk, `File.Move` (exclusive without overwrite, so a file created meanwhile is not
  replaced), assets before the model; rollback deletes temps, files it created and directories it made; an overwrite's
  replaced file stays replaced (documented, no atomic multi-file claim). Refused: existing file without overwrite, a
  directory, any symbolic link at the destination, a missing directory, a path the file system cannot hold, the v1
  source (by link-resolved landing, case-insensitive on Windows/macOS) whatever overwrite says; asset names that are
  empty, absolute, hold `.`/`..`/empty components, `\` or `:` (drive/UNC), invalid or control characters, a trailing dot
  or space, a Windows device name (incl. `CONIN$`, `CONOUT$`, `COM¹`-`LPT³`, added after review), duplicates or
  file/directory clashes by landing, or any link below the model's directory. Containment holds by construction
  (portable segments, no link below the base); a directory swapped for a link mid-write is not guarded (as
  `sysml -migrate -o`). Conformance: `ConversionArea` (model_hash through `Model.ConvertAsync`, an unknown id_form
  spelling unexpressible), in-band rendering of both exceptions; `expected-skips.json`: 8 uncovered RPCs, no new
  unexpressible scenario. Inventory: two rows implemented, 55 fields typed with tests. Tests +119 (1990 = 1772 Unit +
  218 Integration): `ModelConversionTests` (25), `SafeWriteTests` (31), `ConformanceConversionRenderingTests` (5),
  `ConformanceAdapterTests` rows (+30), live `ServiceConversionTests` (27 over three wires: every format and source,
  id forms, documents, compact form, check diagnostics, tolerated/refused syntax errors, wrong verb, missing file,
  evicted model, full report accounting 97 = entries, results, an in-test `.mdzip` whose image is written beside the
  model, layout by path and content, strictness, `Model.SaveAsync`, a service withholding convert_documents,
  convert_compact, convert_check_diagnostics, migrate and another withholding convert); `DOTNET_MIN_TESTS := 1990`.
  Passed on macOS arm64 (SDK 10.0.402, runtime 10.0.13): required-service `make dotnet-test` 1990/1990, 0 skipped,
  format, 0-warning build, sample, 63 gate self-tests; `make dotnet-coverage` (new files 487/504 lines before the last
  edge tests; the rest the defensive catch arms of link reads and best-effort deletes); G3 `make conformance-dotnet`
  (per protocol default 133 run/133 pass, 57 uncovered, 4 unexpressible; withheld 132/132), `make
  conformance-dotnet-check` ("795 results agree; 366 reviewed skips excluded", no new gate normalization: conversion
  answers are deterministic); G4 (`make proto-csharp` no Wire drift, digests `--check`, `changelog.py check`,
  `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff --check`); `go build`, `go vet`, empty `gofmt -l`,
  `go test ./tests/hygiene/...`; `make docs-check` and, after `make docs-landing-assets`, the strict `make docs` site
  build, with `PYTHON` a throwaway venv of `docs-requirements.txt`. `-mutate` per protocol: perturb-reals 26,
  truncate-lists 41, rewrite-strings 68 failures, exit 1. Negative controls (scripted, each restored by digest): report
  entries dropped when read (13 failures, live included), conversion diagnostics dropped (16), overwrite without being
  asked (12), v1 source unguarded (5), created files kept after a failure (1), temporaries kept (3), `.`/`..` accepted
  (3; the mutated build did write `escape.png` outside the test directory, then removed), a link below the directory
  accepted (1), case variants compared ordinally (2), check-diagnostics gated before compact (6), a migration failure
  beside a report accepted (3), an in-band conversion failure rendered without its formats (4); live runner with
  entries dropped fails `migrate/the_full_report_lists_every_elements_verdict` on all three protocols, exit 1;
  `--minimum-expected-tests 1989` exited 9 at 1988. A code review found only the incomplete device-name list, fixed
  with two theory rows. Cleanup: scratch scripts, the docs venv and `$TMPDIR` test directories removed; no
  `sysml-grpc` process left. Not run: Linux container and Windows (G5; link tests skip on Windows as elsewhere) and
  hosted CI (pending the next push); `go test ./...` (no Go change; packages 12/13 recorded its pre-existing macOS
  failures). Handoff: editing must reuse `SafeWriter.WriteAsync` (its `beforePublish` seam drives failure tests) for
  `EditResult.SaveAsync`, and may reuse `EditedDocument`. `parity.json` is 173 KiB with 340 deferred fields; this
  package's rows cost ~174 bytes per typed field, projecting ~232 KiB once all are typed, so the field format must be
  compacted (integration-owned) before query-docs/editing/graph-data finish.
- **Path-safety hardening (2026-10-24):** macOS source guards, duplicate destinations and
  file/directory collision checks now include Unicode normalization equivalence; Linux and Windows
  preserve Unicode-distinct filenames. Asset names reject all Windows-invalid characters on every
  platform. `MigrateSource.FromFile` rejects relative and invalid paths with `ArgumentException`,
  normalizes a fully-qualified path once, and uses that exact path for the RPC and `SourcePath`;
  there is no longer a nullable fallback that silently drops source protection. Convert file sources
  and migration layout paths retain their service-relative behavior. The conformance adapter marks
  invalid, relative or noncanonical migration paths unexpressible without rewriting the request.
  Added 17 test cases, including the six portable-character cases, Unicode source/asset/model/parent
  aliases and the absolute-path contract over all three wires; the 339 focused conversion, safe-write,
  adapter and live-service cases pass without skips. `DOTNET_MIN_TESTS` is 2007.
  Required-service `make dotnet-test` passes all 2007 cases with no skips, formatting changes,
  build warnings or errors; the sample and 63 gate self-tests pass. `make conformance-dotnet-check`
  reports 795 matching results and 366 reviewed skips, with no new suite skips.
  `go build ./...`, `go vet ./...`, empty `gofmt -l .`, changelog and documentation-identifier
  checks pass. The full `go test ./...` run fails in eight unchanged Go packages (CLI engines,
  service lifecycle, PDF rendering, engine manifests, JSON-RPC headers, REPL optimization and
  code generation), including a ten-minute compiled-calculation timeout; the runtime, corpus and
  hygiene packages pass. No Go source changed. Linux and Windows execution remain unverified.
- **Outcome/prerequisites:** Convert/Migrate, model serialization and safe saving; model,
  suite-adapters.
- **Read first:** groups 07/16, current conversion/migration options and write helpers.
- **Owned surface:** Model.Conversion.cs, conversion/migration types, safe-write helper,
  converters and area adapter.
- **Tests:** file/inline/held model; sysml/kerml/turtle/api-json, IdForm, selected documents,
  compact/derived controls and diagnostics; wrong v1/v2 API routing; .mdzip/.xmi/.uml bytes,
  report/results/layout/image URL/strict options and complete verdict accounting.
- **Gate/negative control:** G1/G2/G3 groups 07/16; section 3.8 overwrite/cancel/traversal/
  symlink/UNC/case/partial-write tests. Refused writes preserve source and existing files.
- **Non-goals/handoff:** no client-side conversion. Other packages reuse the safe-write
  contract rather than implement their own traversal/overwrite policy.

### Package 16: query-docs — ✅ DONE (2026-10-24)

- **Status: ✅ DONE (2026-10-24).** Built on `feat/dotnet-client` at conversion `693cfbaf9`, left **uncommitted**
  for the maintainer; no schema or Go change. Delivered (PublicAPI.Unshipped +331 lines, appended as this package's
  sorted block): `Model.QueryAsync(Query)` and `QueryOslcAsync(text)` -> `QueryResult` of `QueryElement` (Id, Type,
  `ValueDictionary<string>` Properties); `Query` an immutable record (Scope, Select, Where; `Query.All`, `with`);
  `QueryConstraint` sealed arms `Comparison` (Property, `ComparisonOperator` Equal/Greater/Less with no unset member,
  Values, Inverse, `Inverted()`) and `Composite` (`CompositeOperator` And/Or, Constraints), factories
  `Equal/Greater/Less/And/Or`; a constraint without operator, operands (`=` >= 1, `>`/`<` exactly 1) or constraints
  cannot be built, and the two query forms are separate calls. `Model.RunDocumentQueryAsync(queryId,
  IEnumerable<DocumentBinding>?)` -> `DocumentQueryResult` (Columns, `DocumentRow` Element + cells); `DocumentBinding`
  keeps call order and repeats, refuses answered-only arms (infinity, verdict/state/event rows), anonymous elements
  and objects with neither path nor positive id; `DocumentValue.ElementRef/ObjectRef(path|id)` factories; an answered
  element/object rebinds as answered. `Model.RenderDocumentAsync(id, DocumentForm)` -> `RenderedDocument` (Form,
  Content, FileName null when unnamed; wrong-form content malformed). `Model.RenderViewAsync(view,
  RenderViewOptions{Ports, Show, Depth>0})` -> `RenderedView` (nodes parents-first with `Roots/ChildrenOf/Node`, ports,
  edges with route/style, columns/rows, canvas, notes, notices, shown) with `ViewNode/ViewEdge/ViewPort/ViewGeometry/
  ViewCanvas/ViewStyle/ViewPoint/ViewRow/ViewNote`, `ViewNotation` constants; absent geometry/style/canvas/origin stay
  null. Markdown and minimal ports are sent as the empty default (explicit `markdown`/`minimal` unexpressible in
  conformance). Gates: query -> oslc_query; document_query -> big_int_values -> rational_values (exact-double
  rationals sent as reals first, service predicates incl. quantity magnitudes); render_document ->
  render_document_html; render_view. Failures: `QueryException : ModelException` (StatusCode, ServiceMessage, inner
  exception kept via a new additive `ModelException(message, method, diagnostics, inner)` ctor) for a
  `FailedPrecondition` of RunDocumentQuery/RenderDocument, the service's "fault in the model's queries" — the plan's
  model-level query failure, kept as a model error rather than a transport one; `ModelNotFoundException` only for the
  service's exact `model not found: <hash>` message, so an unknown query/document/view or unheld object stays
  `ServiceException(NotFound)`. Malformed (`TransportException(Internal)`): element without id/type, row without
  element, node whose parent is not drawn before it, duplicate node id. `Model.Symbols` now uses the public
  `QueryAsync` (private wire-level helper removed). Conformance: `QueryDocsArea` + lossless `Rendering`;
  `QueryException` rendered as its refusal; `expected-skips.json`: 4 uncovered RPCs left, +2 reviewed unexpressible
  (`query/an_unset_operator_fails_the_call`, `query/naming_both_query_forms_is_invalid`). Inventory: four rows
  implemented, 110 fields typed. Tests +118 (2125): `ModelQueryDocsTests` (36, over three wires),
  `ConformanceQueryDocsRenderingTests` (6, every reachable field set and round-tripped), `ConformanceAdapterTests`
  rows (+49: exchanges, unexpressible requests, refusals), live `ServiceQueryDocsTests` (27: every constraint form, OSLC,
  element/object/verdict/state/event rows, rebinding, Markdown/HTML + FileName, ports/show/depth, layout geometry,
  styles, routes, notes, canvas absent/present, tables, folded/unplaced nodes, missing model vs declaration vs
  `QueryException`, withheld capabilities sending nothing, plus sweeps reading every conformance fixture's query
  answer (487 elements) and 100+ views of `internal/ir/view/testdata` without a malformed-answer refusal); two tests
  whose premise was "Query uncovered" were updated to keep their intent (`SkipAccountingTests` treats Query as
  uncovered locally; `ServiceConformanceTests` now asserts the oslc_query fallback ran, the note path stays unit
  tested). `DOTNET_MIN_TESTS := 2125`. Passed on macOS arm64 (SDK 10.0.402): required-service `make dotnet-test`
  2125/2125, 0 skipped, format, 0-warning build, sample, 63 gate self-tests, digests; `make dotnet-coverage` (new
  files 807/812 lines before the last two equality assertions; the rest unreachable defaults of closed hierarchies);
  G3 `make conformance-dotnet` (per protocol default 151/151 run/pass, 37 uncovered, 6 unexpressible; withheld
  150/150), `make conformance-dotnet-check` ("906 results agree; 255 reviewed skips excluded", no new
  normalization); G4 (`make proto-csharp` no Wire drift, digests `--check`, `changelog.py check`,
  `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff --check`); `go build`, `go vet`, empty `gofmt -l`,
  `go test ./tests/hygiene/...` (size caps); `make docs-check` and, after `make docs-landing-assets`, the strict
  `make docs` build with `PYTHON` a throwaway venv. `-mutate` per protocol: perturb-reals 27, truncate-lists 51,
  rewrite-strings 77 failures, exit 1. Negative controls (scripted, each restored by digest): geometry fabricated
  when absent (19 failures), edge label dropped (7), note `has_size` dropped in rendering (4), document cells dropped
  (13), comparison inverse not sent (9, live included), every NotFound taken for the model (6), FailedPrecondition
  left a ServiceException (6), exact rationals not downgraded (3), rational gated before big integers (3), a
  comparison built without operator (1), nodes out of order accepted (3), other-form content accepted (3),
  answered-only values bound (2); live runner with query properties dropped fails
  `query/a_selection_projects_only_those_properties` on all three protocols, exit 1; `--minimum-expected-tests
  2126` exited 9. A code review found no issues. Cleanup: scratch scripts, mutation logs and the docs venv removed;
  no `sysml-grpc` left. Not run: Linux container and Windows (G5) and hosted CI (pending the next push); `go test
  ./...` (no Go change; packages 12/13/15 recorded its pre-existing failures). Handoff: editing may reuse
  `DocumentWire`'s binding writer pattern and `Model.DeclarationCallAsync`'s NotFound discrimination.
- **Inventory compaction (2026-10-24):** `parity.json` stood at 193,413 bytes against the 200 KiB fixture cap,
  with 230 deferred fields left (~226 KiB projected once typed). Its `fields` section, 123 KB of it, wrote each of
  995 fields as its own object although 765 typed classifications named only 74 tests. Each message now lists its
  fields in groups of one classification (`{"status", "reason"?, "test"?, "fields": [...]}`); the reader
  (`FieldGroups` in `ParityInventory.cs`) expands them into the same message -> field -> `FieldClass` model, so
  `ParityChecker` and every consumer are unchanged, and refuses a field classified twice, a group naming no field,
  two groups of one classification, a repeated or unknown group member and a repeated message. Migrated by script,
  proven by expanding the new file and comparing it with the old one field by field (175 messages, 995 fields,
  identical; every other section byte-identical). The file is 130,818 bytes (`fields` 60.7 KB), about 140 KB
  projected fully typed. Tests +12 (`FieldGroupsExpandToOneClassificationPerField`, 11 refusals);
  `DOTNET_MIN_TESTS := 2137`. Required-service `make dotnet-test` 2137/2137, 0 skipped, sample, 63 gate
  self-tests; `go test ./tests/hygiene/...`; `check-doc-ids.py`; `git diff --check`. Negative controls on the
  committed file, each restored by digest, all failing the committed-inventory test with the expected message: a
  field dropped from its group, an undeclared field added, a field in two groups, a group naming a test the suite
  lacks, a typed group without its test, a message dropped. The client README's inventory description says how
  the section is written.
- **Outcome/prerequisites:** Query/OSLC, document queries/rendering and views; model,
  values, suite-adapters.
- **Read first:** groups 11/12/17, query schema, current RenderView options and document values.
- **Owned surface:** Model.Query.cs/Document.cs/View.cs, immutable builders/results,
  converters and area adapter.
- **Tests:** builders cannot express conflicting query forms or missing comparison operator;
  OSLC gate; ElementRef/ObjectRef bindings and all DocumentValue arms; rational gates;
  Markdown/HTML and FileName; views with ports/show/depth, nullable geometry/style/canvas,
  nested nodes, notes/notices, columns/rows and reported shown options.
- **Gate/negative control:** G1/G2/G3 owning groups; omitted geometry must stay null, not
  fabricated zeros; a deliberately omitted document/view field fails mapping tests.
- **Non-goals/handoff:** no PDF converter toolchain or browser client. QueryException retains
  a model-level query failure rather than changing it to transport failure.

### Package 17: editing — ✅ DONE (2026-10-24)

- **Status: ✅ DONE (2026-10-24).** Built on `feat/dotnet-client` at query-docs `3c3b91e13`, left **uncommitted** for the
  maintainer; no schema or Go change. Delivered (PublicAPI.Unshipped +519 lines, appended as this package's sorted block;
  records synthesize most): `Model.Edit()` -> `Editor` (call-order list under a lock; `ApplyAsync` snapshots it, refuses
  `Add`/`ApplyAsync` while applying and after a success, and reopens after a failure) and `Model.ApplyEditsAsync(
  IEnumerable<Edit>, EditOptions?)` -> `EditResult`. `Edit` is an abstract record with an `internal abstract Arm`, so no
  outside type derives from it (CS0534, proven with Roslyn), and one sealed nested record per arm: SetValue, Rename,
  AddMember (Type/Multiplicity/Value/Specializes/IsAbstract/Redefines/IsDefault/Direction/MetadataPrefixes/Doc/
  BodyExpression), Delete(Cascade), Move, AddConnection(Name/Type), AddSatisfy(SatisfyingFeature/IsAsserted/IsNegated),
  AddRequirementConstraint(Name), AddTransition(Name/Trigger/Guard/Effect/Initial, `Entry`), AddImport(Visibility/
  IsRecursive/IsImportAll/Filters), AddDocumentation(Name/Locale/Replace), AddVerify, AddMetadata(Name/About/Values of
  `MetadataFeatureValue`/Shorthand), AddSequence (all 16 fields, recursive Body/ElseBody; factories First/Then/ThenMember/
  Node/Accept/Send/Assign/If/While/Loop/For/Terminate/GuardedThen/Else, `WithoutThen`), AddMetadataPrefix, AddComment
  (Name/About/Locale), AddNote. Required strings non-null (init accessors validate, so `with` does too), optional ones
  null when unwritten; collections copied into `ValueList`. `AddSequence.MaxDepth = 128` (the service's limit): a stored
  height makes an item nested deeper unbuildable (`ArgumentException`), which also bounds recursion in equality, hashing,
  printing and conversion. `Editor` has a method per arm plus `AddParameter`, `AddEntryTransition` and the sequencing
  shorthands; `Editor.Body` collects nested items, writing the first statement without `then`, and is read when it is
  passed. `EditOptions { InDocument, AcceptDocuments = true }`. `EditResult` (Content null for several documents,
  `HasSeveralDocuments`, Applied of `AppliedEdit` with a null Document from a service that names none, Documents,
  Diagnostics, `SaveAsync` through `SafeWriter` refusing several documents); `EditedDocument.SaveAsync` added (also
  serves Recording). `EditException : ModelException` (Failure `EditFailure`, an unknown number kept; ReferringElements;
  Referrers of `Referrer`). Order: shape (null edits, empty optional strings anywhere in a nested body, empty
  InDocument -> `InvalidRequestException`, naming the path) -> apply_edits -> the capabilities the batch needs, judged
  over the whole batch in the service's order (authoring, connection, satisfy, requirement constraint, transition,
  verification objective, metadata, metadata prefix, sequence, action-body statement, import, member_modifiers,
  implicit_parameters, constraint_body, state_action, documentation, comment) -> edit_documents when a document is named
  -> send. The order and every predicate (arms per capability, member kinds, statement parts, keywords and kinds, the
  depth) are read from `internal/frontend/grpc/edit.go` by `EditInventoryTests`, not restated. The service's walk of
  nested items is redundant (any nested item means a non-empty body, which already needs the capability), so the client
  judges the top item alone; the "nested body not judged" control below proves a deep item still fails a zero-request
  test. Document count: 1 for ParseFile, n for ParseSources; otherwise (by hash, RecordRun) one document when the
  service lacks edit_documents or answers non-empty content, else several. Known limitation (documented on
  `EditResult.Content` and in the README; a unit test pins it): a model named by hash whose edit empties its only
  document reads as several documents (`Content` null, `SaveAsync` refuses, `Documents[0]` holds the empty notation).
  It fails safe rather than risk reading one document of several as the model. The code review raised this case; it is
  kept. NotFound is `ModelNotFoundException` (`ModelCallAsync`); InvalidArgument (unknown document) and
  FailedPrecondition (several without accept/without edit_documents) stay `ServiceException`. Malformed
  (`TransportException(Internal)`): a refusal beside content/documents/applied, a failure kind or referrers without a
  refusal, a document unnamed or repeated, an applied edit outside the batch, at a negative offset or length or in an
  unlisted document, content for a known multi-document model, or documents other than the one equal to content for a
  single-document one. Conformance: `EditingArea` (empty field -> null optional, an unset operation and nesting past
  the depth unexpressible), lossless `Rendering.EditResult` for results and refusals; `expected-skips.json`: ApplyEdits
  no longer uncovered (3 uncovered RPCs left), no new unexpressible scenario. Inventory: ApplyEdits row implemented
  (`Model.Edit().ApplyAsync`, `Model.ApplyEditsAsync`), 17 edit arms implemented, 129 fields typed (893 typed, 101
  deferred overall); parity.json 135,420 bytes. Tests +208 (2345): `ModelEditingTests` (every arm and option over three
  wires, the sample setting every request field and each arm once, editor shorthands and order, Body, apply-once
  semantics with an in-flight apply, 44 gated edits x 3 wires withheld one by one, ungated forms sent, batch order and
  edit_documents last, depth 128 sent and judged (decoded with a raised protobuf recursion limit), 129 unbuildable,
  empty optional parts, single/several/hashed/predating answers, refusals with unknown kinds, 15 malformed answers,
  status mapping, save and failed/cancelled save), `EditInventoryTests` (5), `ConformanceEditingRenderingTests` (3),
  `ConformanceAdapterTests` rows (exchanges for one and several documents, in-band refusal, 2 unexpressible requests, a
  depth-130 request unexpressible), live `ServiceEditingTests` (6 x 3 wires: every arm with the service's exact notation
  and a clean reparse, nested bodies from `Editor.Body`, a model of several documents (rename, named document, cascade,
  referrers, wrong document, refused content-only), nine refusal kinds plus a missing model, save/reload round trip,
  and a service without edit_documents and authoring answering content alone and refusing before sending, counted by
  a recording handler); `FakeModels` gains the edit capabilities; `ValueInventoryTests.Errors` made internal for reuse.
  `DOTNET_MIN_TESTS := 2345`. Passed on macOS arm64 (SDK 10.0.402): required-service `make dotnet-test` 2345/2345, 0
  skipped, format, 0-warning build, sample, gate self-tests, digests; `make dotnet-coverage` (new files: every line but
  two unreachable defaults of the closed edit hierarchy in `EditWire`); G3 `make conformance-dotnet` (per protocol
  default 175/175 run/pass, 13 uncovered, 6 unexpressible; withheld 174/174), `make conformance-dotnet-check` ("1050
  results agree; 111 reviewed skips excluded", no new normalization); G4 (`make proto-csharp` no Wire drift after a
  transient buf plugin error was retried, digests `--check`, `changelog.py check`, `ci-changed-areas-test.sh`,
  `check-doc-ids.py`, `git diff --check`); `go build`, `go vet`, empty `gofmt -l`, `go test ./tests/hygiene/...`;
  `make docs-check` and, after `make docs-landing-assets`, the strict `make docs` build with `PYTHON` a throwaway venv.
  `-mutate` per protocol: perturb-reals 27, truncate-lists 69, rewrite-strings 96 failures, exit 1. Negative controls
  (scripted, each restored by digest): nested body not judged (10 failures), gate order swapped (1, the source-read
  order test), edit_documents judged before the edit gates (3), referrers dropped (10, live included), every model read
  as single (10), empty optional parts sent (3), edits accepted while applying (3), a body's first statement keeping
  `then` (5, live included), depth unbounded (1), applied document not rendered (1), save overwriting by default (3,
  live), a refusal beside notation accepted (3); live runner with edited documents not rendered fails four apply_edits
  scenarios on all three protocols, exit 1; `--minimum-expected-tests 2346` fails the run ("tests ran 2345, minimum
  expected 2346", exit 2). Code review: the empty-single-document case above; nothing else. Cleanup: scratch
  control script (session folder only), docs venv and temp files removed; no `sysml-grpc` left. Not run: Linux
  container and Windows (G5) and hosted CI (pending the next push); `go test ./...` (no Go change; packages
  12/13/15 recorded its pre-existing failures). Handoff: graph-data's ImportData answers `EditedDocument`s too and can
  reuse `EditedDocument.SaveAsync` and the several-documents rule of `EditWire.Result`.
- **Outcome/prerequisites:** all 17 edit kinds, recursive bodies and multidocument batches;
  model, values, suite-adapters, safe-write helper from conversion.
- **Read first:** group 08, EditOperation and recursive body schema, Java capability traversal.
- **Owned surface:** Model.Editing.cs, Editor/Edit/Body, edit converters/results and adapter.
- **Tests:** SetValue/Rename/AddMember/Delete/Move/AddConnection/AddSatisfy/
  AddRequirementConstraint/AddTransition/AddImport/AddDocumentation/AddVerify/AddMetadata/
  AddSequence/AddMetadataPrefix/AddComment/AddNote; every schema option and nested body.
- **Contract:** AddMember type/specializes/multiplicity/value versus body-expression,
  abstract/redefines/default/direction/metadata/doc; transition Entry; sequence First/Then/
  ThenMember and recursive condition/value/target/via/until/body/else/multiplicity/parameter.
  Builder preserves call order and snapshots collections at apply. Reuse explicit result
  immutability; do not require an unnecessarily persistent immutable builder implementation.
- **Gating:** apply_edits -> deterministic per-operation gates, including authoring,
  connection_authoring/import_authoring and modifier/requirement/transition/verification/
  metadata/documentation/comment/sequence/action-body/implicit-parameter gates -> document
  acceptance -> nested values. Freeze exact predicates from current implementation in wire.
  InDocument and accept_documents preserve parse names and Applied[].Document.
- **Gate/negative control:** G1/G2/G3 group 08 with multi-document cases actually executed.
  Deep nested omitted gate fails a zero-request test; refusal preserves content/hash/files
  and EditException carries failure/referrers. Save uses section 3.8.
- **Non-goals/handoff:** no local source rewrite fallback. Exact edit arm/field coverage
  comes from descriptors, not just one test per top-level kind.

### Package 18: graph-data — ✅ DONE (2026-10-24)

- **Status: ✅ DONE (2026-10-24).** Built on `feat/dotnet-client` at editing `cb22a9347`, left **uncommitted** for the
  maintainer; no schema or Go change. Delivered (PublicAPI.Unshipped +250 lines, appended as this package's sorted block):
  `Model.ExportGraphsAsync(subject)` -> `Graphs` (Content kept byte for byte, Version, Subject; `ToString` the form);
  `Model.ImportDataAsync(IEnumerable<DataImport>, ImportOptions?)` -> `ImportResult` (Files of `ImportedFile` with Values,
  Elements and `ImportedValue` assignments; Content null on a dry run, when nothing was set, or for several documents;
  Documents; Model, a new hash-only handle borrowing the connection, null on a dry run; Diagnostics; `IsDryRun`,
  `Values`, `SaveAsync` through `SafeWriter`); `DataImport(Name, Content) { Format, Map }` (empty name unbuildable,
  init accessors validate); `ImportOptions { As, DryRun, AcceptDocuments = true }`; `Model.CompareResultsAsync(results,
  CompareOptions?)` -> `Comparison` (Source, Configurations, Diagnostics, Standing, `Holds`, `ToString` the lines) of
  `ConfigurationComparison` (every field, Seed null when absent, Rows as `SweepRow`, Instances with
  FindInstance/Resolve, Status kept as spelled), `ObservableComparison`, `ComparedDistribution`, `StatisticDifference`,
  `CaseStatistics`/`CaseStatistic`; `CompareOptions` (Actions, Runs, Seed, Draws, ClockStep, Observe of
  `ObservablePair(Stored, Feature?)`, Engine); `CompareException : VerificationException` (Reason, Standing,
  Diagnostics, Partial comparison). A refused import is a plain `ModelException` (message + diagnostics, the precedent
  of Evaluate/SelfCheck; no distinct payload warrants a subclass). Order: option shape (empty Format/Map/As, Draws,
  Engine or observable Feature -> `InvalidRequestException`) -> import_data | compare_results -> engines ->
  schedule_explore -> send; an empty file list, a negative run count and an import form other than `values` are sent
  and refused by the service (scenarios stay expressible). Statuses: ExportGraphs NotFound is the model only for the
  exact `model not found: <hash>` message (`DeclarationCallAsync`), else `ServiceException`; ImportData's and
  CompareResults' NotFound only ever mean the model (`ModelCallAsync`, the import's message differs). Malformed
  (`TransportException(Internal)`): an export without form, positive version or subject; a refused import beside
  files/model/notation/documents; other than one file per import in order, or a negative count; a dry run's model,
  notation or documents; no model hash; an unnamed or repeated document (`EditWire.DocumentNames`, extracted and shared);
  notation for a known model of several documents; documents other than a single-document model's one; a
  single-document model's document without content; notation under the hash imported into; a failure kind without its
  failure. Conformance: `GraphDataArea`, lossless `Rendering.Graphs/ImportResult/Comparison`; `expected-skips.json`: no
  uncovered RPC left, no new unexpressible scenario. Inventory: all 31 rows implemented, 994 fields typed, 1 reviewed
  unsupported, 0 deferred; parity.json 140,024 bytes. `dotnet-gates.py` excludes the wall time of comparison rows
  (`configurations.*.rows.*.elapsed_micros`) as it does sweep/runs rows, with a self-test that the rest of the row
  is compared (64 gate self-tests). Tests +180 (2525): `ModelGraphDataTests` (every field and option over three wires,
  sample requests setting every reachable request field, byte-faithful form, model-vs-subject NotFound, 4 malformed
  exports, new model vs old, dry run, several documents (parsed and hashed), nothing set, refusal, 20 malformed imports x
  3 wires, shapes and gates before sending, statuses, save/overwrite, presence of seed and a 0 clock step, every
  comparison field incl. NaN/infinity and absent distributions, refusal with an unknown kind kept on every wire, gate
  order), `ConformanceGraphDataRenderingTests` (6), `ConformanceAdapterTests` rows (6 exchanges, 2 in-band refusals, 5
  unexpressible requests), live `ServiceGraphDataTests` (11, 34 executions: action and state machine forms with the
  service's `\u003c` escapes kept, the same text over all wires, export refusals, CSV/TSV/JSON/JSON Lines with a
  JSON-Pointer mapping in order into a new model with the old one's notation unchanged, dry run equal to the real
  assignments and changing nothing, refusals changing nothing, a model of several documents and its declining, chained
  imports saved and reloaded, pooled/declared/stored distributions with seeded, failed and unresolved configurations,
  determinism by seed and fixed draws, service refusals, and a service withholding the three capabilities asked nothing,
  counted by a recording handler). `DOTNET_MIN_TESTS := 2525`. Passed on macOS arm64 (SDK 10.0.402): required-service
  `make dotnet-test` 2525/2525, 0 skipped, format, 0-warning build, sample, gate self-tests, digests;
  `make dotnet-coverage` (new files: every line covered after adding the `ImportResult` empty-notation assertion; the
  Rendering misses are pre-existing unreachable defaults); G3 `make conformance-dotnet` (per protocol default 188/188,
  0 uncovered, 6 unexpressible; withheld 187/187), `make conformance-dotnet-check` ("1128 results agree; 33 reviewed
  skips excluded"); G4 (`make proto-csharp` no Wire drift after a transient buf plugin error was retried, digests
  `--check`, `changelog.py check`, `ci-changed-areas-test.sh`, `check-doc-ids.py`, `git diff --check`); `go build`,
  `go vet`, empty `gofmt -l`, `go test ./tests/hygiene/...`; `make docs-check` and, after `make docs-landing-assets`, the
  strict `make docs` build with `PYTHON` a throwaway venv. `-mutate` per protocol: perturb-reals 28, truncate-lists 74,
  rewrite-strings 104 failures, exit 1 (graph-data scenarios asserting content fail, status-only ones unaffected).
  Negative controls (scripted, each restored by digest): graph form trimmed (10 failures, live included), dry run
  fabricating a model (10), refused import answering the old model (10), absent distribution read as zeros (8), absent
  seed read as 0 (9, live), a 0 clock step not sent (3), files out of order accepted (3), engine gate skipped (1),
  several documents read as one (3), assignment location not rendered (10), comparison refusal dropping its source (7),
  comparison refusal read as a comparison (7), export model-not-found not told apart (6, live), empty optional import
  parts sent (1); the cross-check without the comparison-row timing exclusion failed on `elapsed_micros` (observed
  before the fix); `--minimum-expected-tests 2526` fails the run ("tests ran 2525, minimum expected 2526", exit 9).
  Code review: no issues. Two unrelated environment-sensitive tests failed once each under full-suite load and passed
  on rerun (`PrivateConnectionTests.KillingTheChildAsItStartsFailsTheOpenAndLeavesNoHold`, 3/3 isolated;
  `NetworkFailureTests.ARefusedPortIsATransportUnavailableCausedByTheRefusal`, a freed port reused); untouched by this
  package. Cleanup: probe service stopped, control script in the session folder only, docs venv and temp files
  removed; no `sysml-grpc` left. Not run: Linux container and Windows (G5) and hosted CI (pending the next push);
  `go test ./...` (no Go change; packages 12/13/15 recorded its pre-existing failures). Handoff: every service RPC is now typed and run by the conformance runner (no uncovered RPCs, no deferred field); docs
  can describe graph export, data import and result comparison.
- **Outcome/prerequisites:** ExportGraphs, ImportData, CompareResults; model, values,
  suite-adapters. Does not depend on implementing the fluent Editor.
- **Read first:** groups 18/21, current Java graph/import/comparison facades and proto fields.
- **Owned surface:** Model.Graphs.cs/Data.cs, options/results/converters and adapter.
- **Tests:** graph content/version/resolved subject; CSV/TSV/JSON/JSONL import mapping/order/
  dry run/document acceptance/assignments/new hash; comparison configurations/observables/
  stored-versus-computed distributions/differences, seeded execution and partial failures.
- **Gate/negative control:** G1/G2/G3 groups 18/21; dry run must not change model content,
  refused import must return no fabricated new model, graph content stays byte-faithful.
- **Non-goals/handoff:** no client-side graph reconstruction/statistics. Do not drop a field
  because an older language facade has not yet wrapped it.

### Package 19: cache — ✅ DONE (2026-10-26)

- **Status: ✅ DONE (2026-10-26).** Implementation and local G1/G2/G4/G5 evidence complete on macOS arm64 and
  Linux arm64; Windows compatibility is deferred to a later check, as for the earlier packages (see "Not run").
  Built on `feat/dotnet-client` at `4fa395c89` (graph-data), left **uncommitted**; no schema change, `parity.json` and the public API
  baseline unchanged (everything is internal). Delivered: `Internal/CacheLayout` (the shared names, read from
  each client's source: `sysml-grpc[.exe]`, `.json`, `.lock`, snapshot `sysml-grpc-<16 hex>[.exe]`; guard
  `sysml-grpc[.exe].lock.dotnet`; staged names `<file>.dotnet-<16 hex>.tmp`); `Internal/CacheNative` (libc
  `open(path, O_RDWR|O_CLOEXEC)` with per-OS constants, `lockf(F_TLOCK/F_ULOCK, 0)`, `flock(LOCK_EX|LOCK_NB)`,
  `link(2)`; kernel32 `LockFileEx`/`UnlockFileEx` whole range with a local blittable OVERLAPPED, `CreateHardLinkW`;
  errno captured before releasing the handle reference; `AllowUnsafeBlocks` for LibraryImport, reason in the
  csproj); `Internal/CacheLock` (ALC semaphore → guard (flock on a libc descriptor / `FileShare.None` on Windows)
  → cross-client lock (lockf on a libc descriptor, file made beforehand with `CreateNew`, 0600 / `LockFileEx` on a
  `ReadWrite|Delete`-shared handle); non-blocking attempts with 10 → 250 ms doubling backoff; caller-token
  cancellation; `DefaultWait` 2 min (the Java client's) → `ServiceStartException` naming the lock and holder; one
  warning after 2 s; idempotent release in reverse; I/O failure → `IOException`, nothing held; not reentrant,
  documented); `Internal/CacheMetadata` (Python `json.dump` byte format incl. `ensure_ascii` escapes, bounded 64 KiB
  read, last repeated member wins, problems named, case-insensitive digest match); `Internal/CacheTransaction`
  (begin = lock + sweep of .NET leftovers only; `InspectAsync`; `InstallAsync` stage 0700 → fsync → verify →
  atomic replace → record via staged 0600 file, record written even if cancelled after publication, unrecordable
  record → warning and `Recorded=false`; `SnapshotAsync` reuse only a verified snapshot, else hard link, else copy
  with warning, else the executable itself with warning; `CacheSeams` for step and link injection).
  `BinaryResolver.Find` now returns `FoundBinary(Path, Cache)`; a binary found in the shared cache is resolved to
  its snapshot under the lock (a cache that vanished while its lock was awaited is looked for again; a cache that
  cannot be locked is used in place with a warning, as the other clients do); named and PATH binaries untouched.
  Test-side: `test/interop` holders mirroring each client's exact calls (`lock_holder.py`, `LockHolder.java` run
  from source, `rust-lock-holder` at the Rust client's `rustix =1.1.4`/`fs4 =1.1.0` pins, Cargo.lock committed,
  built `--locked` into `bin/dotnet/interop`); `test/OpenMBEE.OpenSysML.CacheProbe` (hold/try through the client;
  hold-shared/try-shared with its own syscalls as an independent detector; hold-and-start; copies = two collectible
  ALCs; crash at any `CacheStep`); new `Interop` test category with `InteropGate` (`OPENSYSML_REQUIRE_INTEROP`;
  `$PYTHON`/python3/python, JDK from `JAVA_HOME` or PATH with javac, cargo from PATH or `~/.cargo/bin`; a found tool
  must answer its version option, so macOS's `/usr/bin/java` placeholder is no JDK). Tests +112 (2637,
  `DOTNET_MIN_TESTS := 2637`): `CacheFormatTests` (names, escapes vs Python output, parse problems, torn record),
  `CacheLockTests` (14: backoff, files/modes, threads, idempotent release, processes both ways, another client,
  cancellation leaves nothing held, give-up + one warning, killed holders, a killed holder's child keeps nothing
  (with `/proc` fd check on Linux), two ALCs + collection, uncreatable cache), `CacheTransactionTests` (install,
  replace, mismatch, failed/cancelled write, refusals, torn and recordless interruptions, record after publish,
  unrecordable record, leftover sweep, cross-client lock at every step, ended transaction, snapshot
  link/reuse/replace/exec bit/copy/neither/unpublishable/unreadable, verified bytes kept, process killed at each of
  the 5 steps), `CacheResolutionTests` (8 incl. a private service from the cache running its snapshot),
  `CacheInteropTests` (19: six theories × Python/Java/Rust + Python record round trip), `InteropGateTests` (17).
  CI: both .NET jobs install Temurin 21 and Rust 1.83 and set `OPENSYSML_REQUIRE_INTEROP=1` (CircleCI unchanged:
  its Java/Rust cases skip by name). README "The shared cache", failures row, testing; changelog
  `dotnet-client-cache-lock.added.md`.
  **Passed — macOS arm64** (SDK 10.0.402, runtime 10.0.13, Python 3.15, OpenJDK 25.0.2, cargo 1.99.1): with
  `JAVA_HOME`, `OPENSYSML_GRPC_BINARY`, `OPENSYSML_REQUIRE_SERVICE=1`, `OPENSYSML_REQUIRE_INTEROP=1`: `make
  dotnet-test` 2637/2637, 0 skipped (format, 0-warning build, sample, gate self-tests, digests); `make
  dotnet-coverage` 2637/2637, 9794/10322 lines (new-file misses: Windows branches and OS-error catches that cannot
  be induced portably); cache classes 3 consecutive runs. **Linux arm64** (SDK image by digest, `--init`, user
  `app`, Python 3.12.3, OpenJDK 21.0.12.1, cargo 1.83.0, service cross-built from this tree): locked restore,
  0-warning build, whole suite 2637/2637, 0 skipped, no leftovers. G4: `make proto-csharp` no drift,
  `changelog.py check`, `check-doc-ids.py`, `ci-changed-areas-test.sh`, `sync-release-digests.py --check`, `git diff
  --check`, actionlint v1.7.7; `make docs-check` (throwaway venv); `go build`, `go vet`, empty `gofmt -l`, `go test
  ./tests/hygiene/...`. **Negative controls** (scripted, each restored by digest): FileStream-opened descriptor for
  the cross-client lock — macOS 47 failures (transaction tests each waited out the 2-minute default, the run was
  killed); `flock` for `lockf` — macOS 0 (BSD flock/fcntl interact, as 1.3.3 predicts), **Linux 31**; no guard 5; no
  `O_CLOEXEC` macOS 1, Linux 1 (the first version of the inheritance test missed it, since an explicit `flock(LOCK_UN)`
  releases the shared open file description; rewritten around a killed holder whose child lives on); second
  descriptor opened and closed while held 16; record vouching without the digest 2; no leftover sweep 3; snapshot
  reused unverified 1; cancellation leaking holds 12; non-idempotent release 1; publish before verify 2; resolver
  starting the cache itself 4; interop gate never required macOS 5, Linux 8. Code review: one finding (macOS
  `/usr/bin/java` placeholder taken for a JDK, failing rather than skipping), fixed and reproduced fixed. Cleanup:
  no holder, probe, `FakeService` or `sysml-grpc` process after any run; one temp cache left by the deliberately
  killed control host removed; scratch scripts, diagnostic tools and venv deleted. **Not run:** native
  Windows, deferred to the later Windows compatibility check (the `LockFileEx`, `FileShare.None` guard and
  `CreateHardLinkW` paths have not executed anywhere yet; that check must run the whole suite there with
  `OPENSYSML_REQUIRE_INTEROP=1`, which the `dotnet-native` job now sets), Linux x64 and hosted macOS (pending the
  next push); G3 (no conformance change); `go test ./...` (no Go
  change). Handoff for downloads: `CacheTransaction.BeginAsync(layout, logger, token)` → `InspectAsync`
  (`CacheContents.Vouched`, recorded repo/version) → `InstallAsync(write, sha256, version, repo)` →
  `SnapshotAsync`; `FoundBinary.Cache` marks the cache step the downloader replaces.
- **Outcome/prerequisites:** interoperable cache locking and crash-consistent transaction;
  lifecycle-local and proven locking decisions.
- **Read first:** section 3.7 and actual existing client lock paths/metadata/snapshot handling.
- **Owned surface:** CacheLock, cache transaction/snapshot helpers and foreign lock holders.
- **Tests:** two directions with Python/Java/Rust; separate .NET processes and library ALCs;
  cancellation/backoff, process crash, descriptor lifetime; stale/torn metadata; hard-link/
  copy behavior and warning; injected failure between executable and metadata publication.
- **Gate/negative control:** G1/G5 with OPENSYSML_REQUIRE_INTEROP=1; deliberately substitute
  flock for the POSIX cross-client fcntl lock (fails on Linux) and a `FileStream`-opened
  descriptor (fails on macOS), and require the exclusion detector to fail for each (1.3.3).
- **Non-goals/handoff:** no network or signature verifier yet. Deliver a cancellable lock/
  transaction API with documented acquisition order and no live lock holders left behind.

### Package 20: downloads — ✅ DONE (2026-10-26)

- **Status: ✅ DONE (2026-10-26).** Implementation and local G1/G2/G4/G5 evidence complete on macOS arm64 and
  Linux arm64; Windows compatibility is deferred to a later check, as for the earlier packages (see "Not run").
  Built on `feat/dotnet-client` at `9bc597a22` (cache), left **uncommitted**; no schema change, `parity.json`
  unchanged (downloads are no RPC). Public API added (unshipped baseline): `ConnectionOptions.DownloadVersion`,
  `GithubRepo`, consts `VersionVariable` (`OPENSYSML_GRPC_VERSION`), `GithubRepoVariable` (`OPENSYSML_GITHUB_REPO`),
  `AllowUnpinnedDownloadVariable` (`OPENSYSML_ALLOW_UNPINNED_DOWNLOAD`), `DefaultGithubRepo`;
  `UnpinnedReleaseException : ServiceStartException` (`Repository`, `Version`, `Asset`); `BinaryNotFoundException`
  overload carrying the download failure as `InnerException`. `DownloadVersion`/`GithubRepo` are private-service
  options (refused with `Address`, `AutoStart=false`, and with `BinaryPath`; warned unused under `OPENSYSML_SERVICE`
  or `OPENSYSML_GRPC_BINARY`). Internal: `ReleasePlatform` (five assets; any other goos-goarch →
  `UnsupportedPlatformException` naming it); `ReleaseRequest` (option → variable → `v`+assembly informational
  version; repo option → variable → default; `latest`; URL-segment validation of tag/repo, invalid variable →
  `InvalidRequestException`; same-origin opt-out parsed as every client parses it); `ReleaseSource` (GitHub over
  HTTPS, embedded pins, 15 s idle, 512 MiB / 8 MiB, 10 redirects; `LoopbackHttp` for tests only) and `BinaryDownloader`
  (`SocketsHttpHandler` without auto-redirect, every hop checked HTTPS-only and never HTTPS→HTTP; idle timer reset
  on every read and covering headers; declared length refused early; limits counted after automatic
  decompression; `User-Agent: OpenMBEE.OpenSysML/<version>` for the API; caller cancellation rethrown with the
  caller's token; body streamed into `CacheTransaction.InstallAsync`). **Pins-only** (1.2): an unpinned asset is
  refused with `UnpinnedReleaseException` before any request, naming the gap and the opt-out; the served
  `.sha256` must equal the pin; `ExpectedBinarySha256` must equal the pin/served digest before the binary is
  fetched; no `UnsignedReleaseException`/`ManifestSignatureException` types were added, since nothing could throw
  them. `BinaryResolver` now follows the order every other client shares — named option, variable, then the
  cache with download under its lock, then `PATH` **only** as the implicit fallback (a behavior change from
  package 08, where an existing `PATH` binary was used before the network) — and implements the 3.7 table: vouched
  repo+tag, or an implicit request with the cached `ExpectedBinarySha256`, without contradicting a committed pin
  → reuse with no request; implicit + no record file → hand-installed,
  kept; otherwise replace with a warning only after verification; implicit unavailable → keep cache with warning,
  else `PATH` with warning, else `BinaryNotFoundException`; explicit unavailable → fail; `latest` unresolved → keep
  only a vouched release of that repo; `ChecksumMismatchException` always propagates; unlockable cache → explicit
  fails, implicit as before. `Find`/`FoundBinary` replaced by `FindNamed`/`FindOnPath`; `Connection.OpenAsync` gained
  an internal overload taking a `ReleaseSource`. Test-side: `ReleaseServer` (TcpListener HTTP/1.1 on a
  kernel-chosen loopback port: stalls before/after headers, endless, trickled, false/absent lengths, gzip,
  redirects; all connections closed on dispose). Tests +181 (2836, `DOTNET_MIN_TESTS := 2836`):
  `ReleaseRequestTests` (assets/unsupported pairs, Go arch names, built-against = props Version, precedence,
  tag/repo validation, opt-out parsing), `BinaryDownloaderTests` (latest + headers, pin/served/required digests,
  unpinned refusal with no request, opt-out warning, malformed/missing checksum, streamed install 0700 + record,
  tamper, declared/endless/gzip-bomb/metadata limits, short/long bodies, idle before headers and mid-body,
  silence-not-total, caller cancellation token, redirects relative/absolute/loop/no-location/off-loopback HTTP,
  scheme policy, production source), `DownloadResolutionTests` (every 3.7 row, legacy record without repo, offline
  implicit/explicit, integrity with a working cache ×4, `PATH` fallback, `BinaryNotFoundException` carrying why,
  latest once per open, latest unresolved, required digest, same-origin, 8 concurrent opens → one download, named
  binary, invalid variables, unlockable and unstageable cache), `SignedReleaseTests` (the Python
  signed-release fixtures — valid, historically valid, other identity, changed manifest, unsigned — all refused as
  unpinned with nothing fetched, never passed as verified; a pin trusted without fetching the signature;
  same-origin still refusing tampered bytes), `ServiceDownloadTests` (Integration: the real binary served as a
  pinned release, started over all three wires, then reused with no request). CI routing: `client/python/tests/
  fixtures/signed_release/` now also runs the dotnet area (+ routing case). README "Downloading releases",
  private-service order, cache, failures, testing, digests; changelog `dotnet-client-downloads.added.md`.
  **Passed — macOS arm64** (SDK 10.0.402, runtime 10.0.13, Python 3.15, OpenJDK 25, cargo 1.99): with `JAVA_HOME`,
  `OPENSYSML_GRPC_BINARY`, `OPENSYSML_REQUIRE_SERVICE=1`, `OPENSYSML_REQUIRE_INTEROP=1`: `make dotnet-test`
  2836/2836, 0 skipped (format, 0-warning build, sample, gate self-tests, digests); `make dotnet-coverage`
  2836/2836, 10140/10668 lines (new-code misses: an unreadable cache executable and pre-existing `IsExecutable`
  catches). **Linux arm64** (SDK image by digest, `--init`, user `app`, service cross-built from this tree, no
  Python/JDK/Cargo in the image so the 19 interop cases skip by name; this package does not touch locking and they
  ran on macOS): locked restore, 0-warning build, whole suite 2836 in six runs: four clean, two each with one
  `ETXTBSY` ("Text file busy") failure exec'ing a FakeService script in a pre-existing lifecycle test
  (`PrivateConnectionTests`/`LifecycleRunTests`). **That flake is pre-existing**: the baseline `9bc597a22` in the same
  container failed identically in 2 of 8 runs (`PrivateConnectionTests.OpeningWithNoServiceNamedStartsAPrivateOneAndClosingStopsIt`);
  it is a fork/exec race in the FakeService harness (a script exec'd while another test's forked child still
  holds its write descriptor) and must be fixed before release's native-Linux gate. G4: `make proto-csharp` no
  drift, `changelog.py check`, `check-doc-ids.py`, `ci-changed-areas-test.sh`, `sync-release-digests.py --check`,
  `git diff --check`; `make docs-check` (throwaway venv); `go build`, `go vet`, empty `gofmt -l`, `go test
  ./tests/hygiene/...`. **Negative controls** (scripted, each restored by digest), failing tests in brackets:
  served checksum not checked against the pin [1]; unpinned release trusted without the opt-out [7]; idle timer
  not reset per read [1]; HTTPS→HTTP redirect allowed [1]; limit counted before decompression [2]; declared
  length unchecked [1]; required digest not checked before download [5]; integrity failure answered from the
  cache [2]; explicit request falling back like an implicit one [2]; hand-installed build replaced [1]; vouched
  cache not reused [6]; repository ignored on reuse [1]; no idle timeout at all — the stall tests hung and were
  killed at 90 s. Additional cache and response-validation coverage is recorded below. Cleanup: no `sysml-grpc`, FakeService, probe or test-host process,
  container or scratch tree left; temp caches removed by the tests. **Not run:** native Windows, deferred to the
  later Windows compatibility check; Linux x64 and hosted macOS (pending the next push); a live download from
  GitHub (network approval was unavailable in this session; the transfer path is proven only against the
  loopback host, including redirects and the real binary); G3 (no conformance change); `go test ./...` (no Go
  change). Handoff for release: the five pinned assets of the built-against tag are what a default open
  downloads, so the release checklist must sync pins (`sync-release-digests.py`) before announcing; a checkout
  whose version is unreleased refuses its default download as unpinned and falls back to the cache or `PATH`.
- **Release-validation follow-up (2026-10-26):** recorded cached releases now enforce the committed pin for
  their exact repository, tag and platform before reuse, including the digest shortcut, offline/implicit
  fallback, unresolved `latest`, and unlocked-cache reads. Contradictions fail before any network request or
  snapshot publication; the executable and metadata remain unchanged. The digest-only shortcut is limited to
  implicit release selection: explicit options and environment versions, `latest`, another repository, and
  hand-installed builds cannot bypass explicit release constraints with a matching digest. Idle deadlines
  now wrap individual reads of the HTTP plaintext transport, below header parsing and decompression; header
  progress refreshes the wait, local file writes do not consume it, and pooled connections retain correct
  timeout behavior. Lazy JSON string-decoding failures, including unpaired UTF-16 surrogate escapes, are
  reported as `ServiceStartException`, retaining the cause and the ordinary same-repository `latest` fallback.
  Added 30 regression cases and raised `DOTNET_MIN_TESTS` to 2866. The pre-fix run reproduced all four defects.
  **Passed — macOS arm64:** `make dotnet-test` with the installed JDK 25 in `JAVA_HOME`,
  `OPENSYSML_GRPC_BINARY`, `OPENSYSML_REQUIRE_SERVICE=1`, and `OPENSYSML_REQUIRE_INTEROP=1`: 2866/2866, zero
  skipped; locked restore, zero-warning Release build, format verification, real-service downloads over all
  three wires, sample, cross-client checks, 64 gate self-tests, and embedded digest synchronization.
  The focused affected suite also passed 212/212 with the real service required. `go build ./...`,
  `go vet ./...`, empty `gofmt -l .`, the documentation link/internal-label checks, changelog validation,
  digest synchronization, and `git diff --check` passed. The full `go test ./...` run failed
  in unchanged Go packages: macOS `/var` versus `/private/var` path expectations, CLI occupied-port shutdown,
  PDF fixtures, MIME-header error wording, solver output, C real-notation locale assertions, generated-C
  compilation, a ten-minute compiled-step-budget timeout, and WASM tests whose installed Node runtime
  could not load `libllhttp.9.3.dylib`. No Go sources or module manifests changed.
  Native Windows/Linux and a live GitHub download were not repeated
  for this follow-up.
- **Outcome/prerequisites:** verified release provisioning integrated into local resolution;
  cache, lifecycle-local, signing decision.
- **Read first:** section 3.7, release-digests and signed-manifest fixtures.
- **Owned surface:** BinaryDownloader, ReleasePlatform/Digests, SignedManifest/verifier
  adapter if approved, resolver integration; test ReleaseServer.
- **Tests:** all five assets/unsupported pairs; pin/signed/unsigned/tamper/wrong issuer/
  wrong project/invalid signing-time/valid historical expiry; same-origin opt-outs;
  repo/version precedence, latest, offline cache policy, oversized/stalled/redirect bodies,
  temp cleanup, executable permissions and embedded digest equality.
- **Gate/negative control:** G1/G2/G5 plus offline fixture suite; bad signature/pin fails
  even with a working cache; a stalled chunk stream hits idle timeout. Use an in-process
  loopback HTTP server whose chosen implementation works on all supported OSes.
- **Non-goals/handoff:** no automatic use of unsigned releases. If pins-only was selected,
  assert unpinned refusal instead of skipped verification success and document the gap.

### Package 21: release

- **Status: ⛔ BLOCKED (2026-10-26): implementation committed and pushed; the native CI gate cannot run.**
  Committed as `8358ee364` on `feat/dotnet-client` (pushed to the fork's PR #1), on top of downloads
  `b9de6c1af`; no schema change, `parity.json` unchanged. **Blocker:** GitHub stopped running workflows on the
  fork `richetl/OpenSysML` after 2026-10-22 ("Workflows are not currently being run on this fork due to the scale
  of GitHub Actions usage. A repository maintainer can choose to re-enable these.", Actions tab banner; the REST
  API still reports Actions enabled). No pull_request run has been created since `1f270f03a`, for PR #1 or for a
  throwaway draft probe PR from `develop` (opened and closed; its branch was deleted). So the hosted Linux,
  macOS and Windows full tier (G5, and the native-Windows check deferred by packages 08–20) has not run on
  anything after package 11. **To unblock:** a maintainer re-enables workflows in the fork's Actions tab, or the
  branch runs in a PR to upstream; then push or add a label to PR #1 so the full tier runs, and record the
  results here.
  Delivered: (1) **Release lockstep**: CircleCI `build-python-package` runs `check_version.py --dotnet`, plus a
  step `dotnet-gates.py promote-api --check`, both before `python -m build`; `TestDotnetClientIsReleasedAsTheTagsCheckout`
  (tests/hygiene) holds both and their order. (2) **API promotion**: `dotnet-gates.py promote-api [--check]`
  (add entries, `*REMOVED*` deletes, ordinal sort, header kept, every project validated before any write);
  +8 self-tests (72). The API is not promoted in this package; per the non-goal that happens on the release
  branch. Promoting the real baseline in a scratch tree built 0-warning, and dropping one shipped entry there
  then failed with RS0016. (3) **ETXTBSY fix**: `PrivateService.LaunchAsync` retries only errno 26 (non-Windows)
  5× over 50–800 ms, logged at Debug, cancellable; root cause of the pre-existing Linux FakeService flake and a
  real production window (the cache writes binaries it then execs). +3 Linux-only tests (skip by name elsewhere;
  macOS measured not to refuse); `ListLogger.Logged` callback. (4) README **Coverage** table (31 RPCs, status,
  methods) held to `parity.json` by `ParityInventoryTests.TheReadmeCoverageTableIsTheInventorysRows`; README
  release model and busy-binary retry. (5) `docs/project/releasing.md`: "Releasing the .NET client as the tag's
  checkout" (version, public API, release digests, what CI proves) and release-branch checklist step.
  (6) CI routing quick-tier cases (.NET source, gates, SDK pin, signing fixtures → dotnet; scenario → go only).
  (7) `DOTNET_MIN_TESTS := 2870`; fragments `dotnet-client-release.added.md`,
  `dotnet-private-service-busy-binary.fixed.md`.
  **Passed — macOS arm64** (SDK 10.0.402, JDK 25 in `JAVA_HOME`, `OPENSYSML_GRPC_BINARY`, `OPENSYSML_REQUIRE_SERVICE=1`,
  `OPENSYSML_REQUIRE_INTEROP=1`): `make dotnet-test` 2870, 3 skipped (the Linux-only cases), 72 gate self-tests;
  `make dotnet-coverage` 10180/10724 lines; `make conformance-dotnet` 188/188 ×3 default, 187/187 ×3 withheld,
  6 reviewed skips; `make conformance-dotnet-check` 1128 agree, 33 reviewed skips excluded; `make proto-csharp` no
  drift; `sync-release-digests.py --check`; `changelog.py check`; `check-doc-ids.py`; `check-doc-links.py`;
  `ci-changed-areas-test.sh`; `make docs-check` and strict `make docs` (throwaway venv; landing assets prebuilt,
  local Node broken by the `libllhttp` issue); `go build`, `go vet`, empty `gofmt -l`, `go test ./tests/hygiene/...`;
  `git diff --check`. **Digest gate:** in a scratch worktree, `pin_release_checksums.py --version v99.0.0
  --from-manifest <fixture>` synced all seven copies, `sync-release-digests.py --check` passed, and the scratch
  build's `ReleaseDigestsTests` passed 19/19; a drifted shared table failed `--check` naming every copy.
  **Linux arm64** (pinned SDK image, `--init`, user `app`, service cross-built): whole suite 8/8 runs clean, 2870
  each (19 interop skips by name: no Python/JDK/Cargo in the image); the baseline failed 2/8 with ETXTBSY.
  **Negative controls** (each restored): retry disabled [3 failures]; README row changed [1]; either CircleCI
  check removed [hygiene test fails naming it]; shipped entry dropped [RS0016]; broken anchor [check-doc-links].
  Cleanup: worktrees and probe branch removed; no `sysml-grpc`, FakeService or test-host process left; one
  `npm ci` from the failed landing-asset build is stuck in uninterruptible exit (Node/dyld) and will be reaped.
  **Not run:** hosted CI on Linux x64, macOS and Windows (blocked above); native Windows (never run since
  package 11); `go test ./...` (no Go source change; the hygiene package was run); a live GitHub download.
- **Outcome/prerequisites:** complete service parity and reproducible repository delivery;
  execution, verification, sampling, conversion, query-docs, editing, graph-data,
  lifecycle-safety, downloads.
- **Read first:** [release process](../../../docs/project/releasing.md), digest synchronizer,
  version checker and current CI aggregate/release jobs.
- **Owned surface:** final CI/plumbing integration, release checklist, API promotion,
  coverage table and inventory status.
- **Tests:** all current RPCs typed; no unclassified descriptor fields/arms; exact residual
  unexpressible-request skips; all implemented scenarios on three protocols and withheld
  configuration; native lifecycle/cache matrix; lockstep version and digest sync.
- **Gate/negative control:** G6 plus G3/G5; run pin_release_checksums.py against a fixture
  manifest in a scratch checkout, then sync-release-digests --check. CI routing tests prove
  direct .NET changes run quick-tier jobs and service/schema/scenario changes fan out in full.
- **Non-goals/handoff:** no NuGet/package job, no placeholder publication hygiene test.
  Full validation evidence and resolved target-support decision are prerequisites for docs
  announcement. Promote public API entries at the actual release boundary, not every PR.

### Package 22: docs — ✅ DONE (2026-10-27)

- **Status: ✅ DONE (2026-10-27).** Built on `feat/dotnet-client` at `aba40e631` (package 21's `8358ee364` plus a
  `develop` merge), left **uncommitted**. Package 21's hosted CI evidence is still outstanding (fork Actions disabled);
  this package proceeded, and is marked done, at the owner's direction, treating those validations as passed. No
  schema change; `parity.json` and the public API baseline unchanged (no library API change). Delivered: **guide**
  `docs/clients/dotnet.md` (checkout `ProjectReference`, the runnable Vehicle walkthrough, connections/protocols, service
  source and trust, limitations: no NuGet, `net10.0` only, no AOT/trimming/browser, no sync API, no generated classes or
  metamodel reader); **reference** `docs/reference/dotnet-api.md` (options, Connection/Model methods by area, writing
  files, values, exception tree, capability negotiation, binary resolution and pins-only trust, non-goals,
  conformance); C# install/parse tabs and "From C# and .NET" in `docs/guide/09-clients.md` (eight ways); .NET rows/prose
  in `docs/reference/clients.md` (table, choosing, coverage, edit documents, lifecycle scope per load context,
  provisioning, `make conformance-dotnet`), `service-transports.md` (gRPC default, Connect protobuf/JSON, unknown-arm
  behavior), `clients.md` card, `downloads.md`, `mkdocs.yml` nav + site_description, guide/reference indexes,
  `overrides/home.html`, root `README.md` (lists, features, status, tree, release, Clients table), `CONTRIBUTING.md`,
  architecture tree, roadmap Track I intro + item I9. **Vehicle sample** is now the cross-client task (parse, check
  `Errors`, look up `sedan`, evaluate its redefined mass with `SubjectSymbolId`), its walkthrough between
  `// <walkthrough>` markers; the gate-only identity check moved to `SampleGate.cs`; `dotnet-gates.py` expects
  `Demo::sedan (partUsage): mass = 1800.0` (gate self-test updated). **DocumentationTests** (+10, `DOTNET_MIN_TESTS :=
  2880`): the guide's csharp block must equal Program.cs's using directives plus the region; its run command must be
  the complete bash command, including the local service-binary assignment, on a line of its own; the extraction
  refuses missing/duplicate markers. Seven command cases accept that command and refuse an absent/empty/wrong
  binary setting, a commented or short-circuited command, and a help-only invocation. All six negative cases failed
  against the original substring check; all ten documentation tests pass with whole-line matching.
  CI routing: `docs/clients/dotnet.md`
  also runs the dotnet area (+2 cases). Client README: guide links, sample description/output (version and capability
  count as placeholders, per the one-home rule for measured numbers). **Self-model**: `DotnetClient` part def
  (`client/dotnet`, `csharp`, stubs `…/Wire`), `part dotnet`, `connection dotnetCalls : ArtifactChannel`, allocation to
  `clientLibraries`, document intro and README ("six generated clients"). Landing stack role gains `.NET`;
  `editors/vscode/src/landing/stack.json` regenerated (`-update-landing`: hash, one end column, the string). Skills:
  packaged `opensysml-dotnet` (+ README row) and `.agents/skills/testing-dotnet-client`. Fragment
  `dotnet-client-docs.added.md`.
  **Passed — macOS arm64** (SDK 10.0.402 installed to `~/.dotnet`, since Homebrew's 10.0.403 fails the exact-pin
  preflight; runtime 10.0.14; OpenJDK 27 in `JAVA_HOME`; `OPENSYSML_GRPC_BINARY`, `OPENSYSML_REQUIRE_SERVICE=1`,
  `OPENSYSML_REQUIRE_INTEROP=1`): the documented sample run printed the three documented lines; the guide's walkthrough
  block and the chapter 9 C# tab each compiled and ran in a scratch `ProjectReference` consumer (1800.0; a syntax error
  printed its diagnostic, exit 1); the packaged skill's snippets compiled and ran against the real service (which
  showed outputs without `[1]` arrive sequence-shaped, now stated in the skill); `DocumentationTests` 3/3;
  `bash scripts/ci-changed-areas-test.sh`; `check-doc-links.py`, `check-doc-ids.py`, `changelog.py check` (new files
  intent-added, since the checkers read `git ls-files`); `make docs-check` and strict `make docs` (throwaway venv);
  `make self-model`; `go test ./examples/... ./tests/hygiene/... ./internal/frontend/grpc`, `TestLanding*`; with the
  training, pilot and (newly downloaded) spec-example corpora and all three require variables, `TestCorpusRoundTrip`
  and `TestCorpusAPIJSONRoundTrip` (self-model files stay `stable`; without the spec examples the gate printed
  `GATE NOT RUN` and skipped); VS Code `npm test` 407/407; `go build ./...`, `go vet ./...`, empty `gofmt -l .`,
  `git diff --check`. **Failed, pre-existing and out of scope (recorded at the owner's direction):** `make dotnet-test`
  2873 total, 2846 passed, 3 skipped (Linux-only), **24 failed**, identical on a clean `HEAD` worktree with the same
  binary: drift from the `develop` merge — the new `query_order_by` capability without a constant (and Query `orderBy`
  unmodelled), the service now refusing a one-operand composite constraint, RenderView answering a repeated node id
  the client rejects, calc/analysis/recording results now sequence-shaped against scalar expectations, and the
  embedded `release-digests.json` out of sync (`sync-release-digests.py --check` fails). These must be fixed, with the
  conformance re-run, before release. Also failing identically on clean `HEAD` (environment): `tests/wasm`
  `TestBrowserREPLWalkthroughs`/`TestWasmRuns` and `internal/frontend/repl` `TestOptimizeReportsABoundThatIsNotAttained`/
  `TestEngineSMTDecidesSensitivity`.
  **Negative controls** (each restored by digest): guide code drifted from the sample [1 failure]; guide run command
  names another project [1]; sample code changed without the guide [1]; `.NET` page removed from the nav [strict build
  aborted]; a broken docs link [`check-doc-links.py` 1 broken, once the file is tracked]; dotnet routing for the guide
  removed [2 routing cases]; sample gate expecting the old model line [gate failed quoting the actual output]. A first
  attempt restored an untracked file with `git checkout`, which silently did nothing; the digest check caught it and the
  file was restored by hand. Cleanup: scratch consumers, worktrees, docs venv and site removed; no `sysml-grpc`, probe
  or test-host process left. **Not run:** hosted/native Linux, Windows and macOS CI (fork Actions disabled);
  `make conformance-dotnet[-check]` and `make dotnet-coverage` (no library or adapter change, and the suite is red
  from the drift above); full `go test ./...` (no Go source change; owning packages run).
  **Command-gate follow-up validation:** documentation tests 10/10; .NET format check; the Vehicle gate's three
  modes and 72 gate self-tests; documentation link/label/changelog checks; `go build ./...`, `go vet ./...`,
  empty `gofmt -l .` and `git diff --check` passed. The required-service/interop .NET gate now discovers 2880 tests:
  2853 passed, the same 24 failures, 3 Linux-only skips. `go test ./...` was also run and failed in unrelated
  command, rendering, analysis/runtime, SMT, code-generation and WASM tests, including a ten-minute codegen
  timeout; those failures were not adjudicated by this test-only change.
- **Outcome/prerequisites:** publish the fully verified client story and runnable walkthrough;
  release. README updates are not deferred until this package.
- **Read first:** existing Java/Rust client guide/reference, CONTRIBUTING documentation rules.
- **Owned surface:** documentation/ecosystem rows in section 7, Vehicle sample, user/testing
  skills, self-model client surface and generated outputs.
- **Tests:** build and execute the documented checkout sample on net10; verify documented
  protocol/ownership/download limitations against implementation; ensure all client lists/
  nav/support claims agree.
- **Gate/negative control:** G6 documentation/site build plus required corpus roundtrip
  after self-model changes; broken documented command or missing nav page fails the check.
- **Non-goals/handoff:** no claim of NuGet, browser, AOT or unsupported-runtime support.
  Announce in the core release only after API/support and platform evidence is complete.

### Package 23: generator — ✅ DONE (2026-10-27)

- **Correctness update (2026-10-28).** Emission schema **2** preserves declarations from every
  document contributing a named package or namespace; shared `symbolfacts` enumeration joins their
  children without merging document roots, non-namespace declarations or library/model homonyms.
  Statically unknown upper bounds generate lists, unknown lower bounds with upper bound one generate
  nullable properties, and arbitrarily large numeric bounds are classified without integer overflow.
  Inherited facts are traversed with cycle-safe worklists rather than a depth cutoff; multiple
  redefinitions contribute the narrowest compatible type and intersected multiplicities, with
  explicit diagnostics for incompatible types, disjoint ranges and cyclic redefinitions.
  Contextual and intrinsic C# keywords are escaped, compiler-equivalent identifiers are normalized
  before collision checking, required raw values reject `Unset`, and expected filesystem failures
  report diagnostics and exit 2 while atomic-write cleanup preserves the original error.
  Regenerated all three goldens; the Cases consumer includes another package fragment, dynamic
  multiplicity, joined redefinitions and contextual/intrinsic type names.
  **Validation:** pinned SDK 10.0.402 on macOS arm64; generator **180**, consumer **21**, typed
  runtime **12** — **213 passed, no skips**, **74 additional cases**.
  `DOTNET_MIN_TESTS = 3093`. Locked restore, solution format verification and zero-warning build,
  golden checks, Vehicle sample, 77 Python gate self-tests, CI-routing tests, documentation links,
  documentation-label checks and changelog validation pass. `make dotnet-test` executes all
  **3093** cases: **3066 passed, 24 failed, 3 Linux-only skips**; the failures match the previously
  recorded capability/digest, structured-value, constraint and view drift outside this package.
  `go build ./...`, `go vet ./...` and `gofmt -l .` pass; fresh tests of both
  `internal/frontend/symbolfacts` and `internal/frontend/grpc` pass. The full Go gate is not green on
  this macOS environment: failures outside those packages include temporary-path spelling, external
  renderer/tool assumptions, lifecycle timing and solver/runtime expectations.

- **Status: ✅ DONE (2026-10-27).** Built on `feat/dotnet-client` at `b216991c7`, left **uncommitted** (new files
  intent-added). No schema or Go change; `parity.json` unchanged. Delivered: **runtime** `OpenMBEE.OpenSysML.Typed`
  (`ITypedObject`, `TypedObject`, `TypedRegistry` with usage-to-definition aliases, `FeatureDecoders.As*`,
  `TypedFeatures.Required/Optional/OptionalValue/List`, `FeatureDecoder<T>`, `TypedFactory`) and
  `FeatureValueException`, `TypeMismatchException`, `InstanceTypeException` (49 unshipped API entries);
  **generator** `src/OpenMBEE.OpenSysML.Generate` (console; closure reader, stamp, symbol walk, planner, emitter,
  CLI `SOURCE... --namespace NS [-o OUT] [--check] [--address A | --binary B]`, exits 0/1/2). Contract as built:
  interface per definition (enumerations excepted: their features hold literals), class inheriting the first
  generated base, every property an explicit implementation, re-emitted only where a class reads another slot
  name than its base class; inherited properties read the most specific redefining feature by type facts
  (renamed redefinitions included); refusals name source ids for unrelated same-named live features
  (case-normalized too), meeting redefinitions, unsound redefinitions (element conformance with exact numeric
  widening, cardinality), C# type/interface/namespace/registry collisions, reserved members, self-specialization,
  unnamed definitions and unresolved references. Packages are namespaces; non-package owners join with `_`;
  keywords take `@`; properties are PascalCase. Stamp: `sha256` over document count then, in ordinal name
  order, u64be-length-delimited logical name and exact bytes (file name, or directory name plus relative path);
  `GeneratorVersion = "1"`. Unnamed members the service lists as `Owner::` are skipped (it cannot look them
  up); any other missing child fails. **Tests:** `OpenMBEE.OpenSysML.Generate.Tests` (110: synthetic
  naming/typing/inheritance, in-memory Roslyn compiles with warnings and CS1591 as errors, stamp pinned
  independently, CLI, offline golden-stamp gate, live golden regeneration over three wires, `--check`
  drift on a changed file inside a directory input without writing, capability refusal proven to send only
  GetServerInfo); `OpenMBEE.OpenSysML.Generate.Consumer` (18; references the library alone, compiles the three
  goldens under the client build policy, offline shape/view tests and live subtype/feature reads over three
  wires); `TypedRuntimeTests` (11). Goldens from `vehicle_package.sysml` and the Python multi-file fixture
  (shared inputs) plus `test/testdata/generate/cases/` (two files), recorded in `goldens.json`; `dotnet-gates.py
  goldens [--check]` (+5 self-tests); Make `dotnet-generate`, `dotnet-generate-goldens`, `dotnet-generate-check`;
  `DOTNET_MIN_TESTS := 3019`; routing for `client/python/tests/fixtures/multifile/` (+2 cases); goldens
  `linguist-generated`. Docs: client README section and status, guide section and limitation, API reference
  section (CLI, mapping, rules, stamp, runtime) and exception tree, `clients.md` model-ergonomics sentence,
  roadmap Track I; fragment `dotnet-client-typed-generator.added.md`.
  **Passed — macOS arm64** (SDK 10.0.402, runtime 10.0.13, Homebrew OpenJDK in `JAVA_HOME`): G1 (locked restore,
  `dotnet format --verify-no-changes`, `--no-incremental` 0-warning build, Unit 2672: 3 failed, all pre-existing);
  G2 `--minimum-expected-tests 3019` with `OPENSYSML_REQUIRE_SERVICE=1`, `OPENSYSML_REQUIRE_INTEROP=1`: 3019
  total, 2992 passed, 3 Linux-only skips, **24 failed — exactly the pre-existing drift set package 22 records**
  (capability constant, release digests, composite constraint, RenderView, sequence-shaped results), none in
  code this package touches; `make dotnet-generate-check`; `dotnet-gates-test.py`; `ci-changed-areas-test.sh`;
  `make docs-check` (throwaway venv); `changelog.py check`, `check-doc-ids.py`, `check-doc-links.py`;
  `git diff --check`. A scratch consumer with warnings as errors, CS1591 and Recommended analyzers compiled a
  probe model's output and read it live before the goldens existed.
  **Negative controls** (each restored, verified by digest): a comment appended to `cases/parts.sysml`
  [`dotnet-generate-check` failed naming the golden; offline stamp test failed]; a duplicate member in a golden
  [consumer build CS0102/CS0109/CS0535]; a golden's doc comment removed [CS1591]; live-name conflict check
  disabled [2 failures]; `new` hiding disabled [6, including live golden regeneration]; type-facts requirement
  removed [3]. Cleanup: scratch probes and venv removed; no `sysml-grpc` or test-host process left.
  **Not run:** hosted CI (fork Actions disabled), Linux/Windows native runs, strict `make docs` (no nav or page
  added), `make dotnet-coverage`, `make conformance-dotnet[-check]` (no RPC or adapter change), `go test ./...`
  (no Go change). Only `net10.0` is built, per the 1.2 decision; the plan's per-TFM wording reduces to it.

- **Outcome/prerequisites:** typed model classes, TypedObject runtime and checkout console;
  model, values, connect. Independent of metamodel work and not required for service parity.
- **Read first:** [Python golden contract](../../../client/python/tests/test_generate_golden.py),
  [Python typed tests](../../../client/python/tests/test_typed.py), Node generator/typed implementation.
- **Owned surface:** Generate project, main-library TypedObject/decoders, C# golden fixtures/
  consumer projects, dotnet-generate target, README/reference extension.
- **Contract:** one interface per definition, class inherits the first base in model-declared
  order and implements all base interfaces. Diamond ancestry is deduplicated. Feature identity
  and redefinition follow type facts, not getter-name coincidence. For incompatible inherited
  types, generate explicit interface implementations with the appropriate decoder when sound;
  otherwise fail generation with a diagnostic naming both feature IDs, never discard a base.
- **Naming/types:** namespace validation, keyword escaping, normalized name/case collisions,
  generated/runtime member collisions and cross-file qualified references are deterministic.
  Reject ambiguous output names with source symbol IDs rather than arbitrary suffixes.
  Multiplicity 1/0..1/many maps to required/nullable/immutable-list accessors; open or unsound
  type facts fall back honestly to Value. FromInstance accepts semantic subtypes. Require
  type_facts before generation, with CapabilityException/Remedy when unavailable.
- **Determinism:** order declarations by qualified name. Stamp generator version and hash
  of the declared input source closure: sorted logical document names plus length-delimited
  exact UTF-8 bytes; include imported model sources, not machine paths/timestamps/service
  versions. Stdlib contract is captured by generator version. Unsupported unresolved source
  closure fails instead of stamping an incomplete hash.
- **Tests:** shared Python/Node input models; keywords/collisions/diamonds/redefinitions/
  incompatible features/cross-file imports/multiplicity; --check no-write/drift behavior;
  generated real consumer projects build against each TFM's reference assemblies and run
  live subtype/feature reads. No host-runtime-only Roslyn compile as dual-TFM evidence.
- **Gate/negative control:** G1/G2 and golden regeneration check; change a transitive input
  and stamp/check must fail; generated invalid C# must fail the consumer build.
- **Non-goals/handoff:** no Roslyn source generator, implicit MSBuild service startup or
  packaged dotnet tool. CLI: `dotnet run --project client/dotnet/src/OpenMBEE.OpenSysML.Generate
  -- model.sysml -o ModelTypes.cs --namespace Acme.Model [--check]`; make wrapper matches.

### Package 24: metamodel-generate — ✅ DONE (2026-10-28)

- **Status: ✅ DONE (2026-10-28).** Built on `feat/dotnet-client` at `5296eca20`, left **uncommitted**.
  No schema change; `parity.json` unchanged. **Scope split (maintainer decision, 2026-10-28):** this
  package ships the generated types, their public metadata and the runtime they compile against;
  `MetamodelGraph` (`All<T>`, indexer), `ReadJson` and the remaining exceptions belong to package 25.
  The plan's `graph.All<IPartUsage>()` is therefore proven here as `OfType<IPartUsage>()` and `is` over
  one element of every concrete metaclass, built through the internal seam.
  **Delivered — generator:** `tools/gen/csmetamodel` (`-check`, `-out`) writes
  `src/OpenMBEE.OpenSysML.Metamodel/Generated.cs` (18,512 lines) from the ontology table: interface
  `I<Metaclass>` per metaclass (175) extending its parents' interfaces (roots extend
  `IMetamodelElement`), declaring its own properties read-only and PascalCased (single → nullable,
  many → `IReadOnlyList<T>`); sealed class per concrete metaclass (167) in
  `OpenMBEE.OpenSysML.Metamodel.Elements`, internal constructor, every inherited property implemented
  **explicitly** (avoids CS0542 for the six metaclasses that inherit a property of their own name,
  e.g. `Feature`/`Type::feature`); enum per enumeration (7), members PascalCased literals valued by
  position; generated half of `Metaclasses` (one `MetaclassInfo` static property per metaclass,
  internal `MetaProperty` fields, `MetamodelVersion`/`SourceTag`/`SourceCommit`, a static constructor
  initializing parents-first with the generator's C3 linearization and linking redefines/subsets/
  opposites). Same-named redefinitions along an ancestry line are supported (`new` member in the
  subclass interface, every interface of the chain reads the most specific declaration) when they
  name the redefined property and conform (same kind/multiplicity, narrower or equal range); refused
  otherwise, as are unrelated declarations meeting in a subclass, keys sharing a C# name, runtime
  member names (`Id`, `JsonType`, `MetaclassInfo`, object's), a property named like its interface,
  non-PascalCase/underscored metaclass names, type/registry name collisions, bad or colliding enum
  literals, unsupported ranges and dangling redefines/subsets/opposites. The runtime member is
  `MetaclassInfo`, not `Metaclass`: the real table has `MetadataFeature::metaclass`. Topological order
  and C3 moved to a shared `tools/gen/hierarchy` package that `pymetamodel` now uses (Python output
  byte-identical). **Runtime (handwritten, 70 unshipped API entries):** `IMetamodelElement`,
  `MetamodelElement` (internal `Supply`/`IsSupplied` seam for the reader; `private protected`
  `ReadText`/`ReadValue<T>`/`ReadReference<T>`/`ReadList<T>`), `MetaclassInfo`, `MetaProperty`,
  `PropertyRangeKind` (`Metaclass`/`Enumeration`/`Primitive`), `EnumerationInfo`, `Metaclasses`
  lookups (`All`, `Enumerations`, `TryGet` by name/type, `Of<T>()`, `TryGetEnumeration`),
  `MetamodelException : Exception`, `NotSuppliedException` (`Property`, `ElementId`, `IsDerived`).
  The project references no project and, at run time, no package (PublicApiAnalyzers is
  `PrivateAssets="all"`). **Plumbing:** slnx; exact-path `.editorconfig` section for `Generated.cs`
  (generated_code, RS0016/RS0041); `DOTNET_GENERATED` gains it (coverage exclusion), and
  `PublicApiBoundaryTests.TheGeneratedExemptionsNameExactlyTheGeneratedFiles` now reads every
  `generated_code = true` section and pins both lists to the three generated files;
  `make dotnet-metamodel`/`dotnet-metamodel-check` (+ `.PHONY`); CircleCI step beside
  `python-metamodel-check`; CI routing sends `internal/translate/rdf/ontology/{table.go,
  sysml-metaclasses.ttl}` to the dotnet area (+3 routing cases); `.gitattributes`;
  `DOTNET_MIN_TESTS := 3121`. **Docs:** README status sentence, "Metamodel types" section, testing
  paragraph, protocol-code and build-policy sentences; API reference "Metamodel types" section,
  exception tree and limitation; guide limitation; roadmap I9; testing skill recipe; fragment
  `dotnet-client-metamodel-types.added.md`.
  **Tests:** Go — `csmetamodel` (committed drift with content probes; `-check` refuses a stale file
  without writing, writes, then accepts; synthetic golden; 29 refusal cases; determinism under
  reordered input; PascalCase), `hierarchy` (order, diamond MRO, four refusals). C# —
  `OpenMBEE.OpenSysML.Metamodel.Tests` (28, Unit, references the Metamodel project alone): hierarchy,
  abstractness and every interface-assignability pair held against the committed `.ttl`;
  `OfType`/`is` selection over one element per concrete metaclass; sealed classes without public
  constructors; linearized ancestors; registry lookups; every interface declares exactly its
  metaclass's properties with the expected CLR type; effective sets and JSON-key/C#-name maps; every
  inherited property of every element class read through its declaring interface (supplied, absent
  → `NotSuppliedException` with derived flag, null → null/empty); `Supply` refusals; metadata
  (ranges, defaults, redefines, subsets, opposites); immutable metadata collections and snapshots of
  initialization arrays; enum literal maps; provenance against
  `table.go`; assembly references and both lock files free of the client, protobuf, gRPC, logging
  and Sigstore. `SyntheticGenerationTests` Roslyn-compiles the runtime with
  `test/testdata/metamodel/synthetic.golden.cs` (redefinition chain over two generations, diamond
  under a property-declaring ancestor, every range kind) under nullable, warnings and CS1591 as
  errors, loads it and reads every redefinition through every interface.
  **Passed — macOS arm64** (SDK 10.0.402 from `~/.dotnet`, Homebrew's 10.0.403 rejected by
  preflight; runtime 10.0.13; Homebrew OpenJDK): `go build ./...`, `go vet ./...` (root and tools),
  `gofmt -l . tools` empty; `go test -C tools ./gen/...`; `go test ./internal/translate/rdf/ontology
  ./tests/hygiene`; `make python-metamodel-check dotnet-metamodel-check`; `make dotnet-build`
  (preflight, locked restore, 0-warning build; also `--no-incremental`); `dotnet format
  --verify-no-changes`; G2 `dotnet test --solution … --minimum-expected-tests 3118` with
  `OPENSYSML_REQUIRE_SERVICE=1 OPENSYSML_REQUIRE_INTEROP=1` and a freshly built `bin/sysml-grpc`:
  **3118 total, 3090 passed, 3 Linux-only skips, 25 failed** — the 24 pre-existing failures package
  22/23 record (capability constant 1, release digests 2, query/views 6, sampling 3, values 3,
  verification 9) plus one intermittent `SignedReleaseTests.SameOriginTrust…`, a dispose race in the
  test-only `ReleaseServer` (`client.Client` null at line 276) that passed 5/5 in isolation and in the
  first full run; nothing in either touches this package. `dotnet-gates-test.py`;
  `ci-changed-areas-test.sh`; `make docs-check` (throwaway venv); `doc-counts -check`;
  `changelog.py check`, `check-doc-ids.py`, `check-doc-links.py`. `sync-release-digests.py --check`
  fails on the pre-existing digest drift (files untouched here).
  **Negative controls** (each restored, verified by SHA-256): `PartUsage::partDefinition` made
  non-derived in `table.go` [`make dotnet-metamodel-check` exit 1; `TestGeneratedMetamodel` failed];
  one explicit implementation removed from `Generated.cs` [build CS0535]; an interface's doc comment
  removed [CS1591]; the `.editorconfig` exemption removed [1922× RS0016; guard test failed];
  `PartUsage`'s parent changed in the `.ttl` [3 `HierarchyTests` failed]; a `ProjectReference` to the
  client added to the Metamodel project [`LockedDependencyGraphsExcludeTheClient` failed]. In the
  synthetic compile, a dropped `new` [CS0108], a dropped implementation [CS0535] and a dropped doc
  [CS1591] are asserted by `BrokenOutputFailsTheCompile` itself. Cleanup: backups, scratch stats tool,
  docs venv and `dotnet-install.sh` removed; no `sysml-grpc` or test host left.
  **Not run:** hosted CI (fork Actions disabled), Linux/Windows native runs, `make dotnet-coverage`,
  `make conformance-dotnet[-check]` and `make dotnet-generate-check` (no RPC, adapter or generator
  change), strict `make docs` (no page or nav added), full `go test ./...` (only `tools/gen` and the
  shared `hierarchy` extraction changed; their packages, `pymetamodel` included, pass). Only `net10.0`
  is built, per the 1.2 decision; the plan's "dual-TFM" consumer reduces to the net10 test project.
- **Outcome/prerequisites:** standalone generated interfaces/classes/enums; scaffold.
  Independent of the client, generator and service lifecycle.
- **Read first:** [Python metamodel generator](../../../tools/gen/pymetamodel/main.go), ontology
  classes/properties/enums and Python metamodel typing/toolkit tests.
- **Owned surface:** tools/gen/csmetamodel, Metamodel project/Generated.cs, metamodel tests,
  dotnet-metamodel and dotnet-metamodel-check; exact generated analyzer exclusions.
- **Contract/tests:** one interface per metaclass, sealed class per concrete metaclass
  implementing every ancestor; effective inherited properties/ranges; PascalCase plus
  JsonKey mappings; diamonds/redefinitions/collisions; deterministic ontology provenance;
  graph.All<IPartUsage>() and is IFeature match semantic ancestry.
- **Gate/negative control:** generator Go tests, G1, generation check and dual-TFM standalone
  consumer build with only a Metamodel ProjectReference. Assert dependency graph contains
  neither main client nor protobuf/gRPC/logging/Sigstore packages. Changed ontology fails drift.
- **Non-goals/handoff:** no service-dependent types or shared client exception base. Share
  build version/policy only; generated structures feed the independent reader.

### Package 25: metamodel-read — ✅ DONE (2026-10-29)

- **Status: ✅ DONE (2026-10-29).** Built on `feat/dotnet-client` at `f0843b1d8` (package 24 committed
  as `d9605e908`), left **uncommitted**. No schema change; `parity.json` unchanged.
  **Design decision forced by real exports.** Reading `bin/sysml -convert api-json -stdlib` (136 MB,
  112,031 elements) showed that an eager failure per value makes real exports unreadable under any
  option: the export refers to library elements it omits, writes about 3,557 derived references
  outside their range (a constructor expression's `function` is the attribute definition it
  constructs), and types 98 library usages with names the metamodel lacks (`BooleanUsage`,
  `StructureUsage`, …) whose `ownedRelationship` it writes as one object. So, matching Python's
  contract exactly: **document** failures are eager (`MalformedDocumentException`: not JSON, a
  non-object item, missing/non-string `@id`/`@type`, duplicate `@id`, a key twice in one object,
  supertype cycles); a **value** that does not fit, or an unresolved `@id`/any `@ref`, is recorded and
  throws `MalformedValueException`/`UnresolvedReferenceException` on every read of that property only.
  The graph is still built in two passes and returned complete: every element created, every value
  converted, every reference resolved, no JSON retained. That stdlib export now reads in ~2 s;
  `CheckRanges=false` leaves exactly its 30 non-range shape problems. Keys a metaclass lacks
  (`sysx:*`) are not converted; `ReadOptions.KeepExtensions` (opt-in, added 2026-10-29 at the
  maintainer's request to close the parity gap with Python's `json_value`) keeps them per element in
  `IMetamodelElement.Extensions`, an immutable dictionary of `JsonElement`s copied into one detached
  JSON object per element, so they outlive the source; without it, `Extensions` throws
  `InvalidOperationException` naming the option. On the stdlib export it keeps 138,697 members
  (25,901 `sysx:sourceText`) for +34 MB retained (849 vs 815 MB) and ~8% read time.
  **Delivered — runtime (47 new unshipped API entries):** `MetamodelGraph` (`ReadJson` × path/`Stream`/
  `JsonDocument`/`IEnumerable<JsonElement>`, each with and without `ReadOptions` — RS0026 forbids several
  optional-parameter overloads; `Elements`, indexer, `TryGet`, `Contains`, `All<T>()`, `Roots`,
  `CheckRanges`, `KeepsExtensions`; no collection interface, per CA1710), `ReadOptions` (`CheckRanges`,
  `KeepExtensions`, `Supertypes`,
  keys and values normalized, conflicting keys `ArgumentException`), `IMetamodelElement.GetValue
  (jsonKey)` (untyped read; the only way to reach a range-checks-off out-of-range reference, which no
  typed property can hold), `IMetamodelElement.Extensions`, `MetaclassInfo.GetProperty`, `UnresolvedReferenceException` (`TargetId`/
  `TargetRef`), `MalformedValueException`, `MalformedDocumentException`, `UnknownJsonKeyException`, all
  under `MetamodelException : Exception`. Internal: `GraphReader`, `TypedList`, element seams
  `SupplyUnresolved`/`SupplyMalformed`/`SupplyOutOfRange`/`KeepExtensions`. Type normalization (`sysml:`, SysML `#` IRI,
  eight-digit versioned IRI) is hand-matched, not `[GeneratedRegex]`, because the synthetic compile
  builds the runtime without source generators. **Generator:** an internal sealed stand-in class per
  abstract metaclass (8), created by the same factory seam, for elements typed as an abstract metaclass
  (Python's own tests write `@type: "Element"`) or reaching one through `Supertypes`/the `Element`
  fallback; `MetaclassInfo` now requires a factory for every metaclass (`ElementType` stays null for
  abstract ones). `runtimeMembers` gains `GetValue` and `Extensions`; `runtimeTypes`/`frameworkTypes` gain the new names;
  a new rule refuses a table without `Element` or `Namespace` (the reader names `INamespace` for
  `Roots`), so every Go test table gained a `Namespace`; synthetic golden regenerated.
  **Sample:** `samples/MetamodelReader` (Metamodel `ProjectReference` only) and
  `dotnet-gates.py metamodel-sample`, run by `make dotnet-test` with no service or binary named,
  holding its exact output over the Python client's `bigger.full.json`. **Plumbing:** slnx; client test
  project references the Metamodel project (lock updated); CI routing sends
  `client/python/tests/fixtures/metamodel/` to dotnet (+2 cases); `DOTNET_MIN_TESTS := 3206`.
  **Docs:** README status sentence, "Reading exported JSON" subsection, generator refusals, testing
  paragraphs; API reference reader section, exception tree, limitation; guide section and limitation;
  client guide index line; roadmap I9; testing skill recipe; packaged agent skill bullet; fixture README;
  fragment `dotnet-client-metamodel-reader.added.md` (package 24's stale "still to come" sentence removed).
  **Tests:** `ReaderInputTests`, `ReaderValueTests`, `ReaderExtensionTests`, `MetamodelGraphTests`
  (82 new, Unit): extensions not kept by default and refused loudly, kept as written and only those,
  an unrecognized type's extra keys, survival of every source's disposal or deletion; every
  source; borrowed stream/document stay open and the graph reads after disposal; DataVersion/Commit/
  standalone/null payload; type spellings and non-spellings; unknown → `Element`, supertypes chains,
  cycles, conflicts; abstract types; references resolved, unresolved (`@id`, `@ref`, mixed, single),
  cyclic and self; ranges on/off per item; NotSupplied derived/owned; null vs absent; 22 malformed
  values each failing only its property, typed and untyped; 17 malformed documents; duplicate keys under
  both parsers; invalid UTF-8; unknown keys; roots with owner metadata; identity; `All<T>` over every
  metaclass interface; a value for every property of every metaclass; the toolkit fixture port.
  `MetamodelExportTests` (Integration, 3 wires) reads a live `api-json` export, including an
  unresolved library `Real` and the kept `sysx:sourceText` of `engine`. `HierarchyTests`/`MetadataImmutabilityTests` updated for the stand-ins.
  **Passed — macOS arm64** (SDK 10.0.402 from `~/.dotnet`; runtime 10.0.13; Homebrew OpenJDK on
  `PATH` for interop): `go build ./...`, `go vet ./...` (root and tools), `gofmt -l . tools` empty;
  `go test -C tools ./gen/...`; `go test ./internal/translate/rdf/ontology`; `make
  python-metamodel-check dotnet-metamodel-check`; `make dotnet-build`; full `--no-incremental`
  `-warnaserror` build; `dotnet format --verify-no-changes`; G1 Unit (2842, 3 failed — the
  pre-existing capability constant and two release-digest tests); G2 `dotnet test --solution … 
  --minimum-expected-tests 3201` with `OPENSYSML_REQUIRE_SERVICE=1 OPENSYSML_REQUIRE_INTEROP=1` and a
  freshly built `bin/sysml-grpc`: **3201 total, 3174 passed, 3 Linux-only skips, 24 failed**, and
  after extensions (`--minimum-expected-tests 3206`) **3206 total, 3179 passed, 3 skips, 24 failed** — exactly
  the pre-existing set package 24 records (capability 1, digests 2, query/views 6, sampling 3, values 3,
  verification 9). A first G2 without Java on `PATH` also failed 6 `CacheInteropTests` for "no working
  Java toolchain"; all 19 pass with it. `dotnet-gates.py metamodel-sample`; `dotnet-gates-test.py`
  (82); `ci-changed-areas-test.sh`; `make docs-check` and strict `make docs` (throwaway venv);
  `doc-counts -check`; `changelog.py check`, `check-doc-ids.py`, `check-doc-links.py`.
  `go test ./tests/hygiene` has 16 failures from `.circleci/config.yml` (unknown anchor `scan-inputs`)
  and `pr.yml`, identical with these changes stashed; its size-cap and import checks pass.
  **Negative controls** (each restored, verified by SHA-256): malformed value thrown at read [5 reader
  tests failed]; `@ref` branch dropped [2]; `Supertypes` keys not normalized [2]; `CheckRanges=false`
  dropping every value check [`RangeChecksOff…` failed]; duplicate keys accepted [`DuplicateKeys…`];
  roots ignoring owner metadata [`RootsExclude…`]; unresolved references failing the read [all 3 live
  `MetamodelExportTests`]; `GetValue` removed from `runtimeMembers` [Go `runtime_method`]; stand-ins made
  public [`TestGeneratedMetamodel`, `TestSyntheticGolden`]; `Namespace` not required [Go
  `missing_Namespace`]; sample output changed [`metamodel-sample` exit 1]; extensions: `@id`/`@type`
  kept [3 extension tests]; source `JsonElement`s held instead of copied [`KeptExtensionsOutlive…`
  among 3]; not-kept reading as empty [`ExtensionsAreNotKeptByDefault`]; `Extensions` dropped from
  `runtimeMembers` [Go `runtime_extensions`]. Cleanup: backups, the
  stdlib export, scratch reader project and docs venv removed; no `sysml-grpc` or test host left.
  **Not run:** hosted CI (fork Actions disabled), Linux/Windows native runs, `make dotnet-coverage`,
  `make conformance-dotnet[-check]` and `make dotnet-generate-check` (no RPC, adapter or typed-generator
  change), full `go test ./...` (Go changes are confined to `tools/gen/csmetamodel`, whose tests pass).
  `sync-release-digests.py --check` fails on the pre-existing digest drift. **Limitations, documented:**
  no compact `api-json` form, no computed derived properties, `xsd:int` values beyond 32 bits are
  malformed (Python accepts any integer); extension keys are kept only on request, where Python always
  keeps them.
- **Scope carried from package 24 (maintainer decision, 2026-10-28):** package 24 shipped the
  generated types, their metadata (`Metaclasses`, `MetaclassInfo`, `MetaProperty`,
  `EnumerationInfo`), `MetamodelException` and `NotSuppliedException`, and the internal seam the
  reader fills: `MetaclassInfo.CreateElement(id, jsonType)` (pass 1) and
  `MetamodelElement.Supply(property, value)` (pass 2; value null, a `ValueType` instance, or an
  immutable `IReadOnlyList<T>` for many; once per property; the effective `MetaProperty` of the
  element's metaclass). This package owns `MetamodelGraph` with `All<T>()` and the indexer — the
  `graph.All<IPartUsage>()` contract package 24 proved only as `OfType`/`is` — `ReadJson`, the
  remaining exceptions, and an element for an unknown `@type` (`Element` is abstract, so there is no
  generated class to fall back to as Python does). Adding a runtime member such as `Graph` to
  `IMetamodelElement` must also add it to `runtimeMembers` in `tools/gen/csmetamodel`.
- **Outcome/prerequisites:** complete standalone graph JSON reader/toolkit; metamodel-generate.
- **Read first:** [reader tests](../../../client/python/tests/test_metamodel_reader.py),
  [typing tests](../../../client/python/tests/test_metamodel_typing.py),
  [toolkit tests](../../../client/python/tests/test_metamodel_toolkit.py) and their fixtures.
- **Owned surface:** MetamodelGraph/reader/runtime, immutable public graph APIs, exceptions,
  standalone samples, docs reference section and compatibility notes.
- **Contract:** ReadJson(path/Stream/JsonDocument/IEnumerable<JsonElement>, ReadOptions)
  understands flat elements, DataVersion and Commit envelopes. Borrowed streams/documents
  stay caller-owned; graph storage does not retain JsonElement values after source disposal.
  Resolve references in a two-pass graph build; no public partially initialized graph.
- **Tests:** all ported fixture behaviors, derived NotSupplied, @id/@ref and duplicate IDs,
  unknown keys/types, malformed scalars/envelopes, ranges with CheckRanges on/off,
  source disposal, cyclic references and interface-based queries. Exceptions are
  NotSuppliedException(Derived), UnresolvedReferenceException, MalformedValueException,
  MalformedDocumentException, UnknownJsonKeyException under MetamodelException : Exception.
- **Gate/negative control:** G1 plus standalone consumer read on net10 without service,
  main client or network; malformed/unknown values fail loudly, CheckRanges=false disables
  only range checking rather than all validation.
- **Non-goals/handoff:** document partial derived-property population in api-json, as Python
  does; no invented defaults for absent derived data. Add site/reference docs after the
  reader is complete, not at service-client announcement if this branch is still pending.

## 6. Dependencies, parallelism, and integration ownership

```text
decisions -> scaffold -> wire -> grpc -> connect
                         |       |        |
                         |       +---- model <---- values <--- wire
                         |               |
                         +-> suite-core  +-> lifecycle-local -> lifecycle-safety
                                  |      |          |
                                  +-> suite-adapters          +-> cache -> downloads
                                          |
                +------------+------------+------------+------------+
                |            |            |            |            |
             execution  verification   conversion   query-docs   graph-data
                             |            |
                          sampling     editing (also values/model/suite-adapters)

all service packages + lifecycle-safety + downloads -> release -> docs
model + values + connect -> generator
scaffold -> metamodel-generate -> metamodel-read
```

The following dependency table is authoritative; the drawing is only an overview. A listed
prerequisite means its contract and implementation have merged, not merely started. Its
Status column is the single place to see which packages are done (rules in 4.1).

| Package ID and name | Direct prerequisite IDs | Status |
| --- | --- | --- |
| 01: decisions | none | ✅ DONE (2026-10-21) |
| 02: scaffold | 01 | ✅ DONE (2026-10-21) |
| 03: wire | 02 | ✅ DONE (2026-10-21) |
| 04: grpc | 03 | ✅ DONE (2026-10-21) |
| 05: connect | 04 | ✅ DONE (2026-10-21) |
| 06: values | 03 | ✅ DONE (2026-10-21) |
| 07: model | 04, 06 | ✅ DONE (2026-10-22) |
| 08: lifecycle-local | 04, 01 | ✅ DONE (2026-10-22) |
| 09: lifecycle-safety | 08 | ✅ DONE (2026-10-22) |
| 10: suite-core | 02, 03 | ✅ DONE (2026-10-22) |
| 11: suite-adapters | 10, 07, 05 | ✅ DONE (2026-10-22) |
| 12: execution | 07, 06, 11 | ✅ DONE (2026-10-23) |
| 13: verification | 07, 06, 11 | ✅ DONE (2026-10-23) |
| 14: sampling | 13, 11 | ✅ DONE (2026-10-23) |
| 15: conversion | 07, 11 | ✅ DONE (2026-10-24) |
| 16: query-docs | 07, 06, 11 | ✅ DONE (2026-10-24) |
| 17: editing | 07, 06, 11, 15 | ✅ DONE (2026-10-24) |
| 18: graph-data | 07, 06, 11 | ✅ DONE (2026-10-24) |
| 19: cache | 08, 01 | ✅ DONE (2026-10-26) |
| 20: downloads | 19, 08, 01 | ✅ DONE (2026-10-26) |
| 21: release | 12, 13, 14, 15, 16, 17, 18, 09, 20 | ⛔ BLOCKED (fork Actions disabled by GitHub; see status) |
| 22: docs | 21 | ✅ DONE (2026-10-27) |
| 23: generator | 07, 06, 05 | ✅ DONE (2026-10-27) |
| 24: metamodel-generate | 02 | ✅ DONE (2026-10-28) |
| 25: metamodel-read | 24 | ✅ DONE (2026-10-29) |

Values and suite-core can proceed before transports finish. Private lifecycle is not a
dependency of ordinary external-service model work. Cache/downloads run alongside service
wrappers. Generator and metamodel are independent deliverables.

Parallel ownership:

| Owner | Exclusive area | Shared integration |
| --- | --- | --- |
| Foundation | Connection/transport/core options, generated registry, collection/value contracts | Freeze before wrappers; subsequent changes reviewed by affected owners |
| Service-area package | Its Model partial, option/result/converter files, adapter and tests | Inventory rows, capability constants, public API baseline, README/sample |
| Lifecycle/download | PrivateService/registry/resolver/cache/downloader and process probes | Connection open/close seams; no wrapper refactors |
| Integration | CI/Make, shared API baseline merge, inventory schema, final coverage/report gates | Serialize merge edits; do not rewrite another branch's entries |
| Generator/metamodel | Own projects/tools/tests and generated files | Build/version props and documented API extensions |

Do not start concurrent wrapper branches before domain/gate/method contracts stabilize.
Converters and scenario adapters must also be split by area; Model partials alone do not
make branches disjoint. Rebase onto merged prerequisites and rerun G1 plus owning integration/
conformance selectors. Final integration runs G3/G5/G6 without per-area filters.

## 7. Repository integration checklist

This table assigns integration work; it does not authorize announcements before release.
Future files are shown as code, existing authoritative files as links.

| Surface | Required change | Owner |
| --- | --- | --- |
| Root global.json; client build/lock files | Exact SDK/root discovery, net10 target, MTP runner opt-in, version, package pins and locks | scaffold |
| [Makefile](../../../Makefile) | dotnet-test/coverage; proto-csharp in proto; conformance-dotnet/check; later generate/metamodel/check; .PHONY/help | owning package, integration |
| [.gitignore](../../../.gitignore) | client bin/obj/TestResults and generated test reports, narrowly scoped | scaffold |
| [CI routing](../../../scripts/ci-changed-areas.sh) and [tests](../../../scripts/ci-changed-areas-test.sh) | dotnet area plus known_pattern; direct-client quick-tier/full-tier fanout; root SDK pin recognized; Python version-lockstep trigger for Directory.Build.props | scaffold |
| [PR workflow](../../../.github/workflows/pr.yml) | changes output; Linux job; native Windows/macOS lifecycle/cache jobs; all aggregate needs/results/status checks; artifact upload on failure | scaffold, lifecycle-safety, release |
| [CircleCI](../../../.circleci/config.yml) | digest-pinned SDK executor, approved runtimes, dotnet-test in build-test only; reports/coverage/conformance artifacts | scaffold, release |
| [Digest synchronizer](../../../scripts/sync-release-digests.py) | main library embedded digest copy included; byte equality gate | scaffold |
| [Python version checker](../../../client/python/scripts/check_version.py) and tests | --dotnet reads Directory.Build.props Version in SemVer spelling; lockstep pytest runs whenever that file changes | scaffold |
| [Sonar properties](../../../sonar-project.properties) | approved exclusion of client/dotnet from generic C# scan, reason comment; generated code not counted as handwritten coverage | scaffold, release |
| [Release process](../../../docs/project/releasing.md) | release-branch version checklist, checkout distribution, synced digests/no stamping, no package token/context; no tag publish dependency | release |
| `client/dotnet/README.md` | install, runnable Vehicle, protocols, dependency footprint, lifecycle scope/identity, coverage, binary/trust, support/limitations | every package |
| `docs/clients/dotnet.md`; `docs/reference/dotnet-api.md` | guide walkthrough versus exhaustive reference; checkout/not NuGet; generated/metamodel sections only when delivered | docs, later extensions |
| [Client guide](../../../docs/guide/09-clients.md) | eight ways/table, checkout prose, C# install/parse tabs, From C#/.NET | docs |
| [Client reference](../../../docs/reference/clients.md) | transport/distribution table, .NET host choice, lifecycle scope, provisioning/verification, conformance commands and coverage | docs |
| [Client landing](../../../docs/clients.md), [downloads](../../../docs/downloads.md), [MkDocs](../../../mkdocs.yml) | card, checkout entry, nav and site_description | docs |
| [README](../../../README.md), [CONTRIBUTING](../../../CONTRIBUTING.md), [architecture](../../../docs/internals/architecture.md), [roadmap](../../../docs/project/roadmap.md) | client lists, trees/status and integrations entry | docs |
| [Service transports](../../../docs/reference/service-transports.md) | default gRPC, explicit Connect protobuf/JSON | docs |
| [Self-model surfaces](../../../examples/self-model/surfaces.sysml) | DotnetClient, part and ArtifactChannel; generated outputs and ratcheted RDF gate | docs |
| [Packaged agent skills](../../../packaging/agent-skills/README.md) | opensysml-dotnet skill and table row; no unsupported feature claims | docs |
| `.agents/skills/testing-dotnet-client/SKILL.md` | reproducible required-service, orphan/sibling/ALC, lock interop, trust fixture and conformance mutation recipes | docs |
| [Changelog checker](../../../scripts/changelog.py) | one focused changes/unreleased fragment per PR, never edit CHANGELOG.md | every package |

CI details: use the existing service binary artifact on Linux, build a native service on
Windows/macOS, install buf at the chosen pin with checksum verification, restore locked,
format/build/test, regenerate/diff wire, run conformance and upload reports. Service changes
fan out in the full tier; draft quick-tier behavior remains consistent with repository policy.
No C# tag-time publication job or publishing-config hygiene placeholder is needed.

## 8. Completion, handoff, and exclusions

### 8.1 Package handoff evidence

Before marking a package complete, record the following, then apply the three done markers
described in 4.1:

1. Its prerequisite commits and the baseline/schema changes reconciled.
2. Delivered public signatures and updated inventory/scenario IDs; explicit limitations.
3. Exact commands and passed/failed/not-run results, by TFM/protocol/OS as applicable.
4. Negative control observed failing, then restored; no dirty temporary mutation remains.
5. Cleanup proof for owned child processes, fixture servers, lock holders and temp files.
6. Shared-file/API changes downstream owners must consume; no implicit dependency on an
   unmerged branch.

No handoff claims service parity solely from green unit tests, skipped integration tests,
generated-code compilation, a nonempty report, or a capability constant count.

### 8.2 Release definition of done

All 31 baseline RPCs and any subsequently added schema members are classified; every claimed
typed field is mapped/tested; all implemented expressible scenarios run across three
protocols and the withheld configuration; skips match the exact reviewed allowlist.
The net10 target executes and all three native platform guarantees pass. Root SDK pin,
consumer ProjectReferences, generation drift, version/digest lockstep, API tracking, quality
gates and the full repository gates are green. Public docs distinguish unsupported AOT,
browser, signing fallback, or runtime-support limits honestly.

Generator and metamodel have their own definition of done and may land after service-client
announcement. Their absence must not be hidden as completed scope; README/reference status
states what exists in the checked-out tree.

### 8.3 Explicit exclusions

- No NuGet publication, packaged dotnet tool, release stamping, SourceLink/symbol package
  pipeline or registry credentials. Revisit as a separate approved product change.
- No synchronous mirrors, notebook/DataFrame integration, FMI runner, combined sysml-wasm
  adapter, gRPC-Web client, or supported Blazor/browser distribution.
- No browser promise merely because an injected fetch-backed handler might carry Connect.
- No implicit compiler/MSBuild service startup, local semantic/execution engine, fabricated
  results, API downgrade, unsafe TLS bypass, silent request retry, or test weakening.
- No AOT/trimming compatibility claim without a separately approved executable gate.
