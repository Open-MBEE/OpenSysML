# Node and TypeScript

`@openmbee/opensysml` is the Connect client for Node.js and browsers. It
offers the service API with immutable, typed values and no native Node addon.
Choose it for a TypeScript application, Node service or browser-based model
tool.

## Install

The package and per-platform service packages are published to npm:

```bash
npm install @openmbee/opensysml
```

## First model

`loads` accepts inline SysML. Check parse diagnostics before relying on a
result; `eval` returns a discriminated value union.

```ts
import { loads } from "@openmbee/opensysml";

const source = `package Demo {
  part def Vehicle {
    attribute mass default = 1500.0;
  }
  part sedan : Vehicle {
    attribute :>> mass = 1800.0;
  }
}`;

const model = await loads(source);
try {
  if (model.hasErrors) {
    for (const diagnostic of model.diagnostics) console.error(diagnostic);
    throw new Error("The model has errors.");
  }

  const mass = await model.eval("mass", { subject: "Demo::sedan" });
  if (mass.kind !== "real") throw new Error(`Expected a real value, got ${mass.kind}`);
  console.log(mass.value.toFixed(1));
} finally {
  await model.close();
}
```

```text
1800.0
```

`Model.close()` closes a connection owned by `loads`; use an explicit
`Connection` when multiple models should share one service session. The same
model API covers symbol lookup, instances, verification, analysis, editing,
conversion and native document queries.

## Service source

In Node, the resolver can use the platform-specific npm service package, the
shared cache, or a verified release download. A browser client connects to a
service address instead of starting a local child. See the
[service-binary reference](../reference/clients.md#providing-the-service-binary)
for resolution order and configuration.

Parse problems appear in `model.diagnostics`; a failed request raises a typed
error rather than returning a false verdict. The README documents transport,
browser-origin and service-resolution errors in detail.

## Next steps

- [Node API reference](../reference/node-api.md)
- [Client README and installation details](https://github.com/Open-MBEE/OpenSysML/blob/main/client/node/README.md)
- [Shared SysML model examples](https://github.com/Open-MBEE/OpenSysML/tree/main/examples)
