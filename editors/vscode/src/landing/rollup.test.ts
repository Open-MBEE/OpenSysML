import assert from "node:assert/strict";
import { test } from "node:test";

import type { RenderResult } from "../protocol";
import { landingModel, rollupLevels, snapshotModel, valueText, type EngineInstance, type LandingModel } from "./model";
import { quantityText } from "./present";
import { MASS_ROLLUP_SCENE } from "./scene";

const STAGES = ["stage1", "stage2", "stage3"];
const ENGINES = [
  ...[1, 2, 3, 4, 5].map((i) => `stage1.engine${i}`),
  ...[1, 2, 3, 4, 5].map((i) => `stage2.engine${i}`),
  "stage3.engine1",
];

function massModel(): LandingModel {
  return snapshotModel(MASS_ROLLUP_SCENE);
}

test("the mass snapshot draws one box per instance: the stages and their engines, nothing else", () => {
  const model = massModel();
  assert.deepEqual([...model.parts.keys()].sort(), [...STAGES, ...ENGINES].sort());
  assert.equal(model.render.nodes.length, model.parts.size, "every drawn node is a part");
  assert.deepEqual(model.render.edges, [], "the bindings between the folded collections leave with them");
  for (const stage of STAGES) {
    const part = model.parts.get(stage)!;
    assert.equal(part.owner, undefined, `${stage} is lifted to the top level`);
    assert.equal(model.render.nodes.find(({ id }) => id === part.id)?.parent, undefined);
  }
  const engine = model.parts.get("stage1.engine3")!;
  assert.equal(engine.owner, "stage1");
  assert.equal(engine.type, "'F-1'");
  assert.equal(engine.symbol, "MassRollup::F-1");
  assert.equal(model.render.nodes.find(({ id }) => id === engine.id)?.parent, model.parts.get("stage1")!.id);
});

test("the mass snapshot carries the engine's quantities, none typed in", () => {
  const model = massModel();
  const totals = Object.fromEntries(
    [...STAGES, "stage1.engine1", "stage3.engine1"].map((path) => [path, model.parts.get(path)!.quantities.totalMass]),
  );
  assert.deepEqual(totals, {
    stage1: { magnitude: "2332000", unit: "kg" },
    stage2: { magnitude: "488940", unit: "kg" },
    stage3: { magnitude: "120788", unit: "kg" },
    "stage1.engine1": { magnitude: "8400", unit: "kg" },
    "stage3.engine1": { magnitude: "1788", unit: "kg" },
  });
  assert.deepEqual(model.parts.get("stage1")!.quantities.propellantMass, { magnitude: "2160000", unit: "kg" });
  assert.deepEqual(model.parts.get("stage1")!.unvalued, {});
  const sum = (paths: string[]): bigint =>
    paths.reduce((total, path) => total + BigInt(model.parts.get(path)!.quantities.totalMass.magnitude), 0n);
  assert.equal(sum(STAGES), 2941728n, "the stages roll up to the vehicle's total");
});

test("rollupLevels orders the boxes from the engines up to the stages", () => {
  const model = massModel();
  const levels = rollupLevels(model).map((ids) =>
    ids.map((id) => [...model.parts.values()].find((part) => part.id === id)!.path).sort(),
  );
  assert.deepEqual(levels, [ENGINES.slice().sort(), STAGES]);
});

test("quantityText groups the engine's digits and keeps a non-integer magnitude as sent", () => {
  assert.equal(quantityText({ magnitude: "2941728", unit: "kg" }), "2,941,728 kg");
  assert.equal(quantityText({ magnitude: "-12", unit: "m" }), "-12 m");
  assert.equal(quantityText({ magnitude: "0.91", unit: "m" }), "0.91 m");
  assert.equal(quantityText({ magnitude: "1/3", unit: "s" }), "1/3 s");
});

test("valueText prints a quantity, an undetermined reason and the other kinds plainly", () => {
  assert.equal(valueText({ kind: "quantity", magnitude: { kind: "int", value: 2941728n }, unit: "kg" }), "2941728 kg");
  assert.equal(valueText({ kind: "quantity", magnitude: { kind: "real", value: 0.91 }, unit: "m" }), "0.91 m");
  assert.equal(
    valueText({ kind: "undetermined", reason: "mass has no value in the model", countLower: "1", countUpper: "1" }),
    "undetermined (mass has no value in the model)",
  );
  assert.equal(valueText({ kind: "undetermined", reason: "", countLower: "", countUpper: "" }), "undetermined");
  assert.equal(valueText({ kind: "int", value: 3n }), "3");
  assert.equal(valueText({ kind: "unset" }), "unset");
});

