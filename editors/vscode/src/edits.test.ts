import assert from "node:assert/strict";
import { test } from "node:test";

import {
  ancestors,
  connectionOwner,
  describeOwner,
  describeRefusal,
  describeStale,
  fileLabel,
  referrersByFile,
  reparentOperations,
  staleDocuments,
  unopenedDocuments,
  DOCUMENT_ROOT,
  editParams,
  endpointPath,
  moveDestinations,
  moveOperation,
  nameSegments,
  offeredOn,
  ownerOf,
  ownersOf,
  placementOperations,
  rootOwner,
  validName,
} from "./edits";
import { declaredHere, ownDeclarations, type ModelEditOperation, type RenderNode, type RenderOwner } from "./protocol";

// The interconnection rendering of
//   package Vehicle { part def Car { part tank { port fuelOut; } part engine { port fuelIn; } } }
// Each declared node carries the namespaces declaring it, nearest first, whether drawn or not.
const vehicle: RenderOwner = { fqn: "Vehicle", feature: false };
const carOwner: RenderOwner = { fqn: "Vehicle::Car", feature: false };
const tankOwner: RenderOwner = { fqn: "Vehicle::Car::tank", feature: true };
const engineOwner: RenderOwner = { fqn: "Vehicle::Car::engine", feature: true };
const car: RenderNode = { id: "n1", kind: "part def", name: "Vehicle::Car", type: "", detail: "", fqn: "Vehicle::Car", declaredHere: true, owners: [vehicle] };
const tank: RenderNode = { id: "n2", kind: "part", name: "tank", type: "", detail: "", parent: "n1", fqn: "Vehicle::Car::tank", declaredHere: true, owners: [carOwner, vehicle] };
const fuelOut: RenderNode = { id: "n3", kind: "port", name: "fuelOut", type: "", detail: "", parent: "n2", fqn: "Vehicle::Car::tank::fuelOut", declaredHere: true, owners: [tankOwner, carOwner, vehicle] };
const engine: RenderNode = { id: "n4", kind: "part", name: "engine", type: "", detail: "", parent: "n1", fqn: "Vehicle::Car::engine", declaredHere: true, owners: [carOwner, vehicle] };
const fuelIn: RenderNode = { id: "n5", kind: "port", name: "fuelIn", type: "", detail: "", parent: "n4", fqn: "Vehicle::Car::engine::fuelIn", declaredHere: true, owners: [engineOwner, carOwner, vehicle] };
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
  const other: RenderNode = { id: "n7", kind: "part def", name: "Bike", type: "", detail: "", fqn: "Vehicle::Bike", declaredHere: true, owners: [vehicle] };
  assert.equal(rootOwner([...nodes, other]), undefined);
  assert.equal(rootOwner([imported]), undefined);
});

// A node another workspace document declares: named, so a layout reaches it,
// but not the document's own, so nothing else does.
const foreign: RenderNode = { id: "n8", kind: "part def", name: "Wheel", type: "", detail: "", fqn: "Wheels::Wheel", notation: "part def", owners: [{ fqn: "Wheels", feature: false }] };

test("a node another document declares owns nothing, moves nowhere, is no root", () => {
  assert.equal(ownerOf(foreign, [...nodes, foreign]), undefined);
  assert.equal(ownersOf(foreign), undefined);
  assert.equal(rootOwner([foreign]), undefined);
  assert.equal(moveOperation(foreign, ""), undefined);
  assert.deepEqual(moveDestinations(foreign, { nodes: [...nodes, foreign], version: 1 }), []);
  assert.deepEqual(
    moveDestinations(tank, { nodes: [...nodes, foreign], version: 1 }).map(({ fqn }) => fqn),
    moveDestinations(tank, { nodes, version: 1 }).map(({ fqn }) => fqn),
  );
});

test("ownersOf ends at the document, and is nothing for a node the document does not declare", () => {
  assert.deepEqual(ownersOf(fuelOut), [tankOwner, carOwner, vehicle, DOCUMENT_ROOT]);
  assert.deepEqual(ownersOf({ ...car, owners: undefined }), [DOCUMENT_ROOT]);
  assert.equal(ownersOf(imported), undefined);
});

test("connectionOwner is the nearest namespace declaring both ends", () => {
  assert.deepEqual(connectionOwner(fuelOut, fuelIn), carOwner);
  assert.deepEqual(connectionOwner(tank, engine), carOwner);
});

