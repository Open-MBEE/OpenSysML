import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import type { RenderNode, RenderResult } from "../protocol";
import { drawCanvas, liftNode } from "./canvas";
import { layoutCanvas, MARGIN } from "./layout";

// The canvas is drawn with the page's document, as it is in the webview.
const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };

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
  const groups = [...svg.querySelectorAll<SVGGElement>("g.opensysml-node")];
  assert.deepEqual(groups.map((group) => group.dataset.opensysmlId), ["a", "b", "c", "d"]);
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

test("liftNode floats the dragged subtree over a canvas that stays put, drawn last", () => {
  const layout = layoutCanvas(result);
  const svg = drawCanvas(layout);
  const before = svg.querySelector('g[data-opensysml-id="c"] > rect')!.getAttribute("x");
  liftNode(svg, layout, "a", 40, -10);
  const groups = [...svg.querySelectorAll<SVGGElement>("g.opensysml-node")];
  // The owner and its child move together; the others keep their place and their drawing order.
  assert.deepEqual(groups.map((group) => group.dataset.opensysmlId), ["c", "d", "a", "b"]);
  assert.deepEqual(groups.map((group) => group.getAttribute("transform")), [null, null, "translate(40 -10)", "translate(40 -10)"]);
  assert.deepEqual(groups.map((group) => group.classList.contains("lifted")), [false, false, true, true]);
  assert.equal(svg.querySelector('g[data-opensysml-id="a"] > rect')!.getAttribute("x"), "10");
  assert.equal(svg.querySelector('g[data-opensysml-id="c"] > rect')!.getAttribute("x"), before);
  // Lifted within the canvas, the canvas keeps its size and origin.
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], [String(layout.width), String(layout.height)]);
  assert.equal(svg.getAttribute("viewBox"), `${layout.origin.x} ${layout.origin.y} ${layout.width} ${layout.height}`);
  // The edge from the lifted valve to the pump is redrawn to follow it, with its handles.
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.lifted")].map((group) => group.className.baseVal.split(" ")[0]), [
    "opensysml-node", "opensysml-node", "opensysml-edge", "edge-handles",
  ]);
  // A node the layout does not hold lifts nothing.
  liftNode(svg, layout, "z", 1, 1);
  assert.equal(svg.querySelectorAll("g.lifted").length, 4);
});

test("liftNode redraws the edges at the lifted subtree: an inner one moves whole, a crossing one follows its lifted end", () => {
  const withInner: RenderResult = {
    ...result,
    nodes: [...result.nodes!, node("e", "gauge", { parent: "a" })],
    edges: [
      ...result.edges!,
      { from: "b", to: "e", label: "reads", kind: "connection", fqn: "M::be", route: [{ x: 60, y: 60 }] },
    ],
  };
  const layout = layoutCanvas(withInner);
  const svg = drawCanvas(layout);
  const polyline = (index: number) => svg.querySelector(`g.opensysml-edge[data-edge="${index}"] polyline`)!.getAttribute("points");
  const label = (index: number) => svg.querySelector(`g.opensysml-edge[data-edge="${index}"] text`)!;
  const outerBefore = polyline(1);
  const crossingBefore = polyline(0);
  liftNode(svg, layout, "a", 100, 30);
  // The inner edge's line, label and handles all move by the lift; the edge between outside nodes does not.
  assert.equal(polyline(2), layout.edges[2].points.map((p) => `${p.x + 100},${p.y + 30}`).join(" "));
  assert.deepEqual([label(2).getAttribute("x"), label(2).getAttribute("y")], [String(layout.edges[2].label.x + 100), String(layout.edges[2].label.y + 30 - 6)]);
  const waypoint = svg.querySelector<SVGCircleElement>('g.edge-handles[data-edge="2"] circle.waypoint')!;
  assert.deepEqual([waypoint.getAttribute("cx"), waypoint.getAttribute("cy")], ["160", "90"]);
  assert.equal(polyline(1), outerBefore);
  // The crossing edge keeps its waypoint and its anchor on the pump; its anchor on the valve moves with the valve.
  assert.notEqual(polyline(0), crossingBefore);
  const crossing = polyline(0)!.split(" ");
  const before = crossingBefore!.split(" ");
  assert.deepEqual(crossing.slice(1), before.slice(1));
  assert.notEqual(crossing[0], before[0]);
  // The redrawn groups are marked lifted and keep their place among the edges and handles.
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")].map((g) => [g.dataset.edge, g.classList.contains("lifted")]), [["0", true], ["1", false], ["2", true]]);
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.edge-handles")].map((g) => [g.dataset.edge, g.classList.contains("lifted")]), [["0", true], ["2", true]]);
  // Each edge is drawn once.
  assert.equal(svg.querySelectorAll("g.opensysml-edge").length, 3);
});

