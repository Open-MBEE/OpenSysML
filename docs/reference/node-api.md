# The Node/TypeScript client API

This page covers what `@opensysml/client` exports, how its two entry points differ, and
where its v1 surface stops. To choose between the clients, see
[client libraries](clients.md); for a task-oriented walkthrough, see
[guide chapter 9](../guide/09-clients.md#from-node-or-a-browser). The client's own
notes on packaging and its conformance run are in
[clients/node/README.md](../../clients/node/README.md).

```bash
npm install @opensysml/client        # once the first release is published
```

Nothing is published yet, so a checkout builds it: `npm install && npm run build`
in `clients/node`.

## The two entry points

| import | reaches | private child service |
| --- | --- | --- |
| `@opensysml/client` | Node, over Connect with protobuf bodies | yes, by default |
| `@opensysml/client/browser` | a page, over `fetch` (`@connectrpc/connect-web`) | no — a browser spawns nothing |

Both re-export the isomorphic core; the browser entry point requires an
`address`, since there is nothing to fall back to.

## Opening a connection

```ts
import { connect, load, loads } from "@opensysml/client";

await using connection = await connect();          // private child of this process
const model = await connection.loads("package Demo { part def Car; }");
```

`load(path)` and `loads(source)` are the one-shot forms: each opens a connection
of its own and closes it with the model. `connect()` is the longer-lived form, and
every model parsed over it shares one service and one parse cache. `Connection`
and `Model` both implement `Symbol.asyncDispose`, so `await using` closes them;
`close()` is the explicit form and is safe to call twice.

`ConnectOptions` extends `TransportOptions`:

| option | effect |
| --- | --- |
| `address` | `host:port` or a URL of a service to use; without it, a private child is started |
| `encoding` | `"protobuf"` (default) or `"json"`; JSON costs ~6x the service's CPU on large answers |
| `protocol` | `"connect"` (default) or `"grpc"`; `{ protocol: "grpc", encoding: "json" }` is refused, not downgraded |
| `timeoutMs` | deadline applied to every call the connection makes |
| `headers` | extra headers sent with every call |
| `onResponse` | called with each response, for logging, metrics or a conformance runner |

`$OPENSYSML_SERVICE=host:port` is the environment form of `address`. A connection
to a service this client did not start is only disconnected from on `close()`,
never stopped.

## Reading a model

```ts
const model = await connection.load("model.sysml");
model.hash;                                  // what the service holds it under
model.diagnostics;                           // in the order the service reported them
model.hasErrors;                             // any error-severity diagnostic

const car = await model.symbol("Demo::Car"); // short name, FQN or id
await car.children();                        // its members, one call each
for await (const symbol of model.walk()) {}  // breadth-first from the root

const value = await model.eval("2 + 2");                       // SysMLValue
const mass = await model.eval("mass", { subject: "Demo::sedan" });
const tree = await model.instantiate("Demo::Car");
tree.get("wheels");                          // a FeatureValue of the root object
tree.byId(id);                               // any object the instantiation produced
```

`connection.model(hash)` adopts a model the service already holds, which lets a
hash pass between processes; an adopted model has no root symbol, so symbols are
looked up by qualified name, and the service answers `NOT_FOUND` once it has
evicted the model. `symbol()` searches breadth-first when given a short name and
raises `SymbolNotFoundError` naming near misses; `symbolById()` is the single call
for a name the service can resolve directly.

`ParseOptions` are `language` (`"sysml"` or `"kerml"`, for inline content) and
`strict`; both are capability-gated, and the client checks before it calls.

## Values are discriminated unions

Every `oneof` the service answers with arrives as a union to switch on, rather
than a message with optional fields:

```ts
switch (value.kind) {
  case "int":      value.value;                  // bigint, never lossy
  case "real":     value.value;                  // number
  case "boolean":
  case "string":   value.value;
  case "quantity": value.magnitude; value.unit;
  case "measurementRef": value.unit; value.unitTerm; value.unitId;  // a bare unit and its reduction
  case "array":    value.dimensions; value.elements;   // row-major SysMLValue[]
  case "vector":   value.components;             // Magnitude[]: int | real, kept apart
  case "vectorQuantity": value.components;       // QuantityValue[], one unit each
  case "function": value.calcId; value.selfId;  // a calc held as a value; selfId when read off an object
  case "enum":     value.value.name;             // and its literal/enumeration ids
  case "instance": value.id;                     // an object in the same tree
  case "sequence": value.elements;               // SysMLValue[]
  case "null":     value.reason;                 // evaluated, no value
  case "unset":    break;                        // declared, never given one
  case "absent":   break;                        // the service sent no value at all
}
```

`unset` and `absent` are distinct on purpose: the first is a feature the model
leaves without a value, the second a field the answer did not carry. `SysMLVerdict`
(`holds` / `fails` / `undecided`) and `FeatureValue` (`single` / `many` / `error`)
are unions of the same shape. Integers are `bigint`, because the service's `int64`
does not fit a `number` and an exact comparison would otherwise be a lie.
`decodeValue`, `decodeVerdict` and `formatValue` are exported for a caller
decoding a response it obtained itself.

## Errors

Everything derives from `OpenSysMLError`, so the family can be caught without
knowing its members.

| error | what happened |
| --- | --- |
| `ServiceError` | the service could not be reached, started, or answered nothing usable |
| `ServiceStartError` | a private child failed to start, or died while it was needed |
| `ClosedConnectionError` | the connection was closed and cannot be used again |
| `ParseError` | a file could not be read, or its content did not parse; carries `diagnostics` |
| `EvaluationError` | the call succeeded and the answer reports a model failure |
| `SymbolNotFoundError` | the model declares no such symbol |
| `MissingCapabilityError` | the service does not advertise a capability the call needs |
| `DownloadError` | a release binary could not be downloaded or installed |
| `ChecksumMismatchError` | a download's digest contradicts the one expected of it |
| `UnpinnedReleaseError` / `UnsignedReleaseError` / `ManifestSignatureError` | nothing pins the release, nothing signs it, or a signature does not verify |

The `ParseError`/`EvaluationError` split against `ServiceError` is the one the
conformance suite draws: an expression that will not evaluate is a successful call
carrying an error, not a service problem.

## Capability negotiation

Clients negotiate on the capability names `GetServerInfo` reports, never on the
version string:

```ts
import { CAPABILITY_EVALUATE_SUBJECT } from "@opensysml/client";

if (connection.info.has(CAPABILITY_EVALUATE_SUBJECT)) {
  await model.eval("mass", { subject: "Demo::sedan" });
}
```

The client checks the advertised list **before** making a gated call, so
`MissingCapabilityError` names the service, its version and how to get one that
has the capability, without a round trip. A service too old to answer
`GetServerInfo` at all is still usable: `connection.info.answered` is `false` and
its capability list is empty. Capabilities that only describe how a response is
populated omit the fields they name rather than refusing the call.

## The service binary

The binary comes from an optional per-platform npm package
(`@opensysml/sysml-grpc-{linux-x64,linux-arm64,darwin-x64,darwin-arm64,win32-x64}`),
selected by npm from its `os`/`cpu` metadata, with no postinstall script.
Resolution order is `$OPENSYSML_BINARY`, that package,
`~/.opensysml/bin/sysml-grpc` (the cache the Python, Java and Rust clients share),
then `sysml-grpc` on `$PATH`. `resolveBinary()` downloads a release into that
cache when nothing above resolved, verifying it against the digests pinned in the
published `release-digests.json`, else against the release's sigstore-signed
`SHA256SUMS.txt`; a release neither pins nor signs is refused unless
`$OPENSYSML_ALLOW_UNPINNED_DOWNLOAD` accepts same-origin trust explicitly.
[clients/node/README.md](../../clients/node/README.md) documents the download and
its trust model in full.

A private child is started with `-port 0 -health-port 0 -report-address
-exit-with-parent`, and the client reads the address from its first stdout line,
so no port is chosen, probed or retried. One child serves every connection of a
**thread** — a `worker_threads` worker gets its own — and stops when the last
connection closes. It cannot be orphaned: the client holds the write end of the
child's stdin and never writes to it, so the kernel closes that pipe however this
process dies, `SIGKILL` included, and the child exits at end of file.

## In a browser

```ts
import { connect } from "@opensysml/client/browser";

await using connection = await connect({ address: "https://sysml.example.com" });
```

Two limits to plan for: the service must allow the page's exact origin
(`-cors-allowed-origins https://app.example.com`, never `*`) and be served over
TLS for an HTTPS page to reach it; and `connect-go` does not implement the base64
`grpc-web-text` variant, which this `fetch`-based client does not need but a
`grpc-web` client requiring `-text` would.

## What v1 does not do

The ergonomic layer covers `GetServerInfo`, `ParseFile`, `GetSymbol`, `Evaluate`
and `Instantiate`. Deliberately absent, rather than half-implemented: generated
model-ergonomics types, the edit API (`ApplyEdits`), RDF conversion (`Convert`),
the verification helpers, `Query`, `GetDiagnostics`, `EvaluateCalc`, `RunAnalysis`,
`ExecuteAction` and `ExecuteState`. The service serves all of them;
`connection.rpc` is the escape hatch, being the generated Connect client, and
`SysMLService` is exported for a caller building its own.

## Conformance

`npm run conformance -- --allow-skips --report report.json` runs the
language-neutral suite through the public API, and emits the report shape
`cmd/conformance` emits. 59 scenarios per protocol over `grpc`, `connect` and
`connect-json`: 23 pass and 36 are skipped, being the 35 scenarios of the RPCs
above plus one the public API cannot express (a `ParseFile` naming no source).
`--mutate <name>` corrupts a response on its way through the client and each
mutation must make a scenario fail, which is what keeps the run from being
vacuous.
