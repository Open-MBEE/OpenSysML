import assert from "node:assert/strict";
import path from "node:path";
import { test } from "node:test";

import type { RenderEdge, RenderNode, RenderPoint, RenderResult } from "../protocol";
import type { AutoLayout } from "./autolayout";
import {
  anchor,
  clampNodeToBounds,
  GAP,
  freePlacement,
  insertedWaypoint,
  labelLines,
  layoutCanvas,
  liftedEdges,
  MARGIN,
  movable,
  movedNode,
  movedWaypoint,
  nodeExtent,
  nodeUnder,
  overridesOf,
  type PlacedNode,
  portBox,
  portCenter,
  portFace,
  portLabelPlacement,
  PORT_SIZE,
  removedWaypoint,
  shapeOf,
  steerable,
  type Box,
} from "./layout";
import { CLEARANCE, loadAvoid } from "./avoid";

const WASM = path.resolve("node_modules/libavoid-js/dist/libavoid.wasm");

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };

function node(id: string, name: string, extra: Partial<RenderNode> = {}): RenderNode {
  return { id, kind: "part", name, type: "", detail: "", fqn: `M::${name}`, origin, ...extra };
}

function rendering(nodes: RenderNode[], edges: RenderEdge[] = [], extra: Partial<RenderResult> = {}): RenderResult {
  return {
    view: "M::V",
    kind: "interconnection",
    stated: "",
    form: "mermaid",
    artifact: "",
    nodes,
    edges,
    notices: [],
    version: 7,
    ...extra,
  };
}

function placedNode(
  id: string,
  x: number,
  y: number,
  width = 80,
  height = 40,
  ports: RenderNode["ports"] = [],
) {
  return layoutCanvas(rendering([node(id, id, { x, y, width, height, ports })])).nodes.get(id)!;
}

function freeAt(node: PlacedNode, at: RenderPoint, others: PlacedNode[]): boolean {
  const extent = nodeExtent({ ...node, box: { ...node.box, x: at.x, y: at.y } });
  const expanded = {
    x: extent.x - CLEARANCE,
    y: extent.y - CLEARANCE,
    width: extent.width + 2 * CLEARANCE,
    height: extent.height + 2 * CLEARANCE,
  };
  return others.every((other) => {
    const obstacle = nodeExtent(other);
    return (
      expanded.x >= obstacle.x + obstacle.width ||
      expanded.x + expanded.width <= obstacle.x ||
      expanded.y >= obstacle.y + obstacle.height ||
      expanded.y + expanded.height <= obstacle.y
    );
  });
}

function nearestFreeByBruteForce(
  node: PlacedNode,
  at: RenderPoint,
  others: PlacedNode[],
  bounds: Box,
): RenderPoint | undefined {
  const min = clampNodeToBounds(
    node,
    { x: Number.NEGATIVE_INFINITY, y: Number.NEGATIVE_INFINITY },
    bounds,
  );
  const max = clampNodeToBounds(
    node,
    { x: Number.POSITIVE_INFINITY, y: Number.POSITIVE_INFINITY },
    bounds,
  );
  let nearest: RenderPoint | undefined;
  let nearestDistance = Number.POSITIVE_INFINITY;
  for (let x = min.x; x <= max.x; x++) {
    for (let y = min.y; y <= max.y; y++) {
      const candidate = { x, y };
      const distance = Math.hypot(candidate.x - at.x, candidate.y - at.y);
      if (distance < nearestDistance && freeAt(node, candidate, others)) {
        nearest = candidate;
        nearestDistance = distance;
      }
    }
  }
  return nearest;
}

test("layoutCanvas draws a pinned node's edge straight until the router has loaded", () => {
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 400, y: 200, width: 100, height: 40 }),
      node("c", "c", { x: 180, y: -20, width: 60, height: 80 }),
    ],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  ));
  assert.equal(layout.edges[0].rerouted, false);
  assert.equal(layout.edges[0].points.length, 2);
});

test("layoutCanvas places unplaced roots in a near-square grid, in order, from the margin", () => {
  const layout = layoutCanvas(rendering([node("a", "a"), node("b", "b"), node("c", "c"), node("d", "d"), node("e", "e")]));
  const boxes = ["a", "b", "c", "d", "e"].map((id) => layout.nodes.get(id)!.box);
  // Five nodes take three columns; every unplaced label box is the minimum width
  // and two lines (name, «part») high.
  assert.deepEqual(boxes.map((box) => [box.x, box.y]), [
    [MARGIN, MARGIN],
    [MARGIN + 96 + GAP, MARGIN],
    [MARGIN + 2 * (96 + GAP), MARGIN],
    [MARGIN, MARGIN + 52 + GAP],
    [MARGIN + 96 + GAP, MARGIN + 52 + GAP],
  ]);
  assert.ok(boxes.every((box) => box.width === 96 && box.height === 52));
  assert.ok(boxes.every((_, i) => !layout.nodes.get(["a", "b", "c", "d", "e"][i])!.pinned));
  assert.equal(layout.width, MARGIN + 3 * 96 + 2 * GAP + MARGIN);
  assert.equal(layout.height, MARGIN + 2 * 52 + GAP + MARGIN);
  assert.equal(layout.placeable, true);
});