test("connectionOwner is the namespace declaring the container when one end contains the other", () => {
  assert.deepEqual(connectionOwner(tank, fuelOut), carOwner);
  assert.deepEqual(connectionOwner(fuelOut, tank), carOwner);
  assert.equal(endpointPath(tank, carOwner), "tank");
  assert.equal(endpointPath(fuelOut, carOwner), "tank.fuelOut");
});

test("connectionOwner reaches the package two roots are declared in", () => {
  const other: RenderNode = { id: "n7", kind: "part def", name: "Bike", type: "", detail: "", fqn: "Vehicle::Bike", declaredHere: true, owners: [vehicle] };
  assert.deepEqual(connectionOwner(car, other), vehicle);
});

test("connectionOwner is nothing when an end is not declared by the document", () => {
  assert.equal(connectionOwner(fuelOut, imported), undefined);
  assert.equal(connectionOwner(imported, fuelOut), undefined);
});

// The interconnection rendering of a view exposing Vehicle::Car::tank and
// Vehicle::Car::engine: both are drawn as roots, Car is not drawn at all, yet
// their owners still name it.
const exposedTank: RenderNode = { ...tank, id: "e1", parent: undefined };
const exposedFuelOut: RenderNode = { ...fuelOut, id: "e2", parent: "e1" };
const exposedEngine: RenderNode = { ...engine, id: "e3", parent: undefined };
const exposedFuelIn: RenderNode = { ...fuelIn, id: "e4", parent: "e3" };

test("connectionOwner finds the declaration exposed siblings share even when it is not drawn", () => {
  assert.deepEqual(connectionOwner(exposedFuelOut, exposedFuelIn), carOwner);
  assert.deepEqual(connectionOwner(exposedTank, exposedEngine), carOwner);
  assert.equal(endpointPath(exposedFuelOut, carOwner), "tank.fuelOut");
  assert.equal(endpointPath(exposedEngine, carOwner), "engine");
});

// The tree rendering of
//   part pump { port outlet; } part tank { port inlet; }
const pumpOwner: RenderOwner = { fqn: "pump", feature: true };
const topTankOwner: RenderOwner = { fqn: "tank", feature: true };
const pump: RenderNode = { id: "r1", kind: "part", name: "pump", type: "", detail: "", fqn: "pump", declaredHere: true };
const outlet: RenderNode = { id: "r2", kind: "port", name: "outlet", type: "", detail: "", parent: "r1", fqn: "pump::outlet", declaredHere: true, owners: [pumpOwner] };
const topTank: RenderNode = { id: "r3", kind: "part", name: "tank", type: "", detail: "", fqn: "tank", declaredHere: true };
const inlet: RenderNode = { id: "r4", kind: "port", name: "inlet", type: "", detail: "", parent: "r3", fqn: "tank::inlet", declaredHere: true, owners: [topTankOwner] };

test("connectionOwner is the document root for two top-level declarations", () => {
  assert.deepEqual(connectionOwner(pump, topTank), DOCUMENT_ROOT);
  assert.deepEqual(connectionOwner(outlet, inlet), DOCUMENT_ROOT);
  assert.deepEqual(connectionOwner(outlet, topTank), DOCUMENT_ROOT);
});

test("connectionOwner prefers a declared common owner to the document root", () => {
  assert.deepEqual(connectionOwner(outlet, { ...inlet, owners: [pumpOwner] }), pumpOwner);
  assert.deepEqual(connectionOwner(outlet, pump), DOCUMENT_ROOT);
});

test("connectionOwner is nothing when a root is not declared by the document", () => {
  const library: RenderNode = { id: "r5", kind: "part", name: "lib", type: "", detail: "" };
  assert.equal(connectionOwner(pump, library), undefined);
});

test("DOCUMENT_ROOT is named by the empty owner and described as the document", () => {
  assert.equal(DOCUMENT_ROOT.fqn, "");
  assert.equal(describeOwner(DOCUMENT_ROOT), "the document");
  assert.equal(describeOwner(carOwner), "Vehicle::Car");
});

test("endpointPath spells the feature chain below the owner", () => {
  assert.equal(endpointPath(fuelOut, carOwner), "tank.fuelOut");
  assert.equal(endpointPath(tank, carOwner), "tank");
  assert.equal(endpointPath(fuelOut, tankOwner), "fuelOut");
});

test("endpointPath refuses the owner itself and a node outside it", () => {
  assert.equal(endpointPath(car, carOwner), undefined);
  assert.equal(endpointPath(fuelIn, tankOwner), undefined);
  assert.equal(endpointPath(imported, carOwner), undefined);
});

test("endpointPath from the document root spells the whole chain", () => {
  assert.equal(endpointPath(outlet, DOCUMENT_ROOT), "pump.outlet");
  assert.equal(endpointPath(topTank, DOCUMENT_ROOT), "tank");
});

