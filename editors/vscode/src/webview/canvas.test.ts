import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import type { RenderNode, RenderResult } from "../protocol";
import { drawCanvas } from "./canvas";
import { layoutCanvas } from "./layout";

// The canvas is drawn with the page's document, as it is in the webview.
const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } } };

function node(id: string, name: string, extra: Partial<RenderNode> = {}): RenderNode {
  return { id, kind: "part", name, type: "", detail: "", fqn: `M::${name}`, origin, ...extra };
}

const result: RenderResult = {
  view: "M::V",
  kind: "interconnection",
  stated: "",
  form: "mermaid",
  artifact: "",
  nodes: [
    node("a", "tank", { type: "Tank", x: 10, y: 20, width: 120, height: 60 }),
    node("b", "valve", { parent: "a" }),
    node("c", "pump", { fqn: undefined }),
    node("d", "", { kind: "fork" }),
  ],
  edges: [
    { from: "b", to: "c", label: "supply", kind: "flow", fqn: "M::bc", route: [{ x: 300, y: 300 }] },
    { from: "c", to: "d", label: "", kind: "succession" },
  ],
  notices: [],
  version: 3,
};

test("drawCanvas draws every node as a group carrying its rendering id, movable when declared", () => {
  const svg = drawCanvas(layoutCanvas(result));
  assert.equal(svg.getAttribute("viewBox")?.startsWith("0 0 "), true);
  const groups = [...svg.querySelectorAll("g.opensysml-node")];
  assert.deepEqual(groups.map((group) => group.getAttribute("data-opensysml-id")), ["a", "b", "c", "d"]);
  assert.deepEqual(groups.map((group) => group.classList.contains("movable")), [true, true, false, true]);
  // The owner's box is drawn where the model put it; its child draws after it, on top.
  const rect = svg.querySelector('g[data-opensysml-id="a"] > rect')!;
  assert.deepEqual([rect.getAttribute("x"), rect.getAttribute("y"), rect.getAttribute("width"), rect.getAttribute("height")], ["10", "20", "120", "60"]);
  assert.equal(rect.classList.contains("container"), true);
  const spans = [...svg.querySelectorAll('g[data-opensysml-id="a"] tspan')].map((span) => span.textContent);
  assert.deepEqual(spans, ["tank : Tank", "«part»"]);
  // A fork is a bar, with no label.
  assert.equal(svg.querySelector('g[data-opensysml-id="d"] > rect')?.classList.contains("filled"), true);
  assert.equal(svg.querySelector('g[data-opensysml-id="d"] text'), null);
});

test("drawCanvas draws edges with the arrowhead their kind takes and handles on a steerable route", () => {
  const svg = drawCanvas(layoutCanvas(result));
  const edges = [...svg.querySelectorAll("g.opensysml-edge")];
  assert.deepEqual(edges.map((edge) => edge.getAttribute("data-edge")), ["0", "1"]);
  assert.equal(edges[0].querySelector("polyline")?.getAttribute("marker-end"), "url(#arrow-open)");
  assert.equal(edges[1].querySelector("polyline")?.getAttribute("marker-end"), "url(#arrow)");
  assert.equal(edges[0].querySelector("text")?.textContent, "supply");
  assert.equal(edges[1].querySelector("text"), null);
  // Only the declared connection gets handles: one waypoint and one per segment.
  const handles = [...svg.querySelectorAll("g.edge-handles")];
  assert.deepEqual(handles.map((group) => group.getAttribute("data-edge")), ["0"]);
  const waypoints = [...handles[0].querySelectorAll("circle.waypoint")];
  assert.deepEqual(waypoints.map((circle) => [circle.getAttribute("data-edge"), circle.getAttribute("data-point"), circle.getAttribute("cx"), circle.getAttribute("cy")]), [
    ["0", "0", "300", "300"],
  ]);
  const segments = [...handles[0].querySelectorAll("circle.segment")].map((circle) => circle.getAttribute("data-segment"));
  assert.deepEqual(segments, ["0", "1"]);
});

test("drawCanvas draws a sequence's lifelines", () => {
  const svg = drawCanvas(layoutCanvas({
    ...result,
    kind: "sequence",
    nodes: [node("a", "a"), node("b", "b")],
    edges: [{ from: "a", to: "b", label: "ask", kind: "flow" }],
  }));
  assert.equal(svg.querySelectorAll("line.lifeline").length, 2);
  assert.equal(svg.querySelectorAll("g.edge-handles").length, 0);
  assert.equal(svg.querySelectorAll("g.opensysml-node.movable").length, 0);
});