test("layoutCanvas is deterministic: the same rendering lays out the same twice", () => {
  const result = rendering([node("a", "a"), node("b", "b", { parent: "a" }), node("c", "c", { parent: "a" })], [
    { from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc" },
  ]);
  const first = layoutCanvas(result);
  const second = layoutCanvas(result);
  assert.deepEqual(
    [...first.nodes.values()].map((entry) => entry.box),
    [...second.nodes.values()].map((entry) => entry.box),
  );
  assert.deepEqual(first.edges.map((edge) => edge.points), second.edges.map((edge) => edge.points));
});

test("layoutCanvas keeps the model's geometry exactly and marks the node pinned", () => {
  const layout = layoutCanvas(
    rendering([node("a", "a", { x: 300.5, y: 12, width: 150, height: 70 }), node("b", "b")]),
  );
  const a = layout.nodes.get("a")!;
  assert.deepEqual(a.box, { x: 300.5, y: 12, width: 150, height: 70 });
  assert.equal(a.pinned, true);
  // The pinned node keeps its grid slot, so its sibling stays where it would be anyway.
  assert.deepEqual([layout.nodes.get("b")!.box.x, layout.nodes.get("b")!.box.y], [MARGIN + 150 + GAP, MARGIN]);
  // The canvas grows to hold what the model placed.
  assert.equal(layout.width, 300.5 + 150 + MARGIN);
});

test("layoutCanvas nests children inside their owner and sizes the owner around them", () => {
  const layout = layoutCanvas(rendering([node("a", "a"), node("b", "b", { parent: "a" }), node("c", "c", { parent: "a" })]));
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  const c = layout.nodes.get("c")!;
  assert.deepEqual(layout.roots.map((root) => root.node.id), ["a"]);
  assert.equal(b.parent, a);
  // Children start under the owner's label, inside its padding.
  assert.deepEqual([b.box.x, b.box.y], [a.box.x + 16, a.box.y + 52]);
  assert.deepEqual([c.box.x, c.box.y], [b.box.x + 96 + GAP, b.box.y]);
  assert.ok(a.box.x + a.box.width >= c.box.x + c.box.width + 16);
  assert.ok(a.box.y + a.box.height >= c.box.y + c.box.height + 16);
});

test("layoutCanvas grows an unsized owner to hold a child the model put beyond it", () => {
  const layout = layoutCanvas(rendering([node("a", "a"), node("b", "b", { parent: "a", x: 500, y: 400 })]));
  const a = layout.nodes.get("a")!;
  assert.ok(a.box.x + a.box.width >= 500 + 96 + 16);
  assert.ok(a.box.y + a.box.height >= 400 + 40 + 16);
});

test("layoutCanvas extends the canvas to a placed child beyond its sized owner", () => {
  const layout = layoutCanvas(rendering([
    node("a", "a", { x: 0, y: 0, width: 100, height: 100 }),
    node("b", "b", { parent: "a", x: 500, y: 400 }),
  ]));
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  assert.deepEqual(a.box, { x: 0, y: 0, width: 100, height: 100 });
  assert.deepEqual([b.box.x, b.box.y], [500, 400]);
  assert.equal(layout.width, 500 + b.box.width + MARGIN);
  assert.equal(layout.height, 400 + b.box.height + MARGIN);
  assert.deepEqual(layout.origin, { x: 0, y: 0 });
});

test("layoutCanvas moves the canvas's corner up and left to geometry the model puts before the origin", () => {
  const layout = layoutCanvas(rendering([
    node("a", "a", { x: -120, y: 40, width: 100, height: 50 }),
    node("b", "b", { x: 200, y: -30, width: 100, height: 50 }),
  ]));
  assert.deepEqual(layout.nodes.get("a")!.box, { x: -120, y: 40, width: 100, height: 50 });
  assert.deepEqual(layout.origin, { x: -120 - MARGIN, y: -30 - MARGIN });
  assert.equal(layout.origin.x + layout.width, 300 + MARGIN);
  assert.equal(layout.origin.y + layout.height, 90 + MARGIN);
});

test("layoutCanvas moves a root's siblings past an owner grown around a placed child", () => {
  const layout = layoutCanvas(rendering([
    node("a", "a"),
    node("b", "b", { parent: "a", x: 500, y: 400 }),
    node("c", "c"),
  ]));
  const a = layout.nodes.get("a")!;
  const c = layout.nodes.get("c")!;
  assert.equal(a.box.x + a.box.width, 500 + 96 + 16);
  assert.equal(c.box.x, a.box.x + a.box.width + GAP);
  assert.equal(c.pinned, false);
});

test("layoutCanvas moves nested siblings past a container grown around a placed grandchild", () => {
  const layout = layoutCanvas(rendering([
    node("root", "root"),
    node("a", "a", { parent: "root" }),
    node("b", "b", { parent: "a", x: 300, y: 300 }),
    node("c", "c", { parent: "root" }),
    node("d", "d", { parent: "root" }),
    node("e", "e", { parent: "root" }),
  ]));
  const root = layout.nodes.get("root")!;
  const a = layout.nodes.get("a")!;
  const c = layout.nodes.get("c")!;
  const d = layout.nodes.get("d")!;
  assert.deepEqual([a.box.x + a.box.width, a.box.y + a.box.height], [300 + 96 + 16, 300 + 52 + 16]);
  // Four children take two columns and two rows: a's column is as wide as a, its row as tall.
  assert.equal(c.box.x, a.box.x + a.box.width + GAP);
  assert.equal(d.box.y, a.box.y + a.box.height + GAP);
  assert.ok(root.box.x + root.box.width >= c.box.x + c.box.width + 16);
  assert.ok(root.box.y + root.box.height >= d.box.y + d.box.height + 16);
});

test("layoutCanvas hides the children of a collapsed node", () => {
  const layout = layoutCanvas(rendering(
    [node("a", "a", { x: 10, y: 10, collapsed: true }), node("b", "b", { parent: "a" }), node("c", "c", { parent: "b" }), node("d", "d", { x: 200, y: 5 })],
    [
      { from: "b", to: "c", label: "", kind: "connection", route: [{ x: 900, y: 900 }] },
      { from: "c", to: "d", label: "", kind: "connection" },
      { from: "a", to: "d", label: "", kind: "connection" },
    ],
  ));
  const a = layout.nodes.get("a")!;
  assert.deepEqual(a.box, { x: 10, y: 10, width: 96, height: 52 });
  assert.equal(a.collapsed, true);
  assert.deepEqual(["b", "c"].map((id) => layout.nodes.get(id)!.hidden), [true, true]);
  assert.equal(layout.nodes.get("d")!.hidden, false);
  // An edge at a hidden node is hidden with it, its route not counted toward the canvas.
  assert.deepEqual(layout.edges.map((edge) => edge.hidden), [true, true, false]);
  assert.equal(layout.height, 10 + 52 + MARGIN);
});

test("layoutCanvas honours the view's canvas size as a minimum", () => {
  const layout = layoutCanvas(rendering([node("a", "a")], [], { canvas: { unit: "px", width: 800, height: 600 } }));
  assert.deepEqual([layout.width, layout.height], [800, 600]);
});

test("layoutCanvas routes an edge from border to border through its waypoints", () => {
  const edge: RenderEdge = { from: "a", to: "b", label: "wire", kind: "connection", fqn: "M::ab", route: [{ x: 200, y: 200 }] };
  const layout = layoutCanvas(rendering([node("a", "a", { x: 0, y: 0, width: 100, height: 40 }), node("b", "b", { x: 300, y: 300, width: 100, height: 40 })], [edge]));
  const [placed] = layout.edges;
  assert.equal(placed.index, 0);
  assert.deepEqual(placed.route, [{ x: 200, y: 200 }]);
  assert.equal(placed.points.length, 3);
  assert.deepEqual(placed.points[1], { x: 200, y: 200 });
  // The anchors sit on the boxes' borders, on the line toward the waypoint.
  assert.equal(placed.points[0].y, 40);
  assert.equal(placed.points[2].y, 300);
  assert.equal(steerable(layout, placed), true);
});

test("layoutCanvas swings a self-loop out to the right of its node", () => {
  const layout = layoutCanvas(rendering([node("a", "a", { x: 0, y: 0, width: 100, height: 60 })], [
    { from: "a", to: "a", label: "", kind: "transition" },
  ]));
  const [placed] = layout.edges;
  assert.equal(placed.points.length, 4);
  assert.ok(placed.points.slice(1, 3).every((point) => point.x > 100));
  assert.deepEqual(placed.route, []);
  assert.equal(steerable(layout, placed), false);
});

test("layoutCanvas lays a sequence out as lifelines in a row with messages down them", () => {
  const layout = layoutCanvas(rendering(
    [node("a", "a"), node("b", "b")],
    [
      { from: "a", to: "b", label: "ask", kind: "flow" },
      { from: "b", to: "a", label: "answer", kind: "flow" },
    ],
    { kind: "sequence" },
  ));
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  assert.equal(a.box.y, b.box.y);
  assert.ok(b.box.x > a.box.x + a.box.width);
  assert.ok(a.lifeline !== undefined && a.lifeline > a.box.y + a.box.height);
  const [ask, answer] = layout.edges;
  assert.equal(ask.points[0].y, ask.points[1].y);
  assert.ok(answer.points[0].y > ask.points[0].y);
  assert.equal(ask.points[0].x, a.box.x + a.box.width / 2);
  assert.equal(ask.points[1].x, b.box.x + b.box.width / 2);
  assert.equal(layout.placeable, false);
});

test("labelLines and shapeOf follow the graphical notation", () => {
  assert.deepEqual(labelLines(node("a", "tank", { type: "Tank", detail: "[2]" })), ["tank : Tank", "«part»", "[2]"]);
  assert.deepEqual(labelLines(node("a", "", { kind: "region" })), ["region"]);
  assert.deepEqual(labelLines(node("a", "", { kind: "fork" })), []);
  assert.equal(shapeOf("part def"), "box");
  assert.equal(shapeOf("initial"), "circle");
  assert.equal(shapeOf("final"), "ring");
  assert.equal(shapeOf("fork"), "bar");
  assert.equal(shapeOf("decision"), "diamond");
  assert.equal(shapeOf("deep history"), "history");
  assert.equal(shapeOf("start"), "point");
});

test("anchor leaves a box on the border toward the point", () => {
  const box = { x: 0, y: 0, width: 100, height: 50 };
  assert.deepEqual(anchor(box, { x: 200, y: 25 }), { x: 100, y: 25 });
  assert.deepEqual(anchor(box, { x: 50, y: -100 }), { x: 50, y: 0 });
  assert.deepEqual(anchor(box, { x: 50, y: 25 }), { x: 50, y: 25 });
});

test("movable and steerable need the placeable kind and a declared target", () => {
  const declared = node("a", "a");
  const imported = node("b", "b", { fqn: undefined });
  const placeable = layoutCanvas(rendering([declared, imported], [
    { from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" },
    { from: "b", to: "a", label: "", kind: "connection" },
  ]));
  assert.equal(movable(placeable, placeable.nodes.get("a")!), true);
  assert.equal(movable(placeable, placeable.nodes.get("b")!), false);
  assert.equal(steerable(placeable, placeable.edges[0]), true);
  assert.equal(steerable(placeable, placeable.edges[1]), false);
  const table = layoutCanvas(rendering([declared], [], { kind: "table" }));
  assert.equal(movable(table, table.nodes.get("a")!), false);
});

test("movable and steerable take a target another document declares, named or not, and no library's", () => {
  const foreignOrigin = { ...origin, uri: "file:///work/parts.sysml" };
  const declaration = { start: { line: 2, character: 4 }, end: { line: 2, character: 30 } };
  const library = layoutCanvas(rendering([
    node("a", "a", { declaredHere: true }),
    node("b", "b", { origin: foreignOrigin }),
    node("c", "", { fqn: undefined, declaration, origin: foreignOrigin }),
    node("d", "d", { fqn: undefined, origin: { ...origin, uri: "sysml-stdlib:///Systems%20Library/Parts.sysml" } }),
  ], [
    { from: "a", to: "b", label: "", kind: "connection", fqn: "Parts::ab", origin: foreignOrigin },
    { from: "a", to: "d", label: "", kind: "connection", origin: foreignOrigin },
  ]));
  assert.equal(movable(library, library.nodes.get("b")!), true);
  assert.equal(movable(library, library.nodes.get("c")!), true);
  assert.equal(movable(library, library.nodes.get("d")!), false);
  assert.equal(steerable(library, library.edges[0]), true);
  assert.equal(steerable(library, library.edges[1]), false);
});

test("movable and steerable accept a target reached by its declaration alone", () => {
  const declaration = { start: { line: 2, character: 4 }, end: { line: 2, character: 30 } };
  const unnamed = layoutCanvas(rendering([node("a", "a"), node("b", "", { fqn: undefined, declaration })], [
    { from: "a", to: "b", label: "", kind: "transition", declaration },
  ]));
  assert.equal(movable(unnamed, unnamed.nodes.get("b")!), true);
  assert.equal(steerable(unnamed, unnamed.edges[0]), true);
});

test("nodeUnder is the innermost drawn node holding the point, the later sibling of two that overlap", () => {
  const layout = layoutCanvas(rendering([
    node("a", "a", { x: 0, y: 0, width: 300, height: 200 }),
    node("b", "b", { parent: "a", x: 20, y: 60, width: 100, height: 50 }),
    node("c", "c", { x: 250, y: 100, width: 100, height: 50 }),
    node("d", "d", { x: 600, y: 600, width: 100, height: 50 }),
  ]));
  assert.equal(nodeUnder(layout, { x: 10, y: 10 })?.node.id, "a");
  assert.equal(nodeUnder(layout, { x: 50, y: 80 })?.node.id, "b");
  // Where a's and c's boxes overlap, c is drawn later, on top; a border counts as inside.
  assert.equal(nodeUnder(layout, { x: 280, y: 120 })?.node.id, "c");
  assert.equal(nodeUnder(layout, { x: 700, y: 650 })?.node.id, "d");
  assert.equal(nodeUnder(layout, { x: 500, y: 500 }), undefined);
});

test("nodeUnder passes over the dragged subtree and the children a collapsed node hides", () => {
  const layout = layoutCanvas(rendering([
    node("a", "a", { x: 0, y: 0, width: 300, height: 200 }),
    node("b", "b", { parent: "a", x: 20, y: 60, width: 100, height: 50 }),
    node("e", "e", { parent: "b", x: 30, y: 80, width: 40, height: 20 }),
    node("c", "c", { x: 400, y: 0, width: 200, height: 200, collapsed: true }),
    node("f", "f", { parent: "c", x: 420, y: 50, width: 40, height: 20 }),
  ]));
  // The dragged node b is held over its own place: what is under the pointer is its owner.
  assert.equal(nodeUnder(layout, { x: 40, y: 85 }, "b")?.node.id, "a");
  assert.equal(nodeUnder(layout, { x: 40, y: 85 })?.node.id, "e");
  assert.equal(nodeUnder(layout, { x: 430, y: 60 })?.node.id, "c");
});

test("nodeUnder reads the layout it is given, so a node placed aside no longer covers its old point", () => {
  const result = rendering([node("a", "a", { x: 0, y: 0, width: 100, height: 50 }), node("b", "b", { x: 200, y: 0, width: 100, height: 50 })]);
  const shown = layoutCanvas(result, overridesOf(movedNode(layoutCanvas(result), "a", 200, 0)!));
  assert.equal(nodeUnder(shown, { x: 50, y: 25 }), undefined);
  assert.equal(nodeUnder(shown, { x: 250, y: 25 }, "a")?.node.id, "b");
});

test("movedNode writes the dragged node's new position, snapped, and nothing else about it", () => {
  const layout = layoutCanvas(rendering([node("a", "a"), node("b", "b")]));
  const placements = movedNode(layout, "a", 10.4, -3.6)!;
  assert.deepEqual(placements, {
    nodes: [{ id: "a", layout: { x: MARGIN + 10, y: MARGIN - 4 } }],
    edges: [],
  });
});

test("movedNode keeps a sized, collapsed node's size and collapse", () => {
  const layout = layoutCanvas(rendering([node("a", "a", { x: 100, y: 100, width: 200, height: 80, collapsed: true })]));
  assert.deepEqual(movedNode(layout, "a", 5, 5)!.nodes, [
    { id: "a", layout: { x: 105, y: 105, width: 200, height: 80, collapsed: true } },
  ]);
});

test("movedNode carries the placed descendants and inner routes along, leaving unplaced ones to follow", () => {
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0 }),
      node("b", "b", { parent: "a", x: 20, y: 60 }),
      node("c", "c", { parent: "a" }),
      node("d", "d"),
    ],
    [
      { from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc", route: [{ x: 50, y: 50 }] },
      { from: "b", to: "d", label: "", kind: "connection", fqn: "M::bd", route: [{ x: 70, y: 70 }] },
    ],
  ));
  const placements = movedNode(layout, "a", 100, 0)!;
  assert.deepEqual(placements.nodes, [
    { id: "a", layout: { x: 100, y: 0 } },
    { id: "b", layout: { x: 120, y: 60 } },
  ]);
  // Only the route between two nodes of the moved subtree moves with it.
  assert.deepEqual(placements.edges, [{ index: 0, route: [{ x: 150, y: 50 }] }]);
});

