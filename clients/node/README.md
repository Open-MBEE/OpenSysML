# @opensysml/client

Node and browser client for OpenSysML: parse, inspect and instantiate SysML v2
models over the `sysml-grpc` service, using the [Connect
protocol](https://connectrpc.com/docs/protocol) with protobuf bodies. No native
addon, so an install is a plain registry fetch.

```bash
npm install @opensysml/client        # from npm, once the first release is published
```

```ts
import { loads, connect } from "@opensysml/client";

await using model = await loads(`package Demo {
  part def Wheel { attribute radius : ScalarValues::Real = 0.3; }
  part def Car { part wheels : Wheel[4]; attribute mass : ScalarValues::Real = 1500.0; }
}`);

const value = await model.eval("2 + 2");     // { kind: "int", value: 4n }
const car = await model.symbol("Demo::Car"); // by qualified name
await car.children();                        // its members, as symbols

const tree = await model.instantiate("Demo::Car");
const wheels = tree.get("wheels");           // a FeatureValue union
if (wheels?.kind === "many") {
  console.log(wheels.values.length);         // 4
}
```

`loads`/`load` open a connection of their own and close it with the model.
`connect()` is the longer-lived form, and parses more than one source over one
service and one parse cache:

```ts
await using connection = await connect();
const first = await connection.load("model.sysml");
const second = await connection.loads("package Inline { part def Thing; }");
connection.info.capabilities;                 // what this service can do
connection.model(first.hash);                 // a model the service already holds
```

Both `Connection` and `Model` implement `Symbol.asyncDispose`, so `await using`
closes them; `close()` is the explicit form, and is safe to call twice.

## Values are discriminated unions

Every `oneof` the service answers with arrives as a union to switch on, rather
than a generated message with optional fields:

```ts
switch (value.kind) {
  case "int":      value.value;                      // bigint, never lossy
  case "real":     value.value;                       // number
  case "complex":  value.value.real; value.value.imaginary;  // 1.5 - 2.0i, one value
  case "boolean":  value.value;
  case "string":   value.value;
  case "quantity": value.magnitude; value.unit;       // 1500.0 [kg]
  case "measurementRef": value.unit; value.unitTerm; value.unitId;  // a bare unit: km, reduced to 1000·metre
  case "function": value.calcId; value.selfId;       // a calc as a value: Demo::Sq, or holder.scale read off an object
  case "array":    value.dimensions; value.elements;  // row-major, an element is any SysMLValue
  case "vector":   value.components;                   // { kind: "int" | "real" }[]
  case "vectorQuantity": value.components;             // QuantityValue[], a unit per component
  case "set":      value.elements;                     // SysMLValue[], each once, unordered
  case "tensorQuantity": value.dimensions; value.components;  // any rank, row-major QuantityValue[]
  case "enum":     value.value.name;                   // and its literal/enumeration ids
  case "instance": value.id;                          // an object in the same tree
  case "sequence": value.elements;                    // SysMLValue[]
  case "null":     value.reason;                       // evaluated, no value
  case "unset":    break;                              // declared, never given one
  case "absent":   break;                              // the service sent no value at all
}
```

`unset` and `absent` are distinct on purpose: the first is a feature the model
leaves without a value, the second is a field the answer did not carry.
`SysMLVerdict` (`holds` / `fails` / `undecided`) and `FeatureValue` (`single` /
`many` / `error`) are unions of the same shape. Integers are `bigint`, because
the service's `int64` does not fit a `number` — an exact comparison against a
scenario expectation would otherwise be a lie.

## Two lifecycle modes

### A private child of this process (the default)

`connect()` with no address starts `sysml-grpc` as a child of this process with
`-port 0 -health-port 0 -report-address -exit-with-parent`, and reads the address
it bound from the child's first stdout line. No port is chosen, probed or
retried, so two processes starting at once cannot collide.

One child serves **every connection of a thread**: the first `connect()` starts
it, the last `close()` stops it, and sharing it shares the service's parse cache.
It is per thread rather than per process because the module state that holds it
is per thread — a `worker_threads` worker gets its own child, and closing the
worker's connections stops that child alone.

**No orphans, and the mechanism is not an exit hook.** The client holds the write
end of the child's stdin pipe and never writes to it; the child exits at end of
file. The kernel closes that pipe when the holder dies however it dies, which
survives what a `process.on("exit")` hook does not: `SIGKILL`, `process.abort()`,
an uncaught fatal error, a crash during shutdown.
`test/orphan.test.ts` proves it by `SIGKILL`ing a parent that holds a connection
and asserting the child is gone.

Node adds three wrinkles Python does not have, all deliberate here:

- **The event loop.** A referenced child handle keeps Node alive, so a script that
  forgot `close()` would never exit. The child and its three stdio handles are
  `unref()`ed as soon as the address arrives — before that they stay referenced,
  or Node could exit in the middle of starting the service. `stop()` `ref()`s the
  child again for as long as it waits for it to exit.
- **`detached: true`.** The child is started in a session of its own, so a
  `SIGINT` meant for this process does not reach it mid-call; stdin is what ends
  it. It is never `unref`ed *and* left running: this process holds the only write
  end of its stdin.
- **Worker threads.** Each thread owns its own child, as above.

On **Windows** the guarantee is the same and rests on the same mechanism: the OS
closes the anonymous stdin pipe when the owning process exits, however it exits,
so the child sees end of file. What differs is that there is no process group to
signal and no `SIGKILL` — `child.kill()` is `TerminateProcess` — so the orphan
test is POSIX-only (`process.kill(pid, 0)` and `SIGKILL` have no equivalent), and
prompt shutdown on Windows comes from closing stdin rather than from a signal.

### A service someone else runs (explicit opt-in)

```ts
const connection = await connect({ address: "localhost:50051" });
await connection.close();     // that service keeps running
```

or set `$OPENSYSML_SERVICE=host:port`. A connection made this way never owns the
service: closing it disconnects and nothing else. There is no adoption of a
service left listening by another process, no pidfile and no port probing.

## The browser

```ts
import { connect } from "@opensysml/client/browser";

await using connection = await connect({ address: "https://sysml.example.com" });
```

The browser entry point is the **explicit-address path only**: a browser cannot
spawn a process, so there is no private child there and nothing to fall back to.
It uses `@connectrpc/connect-web`, which is `fetch` and needs no proxy and no
sidecar.

Two limits to plan for rather than discover:

- The service must allow the page's **exact origin** — start it with
  `-cors-allowed-origins https://app.example.com`, never `*` — and must be
  served over TLS (`-tls-cert`/`-tls-key`) for an HTTPS page to reach it.
- `connect-go` v1.20 does not implement the base64 **`grpc-web-text`** variant.
  This client does not need it: a `fetch`-based Connect client sends and reads
  binary bodies directly. A `grpc-web` client that requires `-text` will not work
  against this service, whatever the client.

`test/browser.test.ts` runs this entry point against a real service over the same
`fetch` transport, and asserts the allowed origin is answered on the preflight
while another origin is not.

## Protobuf, not JSON

Bodies are protobuf by default. JSON is available (`connect({ encoding: "json" })`)
for `curl`-shaped debugging, and it is the same answers, but
[`docs/internals/design/transport-evaluation.md`](../../docs/internals/design/transport-evaluation.md)
measured a 468 KB response at ~6.5 ms with a protobuf body against ~42 ms with
JSON — `protojson` CPU on the service, not the ~10% difference in bytes. The
gRPC protocol is also available (`connect({ protocol: "grpc" })`) and carries
protobuf only; asking for `{ protocol: "grpc", encoding: "json" }` is refused
rather than silently downgraded.

## Capability negotiation

Clients negotiate on the capability names `GetServerInfo` reports, not on
versions:

```ts
import { CAPABILITY_EVALUATE_SUBJECT } from "@opensysml/client";

if (connection.info.has(CAPABILITY_EVALUATE_SUBJECT)) {
  await model.eval("mass", { subject: "Demo::sedan" });
}
```

The client checks the advertised list **before** making such a call so it can
raise a `MissingCapabilityError` naming the service, its version and the way to
get one that has it. A direct capability-gated request to a service without the
capability is refused with `UNIMPLEMENTED`; response-population capabilities
instead omit the fields they name. A service without `structured_values`,
`measurement_refs`, `function_values`, `set_values` or `tensor_values` sends the
value kinds those name (`array`, `vector`, `vectorQuantity`; `measurementRef`;
`function`; `set`; `tensorQuantity`) as `null` with an `unsupported: …` reason.
A function closing over the bindings of a behavior body has no wire form and is
sent as `null` by every service. A `set` arrives
with its elements in the service's canonical order, so two equal sets arrive
alike, and one listing a member twice is a `MalformedValueError`, whether it
arrives or is about to be sent; one sent to the service may list its members in
any order. `valuesEqual` is the membership test, as the service judges it: sets
by membership, sequences in order, numbers by value — `1` and `1.0` are one
member, exactly across the whole `int` range — and a quantity by magnitude
through its `unitTerm`, so `1 [m]` is `100 [cm]` (exactly, while the magnitude
is an `int` and the scale a whole ratio); one without a `unitTerm` is compared
in its unit as written. A `tensorQuantity` carries its `dimensions` and one
quantity per component, row-major.

## Failures are typed

Every failure is an `OpenSysMLError`. A call the service refused is a
`ServiceError` whose `code` is the RPC status it came back with (`"NOT_FOUND"`,
`"INVALID_ARGUMENT"`, …), and the statuses worth catching by themselves have a
subclass: `ModelNotFoundError` (the service no longer holds that hash),
`ModelFileNotFoundError`, `InvalidRequestError`, `ServiceTimeoutError`,
`UnsupportedOperationError`. A name the model has not got is a
`SymbolNotFoundError`, which carries the `symbolName` it looked for and the
`suggestions` closest to it:

```ts
try {
  await model.symbol("Wheeel");
} catch (error) {
  if (error instanceof SymbolNotFoundError) {
    console.error(`no ${error.symbolName}; did you mean ${error.suggestions[0]}?`);
  }
}
```

Source that does not parse is not a failure: `load`/`loads` return a model whose
`hasErrors` is true and whose `diagnostics` say where. Each `ModelDiagnostic` has
`severity`, `message`, `code` and an optional location; branch on `code`
(`"syntax"`, a validation code such as `"unresolved"`, `"choice-point"`,
`"guard-unevaluable"`; `""` when the service assigned none), not on the message
text. A service that populates `code` advertises `CAPABILITY_DIAGNOSTIC_CODES`;
without it every code is `""`. Options that cannot work
(an encoding that is not one, a timeout that cannot elapse, `grpc` with `json`)
are refused before a connection is opened or a service started.

## The service binary

The binary comes from an **optional per-platform npm package**, selected by npm
from its `os`/`cpu` metadata:

| package | platform |
| --- | --- |
| `@opensysml/sysml-grpc-linux-x64` | Linux x86-64 |
| `@opensysml/sysml-grpc-linux-arm64` | Linux arm64 |
| `@opensysml/sysml-grpc-darwin-x64` | macOS Intel |
| `@opensysml/sysml-grpc-darwin-arm64` | macOS Apple silicon |
| `@opensysml/sysml-grpc-win32-x64` | Windows x86-64 |

That is a normal registry install: npm verifies the tarball against the
registry's integrity hash, and **there is no postinstall script**, so a platform
with a package never downloads anything.

Resolution order:

1. `$OPENSYSML_BINARY` — a path to a binary, which wins over everything;
2. the platform package above;
3. `~/.opensysml/bin/sysml-grpc`, the cache the Python client also uses, filled
   by a verified download of a release when nothing above resolved;
4. `sysml-grpc` on `$PATH`;
5. otherwise: an error, or connect to a service someone else runs.

### Downloading a release

`resolveBinary()` downloads a `sysml-grpc-<os>-<arch>` release asset into
`~/.opensysml/bin/sysml-grpc` (`.exe` on Windows) when the steps above resolved
nothing — the same cache, the same metadata beside it in `sysml-grpc.json`, and
the same trust model as the Python client, so either client can use what the
other downloaded. `process.platform`/`process.arch` map to the five published
pairs (`linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`,
`windows-amd64`); any other pair is an error naming it rather than a fetch.

| variable | effect |
| --- | --- |
| `$OPENSYSML_BINARY` | a path to a binary, which wins over everything |
| `$OPENSYSML_GRPC_VERSION` | the release to download, else `latest` from the releases API |
| `$OPENSYSML_GITHUB_REPO` | the release repository, default `Open-MBEE/OpenSysML` |
| `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` | `<owner/repo>`, or `1` for any repository: accept same-origin trust |

The download goes to a temporary file, is hashed, and only then replaces the
cache path atomically and is `chmod 0700`ed (POSIX); a download that does not
verify leaves an existing cached binary and its metadata untouched and removes
the temporary file. A cached binary of another version is replaced with a
warning rather than used, and every request times out after 15 seconds. A
transport failure falls back to a cached binary that still verifies; a digest or
signature failure never does.

### What a download is verified against

In order, and each step refuses rather than falling back to the next:

1. **A shipped pin.** `release-digests.json`, synced from
   `clients/release-digests.json` by `python3 scripts/sync-release-digests.py`
   and published in the tarball, pins the SHA-256 of every asset of a release.
   Where it pins one, that is what the bytes must hash to, and a served
   `.sha256` that disagrees is tampering: the download fails.
2. **The release's signed manifest.** With no pin, the client downloads
   `SHA256SUMS.txt` and its sigstore bundle `SHA256SUMS.txt.bundle`, verifies
   the bundle against the release pipeline's certificate identity (the CircleCI
   OIDC issuer and project in `src/node/signing.ts`), and takes the digest from
   the verified manifest. Anything short of that — no bundle, a signature that
   does not verify, another signer, an expired certificate, a manifest changed
   after signing, a repository with no known signer, or the optional sigstore
   packages not installed — is refused exactly as an unpinned release is. A
   manifest digest that contradicts a pin is an error, not a downgrade.
