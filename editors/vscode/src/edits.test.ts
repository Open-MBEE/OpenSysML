import assert from "node:assert/strict";
import { test } from "node:test";

import {
  ancestors,
  connectionOwner,
  describeRefusal,
  editParams,
  endpointPath,
  ownerOf,
  rootOwner,
  validName,
} from "./edits";
import type { ModelEditOperation, RenderNode } from "./protocol";

// The interconnection rendering of
//   package Vehicle { part def Car { part tank { port fuelOut; } part engine { port fuelIn; } } }
const car: RenderNode = { id: "n1", kind: "part def", name: "Vehicle::Car", type: "", detail: "", fqn: "Vehicle::Car" };
const tank: RenderNode = { id: "n2", kind: "part", name: "tank", type: "", detail: "", parent: "n1", fqn: "Vehicle::Car::tank" };
const fuelOut: RenderNode = { id: "n3", kind: "port", name: "fuelOut", type: "", detail: "", parent: "n2", fqn: "Vehicle::Car::tank::fuelOut" };
const engine: RenderNode = { id: "n4", kind: "part", name: "engine", type: "", detail: "", parent: "n1", fqn: "Vehicle::Car::engine" };
const fuelIn: RenderNode = { id: "n5", kind: "port", name: "fuelIn", type: "", detail: "", parent: "n4", fqn: "Vehicle::Car::engine::fuelIn" };
// A node drawn from a library the document does not declare: no fqn.
const imported: RenderNode = { id: "n6", kind: "part", name: "wheel", type: "Wheel", detail: "", parent: "n1" };
const nodes = [car, tank, fuelOut, engine, fuelIn, imported];

test("ancestors walks to the root, nearest first", () => {
  assert.deepEqual(ancestors(fuelOut, nodes).map((node) => node.id), ["n2", "n1"]);
  assert.deepEqual(ancestors(car, nodes), []);
});

test("ancestors stops at a parent cycle instead of looping", () => {
  const a: RenderNode = { id: "a", kind: "part", name: "a", type: "", detail: "", parent: "b" };
  const b: RenderNode = { id: "b", kind: "part", name: "b", type: "", detail: "", parent: "a" };
  assert.deepEqual(ancestors(a, [a, b]).map((node) => node.id), ["b"]);
});

test("ownerOf is the node itself when it is declared here", () => {
  assert.equal(ownerOf(tank, nodes), tank);
});

test("ownerOf climbs past a node the document does not declare", () => {
  assert.equal(ownerOf(imported, nodes), car);
  assert.equal(ownerOf(undefined, nodes), undefined);
});

test("rootOwner is the single declared root, or nothing", () => {
  assert.equal(rootOwner(nodes), car);
  const other: RenderNode = { id: "n7", kind: "part def", name: "Bike", type: "", detail: "", fqn: "Vehicle::Bike" };
  assert.equal(rootOwner([...nodes, other]), undefined);
  assert.equal(rootOwner([imported]), undefined);
});

test("connectionOwner is the nearest common ancestor", () => {
  assert.equal(connectionOwner(fuelOut, fuelIn, nodes), car);
  assert.equal(connectionOwner(tank, engine, nodes), car);
});

test("connectionOwner is the container when one end contains the other", () => {
  assert.equal(connectionOwner(tank, fuelOut, nodes), tank);
  assert.equal(connectionOwner(fuelOut, tank, nodes), tank);
});

test("connectionOwner is nothing for unrelated roots", () => {
  const other: RenderNode = { id: "n7", kind: "part def", name: "Bike", type: "", detail: "", fqn: "Vehicle::Bike" };
  assert.equal(connectionOwner(car, other, [...nodes, other]), undefined);
});

test("endpointPath spells the feature chain below the owner", () => {
  assert.equal(endpointPath(fuelOut, car, nodes), "tank.fuelOut");
  assert.equal(endpointPath(tank, car, nodes), "tank");
  assert.equal(endpointPath(fuelOut, tank, nodes), "fuelOut");
});

test("endpointPath refuses the owner itself and a node outside it", () => {
  assert.equal(endpointPath(car, car, nodes), undefined);
  assert.equal(endpointPath(fuelIn, tank, nodes), undefined);
});

test("endpointPath refuses a step that has no name", () => {
  const anonymous: RenderNode = { id: "n8", kind: "part", name: "", type: "T", detail: "", parent: "n1", fqn: "Vehicle::Car::part1" };
  const inner: RenderNode = { id: "n9", kind: "port", name: "p", type: "", detail: "", parent: "n8" };
  assert.equal(endpointPath(inner, car, [...nodes, anonymous, inner]), undefined);
});

test("editParams asks for the version the rendering drew, not the buffer's", () => {
  const operations: ModelEditOperation[] = [{ kind: "delete", target: tank.fqn! }];
  const params = editParams("file:///vehicle.sysml", { nodes, version: 3 }, operations);
  assert.deepEqual(params, { textDocument: { uri: "file:///vehicle.sysml" }, version: 3, operations });
});

test("describeRefusal quotes the message, diagnostics and referrers", () => {
  const text = describeRefusal([
    {
      operation: 0,
      failure: "delete-referenced",
      message: "Vehicle::Car::tank is referenced",
      referring: ["Vehicle::Car::c1", "Vehicle::Car::c2"],
    },
    {
      operation: 1,
      failure: "semantic-error",
      message: "edit introduces 1 error",
      diagnostics: [{ range: { start: { line: 2, character: 4 }, end: { line: 2, character: 9 } }, message: "unresolved name X" }],
    },
  ]);
  assert.equal(
    text,
    [
      "Vehicle::Car::tank is referenced",
      "Referenced by Vehicle::Car::c1, Vehicle::Car::c2.",
      "edit introduces 1 error",
      "3:5: unresolved name X",
    ].join("\n"),
  );
});

test("validName accepts identifiers and quoted names", () => {
  assert.equal(validName("engine"), undefined);
  assert.equal(validName("_x1"), undefined);
  assert.equal(validName("'front wheel'"), undefined);
  assert.equal(validName("  padded  "), undefined);
});

test("validName says what is wrong", () => {
  assert.match(validName("") ?? "", /required/);
  assert.match(validName("   ") ?? "", /required/);
  assert.match(validName("front wheel") ?? "", /identifier/);
  assert.match(validName("1st") ?? "", /identifier/);
  assert.match(validName("a::b") ?? "", /identifier/);
});

// The scanner follows the lexer's UNRESTRICTED_NAME: a backslash escapes one
// of b t n f r " ' \, so a quote after a backslash does not end the name.
test("validName reads escapes in quoted names as the lexer does", () => {
  assert.equal(validName("'it\\'s'"), undefined);
  assert.equal(validName("'tab\\there'"), undefined);
  assert.equal(validName("'back\\\\slash'"), undefined);
  assert.equal(validName("'say \\\"hi\\\"'"), undefined);
  assert.equal(validName("'\\''"), undefined);
  assert.match(validName("'it's'") ?? "", /closing quote/);
  assert.match(validName("'open") ?? "", /closing quote/);
  assert.match(validName("'two\nlines'") ?? "", /closing quote/);
  assert.match(validName("'ends in backslash\\'") ?? "", /closing quote/);
  assert.match(validName("'bad \\q escape'") ?? "", /backslash/);
  assert.match(validName("'trailing\\") ?? "", /backslash/);
  assert.match(validName("''") ?? "", /between the quotes/);
  assert.match(validName("'a' b") ?? "", /closing quote/);
});