test("liftedEdges moves an edge within the lifted subtree whole and keeps a crossing edge's waypoints, re-anchored at its lifted end", () => {
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0 }),
      node("b", "b", { parent: "a", x: 20, y: 60 }),
      node("c", "c", { parent: "a" }),
      node("d", "d"),
    ],
    [
      { from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc", route: [{ x: 50, y: 50 }] },
      { from: "b", to: "d", label: "", kind: "connection", fqn: "M::bd", route: [{ x: 70, y: 70 }] },
      { from: "d", to: "d", label: "", kind: "connection", fqn: "M::dd" },
    ],
  ));
  const lifted = liftedEdges(layout, "a", 100, 30);
  // The edge between two nodes outside the subtree is not touched.
  assert.deepEqual(lifted.map((edge) => edge.index), [0, 1]);
  const [inner, crossing] = lifted;
  const shift = (points: { x: number; y: number }[]) => points.map((p) => ({ x: p.x + 100, y: p.y + 30 }));
  assert.deepEqual(inner.points, shift(layout.edges[0].points));
  assert.deepEqual(inner.route, [{ x: 150, y: 80 }]);
  assert.deepEqual(inner.label, { x: layout.edges[0].label.x + 100, y: layout.edges[0].label.y + 30 });
  // The crossing edge keeps the model's waypoint and its anchor on d; its anchor on b moves with b.
  assert.deepEqual(crossing.route, [{ x: 70, y: 70 }]);
  assert.deepEqual(crossing.points.at(-1), layout.edges[1].points.at(-1));
  const b = layout.nodes.get("b")!.box;
  assert.deepEqual(crossing.points[0], anchor({ ...b, x: b.x + 100, y: b.y + 30 }, { x: 70, y: 70 }));
  // A node the layout does not hold lifts no edge.
  assert.deepEqual(liftedEdges(layout, "z", 1, 1), []);
});