3. **Nothing.** The download fails naming the version, because the `.sha256`
   served beside a binary comes from whoever served the binary: it detects
   corruption but not a compromised release.
   `$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD=<owner/repo>` (or `=1` for any
   repository) accepts that same-origin trust explicitly, with a warning saying
   so. It is never a way around a failed signature or a pin mismatch.

Verification uses `@sigstore/verify` with `@sigstore/bundle`,
`@sigstore/protobuf-specs` and `@sigstore/tuf` — the packages the `sigstore`
package is itself built from — as **optional** dependencies, so the client
installs and works without them and a release with no pin is refused where they
are missing. They are used rather than `sigstore.verify` because that entry
point takes its trusted root only through TUF, and both the Python client and
these tests verify against a recorded trusted root offline.

The per-platform packages are built by
`npm run platform-packages -- --binaries <dir>`, and each binary is packaged
**only** if its bytes match the `.sha256` sidecar beside it; a missing sidecar is
refused rather than trusted. The release job cross-compiles those binaries from
the tagged revision in the same run that publishes the packages and writes the
sidecars there, so nothing is downloaded to authenticate — which is the job the
Python client's pinned digests do — and an install is a normal registry fetch
carrying npm's own integrity hashes. npm's `--provenance` is not used because
the CLI mints attestations only on GitHub Actions and GitLab CI/CD, and this
repository releases from CircleCI. The README of each package records the digest
of the binary it carries.

