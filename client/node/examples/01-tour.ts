// A tour of the whole client in one script: connect, ask what the service can
// do, parse a model, read a symbol, evaluate an expression, instantiate a part.
//
//   npm run example -- 01-tour

import assert from "node:assert/strict";
import { CAPABILITY_TYPE_FACTS, connect, formatValue } from "../src/node/index.js";
import { ROVER, section, show } from "./model.js";

async function main(): Promise<void> {
  section("The service this client talks to");
  // No address: the client starts a private child of this process and stops it
  // when the last connection closes.
  const connection = await connect();
  try {
    show("origin", connection.info.origin);
    show("version", connection.info.version);
    show("capabilities", [...connection.info.capabilities].sort((a, b) => a.localeCompare(b)).join(" "));
    assert.ok(connection.info.has(CAPABILITY_TYPE_FACTS));

    section("A parsed model");
    const model = await connection.loads(ROVER);
    show("hash", model.hash);
    show("errors", model.hasErrors);
    show("root children", model.root.childIds.join(", "));
    assert.ok(!model.hasErrors);

    section("One symbol of it");
    const rover = await model.symbol("Rover::Rover");
    show("kind", rover.kind);
    show("attributes", rover.attributes.map((attribute) => attribute.name).join(", "));
    show("mass", formatValue(rover.attribute("mass")?.value ?? { kind: "absent" }));
    show("children", (await rover.children()).map((child) => child.name).join(", "));

    section("An expression, evaluated in the model");
    show("6 * 4", formatValue(await model.eval("6 * 4")));
    show("mass, in the rover's scope", formatValue(await model.eval("mass", { context: "Rover::Rover" })));

    section("The part, instantiated");
    const tree = await model.instantiate("Rover::Rover");
    show("objects", tree.all.length);
    const wheels = tree.get("wheels");
    assert.ok(wheels?.kind === "many");
    show("wheels", wheels.values.length);
    const wheel = wheels.values[0];
    assert.equal(wheel.kind, "instance");
    show("first wheel's type", tree.byId(wheel.id)?.typeId);
  } finally {
    // The connection owns the child service; closing it stops the process.
    await connection.close();
  }
}

await main();