test("movedNode refuses a node no Layout can name", () => {
  const layout = layoutCanvas(rendering([node("a", "a", { fqn: undefined })]));
  assert.equal(movedNode(layout, "a", 1, 1), undefined);
  assert.equal(movedNode(layout, "missing", 1, 1), undefined);
});

test("movedWaypoint, insertedWaypoint and removedWaypoint edit one edge's route", () => {
  const edge: RenderEdge = { from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab", route: [{ x: 10, y: 10 }, { x: 20, y: 20 }] };
  const layout = layoutCanvas(rendering([node("a", "a"), node("b", "b")], [edge]));
  assert.deepEqual(movedWaypoint(layout, 0, 1, { x: 30.6, y: 40.2 }), {
    nodes: [],
    edges: [{ index: 0, route: [{ x: 10, y: 10 }, { x: 31, y: 40 }] }],
  });
  assert.equal(movedWaypoint(layout, 0, 2, { x: 0, y: 0 }), undefined);
  // Segment 1 runs from the first waypoint to the second; the new one goes between them.
  assert.deepEqual(insertedWaypoint(layout, 0, 1, { x: 15, y: 15 })!.edges, [
    { index: 0, route: [{ x: 10, y: 10 }, { x: 15, y: 15 }, { x: 20, y: 20 }] },
  ]);
  assert.deepEqual(insertedWaypoint(layout, 0, 0, { x: 5, y: 5 })!.edges, [
    { index: 0, route: [{ x: 5, y: 5 }, { x: 10, y: 10 }, { x: 20, y: 20 }] },
  ]);
  assert.equal(insertedWaypoint(layout, 0, 3, { x: 5, y: 5 }), undefined);
  assert.deepEqual(removedWaypoint(layout, 0, 0)!.edges, [{ index: 0, route: [{ x: 20, y: 20 }] }]);
  const single = layoutCanvas(rendering([node("a", "a"), node("b", "b")], [{ ...edge, route: [{ x: 10, y: 10 }] }]));
  // Removing the last waypoint clears the route rather than leaving an empty one.
  assert.deepEqual(removedWaypoint(single, 0, 0)!.edges, [{ index: 0, route: undefined }]);
});

test("overridesOf previews a gesture: the moved node and route show where the drag has them", () => {
  const edge: RenderEdge = { from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab", route: [{ x: 10, y: 10 }] };
  const result = rendering([node("a", "a"), node("b", "b")], [edge]);
  const layout = layoutCanvas(result);
  const preview = layoutCanvas(result, overridesOf({
    nodes: [{ id: "a", layout: { x: 400, y: 300 } }],
    edges: [{ index: 0, route: undefined }],
  }));
  assert.deepEqual([preview.nodes.get("a")!.box.x, preview.nodes.get("a")!.box.y], [400, 300]);
  assert.equal(preview.nodes.get("a")!.pinned, true);
  assert.deepEqual(preview.nodes.get("b")!.box, layout.nodes.get("b")!.box);
  assert.deepEqual(preview.edges[0].route, []);
  assert.equal(preview.edges[0].points.length, 2);
});

test("layoutCanvas takes an auto layout's geometry for nodes the model does not place", async () => {
  const auto: AutoLayout = {
    nodes: new Map([
      ["a", { x: 100, y: 50, width: 140, height: 60 }],
      ["b", { x: 300, y: 200, width: 140, height: 60 }],
      ["c", { x: 500, y: 50, width: 140, height: 60 }],
    ]),
    routes: new Map([[0, [{ x: 240, y: 80 }, { x: 300, y: 80 }, { x: 300, y: 200 }, { x: 300, y: 230 }]]]),
    ports: new Map(),
  };
  const result = rendering(
    [node("a", "a"), node("b", "b"), node("c", "c", { x: 50, y: 400, width: 90, height: 50 })],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }, { from: "a", to: "c", label: "", kind: "connection", fqn: "M::ac" }],
  );
  await loadAvoid(WASM);
  const layout = layoutCanvas(result, {}, auto);
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  const c = layout.nodes.get("c")!;
  // The auto geometry is honored exactly, but does not pin the node.
  assert.deepEqual(a.box, { x: 100, y: 50, width: 140, height: 60 });
  assert.equal(a.pinned, false);
  assert.equal(b.pinned, false);
  // The model's geometry wins over the auto layout and stays pinned.
  assert.deepEqual(c.box, { x: 50, y: 400, width: 90, height: 50 });
  assert.equal(c.pinned, true);
  // An edge between two auto-placed nodes follows the auto route verbatim: its
  // anchors first and last, its inner points the edge's route.
  assert.deepEqual(layout.edges[0].points, auto.routes.get(0));
  assert.deepEqual(layout.edges[0].route, auto.routes.get(0)!.slice(1, -1));
  assert.equal(layout.edges[0].rerouted, false);
  // An edge at a node the model places is routed orthogonally around the other
  // boxes, the auto route void at it; its waypoints are the panel's, not the model's.
  assert.equal(layout.edges[1].rerouted, true);
  assert.deepEqual(layout.edges[1].route, layout.edges[1].points.slice(1, -1));
  for (let i = 1; i < layout.edges[1].points.length; i++) {
    const [from, to] = [layout.edges[1].points[i - 1], layout.edges[1].points[i]];
    assert.ok(from.x === to.x || from.y === to.y);
  }
  // A gesture wins over both; held by the layout it started from, the pinned
  // end's edge stays straight for the drag and is routed again on the drop.
  const overrides = { ...overridesOf({ nodes: [{ id: "a", layout: { x: 10, y: 10 } }], edges: [] }), held: layout };
  const preview = layoutCanvas(result, overrides, auto);
  assert.deepEqual([preview.nodes.get("a")!.box.x, preview.nodes.get("a")!.box.y], [10, 10]);
  assert.equal(preview.nodes.get("a")!.pinned, true);
  assert.equal(preview.edges[0].points.length, 2);
  assert.equal(preview.edges[0].rerouted, false);
});

function onBorder(point: { x: number; y: number }, box: { x: number; y: number; width: number; height: number }): boolean {
  const epsilon = 1e-7;
  const onVertical =
    (Math.abs(point.x - box.x) < epsilon || Math.abs(point.x - (box.x + box.width)) < epsilon) &&
    point.y >= box.y - epsilon &&
    point.y <= box.y + box.height + epsilon;
  const onHorizontal =
    (Math.abs(point.y - box.y) < epsilon || Math.abs(point.y - (box.y + box.height)) < epsilon) &&
    point.x >= box.x - epsilon &&
    point.x <= box.x + box.width + epsilon;
  return onVertical || onHorizontal;
}

function crossesInterior(
  a: { x: number; y: number },
  b: { x: number; y: number },
  box: { x: number; y: number; width: number; height: number },
): boolean {
  if (a.y === b.y) {
    return (
      a.y > box.y &&
      a.y < box.y + box.height &&
      Math.max(Math.min(a.x, b.x), box.x) < Math.min(Math.max(a.x, b.x), box.x + box.width)
    );
  }
  return (
    a.x > box.x &&
    a.x < box.x + box.width &&
    Math.max(Math.min(a.y, b.y), box.y) < Math.min(Math.max(a.y, b.y), box.y + box.height)
  );
}