test("endpointPath steps into a namespace that is not a feature by ::", () => {
  assert.equal(endpointPath(fuelOut, DOCUMENT_ROOT), "Vehicle::Car::tank.fuelOut");
  assert.equal(endpointPath(fuelOut, vehicle), "Car::tank.fuelOut");
});

test("nameSegments splits at :: outside quotes only", () => {
  assert.deepEqual(nameSegments("Vehicle::Car"), ["Vehicle", "Car"]);
  assert.deepEqual(nameSegments("'P::Q'::tank"), ["'P::Q'", "tank"]);
  assert.deepEqual(nameSegments("'fuel::out'"), ["'fuel::out'"]);
  assert.deepEqual(nameSegments("'it\\'s::x'"), ["'it\\'s::x'"]);
  assert.deepEqual(nameSegments(""), [""]);
});

// The tree rendering of
//   part 'x::y' { port 'fuel::out'; } part def 'Top Def' { port in1; } package x { part y; }
// The server quotes each name of an fqn on its own, so 'x::y' and x::y stay apart.
const quotedPartOwner: RenderOwner = { fqn: "'x::y'", feature: true };
const quotedDefOwner: RenderOwner = { fqn: "'Top Def'", feature: false };
const quotedPart: RenderNode = { id: "q1", kind: "part", name: "'x::y'", type: "", detail: "", fqn: "'x::y'", declaredHere: true };
const quotedPort: RenderNode = { id: "q2", kind: "port", name: "'fuel::out'", type: "", detail: "", parent: "q1", fqn: "'x::y'::'fuel::out'", declaredHere: true, owners: [quotedPartOwner] };
const quotedDef: RenderNode = { id: "q3", kind: "part def", name: "'Top Def'", type: "", detail: "", fqn: "'Top Def'", declaredHere: true };
const quotedIn: RenderNode = { id: "q4", kind: "port", name: "in1", type: "", detail: "", parent: "q3", fqn: "'Top Def'::in1", declaredHere: true, owners: [quotedDefOwner] };
const nestedY: RenderNode = { id: "q6", kind: "part", name: "y", type: "", detail: "", parent: "q5", fqn: "x::y", declaredHere: true, owners: [{ fqn: "x", feature: false }] };

test("endpointPath keeps a quoted name holding :: as one step", () => {
  assert.equal(endpointPath(quotedPort, quotedPartOwner), "'fuel::out'");
  assert.equal(endpointPath(quotedPort, DOCUMENT_ROOT), "'x::y'.'fuel::out'");
  assert.equal(endpointPath(quotedIn, DOCUMENT_ROOT), "'Top Def'::in1");
  assert.equal(endpointPath(nestedY, DOCUMENT_ROOT), "x::y");
});

test("connectionOwner takes a quoted top-level name holding :: as the document's", () => {
  assert.deepEqual(connectionOwner(quotedPort, quotedIn), DOCUMENT_ROOT);
  assert.deepEqual(connectionOwner(quotedPart, quotedDef), DOCUMENT_ROOT);
});

test("connectionOwner tells 'x::y' from x::y", () => {
  assert.deepEqual(connectionOwner(quotedPort, nestedY), DOCUMENT_ROOT);
  assert.deepEqual(connectionOwner(quotedPort, { ...quotedPort, id: "q7", name: "sink", fqn: "'x::y'::sink" }), quotedPartOwner);
});

test("editParams asks for the version the rendering drew, not the buffer's", () => {
  const operations: ModelEditOperation[] = [{ kind: "delete", target: tank.fqn! }];
  const params = editParams("file:///vehicle.sysml", { nodes, version: 3 }, operations);
  assert.deepEqual(params, { textDocument: { uri: "file:///vehicle.sysml" }, version: 3, operations });
});

// A menu opened on one drawing and chosen from after a redraw names node ids the new
// drawing may have given to other declarations; the panel numbers drawings, not the document.
test("offeredOn holds only for the drawing the action was offered on", () => {
  assert.equal(offeredOn(3, 3), true);
  assert.equal(offeredOn(4, 3), false);
  assert.equal(offeredOn(0, 3), false);
  assert.equal(offeredOn(3, 0), false);
});

// The same rendering with each declaration's notation, as a server that serves moves sends it.
const notated = nodes.map((node) => (node.fqn ? { ...node, notation: node.kind } : node));
const [carN, tankN, fuelOutN, engineN, fuelInN] = notated;

