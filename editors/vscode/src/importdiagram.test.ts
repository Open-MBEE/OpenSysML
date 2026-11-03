import assert from "node:assert/strict";
import { test } from "node:test";

import { importReport } from "./importdiagram";
import type { ImportDiagramResult } from "./protocol";

const move = { kind: "move", target: "Vehicle::Car", cell: "c1" };
const resize = { kind: "resize", target: "Vehicle::Car::engine", cell: "c2" };

function result(fields: Partial<ImportDiagramResult>): ImportDiagramResult {
  return { view: "Vehicle::Overview", dryRun: false, edits: [], version: 3, ...fields };
}

test("a refused import is an error naming the file and the server's reason, and applies nothing", () => {
  const refused = result({ edits: [move], refused: [{ operation: -1, failure: "stale", message: "the view was edited since the page was exported" }] });
  assert.deepEqual(importReport("car.drawio", refused), {
    severity: "error",
    message: "car.drawio: not imported: the view was edited since the page was exported",
    applies: false,
  });
});

test("the server refuses an import as a whole: the first refusal is the one reported", () => {
  const refused = result({
    refused: [
      { operation: -1, failure: "unknown-target", message: "Vehicle::Boat is not declared" },
      { operation: -1, failure: "stale", message: "never produced alongside another" },
    ],
  });
  assert.equal(importReport("car.drawio", refused).message, "car.drawio: not imported: Vehicle::Boat is not declared");
});

test("a page matching the rendering has nothing to import", () => {
  assert.deepEqual(importReport("car.drawio", result({ edit: {} })), {
    severity: "information",
    message: "car.drawio matches the rendering of Vehicle::Overview; nothing to import",
    applies: false,
  });
});

test("edits without a workspace edit to apply are nothing to import", () => {
  const unapplied = importReport("car.drawio", result({ edits: [move] }));
  assert.equal(unapplied.applies, false);
  assert.equal(unapplied.severity, "information");
});

test("what was left alone is listed after nothing to import, in the rendering's order", () => {
  const left = result({ edit: {}, unmatched: ["Vehicle::Boat"], deleted: ["Vehicle::Car::tank"], notices: ["the page is wider than the view"], unknown: ["c7", "c8"] });
  assert.equal(
    importReport("car.drawio", left).message,
    "car.drawio matches the rendering of Vehicle::Overview; nothing to import (Vehicle::Boat is not in the rendering; Vehicle::Car::tank was deleted in the file; the element stays; the page is wider than the view; 2 cells name no element)",
  );
});

test("an imported edit is counted in the singular and applied first", () => {
  assert.deepEqual(importReport("car.drawio", result({ edit: {}, edits: [move] })), {
    severity: "information",
    message: "1 layout edit imported from car.drawio into Vehicle::Overview",
    applies: true,
  });
});

test("imported edits that left something alone are a warning listing it", () => {
  assert.deepEqual(importReport("car.drawio", result({ edit: {}, edits: [move, resize], unknown: ["c7"] })), {
    severity: "warning",
    message: "2 layout edits imported from car.drawio into Vehicle::Overview; 1 cells name no element",
    applies: true,
  });
});
