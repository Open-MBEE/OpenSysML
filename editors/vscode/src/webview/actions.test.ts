import assert from "node:assert/strict";
import { test } from "node:test";

import type { EditPalette, RenderNode } from "../protocol";
import { nodeMenu, paletteItems } from "./actions";

const palette: EditPalette = { members: ["part", "port", "fork"], connections: ["connection", "flow"], typed: ["part", "port"] };
const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };
const declared: RenderNode = { id: "n1", kind: "part", name: "tank", type: "", detail: "", fqn: "Vehicle::Car::tank", declaredHere: true, notation: "part", origin };
const imported: RenderNode = { id: "n2", kind: "part", name: "wheel", type: "Wheel", detail: "", origin };
const unlocated: RenderNode = { id: "n3", kind: "part", name: "ghost", type: "", detail: "" };
// A node another workspace document declares: named for a layout, not the document's own.
const foreign: RenderNode = { ...declared, id: "n4", declaredHere: undefined, origin: { ...origin, uri: "file:///parts.sysml" } };

test("paletteItems offers every member, then every connection", () => {
  assert.deepEqual(paletteItems(palette), [
    { label: "Add part…", command: { kind: "addMember", memberKind: "part", typed: true } },
    { label: "Add port…", command: { kind: "addMember", memberKind: "port", typed: true } },
    { label: "Add fork…", command: { kind: "addMember", memberKind: "fork", typed: false } },
    { label: "Add connection…", command: { kind: "addConnection", connectionKind: "connection" } },
    { label: "Add flow…", command: { kind: "addConnection", connectionKind: "flow" } },
  ]);
});

test("nodeMenu offers the full set on a node the document declares", () => {
  const items = nodeMenu(declared, palette);
  assert.deepEqual(items[0], { label: "tank", heading: true });
  const commands = items.filter((item) => item.command).map((item) => item.command);
  assert.deepEqual(commands, [
    { kind: "reveal", id: "n1" },
    { kind: "addMember", memberKind: "part", typed: true, owner: "n1" },
    { kind: "addMember", memberKind: "port", typed: true, owner: "n1" },
    { kind: "addMember", memberKind: "fork", typed: false, owner: "n1" },
    { kind: "addConnection", connectionKind: "connection", from: "n1" },
    { kind: "addConnection", connectionKind: "flow", from: "n1" },
    { kind: "rename", id: "n1" },
    { kind: "move", id: "n1" },
    { kind: "delete", id: "n1" },
  ]);
  assert.equal(items.find((item) => item.command?.kind === "move")?.label, "Move to…");
});

// A server that gives no notation cannot say what a moved node asks its owner to admit.
test("nodeMenu only reveals a node another document declares, whatever its name", () => {
  const commands = nodeMenu(foreign, palette).filter((item) => item.command).map((item) => item.command);
  assert.deepEqual(commands, [{ kind: "reveal", id: "n4" }]);
});

test("nodeMenu offers no move on a node without a notation", () => {
  const { notation: _, ...unnotated } = declared;
  const commands = nodeMenu(unnotated, palette).filter((item) => item.command).map((item) => item.command?.kind);
  assert.deepEqual(commands, ["reveal", "addMember", "addMember", "addMember", "addConnection", "addConnection", "rename", "delete"]);
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

test("a member confined to some owners is offered on those nodes alone, and on the toolbar only when one is drawn", () => {
  const requirement: RenderNode = { ...declared, id: "n4", kind: "requirement def", name: "Fit", fqn: "Vehicle::Fit" };
  const confined: EditPalette = {
    members: ["part", "subject", "objective"],
    connections: [],
    typed: ["part", "subject", "objective"],
    owners: { subject: ["n4"], objective: [] },
  };
  const kinds = (node: RenderNode) =>
    nodeMenu(node, confined)
      .map((item) => item.command)
      .filter((command) => command?.kind === "addMember")
      .map((command) => command && "memberKind" in command && command.memberKind);
  assert.deepEqual(kinds(requirement), ["part", "subject"]);
  assert.deepEqual(kinds(declared), ["part"]);
  assert.deepEqual(
    paletteItems(confined).map((item) => item.label),
    ["Add part…", "Add subject…"],
  );
});

test("nodeMenu skips a section the palette leaves empty", () => {
  const items = nodeMenu(declared, { members: [], connections: [], typed: [] });
  assert.equal(items.filter((item) => item.separator).length, 1);
  const commands = items.filter((item) => item.command).map((item) => item.command?.kind);
  assert.deepEqual(commands, ["reveal", "rename", "move", "delete"]);
});
