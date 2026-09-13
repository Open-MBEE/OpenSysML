import assert from "node:assert/strict";
import { test } from "node:test";

import {
  ancestors,
  connectionOwner,
  describeOwner,
  describeRefusal,
  DOCUMENT_ROOT,
  editParams,
  endpointPath,
  nameSegments,
  offeredOn,
  oneName,
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

test("connectionOwner is nothing for unrelated roots the document does not declare", () => {
  const other: RenderNode = { id: "n7", kind: "part def", name: "Bike", type: "", detail: "", fqn: "Vehicle::Bike" };
  assert.equal(connectionOwner(car, other, [...nodes, other]), undefined);
});

// The tree rendering of
//   part pump { port outlet; } part tank { port inlet; }
const pump: RenderNode = { id: "r1", kind: "part", name: "pump", type: "", detail: "", fqn: "pump" };
const outlet: RenderNode = { id: "r2", kind: "port", name: "outlet", type: "", detail: "", parent: "r1", fqn: "pump::outlet" };
const topTank: RenderNode = { id: "r3", kind: "part", name: "tank", type: "", detail: "", fqn: "tank" };
const inlet: RenderNode = { id: "r4", kind: "port", name: "inlet", type: "", detail: "", parent: "r3", fqn: "tank::inlet" };
const topLevelNodes = [pump, outlet, topTank, inlet];

test("connectionOwner is the document root for two top-level declarations", () => {
  assert.equal(connectionOwner(pump, topTank, topLevelNodes), DOCUMENT_ROOT);
  assert.equal(connectionOwner(outlet, inlet, topLevelNodes), DOCUMENT_ROOT);
  assert.equal(connectionOwner(outlet, topTank, topLevelNodes), DOCUMENT_ROOT);
});

test("connectionOwner prefers a declared common ancestor to the document root", () => {
  assert.equal(connectionOwner(outlet, pump, topLevelNodes), pump);
});

test("connectionOwner is nothing when a root is not declared by the document", () => {
  const library: RenderNode = { id: "r5", kind: "part", name: "lib", type: "", detail: "" };
  assert.equal(connectionOwner(pump, library, [...topLevelNodes, library]), undefined);
  assert.equal(connectionOwner(pump, car, [...topLevelNodes, car]), undefined);
});

test("DOCUMENT_ROOT is named by the empty owner and described as the document", () => {
  assert.equal(DOCUMENT_ROOT.fqn ?? "", "");
  assert.equal(describeOwner(DOCUMENT_ROOT), "the document");
  assert.equal(describeOwner(car), "Vehicle::Car");
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

test("endpointPath from the document root spells the whole chain", () => {
  assert.equal(endpointPath(outlet, DOCUMENT_ROOT, topLevelNodes), "pump.outlet");
  assert.equal(endpointPath(topTank, DOCUMENT_ROOT, topLevelNodes), "tank");
});

test("endpointPath from the document root refuses a chain the document does not declare", () => {
  assert.equal(endpointPath(fuelOut, DOCUMENT_ROOT, nodes), undefined);
  assert.equal(endpointPath(DOCUMENT_ROOT, DOCUMENT_ROOT, topLevelNodes), undefined);
});

test("endpointPath refuses a step that has no name", () => {
  const anonymous: RenderNode = { id: "n8", kind: "part", name: "", type: "T", detail: "", parent: "n1", fqn: "Vehicle::Car::part1" };
  const inner: RenderNode = { id: "n9", kind: "port", name: "p", type: "", detail: "", parent: "n8" };
  assert.equal(endpointPath(inner, car, [...nodes, anonymous, inner]), undefined);
});

test("nameSegments splits at :: outside quotes only", () => {
  assert.deepEqual(nameSegments("Vehicle::Car"), ["Vehicle", "Car"]);
  assert.deepEqual(nameSegments("'P::Q'::tank"), ["'P::Q'", "tank"]);
  assert.deepEqual(nameSegments("'fuel::out'"), ["'fuel::out'"]);
  assert.deepEqual(nameSegments("'it\\'s::x'"), ["'it\\'s::x'"]);
  assert.deepEqual(nameSegments(""), [""]);
});

test("oneName accepts a bare or quoted name and refuses a qualified or empty one", () => {
  assert.equal(oneName("tank"), true);
  assert.equal(oneName("'a b'"), true);
  assert.equal(oneName("'fuel::out'"), true);
  assert.equal(oneName("Vehicle::Car"), false);
  assert.equal(oneName("'P::Q'::tank"), false);
  assert.equal(oneName(""), false);
});

// The tree rendering of
//   part 'x::y' { port 'fuel::out'; } part def 'Top Def' { port in1; }
// The server writes each node's name as the notation does, quoted when it must be.
const quotedPart: RenderNode = { id: "q1", kind: "part", name: "'x::y'", type: "", detail: "", fqn: "x::y" };
const quotedPort: RenderNode = { id: "q2", kind: "port", name: "'fuel::out'", type: "", detail: "", parent: "q1", fqn: "x::y::fuel::out" };
const quotedDef: RenderNode = { id: "q3", kind: "part def", name: "'Top Def'", type: "", detail: "", fqn: "Top Def" };
const quotedIn: RenderNode = { id: "q4", kind: "port", name: "in1", type: "", detail: "", parent: "q3", fqn: "Top Def::in1" };
const quotedNodes = [quotedPart, quotedPort, quotedDef, quotedIn];

test("endpointPath keeps a quoted name holding :: as one step", () => {
  assert.equal(endpointPath(quotedPort, quotedPart, quotedNodes), "'fuel::out'");
  assert.equal(endpointPath(quotedPort, DOCUMENT_ROOT, quotedNodes), "'x::y'.'fuel::out'");
  assert.equal(endpointPath(quotedIn, DOCUMENT_ROOT, quotedNodes), "'Top Def'.in1");
});

test("connectionOwner takes a quoted top-level name holding :: as the document's", () => {
  assert.equal(connectionOwner(quotedPort, quotedIn, quotedNodes), DOCUMENT_ROOT);
  assert.equal(connectionOwner(quotedPart, quotedDef, quotedNodes), DOCUMENT_ROOT);
});

test("endpointPath still refuses a root drawn under a qualified name", () => {
  const nested: RenderNode = { id: "q5", kind: "part", name: "'P::Q'::tank", type: "", detail: "", fqn: "P::Q::tank" };
  const port: RenderNode = { id: "q6", kind: "port", name: "p", type: "", detail: "", parent: "q5", fqn: "P::Q::tank::p" };
  assert.equal(endpointPath(port, DOCUMENT_ROOT, [nested, port]), undefined);
  assert.equal(connectionOwner(port, quotedIn, [nested, port, ...quotedNodes]), undefined);
});

test("editParams asks for the version the rendering drew, not the buffer's", () => {
  const operations: ModelEditOperation[] = [{ kind: "delete", target: tank.fqn! }];
  const params = editParams("file:///vehicle.sysml", { nodes, version: 3 }, operations);
  assert.deepEqual(params, { textDocument: { uri: "file:///vehicle.sysml" }, version: 3, operations });
});

// A menu opened on one rendering and chosen from after a redraw names node ids
// the new rendering may have given to other declarations.
test("offeredOn holds only for the rendering the action was offered on", () => {
  assert.equal(offeredOn({ nodes, version: 3 }, 3), true);
  assert.equal(offeredOn({ nodes, version: 4 }, 3), false);
  assert.equal(offeredOn({ nodes: [], version: 0 }, 3), false);
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
