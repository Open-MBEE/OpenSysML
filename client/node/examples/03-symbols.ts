// What the client can tell you about a model's declarations: walking every
// symbol, looking one up by qualified name or by short name, and reading the
// facts the service reports about it — type, multiplicity, specializations,
// attributes, metadata.
//
//   npm run example -- 03-symbols

import assert from "node:assert/strict";
import { connect, formatValue } from "../src/node/index.js";
import { ROVER, section, show } from "./model.js";

async function main(): Promise<void> {
  const connection = await connect();
  try {
    const model = await connection.loads(ROVER);

    section("Every symbol the model declares");
    const names: string[] = [];
    for await (const symbol of model.walk()) {
      names.push(`${symbol.id || "<root>"} (${symbol.kind})`);
    }
    for (const name of names) {
      console.log(`  ${name}`);
    }
    assert.ok(names.some((name) => name.startsWith("Rover::Rover::wheels")));

    section("Looked up three ways");
    show("by qualified name", (await model.symbol("Rover::Wheel")).id);
    show("by short name", (await model.find("Wheel"))?.id);
    show("by id", (await model.symbolById("Rover::Wheel")).id);
    show("a name it has not got", await model.find("Sprocket"));

    section("A part's own facts");
    const wheels = await model.symbol("Rover::Rover::wheels");
    show("kind", wheels.kind);
    show("type", `${wheels.type?.declared ?? "?"} -> ${wheels.type?.resolvedId ?? "?"}`);
    show("multiplicity", `${wheels.multiplicity?.lower ?? "?"}..${wheels.multiplicity?.upper ?? "?"}`);
    show("metadata", JSON.stringify(wheels.metadata));
    assert.equal(wheels.multiplicity?.upper, "6");

    section("An attribute's facts");
    const radius = await model.symbol("Rover::Wheel::radius");
    show("declared type", radius.type?.declared);
    show("resolves to", `${radius.type?.resolvedId ?? "?"} (${radius.type?.resolvedKind ?? "?"})`);
    show("a quantity", `${String(radius.type?.quantity)} in ${radius.type?.unit ?? ""}`);
    assert.equal(radius.type?.quantity, true);
    // The values are reported on the definition that owns the attributes.
    const wheel = await model.symbol("Rover::Wheel");
    for (const attribute of wheel.attributes) {
      const unit = attribute.unit === "" ? "" : ` [${attribute.unit}]`;
      show(`attribute ${attribute.name}`, `${attribute.type}${unit} = ${formatValue(attribute.value)}`);
    }
    show("library attributes withheld", wheel.withheldLibraryAttributes);

    section("What a specialization inherits");
    const heavy = await model.symbol("Rover::HeavyRover");
    show(
      "specializes",
      heavy.specializations.map((one) => `${one.kind} ${one.declared} -> ${one.targetId}`).join(", "),
    );
    show("attributes", heavy.attributes.map((attribute) => attribute.name).join(", "));
    show("inherited mass", formatValue(await model.eval("mass", { context: "Rover::HeavyRover" })));
    assert.ok(heavy.specializations.some((one) => one.targetId === "Rover::Rover"));

    section("The same model, adopted from the service's cache by hash");
    // A second handle on a model already parsed: no source, just its hash.
    const adopted = connection.model(model.hash);
    show("short name", (await adopted.symbol("Rover")).id);
    show("qualified name", (await adopted.symbol("Rover::Wheel")).id);
    show("expression", formatValue(await adopted.eval("wheelCount", { context: "Rover::Rover" })));
  } finally {
    await connection.close();
  }
}

await main();
