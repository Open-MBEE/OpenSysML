import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import type { RenderNode, RenderResult } from "../protocol";
import { drawCanvas, liftNode } from "./canvas";
import { edgeLabelBox, FONT_SIZE, glyphSize, layoutCanvas, MARGIN, portBox, portCenter, portLabelPlacement, PORT_SIZE } from "./layout";

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
    // Placed where the tank's fixed box has no room for it: a stated place is drawn, an unplaced child is not.
    node("b", "valve", { parent: "a", x: 26, y: 72 }),
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
  assert.deepEqual(spans, ["«part»", "tank : Tank"]);
  assert.deepEqual([...svg.querySelectorAll('g[data-opensysml-id="a"] tspan')].map((span) => span.getAttribute("class")), ["keyword", "head"]);
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
  // Lifted within the canvas, the canvas keeps its origin and height; it widens only as far
  // as the label of the redrawn edge, riding the edge's midpoint right, reaches past it.
  const label = svg.querySelector("g.opensysml-edge.lifted text.edge-label")!;
  const width = Math.max(layout.width, Number(label.getAttribute("x")) + glyphSize(label.textContent ?? "").width / 2 + MARGIN - layout.origin.x);
  assert.ok(width > layout.width);
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], [String(width), String(layout.height)]);
  assert.equal(svg.getAttribute("viewBox"), `${layout.origin.x} ${layout.origin.y} ${width} ${layout.height}`);
  // The edge from the lifted valve to the pump is redrawn to follow it, with its handles.
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.lifted")].map((group) => group.classList[0]), [
    "opensysml-node", "opensysml-node", "opensysml-edge", "edge-handles",
  ]);
  // A node the layout does not hold lifts nothing.
  liftNode(svg, layout, "z", 1, 1);
  assert.equal(svg.querySelectorAll("g.lifted").length, 4);
});

