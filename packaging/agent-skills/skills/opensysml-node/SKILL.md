---
name: opensysml-node
description: Parse, query, instantiate, execute and verify SysML v2 models from Node.js, TypeScript or a browser with the `@openmbee/opensysml` client (npm install @openmbee/opensysml), which drives the OpenSysML sysml-grpc service over Connect — requiring a clean model, switching on discriminated-union values, bigint integers, running actions and calcs, checking requirements, handling its error classes. Use when writing JavaScript or TypeScript that reads or runs SysML v2 models.
---

# Using OpenSysML from Node, TypeScript or a browser

`@openmbee/opensysml` talks to the `sysml-grpc` service. Under Node it starts a private one for you;
in a browser it only reaches a service you name. Reference: `docs/guide/09-clients.md` (From Node
or a browser), `client/node/README.md` and `docs/reference/node-api.md` in the OpenSysML repository.

## Install and get the service binary

```bash
npm install @openmbee/opensysml        # ESM; types included
```

The client looks for `sysml-grpc` at `$OPENSYSML_BINARY`, then in the optional per-platform npm
package npm installs alongside, then `~/.opensysml/bin/sysml-grpc` (shared with the other clients),
then on `PATH`. Nothing is downloaded unless a release is asked for: set
`$OPENSYSML_GRPC_VERSION` (a tag, or `latest`) and the client downloads that release, verified,
into the cache. Without a binary or a requested release, `connect()` throws `BinaryNotFoundError`. `$OPENSYSML_SERVICE=host:port`, or `connect({ address })`, uses a running service instead;
closing that connection never stops it.

## Load, and require a clean model

```ts
import { connect } from "@openmbee/opensysml";

await using connection = await connect();          // one private service per thread, shared cache
const model = await connection.load("model.sysml");
if (!model.ok) {
  for (const d of model.errors) console.error(d.code, d.message);
}
model.raiseForErrors();                             // throws ParseError carrying .model
```

`load(path)` / `loads(source)` are one-shot forms that open and close their own connection with the
model; prefer one `connect()` for several models. A model with syntax errors still loads; branch on
`d.code` (`"syntax"`, `"unresolved"`, ...), never on message text. `{ strictConformance: true }`
makes OpenSysML-only notation an error. For several files use
`connection.parseSources([...])`; `load` reads one document, so imports across files do not
resolve. `Connection` and `Model` are async-disposable: use `await using` or call `close()`.

## Evaluate, instantiate, run

```ts
await model.eval("Demo::sedan::mass");                       // { kind: "real", value: 1200 }
await model.eval("mass", { subject: "Demo::sedan" });        // same, evaluated on an object

const tree = await model.instantiate("Demo::sedan");
tree.get("mass");            // { kind: "single", value: {...} } | { kind: "many", values } | { kind: "error", ... }

const run = await model.executeAction("Demo::addFive", { inputs: { x: 10n } });
run.outputs.get("result");                                   // { kind: "int", value: 15n }

const calc = await model.calc("Demo::Margin", { arguments: [1200.0, 2000.0] });
calc.value;                                                  // { kind: "real", value: 800 }
```

Every value is a discriminated union: `switch (value.kind)` over `"int"`, `"real"`, `"boolean"`,
`"string"`, `"quantity"` (`magnitude`, `unit`), `"enum"`, `"instance"`, `"sequence"`, ... Integers
are `bigint`. Three "no value" kinds are not errors and must not be coerced to 0 or `false`:
`"undetermined"` (the model leaves it open), `"unset"` (an object holds nothing for the feature)
and `"null"`; `"absent"` means the answer carried no field at all.

## Members, element properties and object paths

```ts
const m = await model.symbol("Demo::m");
m.members;                        // every member, own then inherited: { id, name, kind, inherited, inheritedFrom, library }
m.inheritedMembers();             // the inherited ones; inheritedFrom names the declaring type
m.withheldLibraryMembers;         // standard-library members counted, not listed, until:
await m.withLibraryMembers();     // ... fetched again with them (or symbolById(id, { includeLibraryMembers: true }))

const read = await model.elementProperties("Demo::Pair", "inheritedFeature", "isAbstract");
read.get("inheritedFeature")?.elements;                      // ElementIdentity[]; their id is accepted back
read.get("connectorEnd")?.unsupported;                       // why a property has no values

const engine = tree.byId(2n)!;
engine.reachedBy;                 // { instanceId, featureName, featureSymbolId, index, path }; undefined for the root
engine.path; tree.owner(engine);  // "engine", the holding object
await model.instancePaths(engine); // InstancePaths: every root-relative path, aliases included
```

`members` is empty from a service without `symbol_members` (`childIds` still names the owned
ones); `includeLibraryMembers`, `elementProperties` and `instancePaths` throw
`MissingCapabilityError` before asking when the service lacks `symbol_members`,
`element_properties` or `instance_paths`.

`annotatingMetadataIds` names the metadata usages annotating a symbol: its own `#M` prefixes and
body `@M` usages, then the ones naming it in an `about` clause elsewhere, which `childIds` and
`members` leave out. `annotatingMetadata()` fetches them as symbols, each with its `type` and
`attributes`, and needs the `symbol_annotating_metadata` capability.

## Verify

```ts
import { explainVerdict } from "@openmbee/opensysml";

const v = await model.verifyRequirement("Demo::Vehicle::lightEnough", { subject: "Demo::truck" });
switch (v.verdict.kind) {
  case "holds": break;
  case "fails": console.log(v.verdict.condition); break;    // "mass < 2000.0"
  case "undecided": console.log(v.verdict.error); break;    // could not evaluate
}
explainVerdict(v.verdict);                                   // one readable line
await model.verifySatisfaction();                            // every `assert satisfy R by x;`
await model.satisfied();                                     // boolean
```

A `"fails"` verdict is an answer, not an exception. `verifyConstraint` takes the same `subject`.

## Errors

Everything thrown descends from `OpenSysMLError`. `ServiceError` and its subclasses
(`ModelFileNotFoundError`, `ModelNotFoundError`, `InvalidRequestError`, `ServiceUnavailableError`,
`ServiceTimeoutError`, `UnsupportedOperationError`, `ServiceStartError`) mean the call was refused
or the service was not reached. The rest mean the call was answered with a failure: `ParseError`
(`raiseForErrors`), `SymbolNotFoundError` (`model.symbol`), `EvaluationError` (an expression that
will not evaluate), `ExecutionError` and `WrongKindError` (execute/verify), `AnalysisRunError`
(with partial results), `EditError` subclasses. `MissingCapabilityError` means the service is
older than the call needs.

## Pitfalls

- A JS `number` is sent as a Real. Pass Integers as `bigint` (`10n`): `{ x: 10 }` into an
  `Integer` parameter is refused with a type mismatch.
- Use qualified names (`Package::Def::feature`) wherever a symbol is named.
- Browser: import from `@openmbee/opensysml/browser` and pass an explicit `address`; the service
  must allow the page's exact origin (`sysml-grpc -cors-allowed-origins https://app.example.com`)
  and use TLS for an HTTPS page.
- `connection.rpc` is the generated Connect client, for an RPC the typed layer does not wrap.
- Write models with the `sysml-v2-modeling` skill; `sysml -validate` reports the same diagnostics as
  `model.diagnostics`.
