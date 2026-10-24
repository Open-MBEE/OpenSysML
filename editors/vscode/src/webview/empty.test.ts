import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import { nothingToDraw, type RenderResult } from "../protocol";
import { emptyElement, emptyMessage, nothingLaidOut } from "./empty";
import { layoutCanvas } from "./layout";

const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

const empty: RenderResult = {
  view: "M::v",
  kind: "tree",
  stated: "",
  form: "mermaid",
  artifact: "",
  nodes: [],
  edges: [],
  notices: [],
  version: 1,
};

test("a rendering with no node, edge or row has nothing to draw", () => {
  assert.equal(nothingToDraw(empty), true);
  assert.equal(nothingToDraw({ ...empty, nodes: [{ id: "n0", kind: "part", name: "a", type: "", detail: "" }] }), false);
  assert.equal(nothingToDraw({ ...empty, rows: [{ cells: ["a"] }] }), false);
});

test("the status line of an empty rendering carries the server's reason, or says none was given", () => {
  assert.equal(emptyMessage(empty), "Nothing to draw: the rendering has no node, edge or row.");
  const reasons = [
    "m.sysml did not analyse cleanly; nothing was rendered: 9:10: expected a body member",
    "the view exposes nothing; the rendering is empty",
  ];
  assert.equal(emptyMessage({ ...empty, notices: [reasons[0]] }), `Nothing to draw: ${reasons[0]}`);
  assert.equal(emptyMessage({ ...empty, notices: reasons }), `Nothing to draw: ${reasons[0]} (1 more notice below)`);
});

test("what stands in for the drawing lists every notice", () => {
  const element = emptyElement({ ...empty, notices: ["one", "two"] });
  assert.equal(element.tagName, "P");
  assert.equal(element.className, "empty");
  assert.equal(element.textContent, "Nothing to draw:\none\ntwo");
});

test("a layout of nothing, or without extent, is not drawn as a canvas", () => {
  assert.equal(nothingLaidOut(layoutCanvas(empty)), true);
  assert.equal(nothingLaidOut({ roots: [{}], edges: [], width: 100, height: 0 }), true);
  const drawn = layoutCanvas({ ...empty, nodes: [{ id: "n0", kind: "part def", name: "M::A", type: "", detail: "" }] });
  assert.equal(nothingLaidOut(drawn), false);
});