test("liftNode grows the canvas right and down to keep a node lifted past its edge in view", () => {
  const layout = layoutCanvas(result);
  const svg = drawCanvas(layout);
  liftNode(svg, layout, "a", 1000, 500);
  // The whole lifted subtree counts: the child reaches below its owner's stated box.
  const boxes = ["a", "b"].map((id) => layout.nodes.get(id)!.box);
  const width = Math.max(...boxes.map((box) => box.x + box.width)) + 1000 + MARGIN - layout.origin.x;
  const height = Math.max(...boxes.map((box) => box.y + box.height)) + 500 + MARGIN - layout.origin.y;
  assert.ok(width > layout.width && height > layout.height);
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], [String(width), String(height)]);
  // The origin holds, so what is not lifted stays where it was drawn.
  assert.equal(svg.getAttribute("viewBox"), `${layout.origin.x} ${layout.origin.y} ${width} ${height}`);
});

test("drawCanvas opens the view on geometry the model puts left of or above the origin", () => {
  const layout = layoutCanvas({ ...result, nodes: [node("a", "tank", { x: -120, y: 40, width: 100, height: 50 })], edges: [] });
  const svg = drawCanvas(layout);
  assert.equal(svg.getAttribute("viewBox"), `${layout.origin.x} ${layout.origin.y} ${layout.width} ${layout.height}`);
  assert.ok(layout.origin.x <= -120 && layout.origin.y <= 0);
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], [String(layout.width), String(layout.height)]);
  const rect = svg.querySelector('g[data-opensysml-id="a"] > rect')!;
  assert.deepEqual([rect.getAttribute("x"), rect.getAttribute("y")], ["-120", "40"]);
});