test("moveDestinations offers the declared nodes but the node, its owner and what it declares, then the document", () => {
  const rendering = { nodes: notated, version: 1 };
  assert.deepEqual(
    moveDestinations(fuelOutN, rendering).map(({ fqn, node }) => [fqn, node?.id]),
    [["Vehicle::Car", "n1"], ["Vehicle::Car::engine", "n4"], ["Vehicle::Car::engine::fuelIn", "n5"], ["", undefined]],
  );
  // The car declares every other drawn node, so only the document is left; the wheel is not declared here.
  assert.deepEqual(moveDestinations(carN, rendering), [{ fqn: "" }]);
  // The tank is owned by the car: not the car again, not the port it declares, not itself.
  assert.deepEqual(
    moveDestinations(tankN, rendering).map(({ fqn }) => fqn),
    ["Vehicle::Car::engine", "Vehicle::Car::engine::fuelIn", ""],
  );
  assert.equal(moveDestinations(engineN, rendering).some(({ node }) => node === fuelInN), false);
});

test("moveDestinations is empty for a node without a notation or a declaration here", () => {
  const rendering = { nodes: notated, version: 1 };
  assert.deepEqual(moveDestinations(tank, rendering), []);
  assert.deepEqual(moveDestinations(imported, rendering), []);
});

test("moveDestinations keeps a confined notation to the nodes the palette admits it in, and off the document", () => {
  const fit: RenderNode = { id: "n8", kind: "requirement def", name: "Fit", type: "", detail: "", fqn: "Vehicle::Fit", declaredHere: true, notation: "requirement def", owners: [vehicle] };
  const check: RenderNode = { id: "n9", kind: "requirement def", name: "Check", type: "", detail: "", fqn: "Vehicle::Check", declaredHere: true, notation: "requirement def", owners: [vehicle] };
  const subject: RenderNode = { id: "n10", kind: "subject", name: "car", type: "Car", detail: "", parent: "n8", fqn: "Vehicle::Fit::car", declaredHere: true, notation: "subject", owners: [{ fqn: "Vehicle::Fit", feature: false }, vehicle] };
  const all = [...notated, fit, check, subject];
  const palette = { members: ["part"], connections: [], typed: [], owners: { subject: ["n8", "n9"] } };
  assert.deepEqual(moveDestinations(subject, { nodes: all, version: 1, palette }), [{ fqn: "Vehicle::Check", node: check }]);
  // A part is not confined: every declared node but the document's own admits it, the document too.
  assert.deepEqual(
    moveDestinations(tankN, { nodes: all, version: 1, palette }).map(({ fqn }) => fqn),
    ["Vehicle::Car::engine", "Vehicle::Car::engine::fuelIn", "Vehicle::Fit", "Vehicle::Check", "Vehicle::Fit::car", ""],
  );
});

// A rendering may draw a declaration as its own root rather than inside its owner; what
// the node declares is still not a place to move it to.
test("moveDestinations excludes what the node declares even when drawn apart from it", () => {
  const a: RenderNode = { id: "n1", kind: "part def", name: "A", type: "", detail: "", fqn: "P::A", declaredHere: true, notation: "part def", owners: [{ fqn: "P", feature: false }] };
  const b: RenderNode = { id: "n2", kind: "part def", name: "B", type: "", detail: "", fqn: "P::A::B", declaredHere: true, notation: "part def", owners: [{ fqn: "P::A", feature: false }, { fqn: "P", feature: false }] };
  const c: RenderNode = { id: "n3", kind: "part def", name: "C", type: "", detail: "", fqn: "P::C", declaredHere: true, notation: "part def", owners: [{ fqn: "P", feature: false }] };
  assert.deepEqual(moveDestinations(a, { nodes: [a, b, c], version: 1 }), [{ fqn: "P::C", node: c }, { fqn: "" }]);
});

test("moveDestinations leaves the document out for a top-level declaration", () => {
  const top: RenderNode = { ...carN, owners: undefined };
  assert.deepEqual(moveDestinations(top, { nodes: [top, tankN], version: 1 }), []);
});