## Generated stubs

`src/generated/sysml_pb.ts` is generated by `buf` from `api/proto/sysml.proto`
through the `protoc-gen-es` entry in `buf.gen.ts.yaml`. The plugin is this
package's devDependency, pinned in `package-lock.json` at the same version as the
`@bufbuild/protobuf` runtime the stubs import, so `make proto` regenerates them
from `node_modules` with no hand steps and no network fetch at generation time.

**The stubs are committed**, for the same reason the Python ones are: `npm
install @opensysml/client` must not need `buf`, Go or a network fetch of a
plugin, and a published tarball has to contain the compiled output. CI runs
`make proto-ts` and fails on any diff, so committed and generated cannot drift.

## What v1 does not do

This client covers `GetServerInfo`, `ParseFile`, `GetSymbol`, `Evaluate` and
`Instantiate` — connection, lifecycle, capability negotiation, and the five RPCs
above. Deliberately **not** in v1, rather than half-implemented:

- generated model-ergonomics types (`python -m opensysml.generate`'s equivalent);
- the edit API (`ApplyEdits`);
- RDF conversion (`Convert`);
- verification helpers (`VerifyConstraint`, `VerifyRequirement`,
  `VerifySatisfaction`);
- `Query`, `GetDiagnostics`, `EvaluateCalc`, `RunAnalysis`, `ExecuteAction`, `ExecuteState`.

`connection.rpc` is the escape hatch: it is the generated Connect client, so any
RPC not covered here can still be called, without the ergonomic layer.

## Examples

`examples/` is six programs against one model, a rover, in `examples/model.ts`.
They are written to be read in order, they assert what they print, and the test
suite runs every one of them, so they cannot drift from the API.

```bash
npm run examples          # all of them, in order
npm run example 03        # one, by number or name
```

| example | what it shows |
| --- | --- |
| `01-tour` | connect, parse, look up, evaluate, instantiate |
| `02-values` | every value kind, and what it decodes to in JavaScript |
| `03-symbols` | walking, lookup by name and id, type facts, adoption by hash |
| `04-instances` | instance trees, single and repeated features, unset and absent |
| `05-diagnostics` | syntax errors, the error each failure raises, refused options |
| `06-connections` | ownership, an external service, both protocols, calls at once |

## Conformance

The suite in `conformance/` is the service contract, and this client runs it
**through its public API** — `load`/`loads`, `eval`, `symbol`, `instantiate` —
not through the generated stubs. A scenario whose RPC v1 does not cover is
skipped with a reason, and the report has the same shape `cmd/conformance` emits:

```bash
npm run conformance -- --allow-skips --report report.json
```

| protocol | ran | passed | failed | skipped |
| --- | --: | --: | --: | --: |
| `grpc` | 59 | 23 | 0 | 36 |
| `connect` | 59 | 23 | 0 | 36 |
| `connect-json` | 59 | 23 | 0 | 36 |
| **total** | **177** | **69** | **0** | **108** |

The 36 skips per protocol are 35 scenarios for the 10 RPCs listed above plus one
the public API cannot express: a `ParseFile` naming no source, since `load` and
`loads` always name one. Every skip carries its reason in the report.

**The runner is not vacuous.** `--mutate <name>` corrupts a response on its way
through the client, and each mutation makes at least one scenario fail:

| `--mutate` | what it breaks | caught by |
| --- | --- | --- |
| `hide-capability` | drops a capability from `GetServerInfo` | `server_info/...` |
| `drop-diagnostics` | drops parse diagnostics | `parse/a_syntax_error_...` |
| `blank-symbol-kind` | blanks `SymbolInfo.kind` | `symbol/...` |
| `shift-integer` | adds 1 to an integer result | `evaluate/arithmetic_is_an_integer` |
| `drop-feature-values` | empties an instance's feature values | `instantiate/...` |

```bash
npm run conformance -- --allow-skips --mutate shift-integer   # must fail
```

## Development

```bash
npm install
npm run build        # dist/, what is published
npm run typecheck
npm run lint
npm test             # unit, lifecycle and service-backed tests
npm run conformance -- --allow-skips
```

The service-backed tests build `sysml-grpc` from this checkout on first use, or
use `$OPENSYSML_BINARY` when it names one. The `node-test` job runs all five
commands, plus the mutation checks and a stub-drift check, in both
`.github/workflows/pr.yml` (pull requests) and `.circleci/config.yml`.

## Release

Nothing here is published yet; the procedure is in
[docs/project/releasing.md](../../docs/project/releasing.md) under "The Node
client". In short: the `release-node` workflow runs on a `client-node-v*` tag,
cross-compiles the binaries, builds the per-platform packages from them, and
publishes those packages and then the client from the `npm` context. It needs
the `@opensysml` npm organization and an automation token, which a maintainer
supplies.