test("drawCanvas draws edges with the arrowhead their kind takes and handles on a steerable route", () => {
  const svg = drawCanvas(layoutCanvas(result));
  const edges = [...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")];
  assert.deepEqual(edges.map((edge) => edge.dataset.edge), ["0", "1"]);
  assert.equal(edges[0].querySelector("polyline")?.getAttribute("marker-end"), "url(#arrow-open)");
  assert.equal(edges[1].querySelector("polyline")?.getAttribute("marker-end"), "url(#arrow)");
  assert.equal(edges[0].querySelector("text")?.textContent, "supply");
  assert.equal(edges[1].querySelector("text"), null);
  // Only the declared connection gets handles: one waypoint and one per segment.
  const handles = [...svg.querySelectorAll<SVGGElement>("g.edge-handles")];
  assert.deepEqual(handles.map((group) => group.dataset.edge), ["0"]);
  const waypoints = [...handles[0].querySelectorAll<SVGCircleElement>("circle.waypoint")];
  assert.deepEqual(waypoints.map((circle) => [circle.dataset.edge, circle.dataset.point, circle.getAttribute("cx"), circle.getAttribute("cy")]), [
    ["0", "0", "300", "300"],
  ]);
  const segments = [...handles[0].querySelectorAll<SVGCircleElement>("circle.segment")].map((circle) => circle.dataset.segment);
  assert.deepEqual(segments, ["0", "1"]);
});

test("drawCanvas leaves out the edges and handles at nodes a collapsed owner hides", () => {
  const svg = drawCanvas(layoutCanvas({
    ...result,
    nodes: [
      node("a", "tank", { x: 10, y: 20, collapsed: true }),
      node("b", "valve", { parent: "a" }),
      node("e", "gauge", { parent: "b" }),
      node("c", "pump"),
    ],
    edges: [
      { from: "b", to: "e", label: "", kind: "connection", fqn: "M::be", route: [{ x: 50, y: 50 }] },
      { from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc", route: [{ x: 300, y: 300 }] },
      { from: "a", to: "c", label: "", kind: "connection", fqn: "M::ac" },
    ],
  }));
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.opensysml-node")].map((group) => group.dataset.opensysmlId), ["a", "c"]);
  assert.equal(svg.querySelector('g[data-opensysml-id="a"] text.collapsed')?.textContent, "+");
  // Neither the edge inside the collapsed owner nor the one crossing its border is drawn; the owner's own is.
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")].map((edge) => edge.dataset.edge), ["2"]);
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.edge-handles")].map((group) => group.dataset.edge), ["2"]);
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

test("drawCanvas classes each box as the PlantUML form stereotypes it, squaring only a definition", () => {
  const boxed: RenderResult = {
    ...result,
    nodes: [
      node("p", "Plant", { kind: "package" }),
      node("q", "Lib", { kind: "library package" }),
      node("def", "Tank", { kind: "part def" }),
      node("cls", "Thing", { kind: "class" }),
      node("reg", "r", { kind: "region" }),
      node("use", "tank", { kind: "part" }),
    ],
    edges: [],
  };
  const svg = drawCanvas(layoutCanvas(boxed));
  const rect = (id: string) => svg.querySelector(`g[data-opensysml-id="${id}"] > rect`)!;
  assert.deepEqual(
    ["p", "q", "def", "cls", "reg", "use"].map((id) => [rect(id).classList[1], rect(id).getAttribute("rx")]),
    [["package", "6"], ["package", "6"], ["definition", null], ["definition", null], ["region", "6"], ["usage", "6"]],
  );
});

test("drawCanvas carries each palette colour on the shape as its own custom property, and nothing on a node given none", () => {
  const coloured: RenderResult = {
    ...result,
    nodes: [
      node("def", "Tank", { kind: "part def", fill: "#E69F00", border: "#E69F00" }),
      node("use", "tank", { kind: "part", parent: "def", fill: "#F5D999", border: "#E69F00" }),
      node("lifeline", "producer", { kind: "part", fill: "#F5D999" }),
      node("plain", "pump"),
      node("d", "", { kind: "fork" }),
    ],
  };
  const svg = drawCanvas(layoutCanvas(coloured));
  const shape = (id: string) => svg.querySelector<SVGElement>(`g[data-opensysml-id="${id}"] > .shape`)!;
  assert.deepEqual([shape("def").style.getPropertyValue("--node-fill"), shape("def").style.getPropertyValue("--node-border")], ["#E69F00", "#E69F00"]);
  assert.deepEqual([shape("use").style.getPropertyValue("--node-fill"), shape("use").style.getPropertyValue("--node-border")], ["#F5D999", "#E69F00"]);
  // A sequence participant is filled alone; its border stays the look's.
  assert.deepEqual([shape("lifeline").style.getPropertyValue("--node-fill"), shape("lifeline").style.getPropertyValue("--node-border")], ["#F5D999", ""]);
  assert.equal(shape("plain").getAttribute("style"), null);
  assert.equal(shape("d").getAttribute("style"), null);
  // Colour changes nothing the editing gestures read.
  const groups = [...svg.querySelectorAll<SVGGElement>("g.opensysml-node")];
  assert.deepEqual(groups.map((group) => [group.dataset.opensysmlId, group.dataset.kind]), [["def", "part def"], ["use", "part"], ["lifeline", "part"], ["plain", "part"], ["d", "fork"]]);
  assert.equal(shape("def").classList.contains("container"), true);
  assert.equal(shape("d").classList.contains("filled"), true);
});
