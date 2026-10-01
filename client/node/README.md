# @openmbee/opensysml

Node and browser client for OpenSysML: parse, inspect and instantiate SysML v2
models over the `sysml-grpc` service, using the [Connect
protocol](https://connectrpc.com/docs/protocol) with protobuf bodies. No native
addon, so an install is a plain registry fetch.

```bash
npm install @openmbee/opensysml        # from npm, once the first release is published
```

```ts
import { loads, connect } from "@openmbee/opensysml";

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
  case "metaobject": value.elementId; value.metaclassId;  // an element reflected on: x meta KerML::Feature
  case "enum":     value.value.name; value.value.value; // its literal/enumeration ids, and the scalar a `high = 3` literal carries
  case "instance": value.id;                          // an object in the same tree
  case "sequence": value.elements;                    // SysMLValue[]
  case "undetermined": value.reason; value.countLower; value.countUpper;  // the model leaves it open
  case "infinity": break;                              // the unbounded `*`
  case "null":     value.reason;                       // evaluated, no value
  case "unset":    break;                              // declared, never given one
  case "absent":   break;                              // the service sent no value at all
}
```

`unset`, `undetermined` and `absent` are distinct on purpose: the first is a feature the
model leaves without a value, the second a model-level answer the model leaves open (an
unbound feature, a count the multiplicity does not fix) that is read but never sent, and the
third a field the answer did not carry.
`SysMLVerdict` (`holds` / `fails` / `undecided`) and `FeatureValue` (`single` /
`many` / `error`) are unions of the same shape; every verdict arm carries a `standing` — the
engine that answered, the strength of its evidence (`observed`, `witnessed`, `bounded`,
`proved`) and the bounds it ran under — empty from a service without the `engines`
capability. Integers are `bigint`, because
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
import { connect } from "@openmbee/opensysml/browser";

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
import { CAPABILITY_EVALUATE_SUBJECT } from "@openmbee/opensysml";

if (connection.info.has(CAPABILITY_EVALUATE_SUBJECT)) {
  await model.eval("mass", { subject: "Demo::sedan" });
}
```

The client checks the advertised list **before** making such a call so it can
raise a `MissingCapabilityError` naming the service, its version and the way to
get one that has it. A direct capability-gated request to a service without the
capability is refused with `UNIMPLEMENTED`; response-population capabilities
instead omit the fields they name. A service without `structured_values`,
`measurement_refs`, `function_values`, `set_values`, `tensor_values` or
`metaobject_values` sends the value kinds those name (`array`, `vector`,
`vectorQuantity`; `measurementRef`; `function`; `set`; `tensorQuantity`;
`metaobject`) as `null` with an `unsupported: …` reason.
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
quantity per component, row-major. A `metaobject` — what `x meta KerML::Feature`
or the last element of `x.metadata` evaluates to — names the element reflected
on (`elementId`, its identity) and the element's own metaclass (`metaclassId`,
not the type it was cast to); two are `valuesEqual` exactly when they name one
element. Its features (`declaredName`, `ownedFeature`, …) are read in the model,
not carried. One sent to the service may leave `metaclassId` empty to have the
model's used; one naming a metaclass that is not the element's is refused, and
one naming no element is a `MalformedValueError`.

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

The wider surface adds its own:

| error | what happened |
| --- | --- |
| `StaleServiceError` | the running service reports another version than `version` asked for |
| `ExecutionError` | an execution the service ran failed; carries `diagnostics` |
| `WrongKindError` | a verification or analysis named a symbol of another kind |
| `AnalysisRunError` | an analysis run failed before it could report |
| `ConversionError` | the service could not write the notation asked for |
| `MigrationError` | the service could not read the SysML v1 model, so nothing of it was migrated |
| `UnsupportedValueError` | the service sent a value this version of the client cannot decode |
| `QueryError` | a `Query` failed in-band |
| `DocumentQueryError` | a `runDocumentQuery` failed in-band |
| `EditError` | an edit was refused — `failure` names which, and the subclasses (`NoEditsError`, `EditTargetError`, `InvalidEditError`, `IllegalMemberKindError`, `RenameReferencedError`, `OverlappingEditsError`, `EditResultError`, `OwnerNotFoundError`, `OwnerNotNamespaceError`, `MemberNameTakenError`, `DeleteReferencedError`, `OwnerInsideTargetError`, `MoveReferencedError`, `ReferencedElsewhereError`) catch one kind of refusal |
| `TypeMismatchError` / `InstanceTypeError` / `FeatureValueError` | a typed view read a feature of another kind, an instance of another type, or a slot that is an error |

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
| `@openmbee/opensysml-sysml-grpc-linux-x64` | Linux x86-64 |
| `@openmbee/opensysml-sysml-grpc-linux-arm64` | Linux arm64 |
| `@openmbee/opensysml-sysml-grpc-darwin-x64` | macOS Intel |
| `@openmbee/opensysml-sysml-grpc-darwin-arm64` | macOS Apple silicon |
| `@openmbee/opensysml-sysml-grpc-win32-x64` | Windows x86-64 |

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
   `client/release-digests.json` by `python3 scripts/sync-release-digests.py`
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
through the `protoc-gen-es` entry in `api/proto/buf.gen.ts.yaml`. The plugin is this
package's devDependency, pinned in `package-lock.json` at the same version as the
`@bufbuild/protobuf` runtime the stubs import, so `make proto` regenerates them
from `node_modules` with no hand steps and no network fetch at generation time.

**The stubs are committed**, for the same reason the Python ones are: `npm
install @openmbee/opensysml` must not need `buf`, Go or a network fetch of a
plugin, and a published tarball has to contain the compiled output. CI runs
`make proto-ts` and fails on any diff, so committed and generated cannot drift.

## Several documents

`connection.parseSources` parses several documents as one model — files the
service reads, or inline content named for its diagnostics:

```ts
import { SourceDocument } from "@openmbee/opensysml";

const model = await connection.parseSources([
  SourceDocument.file("library.sysml"),
  SourceDocument.inline("user.sysml", "package User { import Library::*; }"),
]);
model.documents;   // the names the parse gave them, in order
model.roots;       // one root symbol per document
```

## Conversion and save

`connection.convert` writes a model, a file or source out in another notation —
`"sysml"`, `"kerml"`, `"turtle"`, `"api-json"` — and `save` writes the result to
a path, converting when needed:

```ts
import { save } from "@openmbee/opensysml";

const rdf = await connection.convert("turtle", { modelHash: model.hash });
await save(model, "out.ttl");            // format comes from the extension
await save(rdf, "copy.ttl");             // a Conversion writes what it holds
```

An experimental conversion emits a warning (`process.emitWarning`) rather than
failing, and reports it through `Conversion.experimentalNotice`.

## Migrating a SysML v1 model

A SysML v1 model — a Cameo/MagicDraw `.mdzip`, a UML XMI `.xmi` or an Eclipse
UML2 `.uml` export — is **migrated, not converted**: a conversion is lossless,
and a migration accounts for every v1 element as `mapped`, `approximated`,
`unmapped` or `skipped`. `connection.convert` refuses one with an
`InvalidRequestError` that says so and names `migrate`, whether `fromFormat` is
`xmi`, `uml` or `mdzip` or the path's extension is; `connection.migrate` is the
verb, and `save` writes what it answers with its image files beside it:

```ts
const migration = await connection.migrate("sysml", { path: "Model.mdzip" }, { report: true });
console.log(migration.report.summary);            // migrated 93 element(s): 77 mapped, …
for (const entry of migration.report.byVerdict("unmapped")) {
  console.log(`${entry.kind} ${entry.name}: ${entry.note}`);
}
await save(migration, "Model.sysml");             // and Model_images/… beside it
```

`save` refuses, with a `RangeError` and before writing anything, a path that is
the v1 model itself and an image that would land outside the model's directory
or over the model, as `sysml -migrate -o` does.

Inline `content` is the file's bytes (`Uint8Array`) and needs `fromFormat` to
say which form they are; a v2 `fromFormat` is refused with a pointer at
`convert`. The `Migration` carries the notation (or Turtle, `"ttl"`) and a
`MigrationReport` whose `summary` and four counts always come back; `report:
true` adds every element's `MigrationEntry` and the `text` the command's
`-migration-report` writes, `results: true` the `-migration-results` index,
and `layoutPath`/`layoutContent`, `imageBaseUrl` and `strict` are the other
companion flags. Migration is experimental and warns as the RDF direction does;
a model the service cannot read at all is a `MigrationError`.

## Query and documents

```ts
const elements = await model.query({ oslc: "sysml:name = \"Wheel\"" });
const rows = await model.runDocumentQuery("Observatory::PartsList", {
  subject: new ElementRef("Demo::sedan"),
});
const markdown = await model.renderDocument("Observatory::Overview");
```

`query` also takes a structured form (`payload`/`scope`/`select`/`where`, or a
wire `query`), never both a structured form and `oslc`. A binding value is an
`ElementRef`, an `ObjectRef`, a primitive or a quantity; `runDocumentQuery`
answers the typed `DocumentQueryResult` — `rows` of `DocumentValue`s, the schema
the query declared — and `renderDocument` answers Markdown, or HTML with
`{ form: "html" }`.

## Execution and exploration

```ts
const result = await model.executeAction("Demo::Drive", {
  inputs: { distance: 120 },
});
const state = await model.executeState("Demo::Ignition");
```

`executeAction` and `executeState` answer the run — `outputs`, `events`, the
`trace` of each step — and refuse `schedule: "explore"`, because an exploration
is not a run: `exploreAction`, `exploreState` and `exploreAnalysis` answer an
`Exploration` (`outcomes`, `runs`, `budgetsHit`) instead. `{ schedule: "explore" }`
is their default; any other schedule there is refused the same way. A
`performer` names the engine a run is handed to, where the service schedules.

## Verification, analysis and sweep

```ts
const verdict = await model.verifyConstraint("Demo::MassBudget", {
  subject: "Demo::sedan",
  question: "worst-case",
});
verdict.verdict.kind;                    // "holds" | "fails" | "undecided"
await model.satisfied();                 // every asserted satisfy holds

const validation = await model.validateInstance("Demo::sedan");
const computed = await model.calc("Demo::TotalMass", { arguments: [2] });
const analysis = await model.runAnalysis("Demo::TradeStudy", { subject: "Demo::sedan" });
const table = await model.runSweep("Demo::TradeStudy", {
  power: [{ kind: "int", value: 100n }, { kind: "int", value: 300n }, { kind: "int", value: 50n }],
}, { samples: 25, seed: 7 });
```

A verification raises `WrongKindError` for a symbol of another kind and keeps a
`fails` or `undecided` verdict as an answer rather than a failure.
`runAnalysis` answers the run's `outputs`, `verdicts`, `instances` and
`diagnostics` — a failed analysis still answers what it completed.
`runSweep` takes a `Record` of `parameter → [from, to]` or `[from, to, step]`
ranges and answers the `SweepTable` of rows, each carrying its own error when a
run fails.

## Engines

```ts
for (const engine of await connection.listEngines()) {
  engine.name; engine.authority;           // what it answers: verification, analysis, ...
}
```

`engine` selects one on the calls above (`verifyConstraint`, `validateInstance`,
`calc`, `runAnalysis`, `verifySatisfaction`), and `explainVerdict` /
`explainStanding` say how a verdict was reached — the engine and the strength of
its evidence (`observed`, `witnessed`, `bounded`, `proved`).

## Authoring

`model.edit()` starts a batch of source-preserving edits, applied atomically:

```ts
const editor = model.edit();
editor.addItemDef("Car", "Demo");
editor.rename("Demo::Wheel", "Tire");
const applied = await editor.apply();
applied.documents;      // every document the batch rewrote, by parse name
```

Every `add*` of the Python client is here — `addPackage` … `addRequirement`,
`addCalcDef`, `addParameter`, `addPerformAction`, `addStateAction`,
`addAssertConstraint`, `addTransition`, `addImport`, `addDocumentation`,
`addMetadata`, `addVerify`, `addObjective`, `addSatisfy`, plus `setValue`,
`rename`, `delete(target, { cascade })` and `move(target, owner)` — and
`Body` builds guarded then/else and nested if/while/for blocks for action
bodies. Edits are validated eagerly (a `RangeError` or `TypeError` at the
`add*` call, not at `apply()`) and capability-gated per operation and per
field. `connection.applyEdits(modelHash, operations)` is the low-level form,
taking wire `EditOperation` messages.

An edit the service refuses raises the typed error for the failure —
`EditTargetError`, `RenameReferencedError`, `DeleteReferencedError`,
`MemberNameTakenError`, `OwnerNotFoundError`, `OverlappingEditsError` and the
rest of the `EditError` family, each carrying `failure`, `diagnostics`,
`referringElements` and structured `referrers`.

## Service version and required capabilities

`connect()` negotiates what the service must be, beside what it must advertise:

```ts
await using connection = await connect({
  version: "0.9",
  requireCapabilities: [CAPABILITY_VERIFY, CAPABILITY_DOCUMENT_QUERY],
});
```

A running service that reports another version is a `StaleServiceError`
naming the required and reported versions — the remedy is to point at a service
of the required version, or accept what is running by omitting `version`
(`$OPENSYSML_VERSION` is the environment form). A service missing a required
capability is likewise refused at connect, rather than at the first gated call.

## Typed module generation

`opensysml-generate` writes a TypeScript module of typed views over a model's
definitions — `Vehicle`, `SportsCar` — so `model.instantiate` reads as
`vehicle.wheels`, with each feature's declared type, instead of
`tree.get("wheels")`:

```bash
npx opensysml-generate model.sysml -o src/vehicle_types.ts
npx opensysml-generate model.sysml --check -o src/vehicle_types.ts   # fails when stale
```

```ts
import { Vehicle } from "./vehicle_types.js";
const vehicle = Vehicle.fromInstance(await model.instantiate("Demo::Car"), tree.byId);
vehicle.wheels;                    // readonly Instance[], typed
vehicle.mass;                      // Quantity | undefined, as declared
```

A class `extends` its first base, declares the features of the rest as getters
so every inherited feature stays reachable, and `fromInstance` accepts
subtypes; a feature of another kind raises `TypeMismatchError` or
`InstanceTypeError`. The module is stamped with a hash of the source text, and
`--check` regenerates on drift. Generated code imports only from
`@openmbee/opensysml` and runs under the browser entry point too.

## What this client does not do

- `save` writes a single document's content; a multi-document `EditResult`
  (`applied`/`documents` rather than one `content`) is written by the caller,
  per `EditedDocument`.
- `connection.rpc` remains the escape hatch for anything not wrapped: it is the
  generated Connect client, and `SysMLService` is exported for a caller
  building its own.

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
**through its public API** — `load`/`loads`, `parseSources`, `eval`, `symbol`,
`instantiate`, `executeAction`, `convert`, `migrate`, `applyEdits`, `verifyConstraint`,
`query`, `runDocumentQuery`, `renderDocument`, `runAnalysis`, `runSweep`,
`listEngines` — not through the generated stubs. The only scenarios skipped are
the ones whose request the public API refuses eagerly rather than asking the
service, and the report has the same shape `tools/cmd/conformance` emits:

```bash
npm run conformance -- --allow-skips --report report.json
```

| protocol | ran | passed | failed | skipped |
| --- | --: | --: | --: | --: |
| `grpc` | 158 | 155 | 0 | 3 |
| `connect` | 158 | 155 | 0 | 3 |
| `connect-json` | 158 | 155 | 0 | 3 |
| **total** | **474** | **465** | **0** | **9** |

The 3 skips per protocol are the requests the public API cannot express because
it validates them before calling: a `ParseFile` naming no source (`load` and
`loads` always name one), a `ParseSources` naming no document and a
`ParseSources` naming two documents alike (`parseSources` refuses both). Every
skip carries its reason in the report. A refusal the client makes before asking,
in the service's own words and with the status the service would answer — a v1
model offered to `convert`, a v2 one to `migrate` — is reported as that status,
so those scenarios run rather than skip.

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
[docs/project/releasing.md](../../docs/project/releasing.md) under "Releasing
@openmbee/opensysml to npm". In short: the core `v*` tag's `release` workflow
publishes this package and its five per-platform packages at the version
`client/python/opensysml/_version.py` declares, carrying the release's own
`sysml-grpc` binaries. The granular npm token it needs already lives in the
`npm` context; granular tokens expire after at most 90 days, so rotation before
a release is a standing task.
