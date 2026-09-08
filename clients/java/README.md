# opensysml-client (Java)

Java client for OpenSysML: parse, inspect and evaluate SysML v2 models over the
`sysml-grpc` service, from inside a JVM host application it does not own — an
Eclipse-based tool, a Cameo plugin, a web service.

```xml
<dependency>
  <groupId>org.openmbee</groupId>
  <artifactId>opensysml-client</artifactId>
  <version>0.1.0-SNAPSHOT</version>
</dependency>
```

Nothing is published yet. Build and install it into the local repository from a
checkout:

```bash
make build                                  # bin/sysml-grpc, which the tests start
mvn -f clients/java/pom.xml install          # sources and javadoc jars included
```

```java
try (Connection connection = Connection.open()) {      // starts a private sysml-grpc
  Model model = connection.load(Path.of("model.sysml"));

  Value sum = model.eval("1 + 2 * 3");                 // Value.IntegerValue[value=7]
  Value mass = model.evalWithSubject("mass", "Demo::sedan");

  Symbol vehicle = model.symbol("Demo::Vehicle");      // findSymbol returns Optional
  Instantiation built = model.instantiate("Demo::Vehicle");

  connection.capabilities().require(Capabilities.FEATURE_VALUES);
}
```

Every value the API answers with is immutable: `Value` is a sealed interface over
records (`IntegerValue`, `RealValue`, `ComplexValue`, `QuantityValue`, `ArrayValue`,
`VectorValue`, `VectorQuantityValue`, `MeasurementRefValue`, `FunctionValue`, `EnumerationValue`,
`InstanceReference`, `Sequence`, `NullValue`, `UnsetValue`, …), and `Symbol`,
`Diagnostic`, `Instance` and `Instantiation` are records with copied collections.
No generated protobuf message or builder appears in the public API.

## Exceptions: unchecked, and the distinction that matters

Everything the client throws is unchecked and descends from `OpenSysMLException`.
A host application handling a model is not helped by checked exceptions on every
call, and `AutoCloseable`'s `close()` here throws nothing.

| exception              | what happened                                                     |
| ---------------------- | ----------------------------------------------------------------- |
| `ServiceException`     | the call was refused, with a `StatusCode` (`NOT_FOUND`, …)         |
| `ModelException`       | the call succeeded and the answer reports a model failure          |
| `TransportException`   | HTTP or IO failure; the service was not reached or answered. `UNAVAILABLE`, except `DEADLINE_EXCEEDED` for a call that outlived its `requestTimeout` |
| `CapabilityException`  | the service does not advertise a capability the call needs         |
| `ServiceStartException`| no binary, a digest mismatch, or a child that would not start      |
| `ChecksumMismatchException` | a binary's bytes are not the digest required of them ([the service binary](#the-service-binary)) |

The `ServiceException`/`ModelException` split is the one the conformance suite
draws too: an expression that will not evaluate is a successful call carrying an
error, not a service problem.

## Maven, and a JDK 17 baseline

Maven, because a consumer of this ecosystem expects a POM: Eclipse tooling, Cameo
plugin builds and `mvn dependency:tree` all read one, and a Gradle consumer reads
the published POM as well.

`maven.compiler.release` is **17**, not the 21 this repository's environment has.
17 is the lowest baseline a realistic host can offer: Eclipse 2023-03 and later
require 17, so does IntelliJ 2023.2+, and Spring Boot 3 requires it. The client
uses records, sealed interfaces, switch patterns and text blocks — all 17 — and
nothing from 21, so 21 would exclude hosts for no gain.

## Dependency footprint

The compile-scope dependency is protobuf-java. Nothing else, by default:

| jar                          |  size | when                                    |
| ---------------------------- | ----: | --------------------------------------- |
| `protobuf-java` 4.33.1       | 1.8 M | always: the generated messages need it  |
| `protobuf-java-util` 4.33.1  |  76 K | only `Encoding.JSON`, declared optional  |
| `gson` 2.11.0                | 291 K | only `Encoding.JSON`, behind the above   |

