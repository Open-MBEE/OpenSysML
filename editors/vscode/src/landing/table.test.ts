import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import { snapshotModel, type LandingModel } from "./model";
import { MASS_ROLLUP_SCENE } from "./scene";
import { PENDING, cellOf, fillCell, rollupTable, tableElement } from "./table";

// The table is drawn with the page's document, as it is in the hero.
const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

const LISTED = ["mass", "propellantMass"];

function massModel(): LandingModel {
  return snapshotModel(MASS_ROLLUP_SCENE);
}

// withPart replaces one part of the model, by path, keeping the others.
function withPart(model: LandingModel, path: string, change: Partial<LandingModel["root"]>): LandingModel {
  const parts = new Map(model.parts);
  parts.set(path, { ...model.parts.get(path)!, ...change });
  return { ...model, parts };
}

test("rollupTable lists the root first, then every part in composition order, indented as it nests", () => {
  const model = massModel();
  const table = rollupTable(model, LISTED, "totalMass");
  assert.deepEqual(
    table.rows.map((row) => `${row.depth}:${row.part.feature}`),
    [
      "0:saturnV",
      "1:stage1",
      ...[1, 2, 3, 4, 5].map((i) => `2:engine${i}`),
      "1:stage2",
      ...[1, 2, 3, 4, 5].map((i) => `2:engine${i}`),
      "1:stage3",
      "2:engine1",
    ],
  );
  assert.equal(table.rows[0].id, model.root.id);
  assert.equal(table.rows[0].part.type, "LaunchVehicle");
  assert.equal(table.rows[1].part.type, "'S-IC'");
  assert.equal(table.rows[2].id, model.parts.get("stage1.engine1")!.id);
});

test("rollupTable's columns are the listed quantities and then the roll-up, listed once", () => {
  const model = massModel();
  const table = rollupTable(model, LISTED, "totalMass");
  assert.deepEqual(table.columns, ["mass", "propellantMass", "totalMass"]);
  assert.deepEqual(rollupTable(model, ["totalMass", "mass"], "totalMass").columns, ["mass", "totalMass"]);
  assert.ok(Object.keys(model.parts.get("stage1.engine1")!.quantities).includes("thrust"), "an engine's thrust is held but not listed");
  const cells = (feature: string) => table.rows.find((row) => row.part.feature === feature)!.cells.map((cell) => cell.text);
  assert.deepEqual(cells("saturnV"), ["0 kg", "", "2,941,728 kg"]);
  assert.deepEqual(cells("stage1"), ["130,000 kg", "2,160,000 kg", "2,332,000 kg"]);
  assert.deepEqual(table.rows[2].cells.map((cell) => cell.text), ["8,400 kg", "", "8,400 kg"]);
  assert.ok(table.rows.every((row) => row.cells.every((cell) => !cell.undetermined)));
});

test("a part without a listed quantity has an empty cell, and an unvalued roll-up its reason", () => {
  const model = withPart(massModel(), "stage3", {
    quantities: { height: { magnitude: "0.91", unit: "m" } },
    unvalued: { totalMass: "mass has no value in the model" },
  });
  const table = rollupTable(model, LISTED, "totalMass");
  const stage3 = table.rows.find((row) => row.part.feature === "stage3")!;
  assert.deepEqual(stage3.cells, [
    { text: "", undetermined: false },
    { text: "", undetermined: false },
    { text: "undetermined: mass has no value in the model", undetermined: true },
  ]);
  assert.deepEqual(cellOf(model.root, "nonesuch"), { text: "", undetermined: false });
});

test("tableElement heads the columns, indents the rows and leaves the roll-up pending until filled", () => {
  const model = massModel();
  const table = rollupTable(model, LISTED, "totalMass");
  const pending = tableElement(table, "the Saturn V", false);
  assert.equal(pending.getAttribute("aria-label"), "the Saturn V");
  assert.deepEqual(
    [...pending.tHead!.rows[0].cells].map((cell) => cell.textContent),
    ["Part", "Type", "mass", "propellantMass", "totalMass"],
  );
  const rows = [...pending.tBodies[0].rows];
  assert.equal(rows.length, table.rows.length);
  assert.ok(rows[0].classList.contains("is-root"));
  assert.equal(rows[0].dataset.osmlId, model.root.id);
  assert.equal(rows[0].cells[0].textContent, "partsaturnV");
  assert.equal(rows[0].cells[0].style.getPropertyValue("--osml-depth"), "0");
  assert.equal(rows[2].cells[0].style.getPropertyValue("--osml-depth"), "2");
  assert.equal(rows[1].cells[1].textContent, "'S-IC'");
  assert.equal(rows[1].cells[3].textContent, "2,160,000 kg");
  const totals = rows.map((row) => row.cells[4]);
  assert.ok(totals.every((cell) => cell.textContent === PENDING && cell.classList.contains("is-pending")));
  assert.equal(totals[1].dataset.osmlCell, model.parts.get("stage1")!.id);

  const filled = tableElement(table, "the Saturn V", true);
  const filledTotals = [...filled.tBodies[0].rows].map((row) => row.cells[4].textContent);
  assert.equal(filledTotals[0], "2,941,728 kg");
  assert.equal(filledTotals[1], "2,332,000 kg");
  assert.ok(!filled.querySelector(".is-pending"));
});

test("fillCell replaces the placeholder with the value, or accents the engine's reason", () => {
  const model = massModel();
  const element = tableElement(rollupTable(model, LISTED, "totalMass"), "", false);
  const cell = element.querySelector<HTMLElement>(`td[data-osml-cell="${model.parts.get("stage2")!.id}"]`)!;
  fillCell(cell, { text: "488,940 kg", undetermined: false });
  assert.equal(cell.textContent, "488,940 kg");
  assert.ok(!cell.classList.contains("is-pending"));
  assert.equal(cell.getAttribute("aria-label"), null);
  fillCell(cell, { text: "undetermined: mass has no value in the model", undetermined: true });
  assert.ok(cell.classList.contains("is-undetermined"));
  fillCell(cell, { text: "1 kg", undetermined: false });
  assert.ok(!cell.classList.contains("is-undetermined"));
});