test("liftNode redraws the edges at the lifted subtree: an inner one moves whole, a crossing one follows its lifted end", () => {
  const withInner: RenderResult = {
    ...result,
    nodes: [...result.nodes!, node("e", "gauge", { parent: "a", x: 154, y: 72 })],
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

test("liftNode keeps the zoom a canvas is drawn at while it grows", () => {
  const layout = layoutCanvas(result);
  const svg = drawCanvas(layout);
  svg.setAttribute("width", String(layout.width * 2));
  svg.setAttribute("height", String(layout.height * 2));
  liftNode(svg, layout, "a", 1000, 500);
  const viewBox = (svg.getAttribute("viewBox") ?? "").split(" ").map(Number);
  assert.ok(viewBox[2] > layout.width && viewBox[3] > layout.height);
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], [String(viewBox[2] * 2), String(viewBox[3] * 2)]);
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

// labelled is a and b, placed where stated, joined by one edge routed through the points and labelled.
function labelled(a: Partial<RenderNode>, b: Partial<RenderNode>, label: string, route: { x: number; y: number }[]): RenderResult {
  return {
    ...result,
    nodes: [node("a", "a", { kind: "requirement", width: 120, height: 60, ...a }), node("b", "b", { kind: "requirement", width: 120, height: 60, ...b })],
    edges: [{ from: "a", to: "b", label, kind: "derive", route }],
  };
}

// drawnLabel is where the canvas put an edge's label: the text's anchor and baseline, with the width the layout allows it.
function drawnLabel(svg: SVGSVGElement): { x: number; baseline: number; width: number } {
  const text = svg.querySelector("text.edge-label")!;
  return { x: Number(text.getAttribute("x")), baseline: Number(text.getAttribute("y")), width: glyphSize(text.textContent ?? "").width };
}

test("drawCanvas opens the view on an edge label routed above the top row of nodes", () => {
  // The router takes the edge over the top of the row; its label sits above the line, above every box.
  const layout = layoutCanvas(labelled({ x: 24, y: 24 }, { x: 324, y: 24 }, "«derive»", [{ x: 84, y: 0 }, { x: 384, y: 0 }]));
  const svg = drawCanvas(layout);
  const label = drawnLabel(svg);
  assert.ok(label.baseline < 24, `the label at ${label.baseline} lies above the row`);
  assert.ok(layout.origin.y <= label.baseline - FONT_SIZE, `the canvas top ${layout.origin.y} cuts the label above ${label.baseline}`);
  assert.equal(svg.getAttribute("viewBox"), `${layout.origin.x} ${layout.origin.y} ${layout.width} ${layout.height}`);
  assert.deepEqual([svg.getAttribute("width"), svg.getAttribute("height")], [String(layout.width), String(layout.height)]);
});

test("drawCanvas opens the view on an edge label wider than the segment it is centred on, left and right of the boxes", () => {
  const label = "«a long allocation label»";
  const right = layoutCanvas(labelled({ x: 24, y: 24 }, { x: 24, y: 224 }, label, [{ x: 168, y: 54 }, { x: 168, y: 254 }]));
  const onRight = drawnLabel(drawCanvas(right));
  assert.ok(right.origin.x + right.width >= onRight.x + onRight.width / 2, `the canvas right ${right.origin.x + right.width} cuts the label at ${onRight.x}`);
  const left = layoutCanvas(labelled({ x: 48, y: 24 }, { x: 48, y: 224 }, label, [{ x: 0, y: 54 }, { x: 0, y: 254 }]));
  const onLeft = drawnLabel(drawCanvas(left));
  assert.ok(left.origin.x <= onLeft.x - onLeft.width / 2, `the canvas left ${left.origin.x} cuts the label at ${onLeft.x}`);
});

test("drawCanvas opens the view on the arrowhead of an edge ending on the canvas's edge", () => {
  // The edge runs down x = 0 and ends there; its arrowhead is drawn to either side of the line.
  const layout = layoutCanvas(labelled({ x: 0, y: 24 }, { x: 0, y: 224 }, "", [{ x: 0, y: 54 }, { x: 0, y: 254 }]));
  const points = (drawCanvas(layout).querySelector("polyline")?.getAttribute("points") ?? "").split(" ");
  assert.equal(points.at(-1)?.split(",")[0], "0");
  assert.ok(layout.origin.x < 0, `the canvas left ${layout.origin.x} cuts the arrowhead on x = 0`);
});

test("drawCanvas draws edges with the arrowhead their kind takes and handles on a steerable route", () => {
  const svg = drawCanvas(layoutCanvas(result));
  const edges = [...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")];
  assert.deepEqual(edges.map((edge) => edge.dataset.edge), ["0", "1"]);
  assert.equal(edges[0].querySelector("polyline")?.getAttribute("marker-end"), "url(#arrow)");
  assert.equal(edges[1].querySelector("polyline")?.getAttribute("marker-end"), "url(#arrow-open)");
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

test("drawCanvas marks a tree's relationship edges the way the graphical notation does", () => {
  // A migrated block definition diagram: the specialization, typing and
  // compositions are implied by the declarations, so none has an FQN of its
  // own to steer a route by; the client routes each itself.
  const tree: RenderResult = {
    ...result,
    kind: "tree",
    nodes: [node("v", "Vehicle"), node("c", "Car"), node("w", "Wheel"), node("d", "Driver"), node("f", "fleetCar")],
    edges: [
      { from: "c", to: "v", label: "", kind: "specialization" },
      { from: "v", to: "w", label: "wheels[4]", kind: "composition" },
      { from: "v", to: "d", label: "driver[0..1]", kind: "reference" },
      { from: "f", to: "v", label: "", kind: "typing" },
    ],
  };
  const svg = drawCanvas(layoutCanvas(tree));
  const defs = [...svg.querySelectorAll("defs marker")].map((marker) => marker.id);
  assert.deepEqual(defs, ["arrow", "arrow-open", "triangle-hollow", "triangle-dots", "triangle-bar", "diamond-filled", "diamond-hollow", "circle-plus", "triangle-four-dots", "portion", "circle-hollow"]);
  assert.equal(svg.querySelector("#triangle-hollow path")?.getAttribute("class"), "arrow-hollow");
  assert.equal(svg.querySelector("#diamond-filled path")?.getAttribute("class"), "arrow-fill");
  const lines = [...svg.querySelectorAll<SVGPolylineElement>("g.opensysml-edge polyline")];
  assert.deepEqual(lines.map((line) => [line.getAttribute("marker-start"), line.getAttribute("marker-end")]), [
    [null, "url(#triangle-hollow)"],
    ["url(#diamond-filled)", null],
    ["url(#diamond-hollow)", null],
    [null, "url(#triangle-dots)"],
  ]);
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")].map((group) => group.classList.contains("typing")), [false, false, false, true]);
  assert.deepEqual([...svg.querySelectorAll("g.opensysml-edge text")].map((text) => text.textContent), ["wheels[4]", "driver[0..1]"]);
  // Every edge is routed, none is steerable.
  assert.ok(lines.every((line) => (line.getAttribute("points") ?? "").split(" ").length >= 2));
  assert.equal(svg.querySelectorAll("g.edge-handles").length, 0);
});

test("drawCanvas draws a collapsed node's ports, the edges at them and its collapsed mark", () => {
  const svg = drawCanvas(layoutCanvas({
    ...result,
    nodes: [
      node("a", "tank", { x: 10, y: 20, collapsed: true }),
      node("b", "out", { kind: "port", parent: "a" }),
      node("c", "pump"),
    ],
    edges: [
      { from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc", route: [{ x: 300, y: 300 }] },
      { from: "a", to: "c", label: "", kind: "connection", fqn: "M::ac" },
    ],
  }));
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.opensysml-node")].map((group) => group.dataset.opensysmlId), ["a", "b", "c"]);
  assert.equal(svg.querySelector('g[data-opensysml-id="a"] text.collapsed')?.textContent, "+");
  assert.ok(svg.querySelector('g[data-opensysml-id="a"] .shape.container'), "a collapsed node with ports is drawn as a container");
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")].map((edge) => edge.dataset.edge), ["0", "1"]);
  assert.deepEqual([...svg.querySelectorAll<SVGGElement>("g.edge-handles")].map((group) => group.dataset.edge), ["0", "1"]);
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

test("drawCanvas draws the port group, square, label, title, and includes them in its extents", () => {
  const ported = {
    ...result,
    nodes: [node("api", "service", {
      x: 20,
      y: 30,
      width: 100,
      height: 60,
      ports: [
        { id: "service.api", name: "api", type: "Api", direction: "out" },
        { id: "service.log", name: "log", type: "Log", direction: "out", inherited: true },
      ],
    })],
    edges: [],
  };
  const layout = layoutCanvas(ported);
  const entry = layout.nodes.get("api")!;
  const placed = entry.ports[0];
  const svg = drawCanvas(layout);
  const group = svg.querySelector<SVGGElement>('g[data-opensysml-id="api"] > g.port[data-port="service.api"]')!;
  assert.deepEqual([...group.children].map((child) => child.tagName.toLowerCase()), ["title", "rect", "text"]);
  assert.equal(group.querySelector("title")?.textContent, "api : Api");
  const inherited = svg.querySelector<SVGGElement>('g[data-opensysml-id="api"] > g.port[data-port="service.log"]')!;
  assert.equal(inherited.querySelector("title")?.textContent, "^log : Log");
  assert.equal(inherited.querySelector("text.port-label")?.textContent, "^log");
  const square = portBox(entry.box, placed);
  const center = portCenter(entry.box, placed);
  const rect = group.querySelector("rect.port-shape")!;
  assert.deepEqual(["x", "y", "width", "height"].map((name) => rect.getAttribute(name)), [
    String(square.x),
    String(square.y),
    String(PORT_SIZE),
    String(PORT_SIZE),
  ]);
  assert.equal(center.y, entry.box.y + entry.box.height);
  const placement = portLabelPlacement(entry.box, placed);
  const label = group.querySelector("text.port-label")!;
  assert.deepEqual([label.textContent, label.getAttribute("x"), label.getAttribute("y"), label.getAttribute("text-anchor")], [
    "api",
    String(placement.x),
    String(placement.y),
    placement.anchor,
  ]);
  assert.ok(layout.width >= placement.bounds.x + placement.bounds.width + MARGIN);
  assert.ok(layout.height >= placement.bounds.y + placement.bounds.height + MARGIN);
});

test("drawCanvas preserves the portless SVG output", () => {
  const svg = drawCanvas(layoutCanvas(result));
  assert.equal(
    createHash("sha256").update(svg.outerHTML).digest("hex"),
    "43c3b5225634001e08b36d080f8687204fe684ed2057923b0ef65ffda16bfcaa",
  );
});

test("drawCanvas marks the notation's remaining relationship glyphs", () => {
  const graph: RenderResult = {
    ...result,
    kind: "definition",
    nodes: [node("p", "Plant"), node("t", "tank"), node("s", "spare"), node("f", "filling"), node("e", "startup"), node("c", "keepLevel"), node("u", "Uses"), node("l", "Lib"), node("v", "tankView"), node("k", "Tank"), node("a", "open"), node("b", "pour")],
    edges: [
      { from: "s", to: "t", label: "references", kind: "reference subsetting" },
      { from: "p", to: "f", label: "", kind: "portion" },
      { from: "p", to: "e", label: "event", kind: "event" },
      { from: "p", to: "c", label: "require", kind: "require" },
      { from: "u", to: "l", label: "L", kind: "unowned membership" },
      { from: "v", to: "k", label: "expose", kind: "expose" },
      { from: "a", to: "b", label: "«succession flow»", kind: "succession flow" },
    ],
  };
  const svg = drawCanvas(layoutCanvas(graph));
  const defs = [...svg.querySelectorAll("defs marker")].map((marker) => marker.id);
  assert.deepEqual(defs, ["arrow", "arrow-open", "triangle-hollow", "triangle-dots", "triangle-bar", "diamond-filled", "diamond-hollow", "circle-plus", "triangle-four-dots", "portion", "circle-hollow"]);
  assert.equal(svg.querySelectorAll("#triangle-four-dots circle").length, 4);
  assert.equal(svg.querySelector("#portion path")?.getAttribute("class"), "arrow-fill");
  assert.equal(svg.querySelector("#circle-hollow circle")?.getAttribute("class"), "arrow-hollow");
  const lines = [...svg.querySelectorAll<SVGPolylineElement>("g.opensysml-edge polyline")];
  assert.deepEqual(lines.map((line) => [line.getAttribute("marker-start"), line.getAttribute("marker-end")]), [
    [null, "url(#triangle-four-dots)"],
    ["url(#portion)", null],
    [null, "url(#arrow-open)"],
    [null, "url(#arrow-open)"],
    ["url(#circle-hollow)", null],
    [null, "url(#arrow-open)"],
    [null, "url(#arrow)"],
  ]);
  // A kind of more than one word is classed with hyphens, so a style sheet can address it.
  const classes = [...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")].map((group) => group.getAttribute("class"));
  assert.deepEqual(classes.slice(0, 2), ["opensysml-edge reference-subsetting", "opensysml-edge portion"]);
  assert.equal(classes[6], "opensysml-edge succession-flow");
});

test("drawCanvas ends every edge kind in the marker the notation table states", () => {
  // The table in docs/project/view-rendering-forms.md is the oracle the Go
  // writers are held to; its canvas column names the marker (`end` or `start`)
  // or `none`, and this reads the same rows so the two cannot drift.
  const document = readFileSync("../../docs/project/view-rendering-forms.md", "utf8");
  const section = document.split("\n### Notation table\n")[1]?.split("\n### ")[0] ?? "";
  const rows = section.split("\n").filter((line) => line.startsWith("| `") || line.startsWith("| objective"));
  assert.ok(rows.length > 0, "the notation table has rows");
  const kinds: string[] = [];
  const want: Record<string, [string | null, string | null]> = {};
  for (const row of rows) {
    const cells = row.replace(/\\\|/g, "\u0000").split("|").map((cell) => cell.trim().replace(/\u0000/g, "|"));
    const canvas = cells[7].replace(/`/g, "");
    for (const kind of [...cells[2].matchAll(/`([^`]*)`/g)].map((m) => m[1])) {
      kinds.push(kind);
      if (canvas === "none") {
        want[kind] = [null, null];
      } else {
        const [position, marker] = canvas.split(" ");
        want[kind] = position === "start" ? [`url(#${marker})`, null] : [null, `url(#${marker})`];
      }
    }
  }
  const graph: RenderResult = {
    ...result,
    kind: "tree",
    nodes: [node("a", "A"), node("b", "B")],
    edges: kinds.map((kind) => ({ from: "a", to: "b", label: "", kind })),
  };
  const svg = drawCanvas(layoutCanvas(graph));
  const lines = [...svg.querySelectorAll<SVGPolylineElement>("g.opensysml-edge polyline")];
  assert.equal(lines.length, kinds.length);
  const markers = new Set([...svg.querySelectorAll("defs marker")].map((marker) => `url(#${marker.id})`));
  kinds.forEach((kind, i) => {
    const got: [string | null, string | null] = [lines[i].getAttribute("marker-start"), lines[i].getAttribute("marker-end")];
    assert.deepEqual(got, want[kind], `${kind} edge markers`);
    for (const marker of got) {
      if (marker !== null) assert.ok(markers.has(marker), `${kind} ends in a marker the defs declare`);
    }
  });
});

// Without libavoid the canvas draws an edge straight; a membership edge to a member drawn
// inside its owner still runs from the owner's header line, inside the owner, with its
// keyword off every header.
test("layoutCanvas draws a membership edge from the owner's header line inside the owner when no router is loaded", () => {
  const owner: RenderResult = {
    ...result,
    kind: "mixed",
    nodes: [
      node("pump", "Pump", { kind: "part def" }),
      node("pressure", "pressure", { kind: "attribute", parent: "pump", type: "Pressure" }),
      node("impeller", "impeller", { parent: "pump", type: "Impeller" }),
      node("prime", "prime", { kind: "action", parent: "pump" }),
    ],
    edges: [
      { from: "pump", to: "pressure", label: "require", kind: "require", fqn: "M::pressure" },
      { from: "pump", to: "prime", label: "", kind: "composition", fqn: "M::prime" },
    ],
  };
  const layout = layoutCanvas(owner);
  const pump = layout.nodes.get("pump")!;
  const headerLine = pump.box.y + glyphSize(pump.lines.join("\n")).height;
  const inside = (box: { x: number; y: number; width: number; height: number }, { x, y }: { x: number; y: number }) =>
    x >= box.x && x <= box.x + box.width && y >= box.y && y <= box.y + box.height;
  for (const edge of layout.edges) {
    assert.equal(edge.rerouted, false);
    assert.equal(edge.points.length, 2, "drawn straight");
    assert.ok(edge.points.every((point) => inside(pump.box, point)), `${edge.edge.to} edge stays inside Pump`);
    assert.ok(edge.points[0].y >= headerLine, `${edge.edge.to} edge starts at or below Pump's header line, not on its outer border`);
  }
  const label = edgeLabelBox(layout.edges[0])!;
  for (const entry of layout.nodes.values()) {
    const header = entry.children.length === 0
      ? entry.box
      : { ...entry.box, height: glyphSize(entry.lines.join("\n")).height };
    assert.ok(
      !(label.x < header.x + header.width && header.x < label.x + label.width && label.y < header.y + header.height && header.y < label.y + label.height),
      `the require label covers ${entry.node.name}`,
    );
  }
});