// lengthMidpoint is the point halfway along a polyline's length.
function lengthMidpoint(points: { x: number; y: number }[]): { x: number; y: number } {
  let total = 0;
  for (let i = 1; i < points.length; i++) {
    total += Math.hypot(points[i].x - points[i - 1].x, points[i].y - points[i - 1].y);
  }
  let remaining = total / 2;
  for (let i = 1; i < points.length; i++) {
    const length = Math.hypot(points[i].x - points[i - 1].x, points[i].y - points[i - 1].y);
    if (remaining <= length || i === points.length - 1) {
      const t = length === 0 ? 0 : remaining / length;
      return { x: points[i - 1].x + (points[i].x - points[i - 1].x) * t, y: points[i - 1].y + (points[i].y - points[i - 1].y) * t };
    }
    remaining -= length;
  }
  return points[0] ?? { x: 0, y: 0 };
}

test("layoutCanvas routes a pinned node's edge orthogonally around the box between its ends", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 400, y: 200, width: 100, height: 40 }),
      node("c", "c", { x: 180, y: -20, width: 60, height: 80 }),
    ],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  ));
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  assert.ok(edge.points.length > 2);
  assert.deepEqual(edge.route, edge.points.slice(1, -1));
  assert.ok(onBorder(edge.points[0], layout.nodes.get("a")!.box));
  assert.ok(onBorder(edge.points.at(-1)!, layout.nodes.get("b")!.box));
  const c = layout.nodes.get("c")!.box;
  const inflated = { x: c.x - CLEARANCE, y: c.y - CLEARANCE, width: c.width + 2 * CLEARANCE, height: c.height + 2 * CLEARANCE };
  for (let i = 1; i < edge.points.length; i++) {
    const [from, to] = [edge.points[i - 1], edge.points[i]];
    assert.ok(from.x === to.x || from.y === to.y, `non-orthogonal segment: ${JSON.stringify([from, to])}`);
    assert.ok(!crossesInterior(from, to, inflated), `segment crosses c: ${JSON.stringify([from, to])}`);
  }
  assert.deepEqual(edge.label, lengthMidpoint(edge.points));
});

test("layoutCanvas leaves a stated route and an overridden straight edge alone", () => {
  const edge: RenderEdge = { from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab", route: [{ x: 200, y: 200 }] };
  const result = rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 400, y: 0, width: 100, height: 40 }),
      node("c", "c", { x: 200, y: -20, width: 60, height: 80 }),
    ],
    [edge],
  );
  const [placed] = layoutCanvas(result).edges;
  assert.equal(placed.rerouted, false);
  assert.deepEqual(placed.route, [{ x: 200, y: 200 }]);
  assert.equal(placed.points.length, 3);
  assert.deepEqual(placed.points[1], { x: 200, y: 200 });
  // An overrides entry of undefined draws the edge straight rather than re-routing it.
  const straight = layoutCanvas(result, { routes: new Map([[0, undefined]]) }).edges[0];
  assert.equal(straight.rerouted, false);
  assert.equal(straight.points.length, 2);
  assert.deepEqual(straight.route, []);
});

test("liftedEdges draws a crossing rerouted edge straight and shifts an inner one whole", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0 }),
      node("b", "b", { parent: "a", x: 20, y: 60, width: 100, height: 40 }),
      node("c", "c", { parent: "a", x: 200, y: 200, width: 100, height: 40 }),
      node("d", "d", { x: 700, y: 0, width: 100, height: 40 }),
    ],
    [
      { from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc" },
      { from: "b", to: "d", label: "", kind: "connection", fqn: "M::bd" },
    ],
  ));
  assert.ok(layout.edges.every((edge) => edge.rerouted));
  const lifted = liftedEdges(layout, "a", 50, 0);
  assert.deepEqual(lifted.map((edge) => edge.index), [0, 1]);
  const [inner, crossing] = lifted;
  // The inner edge keeps its rerouted waypoints, shifted with the subtree.
  assert.deepEqual(inner.route, layout.edges[0].route.map((p) => ({ x: p.x + 50, y: p.y })));
  // The crossing edge is drawn straight across the lifted border.
  assert.equal(crossing.points.length, 2);
  assert.deepEqual(crossing.route, []);
});

test("held keeps an unmoved rerouted edge's route and draws a moved end's straight", async () => {
  await loadAvoid(WASM);
  const result = rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 400, y: 200, width: 100, height: 40 }),
      node("c", "c", { x: 0, y: 400, width: 100, height: 40 }),
      node("d", "d", { x: 400, y: 400, width: 100, height: 40 }),
    ],
    [
      { from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" },
      { from: "c", to: "d", label: "", kind: "connection", fqn: "M::cd" },
    ],
  );
  const layout = layoutCanvas(result);
  assert.ok(layout.edges.every((edge) => edge.rerouted));
  const moved = movedNode(layout, "a", 50, 0)!;
  const preview = layoutCanvas(result, { ...overridesOf(moved), held: layout });
  // The edge at the moved node is drawn straight for the drag.
  assert.equal(preview.edges[0].points.length, 2);
  assert.deepEqual(preview.edges[0].route, []);
  // The edge between two unmoved nodes keeps the held layout's route exactly.
  assert.deepEqual(preview.edges[1].points, layout.edges[1].points);
  assert.equal(preview.edges[1].rerouted, true);
});

test("movedNode writes no route for a rerouted edge, and movedWaypoint writes one", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0 }),
      node("b", "b", { parent: "a", x: 20, y: 60, width: 100, height: 40 }),
      node("c", "c", { parent: "a", x: 200, y: 200, width: 100, height: 40 }),
    ],
    [{ from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc" }],
  ));
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  assert.ok(edge.route.length > 0);
  // The panel's route is not the model's: a move does not write it.
  assert.deepEqual(movedNode(layout, "a", 30, 10)!.edges, []);
  // Dragging one of its waypoints states it: the route is written with the point replaced.
  const placements = movedWaypoint(layout, 0, 0, { x: 33.4, y: 44.6 })!;
  assert.deepEqual(placements.edges, [
    { index: 0, route: [{ x: 33, y: 45 }, ...edge.route.slice(1)] },
  ]);
});

// overlapLength is how much two axis-aligned collinear segments share.
function overlapLength(
  a0: { x: number; y: number },
  a1: { x: number; y: number },
  b0: { x: number; y: number },
  b1: { x: number; y: number },
): number {
  if (a0.y === a1.y && b0.y === b1.y && a0.y === b0.y) {
    return Math.max(0, Math.min(Math.max(a0.x, a1.x), Math.max(b0.x, b1.x)) - Math.max(Math.min(a0.x, a1.x), Math.min(b0.x, b1.x)));
  }
  if (a0.x === a1.x && b0.x === b1.x && a0.x === b0.x) {
    return Math.max(0, Math.min(Math.max(a0.y, a1.y), Math.max(b0.y, b1.y)) - Math.max(Math.min(a0.y, a1.y), Math.min(b0.y, b1.y)));
  }
  return 0;
}

test("layoutCanvas separates three edges into one target: no shared collinear segment over a pixel", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 0, y: 200, width: 100, height: 40 }),
      node("c", "c", { x: 0, y: 400, width: 100, height: 40 }),
      node("d", "d", { x: 400, y: 200, width: 100, height: 40 }),
    ],
    [
      { from: "a", to: "d", label: "", kind: "connection", fqn: "M::ad" },
      { from: "b", to: "d", label: "", kind: "connection", fqn: "M::bd" },
      { from: "c", to: "d", label: "", kind: "connection", fqn: "M::cd" },
    ],
  ));
  assert.ok(layout.edges.every((edge) => edge.rerouted));
  for (let first = 0; first < layout.edges.length; first++) {
    for (let second = first + 1; second < layout.edges.length; second++) {
      const a = layout.edges[first].points;
      const b = layout.edges[second].points;
      for (let i = 1; i < a.length; i++) {
        for (let j = 1; j < b.length; j++) {
          assert.ok(
            overlapLength(a[i - 1], a[i], b[j - 1], b[j]) <= 1,
            `edges ${first} and ${second} share a collinear segment`,
          );
        }
      }
    }
  }
});

