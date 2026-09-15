import assert from "node:assert/strict";
import { test } from "node:test";

import { moveDestinations } from "../edits";
import type { EditPalette, RenderNode, RenderResult } from "../protocol";
import { dragHint, dropOn } from "./drop";

// The interconnection rendering of
//   package Rig { part def Pump { part valve; } part def Motor; requirement def Fit { subject rig; } }
const rig = { fqn: "Rig", feature: false };
const pump: RenderNode = { id: "n1", kind: "part def", name: "Pump", type: "", detail: "", fqn: "Rig::Pump", notation: "part def", owners: [rig] };
const valve: RenderNode = { id: "n2", kind: "part", name: "valve", type: "", detail: "", parent: "n1", fqn: "Rig::Pump::valve", notation: "part", owners: [{ fqn: "Rig::Pump", feature: false }, rig] };
const motor: RenderNode = { id: "n3", kind: "part def", name: "Motor", type: "", detail: "", fqn: "Rig::Motor", notation: "part def", owners: [rig] };
const fit: RenderNode = { id: "n4", kind: "requirement def", name: "Fit", type: "", detail: "", fqn: "Rig::Fit", notation: "requirement def", owners: [rig] };
const subject: RenderNode = { id: "n5", kind: "subject", name: "rig", type: "", detail: "", parent: "n4", fqn: "Rig::Fit::rig", notation: "subject", owners: [{ fqn: "Rig::Fit", feature: false }, rig] };
const wheel: RenderNode = { id: "n6", kind: "part", name: "wheel", type: "Wheel", detail: "", parent: "n3" };
const palette: EditPalette = { members: ["part", "subject"], connections: [], typed: ["part"], owners: { subject: ["n4"] } };

function rendering(extra: Partial<RenderResult> = {}): RenderResult {
  return { view: "", kind: "interconnection", stated: "", form: "mermaid", artifact: "", nodes: [pump, valve, motor, fit, subject, wheel], edges: [], notices: [], version: 2, palette, ...extra };
}

test("dropOn admits exactly the targets Move to… lists for the node", () => {
  const result = rendering();
  const offered = new Set(moveDestinations(valve, result).map(({ node }) => node?.id));
  for (const target of result.nodes!) {
    assert.equal(dropOn(valve, target, result).admits, offered.has(target.id), target.id);
  }
  const drop = dropOn(valve, motor, result);
  assert.deepEqual(drop, { target: motor, admits: true, message: "Release to move valve into Motor." });
});

test("dropOn refuses the current owner, what the node declares, and a body that does not admit the notation", () => {
  const result = rendering();
  assert.deepEqual(dropOn(valve, pump, result), { target: pump, admits: false, message: "valve is already declared in Pump." });
  assert.deepEqual(dropOn(pump, valve, result), { target: valve, admits: false, message: "valve is declared inside Pump, which cannot be moved into it." });
  assert.deepEqual(dropOn(subject, motor, result), { target: motor, admits: false, message: "A subject cannot be declared in Motor." });
  // A node drawn under the dragged one is declared by it even when the rendering gives it no owners.
  const drawnUnder: RenderNode = { ...motor, id: "n7", name: "Inner", fqn: "Rig::Pump::Inner", parent: "n1", owners: undefined };
  assert.equal(dropOn(pump, drawnUnder, rendering({ nodes: [pump, drawnUnder] })).message, "Inner is declared inside Pump, which cannot be moved into it.");
});

test("dropOn refuses a node or target the document does not declare, and a server without model edits", () => {
  const result = rendering();
  assert.deepEqual(dropOn(wheel, pump, result), { target: pump, admits: false, message: "wheel is not declared in this document, so it cannot be moved." });
  assert.deepEqual(dropOn(valve, wheel, result), { target: wheel, admits: false, message: "wheel is not declared in this document, so nothing can be moved into it." });
  const unnotated = { ...valve, notation: undefined };
  assert.equal(dropOn(unnotated, motor, rendering({ nodes: [pump, unnotated, motor] })).message, "valve cannot be moved from the diagram.");
  const noEdits = rendering({ palette: undefined });
  assert.deepEqual(dropOn(valve, motor, noEdits), { target: motor, admits: false, message: "The language server does not serve model edits, so nothing is moved." });
});

test("dropOn names an unnamed node by its kind", () => {
  const fork: RenderNode = { id: "n8", kind: "fork", name: "", type: "", detail: "", fqn: "Rig::Pump::fork1", notation: "fork", owners: [{ fqn: "Rig::Pump", feature: false }, rig] };
  const confined: EditPalette = { ...palette, owners: { ...palette.owners, fork: [] } };
  const drop = dropOn(fork, motor, rendering({ nodes: [pump, fork, motor], palette: confined }));
  assert.equal(drop.message, "A fork cannot be declared in Motor.");
  assert.equal(dropOn(valve, fork, rendering({ nodes: [pump, valve, motor, fork] })).message, "Release to move valve into the fork.");
});

test("dragHint tells how to move a node only when a drawn node would take it", () => {
  assert.equal(dragHint(valve, rendering()), "Hold Shift and release over a node to move valve into it.");
  // The subject's only admitting body is the one declaring it; nothing drawn takes it.
  assert.equal(dragHint(subject, rendering()), undefined);
  assert.equal(dragHint(wheel, rendering()), undefined);
  assert.equal(dragHint(valve, rendering({ palette: undefined })), undefined);
});