There is no gRPC, no Netty and no `tcnative`. The transport is
`java.net.http.HttpClient` from the JDK, speaking the Connect protocol: unary
`POST /sysml.SysMLService/<Method>` with an `application/proto` body and
`Connect-Protocol-Version: 1`. That decision is about the host application, not
about elegance:

- `grpc-java` brings `grpc-netty(-shaded)`, `guava`, `perfmark` and optionally
  `netty-tcnative-boringssl-static`. Inside an OSGi/Eclipse runtime or a Spring
  application that already has its own Netty, that is the classic shading and
  classloader conflict, and a client is a bad reason to inflict it.
- `connect-kotlin` would add the Kotlin stdlib and OkHttp for a Java consumer.
- The service serves gRPC, gRPC-Web and Connect on one port, so choosing Connect
  costs no functionality: the same port, the same protobuf bodies.

Only the client's own transport is affected. A host that already uses `grpc-java`
for something else keeps it; nothing here conflicts with it.

## Protobuf bodies by default

`Encoding.PROTOBUF` is the default. `docs/internals/design/transport-evaluation.md`
measured a 468 KB `Query` answer at ~6.5 ms with protobuf against ~42 ms with
JSON — that is JSON parsing cost, not bytes on the wire. `Encoding.JSON` exists
so an answer can be compared against `curl`, and it needs the optional
`protobuf-java-util`; the conformance suite runs over both.

## Service ownership

A connection uses a service of its own and never stops one it did not start.

- `Connection.open()` starts a **private child**: `sysml-grpc -port 0
  -health-port 0 -report-address -exit-with-parent`. The kernel assigns the port
  and the child prints the address it was given on its first stdout line, so no
  port is chosen, probed or retried.
- **One child per classloader.** The registry holding it is static, so the copy
  of the client an Eclipse plugin loaded, the copy a web application loaded and a
  copy shaded inside a third library each own one child, while every connection
  made through one copy shares a child — and therefore its parse cache, which is
  what makes a second connection and a repeat parse cheap. Per-instance would
  spawn a service per connection and reparse every model; a JVM-wide singleton
  would put one tenant's models in another tenant's cache and would outlive an
  undeployed application.
- A host that must not share a cache across tenants passes
  `ConnectionOptions.builder().isolatedService(true)`, which starts a child for
  that connection alone and stops it when that connection closes.
- A private child stops when the **last** connection holding it closes.
  `Connection.close()` is idempotent, and `Connection.stopSharedServices()` stops
  what this classloader still owns — call it from a plugin's `stop()` or a
  `ServletContextListener`, since unloading a classloader does not by itself stop
  a child (below).
- Reaching a service the client did not start is explicit: `service(host, port)`,
  `$OPENSYSML_SERVICE=host:port`, or `autoStart(false)` to require one. Closing
  such a connection leaves it running, always.

### No orphans

The client holds the write end of the child's **stdin pipe** and never writes to
it; the child exits at end of file. Nothing else holds that write end, so the
kernel closes it when the owning JVM goes away — which is what survives
`SIGKILL`, `Runtime.halt`, an `OutOfMemoryError` during shutdown and a JVM crash.
`ProcessHandle.onExit()` and shutdown hooks do not. On an orderly close the
client closes stdin itself and then destroys the process it started, so exit is
prompt rather than eventual, and it only ever signals the `Process` object of a
child it started — never a pid read from disk.

`OrphanSafetyTest` proves it: a child JVM opens a connection, prints the service
pid, and is killed with `kill -9`; the test then waits for that pid to be gone.

Per platform:

- **Linux**, **macOS**: as above. The JVM does not leak the write end into other
  children, since `ProcessBuilder` does not pass a parent's pipe endpoints on.