test("moveOperation names the target and the owner, the document by the empty owner", () => {
  assert.deepEqual(moveOperation(fuelOutN, "Vehicle::Car"), { kind: "move", target: "Vehicle::Car::tank::fuelOut", owner: "Vehicle::Car" });
  assert.deepEqual(moveOperation(tankN, ""), { kind: "move", target: "Vehicle::Car::tank", owner: "" });
  assert.equal(moveOperation(imported, "Vehicle::Car"), undefined);
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

const carURI = "file:///work/car.sysml";
const fleetURI = "file:///work/fleet.sysml";

test("referrersByFile lists referrers in the edited document flat", () => {
  const lines = referrersByFile(
    [
      {
        operation: 0,
        failure: "delete-referenced",
        message: "Vehicle::Car::tank is referenced",
        referring: ["Vehicle::Car::c1", "Vehicle::Car::c2"],
        referrers: [
          { name: "Vehicle::Car::c1", uri: carURI },
          { name: "Vehicle::Car::c2", uri: carURI },
        ],
      },
    ],
    carURI,
  );
  assert.deepEqual(lines, ["Vehicle::Car::c1", "Vehicle::Car::c2"]);
});

test("referrersByFile groups referrers in other files by file, the edited document first", () => {
  const lines = referrersByFile(
    [
      {
        operation: 0,
        failure: "delete-referenced",
        message: "Vehicle::Car is referenced",
        referring: ["fleet.sysml: Fleet::truck", "Vehicle::Car::c1", "fleet.sysml: Fleet::van"],
        referrers: [
          { name: "Fleet::truck", uri: fleetURI },
          { name: "Vehicle::Car::c1", uri: carURI },
          { name: "Fleet::van", uri: fleetURI },
        ],
      },
    ],
    carURI,
  );
  assert.deepEqual(lines, ["In this document:", "  Vehicle::Car::c1", "In fleet.sysml:", "  Fleet::truck", "  Fleet::van"]);
});

test("referrersByFile names only the other files when the edited document holds no referrer", () => {
  const lines = referrersByFile(
    [
      {
        operation: 0,
        failure: "delete-referenced",
        message: "Vehicle::Car is referenced",
        referrers: [{ name: "Fleet::truck", uri: fleetURI }],
      },
    ],
    carURI,
  );
  assert.deepEqual(lines, ["In fleet.sysml:", "  Fleet::truck"]);
});

test("referrersByFile falls back to the plain names of a server telling no documents", () => {
  const lines = referrersByFile(
    [{ operation: 0, failure: "delete-referenced", message: "referenced", referring: ["Vehicle::Car::c1"] }],
    carURI,
  );
  assert.deepEqual(lines, ["Vehicle::Car::c1"]);
});

test("fileLabel is the decoded last path segment", () => {
  assert.equal(fileLabel("file:///work/my%20models/fleet.sysml"), "fleet.sysml");
  assert.equal(fileLabel("file:///work/my%20fleet.sysml?x=1"), "my fleet.sysml");
});

const twoFileEdit = {
  documentChanges: [
    { textDocument: { uri: carURI, version: 3 }, edits: [] },
    { textDocument: { uri: fleetURI, version: 7 }, edits: [] },
    { textDocument: { uri: "file:///work/disk.sysml", version: null }, edits: [] },
  ],
};

test("staleDocuments is empty when every open document is at the version the edit was computed against", () => {
  const versions = new Map([
    [carURI, 3],
    [fleetURI, 7],
  ]);
  assert.deepEqual(staleDocuments(twoFileEdit, (uri) => versions.get(uri)), []);
});

test("staleDocuments names a document whose buffer moved on", () => {
  const versions = new Map([
    [carURI, 3],
    [fleetURI, 8],
  ]);
  assert.deepEqual(staleDocuments(twoFileEdit, (uri) => versions.get(uri)), [fleetURI]);
});

test("staleDocuments ignores a document no buffer holds, whether the server read it open or from disk", () => {
  assert.deepEqual(staleDocuments(twoFileEdit, (uri) => (uri === carURI ? 3 : undefined)), []);
  assert.deepEqual(staleDocuments({ changes: { [carURI]: [] } }, () => 1), []);
});

test("staleDocuments names a document the server read from disk that a buffer has since been opened for", () => {
  const versions = new Map([
    [carURI, 3],
    [fleetURI, 7],
    ["file:///work/disk.sysml", 1],
  ]);
  assert.deepEqual(staleDocuments(twoFileEdit, (uri) => versions.get(uri)), ["file:///work/disk.sysml"]);
});

test("unopenedDocuments lists the documents no buffer holds, whatever version the server gave them", () => {
  assert.deepEqual(unopenedDocuments(twoFileEdit, (uri) => (uri === carURI ? 3 : undefined)), [
    fleetURI,
    "file:///work/disk.sysml",
  ]);
  assert.deepEqual(unopenedDocuments({ changes: { [carURI]: [] } }, () => undefined), []);
});

test("unopenedDocuments is empty once every document the edit names is open", () => {
  assert.deepEqual(unopenedDocuments(twoFileEdit, () => 1), []);
});

test("describeStale names the files and says the edit was not applied", () => {
  assert.equal(
    describeStale([fleetURI, "file:///work/disk.sysml"]),
    "fleet.sysml, disk.sysml changed while the edit was computed; it is not applied, so repeat the action.",
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

const placed = {
  nodes: [tank, engine, imported],
  edges: [
    { from: "n2", to: "n4", label: "", kind: "connection", fqn: "Vehicle::Car::fuel" },
    { from: "n6", to: "n4", label: "", kind: "connection" },
  ],
  view: "Vehicle::Wiring",
  version: 4,
};

test("placementOperations names each placed node and steered edge by its declaration, in the view", () => {
  assert.deepEqual(
    placementOperations(placed, [{ id: "n2", layout: { x: 10, y: 20 } }, { id: "n4", layout: { x: 30, y: 40, width: 100, height: 50 } }], [{ index: 0, route: [{ x: 5, y: 5 }] }]),
    [
      { kind: "setLayout", target: "Vehicle::Car::tank", view: "Vehicle::Wiring", layout: { x: 10, y: 20 } },
      { kind: "setLayout", target: "Vehicle::Car::engine", view: "Vehicle::Wiring", layout: { x: 30, y: 40, width: 100, height: 50 } },
      { kind: "setRoute", target: "Vehicle::Car::fuel", view: "Vehicle::Wiring", route: [{ x: 5, y: 5 }] },
    ],
  );
});

test("placementOperations places on the element when the rendering has no view, and clears a route", () => {
  assert.deepEqual(placementOperations({ ...placed, view: "" }, [{ id: "n2", layout: { x: 1, y: 2 } }], [{ index: 0 }]), [
    { kind: "setLayout", target: "Vehicle::Car::tank", view: undefined, layout: { x: 1, y: 2 } },
    { kind: "setRoute", target: "Vehicle::Car::fuel", view: undefined, route: undefined },
  ]);
});

const declaration = { start: { line: 4, character: 8 }, end: { line: 4, character: 37 } };

test("placementOperations targets a node or edge no qualified name reaches by its declaration, inline", () => {
  const unnamed = {
    ...placed,
    nodes: [...placed.nodes, { ...imported, id: "n7", declaration }],
    edges: [...placed.edges, { from: "n7", to: "n4", label: "", kind: "transition", declaration }],
  };
  assert.deepEqual(placementOperations(unnamed, [{ id: "n7", layout: { x: 1, y: 2 } }], [{ index: 2, route: [{ x: 3, y: 4 }] }, { index: 0 }]), [
    { kind: "setLayout", declaration, layout: { x: 1, y: 2 } },
    { kind: "setRoute", declaration, route: [{ x: 3, y: 4 }] },
    { kind: "setRoute", target: "Vehicle::Car::fuel", view: "Vehicle::Wiring", route: undefined },
  ]);
});

// A view of views.sysml drawing what parts.sysml declares: the nodes carry their
// qualified names and owners as any declared node does, and the unnamed
// connection its declaration range, all located in parts.sysml.
const partsURI = "file:///work/parts.sysml";
const partsDigest = "3f9c1a2b4d5e6f70";
const partsOrigin = (line: number) => ({ uri: partsURI, range: { start: { line, character: 2 }, end: { line, character: 30 } }, digest: partsDigest });
const engineDef: RenderOwner = { fqn: "Machinery::Engine", feature: false };
const machinery: RenderOwner = { fqn: "Machinery", feature: false };
const rotor: RenderNode = { id: "n1", kind: "part", name: "rotor", type: "", detail: "", parent: "n0", fqn: "Machinery::Engine::rotor", owners: [engineDef, machinery], origin: partsOrigin(3) };
const stator: RenderNode = { id: "n2", kind: "part", name: "stator", type: "", detail: "", parent: "n0", fqn: "Machinery::Engine::stator", owners: [engineDef, machinery], origin: partsOrigin(4) };
const rotorToStator = { from: "n1", to: "n2", label: "", kind: "connection", declaration: partsOrigin(5).range, origin: partsOrigin(5) };
const drawnFromParts = {
  nodes: [{ id: "n0", kind: "part def", name: "Machinery::Engine", type: "", detail: "", fqn: "Machinery::Engine", owners: [machinery], origin: partsOrigin(2) }, rotor, stator],
  edges: [rotorToStator],
  view: "EngineViews::engineView",
  version: 2,
};

test("placementOperations names a node another document declares by its qualified name, in the view, at the text it was rendered from", () => {
  assert.deepEqual(placementOperations(drawnFromParts, [{ id: "n1", layout: { x: 120, y: 40 } }], []), [
    { kind: "setLayout", target: "Machinery::Engine::rotor", declaredIn: partsURI, digest: partsDigest, view: "EngineViews::engineView", layout: { x: 120, y: 40 } },
  ]);
});

test("placementOperations targets a declaration another document holds in that document, at the text it was rendered from", () => {
  assert.deepEqual(placementOperations(drawnFromParts, [], [{ index: 0, route: [{ x: 30, y: 90 }] }]), [
    { kind: "setRoute", declaration: rotorToStator.declaration, declaredIn: partsURI, digest: partsDigest, route: [{ x: 30, y: 90 }] },
  ]);
  const unnamedNode = { ...drawnFromParts, nodes: [...drawnFromParts.nodes, { ...imported, id: "n7", declaration: partsOrigin(6).range, origin: partsOrigin(6) }] };
  assert.deepEqual(placementOperations(unnamedNode, [{ id: "n7", layout: { x: 1, y: 2 } }], [{ index: 0 }]), [
    { kind: "setLayout", declaration: partsOrigin(6).range, declaredIn: partsURI, digest: partsDigest, layout: { x: 1, y: 2 } },
    { kind: "setRoute", declaration: rotorToStator.declaration, declaredIn: partsURI, digest: partsDigest, route: undefined },
  ]);
});

// The edit a drag on drawnFromParts comes back as: the view's document, at the version
// the request named, and parts.sysml at the version the server holds it.
const viewsURI = "file:///work/views.sysml";
const layoutEdit = {
  documentChanges: [
    { textDocument: { uri: viewsURI, version: 2 }, edits: [] },
    { textDocument: { uri: partsURI, version: 5 }, edits: [] },
  ],
};

test("a layout edit reaching another document is applied only while that document is at the version it was computed against", () => {
  const held = new Map([
    [viewsURI, 2],
    [partsURI, 5],
  ]);
  assert.deepEqual(unopenedDocuments(layoutEdit, (uri) => held.get(uri)), []);
  assert.deepEqual(staleDocuments(layoutEdit, (uri) => held.get(uri)), []);
  held.set(partsURI, 6);
  assert.deepEqual(staleDocuments(layoutEdit, (uri) => held.get(uri)), [partsURI]);
  assert.equal(describeStale([partsURI]), "parts.sysml changed while the edit was computed; it is not applied, so repeat the action.");
});

test("a layout edit reaching a document no buffer holds asks for it to be opened first", () => {
  assert.deepEqual(unopenedDocuments(layoutEdit, (uri) => (uri === viewsURI ? 2 : undefined)), [partsURI]);
  const inline = { documentChanges: [{ textDocument: { uri: partsURI, version: null }, edits: [] }] };
  assert.deepEqual(unopenedDocuments(inline, (uri) => (uri === viewsURI ? 2 : undefined)), [partsURI]);
  assert.deepEqual(staleDocuments(inline, (uri) => (uri === viewsURI ? 2 : undefined)), []);
  assert.deepEqual(staleDocuments(inline, () => 1), [partsURI]);
});

// The placed rendering with each declaration's notation, as a server that serves moves sends it.
const reparentable = { ...placed, nodes: [carN, tankN, engineN, fuelInN, imported], palette: { members: ["part"], connections: [], typed: [] } };

test("reparentOperations writes the drop's placements first, then the move, as one batch", () => {
  assert.deepEqual(
    reparentOperations(reparentable, "n2", "n4", [{ id: "n2", layout: { x: 10, y: 20 } }], [{ index: 0, route: [{ x: 5, y: 5 }] }]),
    [
      { kind: "setLayout", target: "Vehicle::Car::tank", view: "Vehicle::Wiring", layout: { x: 10, y: 20 } },
      { kind: "setRoute", target: "Vehicle::Car::fuel", view: "Vehicle::Wiring", route: [{ x: 5, y: 5 }] },
      { kind: "move", target: "Vehicle::Car::tank", owner: "Vehicle::Car::engine" },
    ],
  );
  // A drop with nothing placed is the move alone.
  assert.deepEqual(reparentOperations(reparentable, "n2", "n4", [], []), [{ kind: "move", target: "Vehicle::Car::tank", owner: "Vehicle::Car::engine" }]);
});

test("reparentOperations refuses a target Move to… would not offer: the owner, a descendant, the node, an undeclared node", () => {
  const nodes = [{ id: "n2", layout: { x: 10, y: 20 } }];
  assert.equal(reparentOperations(reparentable, "n2", "n1", nodes, []), undefined);
  assert.equal(reparentOperations(reparentable, "n4", "n5", [{ id: "n4", layout: { x: 1, y: 2 } }], []), undefined);
  assert.equal(reparentOperations(reparentable, "n2", "n2", nodes, []), undefined);
  assert.equal(reparentOperations(reparentable, "n2", "n6", nodes, []), undefined);
  assert.equal(reparentOperations(reparentable, "n6", "n4", [{ id: "n6", layout: { x: 1, y: 2 } }], []), undefined);
  assert.equal(reparentOperations(reparentable, "n2", "missing", nodes, []), undefined);
  // A body the palette does not open for the notation.
  const confined = { ...reparentable, palette: { members: ["part"], connections: [], typed: [], owners: { part: ["n1"] } } };
  assert.equal(reparentOperations(confined, "n2", "n4", nodes, []), undefined);
});

test("a drop is one applyModelEdit request, pinned to the version the canvas was drawn from", () => {
  const operations = reparentOperations(reparentable, "n2", "n4", [{ id: "n2", layout: { x: 10, y: 20 } }], [])!;
  assert.deepEqual(editParams("file:///vehicle.sysml", reparentable, operations), {
    textDocument: { uri: "file:///vehicle.sysml" },
    version: 4,
    operations: [
      { kind: "setLayout", target: "Vehicle::Car::tank", view: "Vehicle::Wiring", layout: { x: 10, y: 20 } },
      { kind: "move", target: "Vehicle::Car::tank", owner: "Vehicle::Car::engine" },
    ],
  });
});

test("reparentOperations refuses a placement the document does not declare, so nothing of the drop is written", () => {
  assert.equal(reparentOperations(reparentable, "n2", "n4", [{ id: "n2", layout: { x: 1, y: 2 } }, { id: "n6", layout: { x: 1, y: 2 } }], []), undefined);
});

// Another view of the same document version draws other declarations under the same ids, so a
// drop begun on the first drawing is refused by its number rather than moving what the ids now name.
test("a drop from a replaced drawing is not resolved against the drawing that replaced it", () => {
  const other = { ...reparentable, view: "Vehicle::Plumbing", nodes: [{ ...engineN, id: "n2" }, { ...tankN, id: "n4" }, carN] };
  assert.deepEqual(
    reparentOperations(other, "n2", "n4", [], [])?.at(-1),
    { kind: "move", target: "Vehicle::Car::engine", owner: "Vehicle::Car::tank" },
  );
  assert.equal(offeredOn(1, 1), true);
  assert.equal(offeredOn(2, 1), false);
});

test("placementOperations refuses a node or edge the document does not declare", () => {
  assert.equal(placementOperations(placed, [{ id: "n6", layout: { x: 1, y: 2 } }], []), undefined);
  assert.equal(placementOperations(placed, [{ id: "missing", layout: { x: 1, y: 2 } }], []), undefined);
  assert.equal(placementOperations(placed, [], [{ index: 1, route: [] }]), undefined);
  assert.equal(placementOperations(placed, [], [{ index: 9, route: [] }]), undefined);
});

// A server predating declaredHere named the requested document's declarations
// alone, so read as its own, every named node keeps the edits it used to offer.
test("ownDeclarations reads a rendering of an older server as declaring every named node", () => {
  const legacy: RenderNode[] = [
    { id: "n1", kind: "part def", name: "Vehicle::Car", type: "", detail: "", fqn: "Vehicle::Car", notation: "part def", owners: [vehicle] },
    { id: "n2", kind: "part", name: "tank", type: "", detail: "", parent: "n1", fqn: "Vehicle::Car::tank", notation: "part", owners: [carOwner, vehicle] },
    { id: "n3", kind: "part", name: "wheel", type: "Wheel", detail: "", parent: "n1" },
  ];
  assert.ok(!legacy.some(declaredHere));
  const own = ownDeclarations(legacy);
  assert.deepEqual(own.map(declaredHere), [true, true, false]);
  assert.equal(own[2], legacy[2]);
  assert.deepEqual(own.map((node) => node.declaredHere), [true, true, undefined]);
  assert.deepEqual(legacy.map((node) => node.declaredHere), [undefined, undefined, undefined]);
  assert.equal(ownerOf(own[1], own), own[1]);
  assert.equal(ownerOf(own[2], own), own[0]);
  assert.deepEqual(moveOperation(own[1], "Vehicle::Car"), { kind: "move", target: "Vehicle::Car::tank", owner: "Vehicle::Car" });
  assert.equal(ownerOf(legacy[1], legacy), undefined);
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
