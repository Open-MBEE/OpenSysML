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
  staleDocuments,
  DOCUMENT_ROOT,
  editParams,
  endpointPath,
  nameSegments,
  offeredOn,
  ownerOf,
  ownersOf,
  rootOwner,
  validName,
} from "./edits";
import type { ModelEditOperation, RenderNode, RenderOwner } from "./protocol";

// The interconnection rendering of
//   package Vehicle { part def Car { part tank { port fuelOut; } part engine { port fuelIn; } } }
// Each declared node carries the namespaces declaring it, nearest first, whether drawn or not.
const vehicle: RenderOwner = { fqn: "Vehicle", feature: false };
const carOwner: RenderOwner = { fqn: "Vehicle::Car", feature: false };
const tankOwner: RenderOwner = { fqn: "Vehicle::Car::tank", feature: true };
const engineOwner: RenderOwner = { fqn: "Vehicle::Car::engine", feature: true };
const car: RenderNode = { id: "n1", kind: "part def", name: "Vehicle::Car", type: "", detail: "", fqn: "Vehicle::Car", owners: [vehicle] };
const tank: RenderNode = { id: "n2", kind: "part", name: "tank", type: "", detail: "", parent: "n1", fqn: "Vehicle::Car::tank", owners: [carOwner, vehicle] };
const fuelOut: RenderNode = { id: "n3", kind: "port", name: "fuelOut", type: "", detail: "", parent: "n2", fqn: "Vehicle::Car::tank::fuelOut", owners: [tankOwner, carOwner, vehicle] };
const engine: RenderNode = { id: "n4", kind: "part", name: "engine", type: "", detail: "", parent: "n1", fqn: "Vehicle::Car::engine", owners: [carOwner, vehicle] };
const fuelIn: RenderNode = { id: "n5", kind: "port", name: "fuelIn", type: "", detail: "", parent: "n4", fqn: "Vehicle::Car::engine::fuelIn", owners: [engineOwner, carOwner, vehicle] };
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
  const other: RenderNode = { id: "n7", kind: "part def", name: "Bike", type: "", detail: "", fqn: "Vehicle::Bike", owners: [vehicle] };
  assert.equal(rootOwner([...nodes, other]), undefined);
  assert.equal(rootOwner([imported]), undefined);
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
  const other: RenderNode = { id: "n7", kind: "part def", name: "Bike", type: "", detail: "", fqn: "Vehicle::Bike", owners: [vehicle] };
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
const pump: RenderNode = { id: "r1", kind: "part", name: "pump", type: "", detail: "", fqn: "pump" };
const outlet: RenderNode = { id: "r2", kind: "port", name: "outlet", type: "", detail: "", parent: "r1", fqn: "pump::outlet", owners: [pumpOwner] };
const topTank: RenderNode = { id: "r3", kind: "part", name: "tank", type: "", detail: "", fqn: "tank" };
const inlet: RenderNode = { id: "r4", kind: "port", name: "inlet", type: "", detail: "", parent: "r3", fqn: "tank::inlet", owners: [topTankOwner] };

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
const quotedPart: RenderNode = { id: "q1", kind: "part", name: "'x::y'", type: "", detail: "", fqn: "'x::y'" };
const quotedPort: RenderNode = { id: "q2", kind: "port", name: "'fuel::out'", type: "", detail: "", parent: "q1", fqn: "'x::y'::'fuel::out'", owners: [quotedPartOwner] };
const quotedDef: RenderNode = { id: "q3", kind: "part def", name: "'Top Def'", type: "", detail: "", fqn: "'Top Def'" };
const quotedIn: RenderNode = { id: "q4", kind: "port", name: "in1", type: "", detail: "", parent: "q3", fqn: "'Top Def'::in1", owners: [quotedDefOwner] };
const nestedY: RenderNode = { id: "q6", kind: "part", name: "y", type: "", detail: "", parent: "q5", fqn: "x::y", owners: [{ fqn: "x", feature: false }] };

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

test("staleDocuments ignores a document without a buffer and one the server read from disk", () => {
  const versions = new Map([
    [carURI, 3],
    ["file:///work/disk.sysml", 12],
  ]);
  assert.deepEqual(staleDocuments(twoFileEdit, (uri) => versions.get(uri)), []);
  assert.deepEqual(staleDocuments({ changes: { [carURI]: [] } }, () => 1), []);
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