// A composition whose collection names no member singly: `engines : Engine[2]` with two
// objects listed, a sibling `engine0` held singly, and `spares` with nothing in it.
function collectionFixture(): { render: RenderResult; instances: EngineInstance[] } {
  const render = {
    view: "", kind: "interconnection", stated: false, shown: [], notices: [],
    nodes: [
      { id: "n0", kind: "part", name: "Fleet::fleet", type: "Fleet", detail: "" },
      { id: "n1", kind: "part", name: "engine0", type: "Engine", detail: "", parent: "n0" },
      { id: "n2", kind: "attribute", name: "mass", type: "", detail: "", parent: "n1" },
      { id: "n3", kind: "part", name: "engines", type: "Engine", detail: "", parent: "n0" },
      { id: "n4", kind: "attribute", name: "mass", type: "", detail: "", parent: "n3" },
      { id: "n5", kind: "part", name: "spares", type: "Engine", detail: "", parent: "n0" },
      { id: "n6", kind: "part", name: "nozzle", type: "Nozzle", detail: "", parent: "n3" },
    ],
    edges: [{ from: "n3", to: "n1", label: "", kind: "binding" }],
  } as unknown as RenderResult;
  const kg = (magnitude: string) => ({ value: { quantity: { magnitude, unit: "kg" } } });
  const instances: EngineInstance[] = [
    {
      id: "1", typeSymbolId: "Fleet::fleet",
      featureValues: {
        engine0: { value: { instanceId: "2" } },
        engines: { values: [{ instanceId: "2" }, { instanceId: "3" }, { instanceId: "4" }] },
      },
    },
    { id: "2", typeSymbolId: "Fleet::Engine", featureValues: { mass: kg("10") } },
    { id: "3", typeSymbolId: "Fleet::Engine", featureValues: { mass: kg("20"), nozzle: { value: { instanceId: "5" } } } },
    { id: "4", typeSymbolId: "Fleet::Engine", featureValues: { mass: kg("30"), nozzle: { value: { instanceId: "6" } }, throat: { error: "no value for feature throat" } } },
    { id: "5", typeSymbolId: "Fleet::Nozzle", featureValues: { mass: kg("1") } },
    { id: "6", typeSymbolId: "Fleet::Nozzle", featureValues: { mass: kg("2") } },
  ];
  return { render, instances };
}

test("landingModel draws a listed object once, under the feature that lists it, and nothing for an empty one", () => {
  const { render, instances } = collectionFixture();
  const model = landingModel({ rootSymbol: "Fleet::fleet", view: "" }, "h", render, instances);
  assert.deepEqual(
    [...model.parts.keys()].sort(),
    ["engine0", "engines[1]", "engines[1].nozzle", "engines[2]", "engines[2].nozzle"],
  );
  assert.equal(model.parts.get("engine0")!.quantities.mass.magnitude, "10");
  assert.equal(model.parts.get("engines[1]")!.quantities.mass.magnitude, "20", "the object engine0 holds is not listed again");
  assert.equal(model.parts.get("engines[2]")!.quantities.mass.magnitude, "30");
  assert.deepEqual(model.parts.get("engines[2]")!.unvalued, { throat: "no value for feature throat" });
  const nozzles = [model.parts.get("engines[1].nozzle")!, model.parts.get("engines[2].nozzle")!];
  assert.deepEqual(nozzles.map((nozzle) => nozzle.owner), ["engines[1]", "engines[2]"]);
  assert.deepEqual(nozzles.map((nozzle) => nozzle.quantities.mass.magnitude), ["1", "2"]);
  assert.notEqual(nozzles[0].id, nozzles[1].id, "each copy's nozzle is its own box");
  assert.equal(new Set(model.render.nodes.map(({ id }) => id)).size, model.render.nodes.length, "node ids are unique");
  for (const [index, nozzle] of nozzles.entries()) {
    assert.equal(model.render.nodes.find(({ id }) => id === nozzle.id)?.parent, model.parts.get(`engines[${index + 1}]`)!.id);
  }
  assert.ok(model.render.nodes.every((node) => node.kind === "part"), "attributes are not drawn");
  assert.deepEqual(model.render.edges, [], "a wire to the folded collection goes with it");
});