test("a waypoint drag in progress keeps its route over the held layout's generated one", async () => {
  await loadAvoid(WASM);
  const result = rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 400, y: 200, width: 100, height: 40 }),
      node("c", "c", { x: 180, y: -20, width: 60, height: 80 }),
    ],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  );
  const layout = layoutCanvas(result);
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  // The gesture's route, not the layout it started from, is what the preview shows.
  const preview = layoutCanvas(result, { ...overridesOf(movedWaypoint(layout, 0, 0, { x: 33.4, y: 44.6 })!), held: layout });
  assert.equal(preview.edges[0].rerouted, false);
  assert.deepEqual(preview.edges[0].route, [{ x: 33, y: 45 }, ...edge.route.slice(1)]);
});

test("layoutCanvas routes a pinned node's edge around a populated container between its ends", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 600, y: 0, width: 100, height: 40 }),
      node("c", "c", { x: 220, y: -40, width: 240, height: 160 }),
      node("k", "k", { parent: "c", x: 240, y: -20, width: 80, height: 40 }),
    ],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  ));
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  const c = layout.nodes.get("c")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(!crossesInterior(edge.points[i - 1], edge.points[i], c), `segment crosses c: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`);
  }
});

test("layoutCanvas routes an edge out of its container and around another populated one", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("c", "c", { x: 0, y: 0 }),
      node("x", "x", { parent: "c", x: 20, y: 60, width: 100, height: 40 }),
      node("d", "d", { x: 500, y: 0, width: 200, height: 200 }),
      node("w", "w", { parent: "d", x: 520, y: 20, width: 60, height: 40 }),
      node("b", "b", { x: 800, y: 300, width: 100, height: 40 }),
    ],
    [{ from: "x", to: "b", label: "", kind: "connection", fqn: "M::xb" }],
  ));
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  assert.ok(onBorder(edge.points[0], layout.nodes.get("x")!.box));
  assert.ok(onBorder(edge.points.at(-1)!, layout.nodes.get("b")!.box));
  const d = layout.nodes.get("d")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(!crossesInterior(edge.points[i - 1], edge.points[i], d), `segment crosses d: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`);
  }
});

test("layoutCanvas keeps a child sticking out of its sized container an obstacle", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 600, y: 0, width: 100, height: 40 }),
      node("c", "c", { x: 200, y: 300, width: 100, height: 100 }),
      node("k", "k", { parent: "c", x: 300, y: 0, width: 100, height: 100 }),
    ],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  ));
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  const k = layout.nodes.get("k")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(!crossesInterior(edge.points[i - 1], edge.points[i], k), `segment crosses k: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`);
  }
});

test("layoutCanvas keeps a child partly protruding from its sized container an obstacle", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 600, y: 0, width: 100, height: 40 }),
      node("c", "c", { x: 200, y: 300, width: 200, height: 200 }),
      node("k", "k", { parent: "c", x: 300, y: 0, width: 100, height: 100 }),
    ],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  ));
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  const k = layout.nodes.get("k")!.box;
  const c = layout.nodes.get("c")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(!crossesInterior(edge.points[i - 1], edge.points[i], k), `segment crosses k: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`);
    assert.ok(!crossesInterior(edge.points[i - 1], edge.points[i], c), `segment crosses c: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`);
  }
});

test("layoutCanvas places edge ports on the sides facing the opposite endpoint", () => {
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 60, ports: [{ id: "a.api", name: "api" }] }),
      node("b", "b", { x: 300, y: 0, width: 100, height: 60, ports: [{ id: "b.api", name: "api" }] }),
    ],
    [{ from: "a", to: "b", fromPort: "a.api", toPort: "b.api", label: "", kind: "connection", fqn: "M::ab" }],
  ));
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  assert.deepEqual([a.ports[0].side, a.ports[0].offset], ["east", 0.5]);
  assert.deepEqual([b.ports[0].side, b.ports[0].offset], ["west", 0.5]);
  assert.deepEqual([layout.edges[0].points[0], layout.edges[0].points.at(-1)], [
    portFace(a.box, a.ports[0]),
    portFace(b.box, b.ports[0]),
  ]);
  assert.deepEqual(portCenter(a.box, a.ports[0]), { x: a.box.x + a.box.width, y: a.box.y + a.box.height / 2 });
  const square = portBox(a.box, a.ports[0]);
  assert.equal(square.width, PORT_SIZE);
  assert.equal(square.height, PORT_SIZE);
});

test("clampNodeToBounds keeps port squares, labels, and pin exit legs inside its bounds", () => {
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 60, ports: [{ id: "a.i", name: "i" }] }),
      node("b", "b", { x: 300, y: 0, width: 100, height: 60, ports: [{ id: "b.i", name: "i" }] }),
    ],
    [{ from: "a", to: "b", fromPort: "a.i", toPort: "b.i", label: "", kind: "connection", fqn: "M::ab" }],
  ));
  const entry = layout.nodes.get("a")!;
  const port = entry.ports[0];
  assert.equal(port.side, "east");
  const bounds = { x: 10, y: 20, width: 200, height: 120 };
  const extent = nodeExtent(entry, CLEARANCE);
  const square = portBox(entry.box, port);
  const label = portLabelPlacement(entry.box, port).bounds;
  const face = portFace(entry.box, port);
  const exit = { x: face.x + CLEARANCE, y: face.y };
  assert.ok(extent.x <= square.x && extent.y <= square.y);
  assert.ok(extent.x + extent.width >= square.x + square.width);
  assert.ok(extent.y + extent.height >= square.y + square.height);
  assert.ok(extent.x <= label.x && extent.y <= label.y);
  assert.ok(extent.x + extent.width >= label.x + label.width);
  assert.ok(extent.y + extent.height >= label.y + label.height);
  assert.ok(extent.x + extent.width >= exit.x);
  assert.ok(exit.y >= extent.y && exit.y <= extent.y + extent.height);

  const at = clampNodeToBounds(entry, { x: 1000, y: 1000 }, bounds, CLEARANCE);
  const dx = at.x - entry.box.x;
  const dy = at.y - entry.box.y;
  const movedExtent = { ...extent, x: extent.x + dx, y: extent.y + dy };
  assert.ok(movedExtent.x >= bounds.x);
  assert.ok(movedExtent.y >= bounds.y);
  assert.ok(movedExtent.x + movedExtent.width <= bounds.x + bounds.width);
  assert.ok(movedExtent.y + movedExtent.height <= bounds.y + bounds.height);
});

test("clampNodeToBounds reserves a different exit reach for each port", () => {
  const layout = layoutCanvas(rendering(
    [
      node("west", "west", { x: 0, y: 60, width: 80, height: 40 }),
      node("target", "target", {
        x: 300,
        y: 60,
        width: 100,
        height: 60,
        ports: [
          { id: "target.in", name: "in" },
          { id: "target.out", name: "out" },
        ],
      }),
      node("east", "east", { x: 700, y: 60, width: 80, height: 40 }),
    ],
    [
      { from: "west", to: "target", toPort: "target.in", label: "", kind: "connection", fqn: "M::westTarget" },
      { from: "target", to: "east", fromPort: "target.out", label: "", kind: "connection", fqn: "M::targetEast" },
    ],
  ));
  const entry = layout.nodes.get("target")!;
  const west = entry.ports.find((port) => port.port.id === "target.in")!;
  const east = entry.ports.find((port) => port.port.id === "target.out")!;
  assert.equal(west.side, "west");
  assert.equal(east.side, "east");
  const exitLeg = (port: (typeof entry.ports)[number]) => (port === west ? 40 : 60);
  const extent = nodeExtent(entry, exitLeg);
  assert.equal(extent.x, portFace(entry.box, west).x - 40);
  assert.equal(extent.x + extent.width, portFace(entry.box, east).x + 60);

  const bounds = { x: 10, y: 0, width: 600, height: 300 };
  const at = clampNodeToBounds(entry, { x: -1000, y: entry.box.y }, bounds, exitLeg);
  const moved = { ...entry, box: { ...entry.box, ...at } };
  const movedExtent = nodeExtent(moved, exitLeg);
  assert.equal(movedExtent.x, bounds.x);
  assert.ok(movedExtent.x + movedExtent.width <= bounds.x + bounds.width);
  assert.equal(portFace(moved.box, west).x - 40, bounds.x);
  assert.ok(portFace(moved.box, east).x + 60 <= bounds.x + bounds.width);
});

