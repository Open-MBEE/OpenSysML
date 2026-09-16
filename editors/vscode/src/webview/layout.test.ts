import assert from "node:assert/strict";
import { test } from "node:test";

import type { RenderEdge, RenderNode, RenderResult } from "../protocol";
import {
  anchor,
  GAP,
  insertedWaypoint,
  labelLines,
  layoutCanvas,
  liftedEdges,
  MARGIN,
  movable,
  movedNode,
  movedWaypoint,
  nodeUnder,
  overridesOf,
  removedWaypoint,
  shapeOf,
  steerable,
} from "./layout";

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