- **Windows**: the same anonymous pipe is closed by the OS when the owning
  process exits however it exits, so the guarantee is unchanged; `taskkill /F` is
  the `kill -9` of the test. The `-exit-with-parent` flag adds a job-object tie
  on the service side.
- **A classloader that is unloaded**: the child is not tied to the classloader,
  so a host that undeploys an application without closing its connections would
  leave a service running until its JVM exits. `ClassLoaderTest` loads three
  isolated copies of the client, checks each owns a different child, checks that
  closing the connections leaves none, and checks the copies are collectable.
  Nothing the client starts is a non-daemon thread, so an unloaded copy holds no
  thread either: the HTTP executor and the child's output pumps are daemon threads
  (`opensysml-http-*`, `opensysml-service-*`). Collection is not instant, though —
  on 17 the JDK's own `HttpClient-N-SelectorManager` thread outlives a closed
  client until the client is collected, and pins the loader meanwhile.

## Thread safety

`Connection` and `Model` are safe for concurrent use by many threads: a
connection holds one `HttpClient`, the shared-service registry is guarded by a
single lock, and `close()` is a compare-and-set. `LifecycleTest` opens
connections from eight threads at once and asserts one child was started and the
reference count reaches zero. Value types are immutable and therefore shareable.

## The service binary

The client resolves an installed binary, in order:

1. `ConnectionOptions.binaryPath(...)`;
2. `$OPENSYSML_GRPC_BINARY`;
3. `~/.opensysml/bin/sysml-grpc` (`.exe` on Windows) — the cache it shares with the
   Python client;
4. `PATH`.

`expectedBinarySha256("<hex>")` verifies the file's digest before it is executed
and refuses it otherwise, whichever binary is resolved and whether it was
downloaded or installed. An external service needs no binary at all.

### Downloading a release

Ask for a release and the client downloads it into that shared cache:

```java
ConnectionOptions.builder().downloadVersion("v0.3.0").build();   // or "latest"
```

The version is the caller's, else `$OPENSYSML_GRPC_VERSION`, else nothing —
**no version, no download**: without one the client only resolves what is
already there, so it never fetches a binary a caller did not ask for. `latest`
is resolved through the GitHub releases API. The repository is
`Open-MBEE/OpenSysML`, overridable with `ConnectionOptions.githubRepo(...)` or
`$OPENSYSML_GITHUB_REPO`. Every request times out after 15 seconds, and a
response body that stops arriving for that long is abandoned too, so a release
of any size may be downloaded but a hung origin never hangs a connection.

The release asset for the running platform (`sysml-grpc-linux-amd64`,
`-linux-arm64`, `-darwin-amd64`, `-darwin-arm64`, `-windows-amd64.exe`; any
other pair fails naming itself) is downloaded to a temporary file, verified, and
only then moved over the cached path, `chmod 0700` where the filesystem keeps
POSIX modes. A download that fails or does not verify leaves the cached binary
untouched and removes the temporary file. A response is read to a bound — 512
MiB for a binary, 8 MiB for a checksum, manifest, bundle or release listing —
so an origin cannot answer a download with an endless body. Deciding whether the
cache is the release asked for and replacing it when it is not is done holding
`~/.opensysml/bin/sysml-grpc.lock`, so concurrent connections — in this JVM, in
another, or in the Python or Rust clients, which take the same lock over the same
span — do not install over each other.

What a service is started from is not the shared path but a hard link to it
named for its digest, `~/.opensysml/bin/sysml-grpc-<first 16 hex>`, made while
the lock is held. A release installed over the cache afterwards is therefore not
the one a connection already resolved runs; a filesystem without hard links gets
a copy, and one that gives neither falls back to the shared path with a warning.