test("freePlacement returns an already-free position unchanged", () => {
  const moving = placedNode("moving", 100, 100);
  const other = placedNode("other", 300, 100);
  const bounds: Box = { x: 0, y: 0, width: 500, height: 300 };

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), { x: moving.box.x, y: moving.box.y });
});

test("freePlacement moves an overlap on the left to its nearest free side", () => {
  const moving = placedNode("moving", 100, 100);
  const other = placedNode("other", 150, 100);
  const bounds: Box = { x: 0, y: 0, width: 500, height: 300 };

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), { x: 54, y: 100 });
});

test("freePlacement respects a wall and chooses the other side of an overlap", () => {
  const moving = placedNode("moving", 100, 20);
  const other = placedNode("other", 70, 20);
  const bounds: Box = { x: 0, y: 0, width: 300, height: 80 };

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), { x: 166, y: 20 });
});

test("freePlacement includes each node's port exit leg in its extent", () => {
  const moving = placedNode("moving", 100, 100, 80, 40, [{ id: "moving.out", name: "out" }]);
  moving.ports[0].side = "east";
  const other = placedNode("other", 230, 100);
  const bounds: Box = { x: 0, y: 0, width: 500, height: 300 };
  const withNoExit = freePlacement(moving, moving.box, [other], bounds, 0);
  const withExit = freePlacement(moving, moving.box, [other], bounds, (entry, port) => {
    assert.equal(entry.node.id, "moving");
    assert.equal(port.port.id, "moving.out");
    return 40;
  });

  assert.deepEqual(withNoExit, { x: moving.box.x, y: moving.box.y });
  assert.ok(withExit);
  assert.notDeepEqual(withExit, moving.box);
});

test("freePlacement returns its clamped position when no free spot exists", () => {
  const moving = placedNode("moving", 100, 20);
  const other = placedNode("other", 0, 0, 200, 80);
  const bounds: Box = { x: 0, y: 0, width: 200, height: 80 };

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), { x: moving.box.x, y: moving.box.y });
});

test("freePlacement jumps past an overlap in the requested direction", () => {
  const moving = placedNode("moving", 100, 20, 40, 40);
  const other = placedNode("other", 110, 20, 40, 40);
  const bounds: Box = { x: 0, y: 0, width: 300, height: 100 };

  assert.deepEqual(
    freePlacement(moving, { x: 100, y: 20 }, [other], bounds, 0, { x: 1, y: 0 }),
    { x: 166, y: 20 },
  );
});

test("freePlacement leaves a directional step unchanged when that direction is blocked", () => {
  const moving = placedNode("moving", 100, 20, 40, 40);
  const other = placedNode("other", 110, 20, 40, 40);
  const bounds: Box = { x: 0, y: 0, width: 200, height: 100 };

  assert.equal(freePlacement(moving, { x: 100, y: 20 }, [other], bounds, 0, { x: 1, y: 0 }), undefined);
});

test("freePlacement finds the nearest free position in the off-grid review scene", () => {
  const bounds: Box = { x: 0, y: 0, width: 120, height: 120 };
  const moving = placedNode("moving", 6, 70, 16, 16);
  const others = [
    placedNode("a", 67, 58, 48, 48),
    placedNode("b", 1, 88, 16, 23),
    placedNode("c", 19, 69, 33, 14),
    placedNode("d", 79, 30, 15, 36),
    placedNode("e", 15, 2, 17, 31),
  ];
  const at = { x: 6, y: 70 };
  const result = freePlacement(moving, at, others, bounds, 0)!;
  const bruteForce = nearestFreeByBruteForce(moving, at, others, bounds);

  assert.ok(bruteForce);
  assert.ok(freeAt(moving, result, others));
  assert.ok(
    Math.hypot(result.x - at.x, result.y - at.y) <=
      Math.hypot(bruteForce.x - at.x, bruteForce.y - at.y) + 1e-9,
  );
});

test("freePlacement matches a one-unit brute-force search over 200 seeded scenes", () => {
  const bounds: Box = { x: 0, y: 0, width: 120, height: 120 };
  let seed = 0x91e10da5;
  const integer = (min: number, max: number): number => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    return min + (seed % (max - min + 1));
  };
  let checked = 0;
  for (let sample = 0; sample < 200; sample++) {
    const width = integer(8, 32);
    const height = integer(8, 32);
    const moving = placedNode(
      "moving",
      integer(0, bounds.width - width),
      integer(0, bounds.height - height),
      width,
      height,
    );
    const at = {
      x: integer(0, bounds.width - width),
      y: integer(0, bounds.height - height),
    };
    const others = Array.from({ length: integer(1, 5) }, (_, index) => {
      const otherWidth = integer(12, 40);
      const otherHeight = integer(12, 40);
      return placedNode(
        `other-${sample}-${index}`,
        integer(0, bounds.width - otherWidth),
        integer(0, bounds.height - otherHeight),
        otherWidth,
        otherHeight,
      );
    });
    const bruteForce = nearestFreeByBruteForce(moving, at, others, bounds);
    if (!bruteForce) {
      continue;
    }
    checked++;
    const result = freePlacement(moving, at, others, bounds, 0);

    assert.ok(result, `no placement returned in seeded scene ${sample}`);
    assert.ok(freeAt(moving, result, others), `placement is occupied in seeded scene ${sample}`);
    assert.ok(
      Math.hypot(result.x - at.x, result.y - at.y) <=
        Math.hypot(bruteForce.x - at.x, bruteForce.y - at.y) + 1,
      `placement is not nearest in seeded scene ${sample}`,
    );
  }
  assert.ok(checked > 100, `only ${checked} scenes had a free spot`);
});

test("layoutCanvas passes bounds through to libavoid's route constraints", async () => {
  await loadAvoid(WASM);
  const bounds: Box = { x: 0, y: 0, width: 500, height: 260 };
  const result = rendering(
    [
      node("source", "source", {
        x: 50,
        y: 190,
        width: 80,
        height: 40,
        ports: [{ id: "source.out", name: "out" }],
      }),
      node("target", "target", {
        x: 370,
        y: 190,
        width: 80,
        height: 40,
        ports: [{ id: "target.in", name: "in" }],
      }),
      node("obstacle", "obstacle", { x: 210, y: 80, width: 80, height: 172 }),
    ],
    [{ from: "source", to: "target", fromPort: "source.out", toPort: "target.in", label: "", kind: "connection", fqn: "M::edge" }],
  );
  const unbounded = layoutCanvas(result);
  assert.ok(unbounded.edges[0].points.some(
    ({ x, y }) => x < bounds.x || y < bounds.y || x > bounds.x + bounds.width || y > bounds.y + bounds.height,
  ));

  const bounded = layoutCanvas(result, { bounds });
  const route = bounded.edges[0].points;
  const source = bounded.nodes.get("source")!;
  const target = bounded.nodes.get("target")!;
  assert.equal(bounded.edges[0].rerouted, true);
  assert.deepEqual(route[0], portFace(source.box, source.ports[0]));
  assert.deepEqual(route.at(-1), portFace(target.box, target.ports[0]));
  assert.ok(route.every(
    ({ x, y }) => x >= bounds.x && y >= bounds.y && x <= bounds.x + bounds.width && y <= bounds.y + bounds.height,
  ));
  for (let index = 1; index < route.length; index++) {
    assert.ok(route[index - 1].x === route[index].x || route[index - 1].y === route[index].y);
  }
});

