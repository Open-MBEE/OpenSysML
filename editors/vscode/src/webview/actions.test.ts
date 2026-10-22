import assert from "node:assert/strict";
import { test } from "node:test";

import type { EditPalette, RenderNode } from "../protocol";
import { nodeMenu } from "./actions";

const palette: EditPalette = {};
const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };
const declared: RenderNode = { id: "n1", kind: "part", name: "tank", type: "", detail: "", fqn: "Vehicle::Car::tank", declaredHere: true, notation: "part", origin };
const imported: RenderNode = { id: "n2", kind: "part", name: "wheel", type: "Wheel", detail: "", origin };
const unlocated: RenderNode = { id: "n3", kind: "part", name: "ghost", type: "", detail: "" };
// A node another workspace document declares: named for a layout, not the document's own.
const foreign: RenderNode = { ...declared, id: "n4", declaredHere: undefined, origin: { ...origin, uri: "file:///parts.sysml" } };

test("nodeMenu offers the full set on a node the document declares", () => {
  const items = nodeMenu(declared, palette);
  assert.deepEqual(items[0], { label: "tank", heading: true });
  assert.deepEqual(
    items.filter((item) => item.command).map((item) => [item.label, item.command]),
    [
      ["Go to declaration", { kind: "reveal", id: "n1" }],
      ["Rename…", { kind: "rename", id: "n1" }],
      ["Move to…", { kind: "move", id: "n1" }],
      ["Delete…", { kind: "delete", id: "n1" }],
    ],
  );
  assert.equal(items.filter((item) => item.separator).length, 1);
});

test("nodeMenu never offers to add to the model", () => {
  const labels = nodeMenu(declared, { owners: { subject: ["n1"] } }).map((item) => item.label);
  assert.ok(!labels.some((label) => label.startsWith("Add ") || label.endsWith("from here…")), labels.join(", "));
});

// A server that gives no notation cannot say what a moved node asks its owner to admit.
test("nodeMenu only reveals a node another document declares, whatever its name", () => {
  const commands = nodeMenu(foreign, palette).filter((item) => item.command).map((item) => item.command);
  assert.deepEqual(commands, [{ kind: "reveal", id: "n4" }]);
});

test("nodeMenu offers no move on a node without a notation", () => {
  const { notation: _, ...unnotated } = declared;
  const commands = nodeMenu(unnotated, palette).filter((item) => item.command).map((item) => item.command?.kind);
  assert.deepEqual(commands, ["reveal", "rename", "delete"]);
});

test("nodeMenu only reveals a node the document does not declare", () => {
  const commands = nodeMenu(imported, palette).filter((item) => item.command).map((item) => item.command);
  assert.deepEqual(commands, [{ kind: "reveal", id: "n2" }]);
});

test("nodeMenu is empty for a node with nothing to do", () => {
  assert.deepEqual(nodeMenu(unlocated, palette), []);
});

test("nodeMenu offers no edits without a palette", () => {
  const commands = nodeMenu(declared, undefined).filter((item) => item.command).map((item) => item.command);
  assert.deepEqual(commands, [{ kind: "reveal", id: "n1" }]);
});


test("nodeMenu offers related elements on a named node when the server answers them, hiding only what the panel included", () => {
  const plain = nodeMenu(imported, undefined, { related: true });
  assert.ok(!plain.some((item) => item.label === "Show related elements…"), "a node with no fqn has nothing to look up by");
  const own = nodeMenu(declared, undefined, { related: true });
  assert.deepEqual(own.slice(1), [
    { label: "Go to declaration", command: { kind: "reveal", id: "n1" } },
    { label: "", separator: true },
    { label: "Show related elements…", command: { kind: "related", id: "n1" } },
    { label: "Hide from diagram", command: { kind: "hide", id: "n1" }, disabled: true },
  ]);
  const included = nodeMenu({ ...declared, included: true }, undefined, { related: true });
  assert.deepEqual(included.at(-1), { label: "Hide from diagram", command: { kind: "hide", id: "n1" }, disabled: false });
  assert.ok(!nodeMenu(declared, undefined).some((item) => item.label === "Hide from diagram"), "a server without the request offers neither entry");
});