The cache is read and written exactly as the Python client does, so the two
share one binary: `~/.opensysml/bin/sysml-grpc.json` beside it records
`{"version": ..., "sha256": ..., "repo": ...}`. A cached binary of another
release, or one from another repository, is replaced with a warning rather than
silently used, and its digest is re-checked so a binary swapped in by hand is
not read as the release it displaced. A release that cannot be reached, or that
nothing vouches for, is no reason to lose a working binary: the installed one is
kept with a warning that it may be older than the release asked for. A download
that contradicts a pin or a signature is never answered from the cache — it
fails.

### What a download is verified against

In order, and each step is a refusal rather than a fallback:

1. **A pinned digest.** `release-digests.json` — this jar's synced copy of
   `clients/release-digests.json`, loaded from the classpath — pins a SHA-256
   per (repository, release, asset). Where a pin exists it is what the bytes
   must hash to, and a served `.sha256` that disagrees with it is a release
   republished with another binary: the download is refused.
2. **The signed checksum manifest.** For a release nothing is pinned for, the
   client downloads `SHA256SUMS.txt` and its sigstore bundle
   `SHA256SUMS.txt.bundle` and verifies the bundle with
   `dev.sigstore:sigstore-java` against the manifest's digest, requiring the
   release pipeline's identity — CircleCI OIDC issuer
   `https://oidc.circleci.com/org/1169df8b-0b59-400f-82d2-c9d8e98bdb62` and a
   pipeline definition of project
   `https://circleci.com/api/v2/projects/eeb0dddd-237f-4f02-9e51-8e24caef589d`.
   The asset's digest then comes from that verified manifest. A bundle that does
   not verify, another signer, an expired certificate or a manifest changed
   after signing is refused — never a quiet fall back to the served checksum.
3. **Nothing.** With no pin and no signature, the download is refused, naming
   the version, because the only checksum left is the one served beside the
   binary, which a compromised release would serve too.

The last step is the one opt-out: `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=1`, or set
to a comma-separated list naming `owner/repo`, accepts that served `.sha256`
with a warning. That is **same-origin trust** — it detects a corrupted transfer,
and nothing at all about a compromised release origin.

Excluding the `dev.sigstore:sigstore-java` dependency does not weaken any of
this: without it nothing verifies, which is refused exactly as an unsigned
release is, so only pinned releases install.

### Limitations

- A release published after this client's `release-digests.json` was synced is
  installed on its signature, so a jar built with `sigstore-java` excluded needs
  a client whose table pins that release.
- `latest` is one unauthenticated call to `api.github.com`, so a rate-limited
  host should name the version instead.
- The cache is one path per user, so two applications asking for different
  releases replace one another's cached binary — each still runs the release it
  asked for, from the digest-named link, but the next start re-downloads. Name
  the binary with `ConnectionOptions.binaryPath()` where that matters.
- The digest-named links are never collected: a cache that has held many
  releases keeps a link per release until `~/.opensysml/bin` is cleared.

## Capability negotiation

`Connection.open` calls `GetServerInfo` once and keeps what it reported.
`connection.capabilities().require(Capabilities.EVALUATE_SUBJECT)` throws
`CapabilityException` when a capability is absent. Negotiation is on the
advertised **names**, never on the version string. The service does refuse a
request that needs a capability it lacks — `UNIMPLEMENTED`, naming the
capability, which arrives here as a `ServiceException` — but a capability that
only describes how a response is populated omits its fields instead, so a call
that relied on failure alone would silently read an answer computed without
them. That is why `Model.evalWithSubject` checks before it calls: a
`CapabilityException` names the missing capability and the service that lacks
it, before a round trip.

## What v1 does not do

Deliberately out of scope, rather than half-implemented:

- **the edit API** (`ApplyEdits`) — authoring notation from Java;
- **RDF conversion** (`Convert`) — Turtle/RDF export;
- **verification helpers** (`VerifyConstraint`, `VerifyRequirement`,
  `VerifySatisfaction`), **behaviour execution** (`ExecuteAction`,
  `ExecuteState`), **`EvaluateCalc`**, **`RunAnalysis`** and **`Query`**/OSLC;