test("layoutCanvas reroutes attached orthogonal wires after a dropped node is freed", async () => {
  await loadAvoid(WASM);
  const bounds: Box = { x: 0, y: 0, width: 700, height: 320 };
  const result = rendering(
    [
      node("source", "source", {
        x: 40,
        y: 120,
        width: 80,
        height: 40,
        ports: [{ id: "source.out", name: "out" }],
      }),
      node("moving", "moving", {
        x: 300,
        y: 120,
        width: 80,
        height: 40,
        ports: [
          { id: "moving.in", name: "in" },
          { id: "moving.out", name: "out" },
        ],
      }),
      node("target", "target", {
        x: 560,
        y: 120,
        width: 80,
        height: 40,
        ports: [{ id: "target.in", name: "in" }],
      }),
    ],
    [
      { from: "source", to: "moving", fromPort: "source.out", toPort: "moving.in", label: "", kind: "connection", fqn: "M::sourceMoving" },
      { from: "moving", to: "target", fromPort: "moving.out", toPort: "target.in", label: "", kind: "connection", fqn: "M::movingTarget" },
    ],
  );
  const before = layoutCanvas(result, { bounds });
  const moving = before.nodes.get("moving")!;
  const at = freePlacement(
    moving,
    before.nodes.get("source")!.box,
    [...before.nodes.values()].filter((entry) => entry.node.id !== "moving"),
    bounds,
    CLEARANCE,
  )!;
  assert.notDeepEqual(at, before.nodes.get("source")!.box);

  const after = layoutCanvas(result, { nodes: new Map([["moving", at]]), bounds });
  for (const edge of after.edges) {
    const source = after.nodes.get(edge.edge.from)!;
    const target = after.nodes.get(edge.edge.to)!;
    const sourcePort = source.ports.find((port) => port.port.id === edge.edge.fromPort)!;
    const targetPort = target.ports.find((port) => port.port.id === edge.edge.toPort)!;
    assert.equal(edge.rerouted, true);
    assert.deepEqual(edge.points[0], portFace(source.box, sourcePort));
    assert.deepEqual(edge.points.at(-1), portFace(target.box, targetPort));
    for (let index = 1; index < edge.points.length; index++) {
      assert.ok(edge.points[index - 1].x === edge.points[index].x || edge.points[index - 1].y === edge.points[index].y);
    }
  }
});

test("layoutCanvas routes a lower-left sender into a west port from outside its face", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(rendering(
    [
      node("sender", "sender", {
        x: 0,
        y: 200,
        width: 100,
        height: 60,
        ports: [{ id: "sender.out1", name: "out1" }],
      }),
      node("receiver", "receiver", {
        x: 320,
        y: 0,
        width: 100,
        height: 60,
        ports: [{ id: "receiver.in1", name: "in1" }],
      }),
    ],
    [{
      from: "sender",
      to: "receiver",
      fromPort: "sender.out1",
      toPort: "receiver.in1",
      label: "wire",
      kind: "connection",
      fqn: "M::sender_receiver",
    }],
  ));
  const receiver = layout.nodes.get("receiver")!;
  const edge = layout.edges[0];
  const face = portFace(receiver.box, receiver.ports[0]);
  assert.equal(receiver.ports[0].side, "west");
  assert.deepEqual(edge.points.at(-1), face);
  const beforeFace = edge.points.at(-2)!;
  assert.equal(beforeFace.y, face.y);
  assert.ok(beforeFace.x < face.x, `receiver port is not approached from the left: ${JSON.stringify(edge.points)}`);
  for (let i = 1; i < edge.points.length; i++) {
    for (const [borderStart, borderEnd] of [
      [{ x: receiver.box.x, y: receiver.box.y }, { x: receiver.box.x + receiver.box.width, y: receiver.box.y }],
      [{ x: receiver.box.x + receiver.box.width, y: receiver.box.y }, { x: receiver.box.x + receiver.box.width, y: receiver.box.y + receiver.box.height }],
      [{ x: receiver.box.x + receiver.box.width, y: receiver.box.y + receiver.box.height }, { x: receiver.box.x, y: receiver.box.y + receiver.box.height }],
      [{ x: receiver.box.x, y: receiver.box.y + receiver.box.height }, { x: receiver.box.x, y: receiver.box.y }],
    ] as const) {
      assert.ok(
        overlapLength(edge.points[i - 1], edge.points[i], borderStart, borderEnd) <= 1,
        `route runs along receiver border: ${JSON.stringify(edge.points)}`,
      );
    }
  }
});

test("layoutCanvas spreads defaulted ports on a side in their source order", () => {
  const ports = ["first", "second", "third"].map((id) => ({ id, name: id }));
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 100, ports }),
      node("b", "b", { x: 300, y: 0, width: 80, height: 40 }),
      node("c", "c", { x: 300, y: 100, width: 80, height: 40 }),
      node("d", "d", { x: 300, y: 200, width: 80, height: 40 }),
    ],
    [
      { from: "a", to: "b", fromPort: "first", label: "", kind: "connection", fqn: "M::ab" },
      { from: "a", to: "c", fromPort: "second", label: "", kind: "connection", fqn: "M::ac" },
      { from: "a", to: "d", fromPort: "third", label: "", kind: "connection", fqn: "M::ad" },
    ],
  ));
  assert.deepEqual(layout.nodes.get("a")!.ports.map(({ side, offset }) => [side, offset]), [
    ["east", 0.25],
    ["east", 0.5],
    ["east", 0.75],
  ]);
});

test("layoutCanvas falls back to the node anchor for a missing endpoint port", () => {
  const layout = layoutCanvas(rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 60, ports: [{ id: "a.api", name: "api" }] }),
      node("b", "b", { x: 300, y: 0, width: 100, height: 60 }),
    ],
    [{
      from: "a",
      to: "b",
      fromPort: "missing",
      label: "",
      kind: "connection",
      fqn: "M::ab",
      route: [{ x: 150, y: 30 }],
    }],
  ));
  const source = layout.nodes.get("a")!.box;
  const target = layout.nodes.get("b")!.box;
  assert.deepEqual(layout.edges[0].points[0], anchor(source, { x: 150, y: 30 }));
  assert.deepEqual(layout.edges[0].points.at(-1), anchor(target, { x: 150, y: 30 }));
});

test("layoutCanvas preserves an automatic port placement when its node moves", () => {
  const result = rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 60, ports: [{ id: "a.api", name: "api" }] }),
      node("b", "b", { x: 300, y: 0, width: 100, height: 60 }),
    ],
    [{ from: "a", to: "b", fromPort: "a.api", label: "", kind: "connection", fqn: "M::ab" }],
  );
  const auto: AutoLayout = {
    nodes: new Map(),
    routes: new Map(),
    ports: new Map([["a.api", { side: "north", offset: 0.2 }]]),
  };
  const layout = layoutCanvas(result, { nodes: new Map([["a", { x: 400, y: 250 }]]) }, auto);
  const source = layout.nodes.get("a")!;
  assert.deepEqual([source.ports[0].side, source.ports[0].offset], ["north", 0.2]);
  assert.deepEqual(layout.edges[0].points[0], portFace(source.box, source.ports[0]));
});

test("liftedEdges recalculates a port anchor from the shifted node box", () => {
  const result = rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 60, ports: [{ id: "a.api", name: "api" }] }),
      node("b", "b", { x: 300, y: 0, width: 100, height: 60, ports: [{ id: "b.api", name: "api" }] }),
    ],
    [{ from: "a", to: "b", fromPort: "a.api", toPort: "b.api", label: "", kind: "connection", fqn: "M::ab" }],
  );
  const layout = layoutCanvas(result);
  const source = layout.nodes.get("a")!;
  const lifted = liftedEdges(layout, "a", 40, 25)[0];
  const shiftedBox = { ...source.box, x: source.box.x + 40, y: source.box.y + 25 };
  assert.deepEqual(lifted.points[0], portFace(shiftedBox, source.ports[0]));
});
