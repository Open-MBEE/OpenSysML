# Developing the OpenSysML .NET client

How the client in this directory is built, tested and versioned; [README.md](README.md) covers
using it.

## Building and testing

```bash
make dotnet-test       # locked restore, build, format check, tests under coverlet, samples
make conformance-dotnet conformance-dotnet-check   # the conformance suite; see Conformance
```

`make dotnet-test` writes JUnit under `bin/dotnet/test` and Cobertura under `bin/dotnet/coverage`.
It runs these commands, which also work on their own:

```bash
dotnet restore client/dotnet/OpenSysML.slnx --locked-mode
dotnet format client/dotnet/OpenSysML.slnx --verify-no-changes --no-restore
dotnet build client/dotnet/OpenSysML.slnx -c Release --no-restore -warnaserror
dotnet test --solution client/dotnet/OpenSysML.slnx -c Release --no-build
```

`dotnet test` runs on Microsoft.Testing.Platform, which `global.json` selects; the VSTest
form `dotnet test <solution>` is not supported. Every test declares a `Category` trait,
`Unit`, `Integration` or `Interop`, and a test enforces that, so a filter never silently
drops one. `Unit` tests need only the checkout and the runtime; `Integration` tests need the
service binary; `Interop` tests need the other clients' toolchains.

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
dotnet-generate-goldens` regenerates them and `make dotnet-generate-check` checks them; CI checks them
through `GoldenTests`, which regenerates each golden over every wire as part of the suite.

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
Marker files are published atomically to prevent partial reads on Windows, including after restarts.

The deadline-only test times the target RPC, not connection setup. Cancellation-precedence checks pass already-cancelled tokens to transport error mapping rather than relying on timer ordering.

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
cache as it was and nothing staged.
The real binary is published there too, under a pin of its digest, and a private
service must start from its download, over each protocol, and a second open must fetch nothing.
The release fixture uses dedicated workers. Timing-sensitive header tests run the real downloader in the cache probe's separate process, preserving timeout bounds, redirect checks and connection-reuse assertions.

CI runs the whole suite on Linux under coverlet against the service its build job makes, then
the conformance comparison with the reference runner, and, in the full tier, natively on macOS
and Windows against a
`sysml-grpc` built on that runner, with the same minimum test count. Every one of those jobs
installs the Python, Java and Rust toolchains and requires the interop tests.
The Python gates use ASCII success markers so sample and coverage reports
also work with legacy Windows console encodings.

## Protocol code

`src/OpenMBEE.OpenSysML/Wire/Sysml.cs` and `SysmlGrpc.cs` are the message classes and the
gRPC client stub buf generates from [`api/proto/sysml.proto`](../../api/proto/sysml.proto)
into the `OpenMBEE.OpenSysML.Wire` namespace. They are committed, so a checkout builds with
neither buf nor network access. Do not edit them; regenerate them after a schema change:

```bash
make proto-csharp      # also part of make proto
```

CI regenerates them with the pinned buf before building and fails on any difference;
`make proto-csharp` followed by `git diff` on the two files does the same locally.

[`api/proto/buf.gen.csharp.yaml`](../../api/proto/buf.gen.csharp.yaml) pins the generators
to the runtime packages `Directory.Packages.props` pins: `protocolbuffers/csharp` v36.2 for
Google.Protobuf 3.36.2 and `grpc/csharp` v1.84.0, client stub only, for Grpc.Net.Client
2.84.0. The generated types are public and mutable protocol code, not the client's API:
they are left out of the public API baseline and of coverage by their exact paths, in
[`.editorconfig`](.editorconfig) and the Makefile's `DOTNET_GENERATED`, as the
[metamodel types](README.md#metamodel-types)' `Generated.cs` is, and a test fails when either names
anything else. Handwritten code refers to them as `Wire.X` rather than
importing the namespace, whose `Array` would otherwise shadow `System.Array`.

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
the request names; InstancePaths calls the method naming the object by id or the one naming it by
path, as the request names it. ApplyEdits reads each operation as the `Edit` of its arm, an empty field
an optional part left null. A failure the service reported in band is rendered as the
response's `error`, failure reason and diagnostics, with what the exception kept of the answer:
a run's clock and trace, a check's or an analysis's partial result, a verification's objects,
verification verdicts and engine standing, the table runs made, the run, sweep or sample a
recording made, with its own failure where it reported one, a conversion's or migration's
formats, notice and diagnostics, a refused edit's kind, diagnostics and referrers, a refused
import's diagnostics, a comparison's kind, source and engine standing, and the service's own words
for an element GetElementProperties does not know, which the API raises as a
`SymbolNotFoundException`; a refusal keeps the service's status and message, as does a
document query or document the model declares wrongly, which the API raises as a
`QueryException`; a
capability the client refuses for before sending is `UNIMPLEMENTED`, which the service would
answer; a transport failure is its status. Nothing is sent raw: a test reads the runner's IL
and fails on any call to `Connection.CallAsync` or the generated stub. A request setting a field
the adapter cannot send, or one the API has no way to express, is skipped for a stated reason;
so is one the API itself refuses with `ArgumentException`, for that exception's message.

Skips are accounted exactly. `expected-skips.json` lists the RPCs no adapter covers, today none,
and the scenarios whose request the API cannot express, each with the reason its adapter gives.
The run fails for any other skip, a listed
skip given another reason, and a listed skip that ran, whether or not `-allow-skips` is
passed; in a configuration withholding capabilities, a skip is accounted only by a withheld
capability, the service must advertise exactly the default set without them, and each must
have its without-capability expectation executed, or, when every such scenario is a listed
skip, the run says so. Per protocol, of 206 scenarios:

| Configuration | ran | passed | skipped as uncovered | skipped as unexpressible |
| --- | --: | --: | --: | --: |
| default | 199 | 199 | 0 | 7 |
| `strict_conformance` and `oslc_query` withheld | 198 | 198 | 0 | 7 |

The unexpressible requests send what the public types refuse to hold or send:
`parse/naming_no_source_is_invalid` names no source, which the API always does, and
`instance_paths/a_request_naming_no_object_is_invalid` names no object, which the API names by a
positive id or a path as a document binding does; three
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
| `truncate-lists` | Drops the last entry of each list | 87 |
| `rewrite-strings` | Replaces each non-empty string but no map key | 113 |

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
build. Both are marked `[Experimental("OSML0001")]` in their project files, so each entry carries
an `[OSML0001]` prefix, and every project here suppresses OSML0001 in `Directory.Build.props`, as a
consumer's project must.

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
While the two assemblies are experimental, a promotion records what a release had without
promising it to the next.
[Releasing OpenSysML](../../docs/project/releasing.md#releasing-the-net-client-as-the-tags-checkout)
describes the procedure.

`src/OpenMBEE.OpenSysML/release-digests.json` is a copy of
[`client/release-digests.json`](../release-digests.json), embedded in the assembly: the digests
a downloaded release must have, and the only ones this client trusts (see
[Downloading releases](README.md#downloading-releases)). Do not edit it:
`python3 scripts/sync-release-digests.py` rewrites it, and its `--check` and the tests fail
when it drifts. A checkout whose version is not yet released pins nothing for it, so its
default download is refused and gives way to the cache or `PATH`; ask for a pinned release
with `DownloadVersion` or `OPENSYSML_GRPC_VERSION` to download one.