- **generated model-ergonomics types** — no code generation from a model into
  Java classes.

The service still serves all of them; reach them from another client, or from the
generated stubs in `org.openmbee.opensysml.proto` with `curl`, until a v2 wraps them.

## Generated messages

`org.openmbee.opensysml.proto` is committed, generated by `buf` from a plugin entry in the
root `buf.gen.yaml` with the version pinned inline, and regenerated by
`make proto` — no Maven plugin calls `protoc` and nothing here is hand-written.
Only the message classes are generated: the Connect protocol needs no service
stubs, so grpc-java never enters the build.

## Conformance

The runner in `opensysml-conformance` reads `conformance/scenarios/*.json` and
`conformance/fixtures/`, makes each call **through the public API** and compares
what the client read out of the answer, by the rules in `conformance/README.md`.
It writes the report shape `cmd/conformance` writes:

```bash
make build
mvn -f clients/java/pom.xml install -DskipTests
mvn -f clients/java/pom.xml -pl opensysml-conformance -q \
  dependency:build-classpath -Dmdep.outputFile=/tmp/cp.txt
java -cp "clients/java/opensysml-conformance/target/classes:$(cat /tmp/cp.txt)" \
  org.openmbee.opensysml.conformance.Main -binary bin/sysml-grpc -allow-skips \
  -protocols connect,connect-json -report bin/conformance-report-java.json
```

`-run <regexp>` selects scenarios by id, `-service host:port` runs against a
service the runner did not start, `-mutate` is below, and without `-allow-skips`
a skipped scenario is an exit code, so a shrinking API surface cannot go
unnoticed.

Or as a test, which is what CI runs: `mvn -f clients/java/pom.xml test`.

Per protocol, of 59 scenarios:

| protocol       | ran | passed | failed | skipped |
| -------------- | --: | -----: | -----: | ------: |
| `connect`      |  25 |     25 |      0 |      34 |
| `connect-json` |  25 |     25 |      0 |      34 |

**34 skipped**, and they are the scenarios of the RPCs v1 does not cover:
`ExecuteAction` (3), `ExecuteState` (2), `Convert` (5), `ApplyEdits` (5),
`VerifyConstraint` (4), `VerifyRequirement` (2), `VerifySatisfaction` (2),
`EvaluateCalc` (2), `Query` (8) — 33 — plus
`parse/naming_no_source_is_invalid`, which asserts that a request naming no
source at all is refused: the public API always names one, so the client cannot
send that request. gRPC is not run at all: this client does not speak it.

The runner is not vacuous. `-mutate` corrupts every answer before it is compared,
and `SuiteTest.aCorruptedAnswerIsCaught` asserts each corruption is caught:

| `-mutate`         | what it does to every answer     | scenarios that fail |
| ----------------- | -------------------------------- | ------------------: |
| `perturb-reals`   | moves each real by a millionth   |                   4 |
| `truncate-lists`  | drops the last repeated element  |                   7 |
| `rewrite-strings` | replaces each string            |                  13 |

## Running the tests

```bash
make build                                   # bin/sysml-grpc; tests skip without it
mvn -f clients/java/pom.xml test             # 119 client tests, 27 conformance tests
mvn -f clients/java/pom.xml test -Dopensysml.requireService=true   # CI: absence fails
```

## Publishing

Nothing has been published. The build produces a correct, signable artifact
(sources and javadoc jars, complete POM metadata, a `release` profile that signs
with GPG and stages to Sonatype Central with `autoPublish=false`), and
`mvn install` works today. What a maintainer must obtain first — a verified
`org.openmbee` namespace, a published GPG key, and Central portal
tokens — is in
[docs/project/releasing.md](../../docs/project/releasing.md#releasing-the-java-client-to-maven-central).
