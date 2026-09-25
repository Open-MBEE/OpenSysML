// Instantiating a part and reading the object that comes back: single features,
// multiplicities, nested objects reached by id, features left unset, and
// evaluation against an instantiated subject.
//
//   npm run example -- 04-instances

import assert from "node:assert/strict";
import { connect, formatValue, type FeatureValue, type InstanceTree } from "../src/node/index.js";
import { ROVER, section, show } from "./model.js";

/** Prints one feature of an object, whichever of the three shapes it has. */
function showFeature(tree: InstanceTree, name: string, feature: FeatureValue | undefined): void {
  if (feature === undefined) {
    show(name, "no such feature");
    return;
  }
  if (feature.kind === "error") {
    show(name, `error: ${feature.error}`);
    return;
  }
  if (feature.kind === "many") {
    const values = feature.values.map((value) =>
      value.kind === "instance" ? `${tree.byId(value.id)?.typeId ?? "?"}#${value.id}` : formatValue(value),
    );
    show(name, `${feature.values.length} values: ${values.join(", ")}`);
    return;
  }
  const value =
    feature.value.kind === "instance"
      ? `${tree.byId(feature.value.id)?.typeId ?? "?"}#${feature.value.id}`
      : formatValue(feature.value);
  show(name, `${value}${feature.materialized ? " (materialized)" : ""}`);
}

async function main(): Promise<void> {
  const connection = await connect();
  try {
    const model = await connection.loads(ROVER);

    section("A rover, instantiated");
    const tree = await model.instantiate("Rover::Rover");
    show("root type", tree.root.typeId);
    show("objects in the tree", tree.all.length);
    show("features", [...tree.root.features.keys()].join(", "));
    show("diagnostics", tree.diagnostics.length);

    section("Each feature of the root object");
    for (const name of tree.root.features.keys()) {
      showFeature(tree, name, tree.get(name));
    }

    section("Down into a nested object");
    const battery = tree.get("battery");
    assert.ok(battery?.kind === "single" && battery.value.kind === "instance");
    const object = tree.byId(battery.value.id);
    assert.ok(object !== undefined);
    show("type", object.typeId);
    for (const name of object.features.keys()) {
      showFeature(tree, name, object.get(name));
    }

    section("A feature declared and never given a value");
    const camera = await model.instantiate("Rover::Camera");
    showFeature(camera, "megapixels", camera.get("megapixels"));
    showFeature(camera, "calibrated", camera.get("calibrated"));
    assert.equal(camera.get("calibrated")?.kind, "single");
    show("a feature it has not got", camera.get("altitude"));

    section("An empty multiplicity, and one bounded above");
    const cameras = tree.get("cameras");
    assert.ok(cameras !== undefined);
    show("cameras kind", cameras.kind);
    showFeature(tree, "cameras", cameras);

    section("Evaluating against an instantiated subject");
    // Without a subject the declared default is read; with one, the object's value.
    show("declared", formatValue(await model.eval("wheelCount", { context: "Rover::Rover" })));
    show(
      "on the object",
      formatValue(await model.eval("wheelCount", { subject: "Rover::Rover" })),
    );
    show(
      "derived from the object",
      formatValue(await model.eval("wheelCount * 2 > 10", { subject: "Rover::Rover" })),
    );

    section("A specialization, instantiated");
    const heavy = await model.instantiate("Rover::HeavyRover");
    show("features", [...heavy.root.features.keys()].join(", "));
    showFeature(heavy, "ballast", heavy.get("ballast"));
    showFeature(heavy, "mass", heavy.get("mass"));
  } finally {
    await connection.close();
  }
}

await main();
