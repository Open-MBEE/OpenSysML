import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";

import type {
  LayoutGeometry,
  RenderEdge,
  RenderNode,
  RenderPoint,
  RenderResult,
} from "../protocol";
import type { EngineInstance } from "../landing/model";
import { landingModel, renderResult, sceneTarget } from "../landing/model";
import { renderedViewFromJson } from "@openmbee/opensysml/browser";
import { STACK_SCENE } from "../landing/scene";
import { carried, obstacles, resettle } from "../landing/carry";
import { presentedStack } from "../landing/present";
import { autoLayout, type AutoLayout } from "./autolayout";
import {
  alignedPlacement,
  anchor,
  clampNodeToBounds,
  type CanvasLayout,
  GAP,
  freePlacement,
  keepOrthogonalRoutes,
  byCodeUnit,
  insertedWaypoint,
  labelLines,
  labelSize,
  edgeLabelBox,
  headerBox,
  listedEdges,
  simpleName,
  simpleNames,
  layoutCanvas,
  liftedEdges,
  MARGIN,
  movable,
  movedNode,
  movedWaypoint,
  nodeExtent,
  nodeUnder,
  overridesOf,
  type PlacedEdge,
  type PlacedNode,
  type PlacedPort,
  portBox,
  portCenter,
  portFace,
  portLabelPlacement,
  PORT_SIZE,
  reattachRoute,
  removedWaypoint,
  shapeOf,
  steerable,
  type Box,
  type Side,
} from "./layout";
import { CLEARANCE, loadAvoid, MIN_JOG, portExitReach } from "./avoid";

const WASM = path.resolve("node_modules/libavoid-js/dist/libavoid.wasm");

interface LandingFixture {
  hash: string;
  /** The protojson RenderViewResponse the module answered. */
  render: unknown;
  instances: EngineInstance[];
}

const origin = {
  uri: "file:///m.sysml",
  range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } },
  digest: "d0",
};

function node(
  id: string,
  name: string,
  extra: Partial<RenderNode> = {},
): RenderNode {
  return {
    id,
    kind: "part",
    name,
    type: "",
    detail: "",
    fqn: `M::${name}`,
    origin,
    ...extra,
  };
}

function rendering(
  nodes: RenderNode[],
  edges: RenderEdge[] = [],
  extra: Partial<RenderResult> = {},
): RenderResult {
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
  return layoutCanvas(
    rendering([node(id, id, { x, y, width, height, ports })]),
  ).nodes.get(id)!;
}

function layoutWithPorts(
  nodes: RenderNode[],
  edges: RenderEdge[],
  ports: AutoLayout["ports"],
  bounds: Box,
): CanvasLayout {
  return layoutCanvas(
    rendering(nodes, edges),
    { bounds },
    { nodes: new Map(), routes: new Map(), ports },
  );
}

function orthogonal(points: RenderPoint[]): boolean {
  return points.every((point, index) => {
    if (index === 0) {
      return true;
    }
    const previous = points[index - 1];
    return (
      Math.abs(point.x - previous.x) <= 1e-6 ||
      Math.abs(point.y - previous.y) <= 1e-6
    );
  });
}

function assertPointNear(
  actual: RenderPoint,
  expected: RenderPoint,
  context: string,
): void {
  assert.ok(
    Math.abs(actual.x - expected.x) <= 1e-6 &&
      Math.abs(actual.y - expected.y) <= 1e-6,
    `${context}: expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
  );
}

function pointDirection(start: RenderPoint, end: RenderPoint): RenderPoint {
  const dx = end.x - start.x;
  const dy = end.y - start.y;
  return Math.abs(dx) >= Math.abs(dy)
    ? { x: Math.sign(dx), y: 0 }
    : { x: 0, y: Math.sign(dy) };
}

function assertRouteDirections(
  points: RenderPoint[],
  start: RenderPoint,
  end: RenderPoint,
  startDirection: RenderPoint,
  endDirection: RenderPoint,
): void {
  assert.ok(orthogonal(points));
  assertPointNear(points[0], start, "route start");
  assertPointNear(points.at(-1)!, end, "route end");
  assert.ok(
    Math.hypot(points[1].x - points[0].x, points[1].y - points[0].y) > 1e-6,
  );
  assert.ok(
    Math.hypot(
      points.at(-1)!.x - points.at(-2)!.x,
      points.at(-1)!.y - points.at(-2)!.y,
    ) > 1e-6,
  );
  assert.deepEqual(pointDirection(points[0], points[1]), startDirection);
  assert.deepEqual(
    pointDirection(points.at(-1)!, points.at(-2)!),
    endDirection,
  );
}

function portNode(
  id: string,
  x: number,
  y: number,
  width = 160,
  height = 70,
): RenderNode {
  return node(id, id, {
    x,
    y,
    width,
    height,
    ports: [{ id: `${id}.api`, name: "api" }],
  });
}

function connectedEdge(from: string, to: string): RenderEdge {
  return {
    from,
    to,
    fromPort: `${from}.api`,
    toPort: `${to}.api`,
    label: "",
    kind: "connection",
    fqn: `M::${from}_${to}`,
  };
}

function fixedPorts(
  ...entries: Array<[string, Side, number]>
): AutoLayout["ports"] {
  const ports: AutoLayout["ports"] = new Map();
  for (const [id, side, offset] of entries) {
    ports.set(id, { side, offset });
  }
  return ports;
}

function freeAt(
  node: PlacedNode,
  at: RenderPoint,
  others: PlacedNode[],
  portExitLeg: number | ((node: PlacedNode, port: PlacedPort) => number) = 0,
): boolean {
  const exitLeg = (entry: PlacedNode, port: PlacedPort): number =>
    typeof portExitLeg === "number" ? portExitLeg : portExitLeg(entry, port);
  const extent = nodeExtent(
    { ...node, box: { ...node.box, x: at.x, y: at.y } },
    (port) => exitLeg(node, port),
  );
  const expanded = {
    x: extent.x - CLEARANCE,
    y: extent.y - CLEARANCE,
    width: extent.width + 2 * CLEARANCE,
    height: extent.height + 2 * CLEARANCE,
  };
  return others.every((other) => {
    const extent = nodeExtent(other, (port) => exitLeg(other, port));
    const obstacle = {
      x: extent.x - CLEARANCE,
      y: extent.y - CLEARANCE,
      width: extent.width + 2 * CLEARANCE,
      height: extent.height + 2 * CLEARANCE,
    };
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

function insetBox(box: Box, distance: number): Box {
  return {
    x: box.x + distance,
    y: box.y + distance,
    width: Math.max(0, box.width - 2 * distance),
    height: Math.max(0, box.height - 2 * distance),
  };
}

function assertOrthogonalRoutes(
  layout: CanvasLayout,
  bounds: Box,
  context = "layout",
): void {
  for (const edge of layout.edges) {
    const source = layout.nodes.get(edge.edge.from)!;
    const target = layout.nodes.get(edge.edge.to)!;
    const sourcePort = source.ports.find(
      (port) => port.port.id === edge.edge.fromPort,
    )!;
    const targetPort = target.ports.find(
      (port) => port.port.id === edge.edge.toPort,
    )!;
    assert.equal(edge.rerouted, true);
    assert.deepEqual(edge.points[0], portFace(source.box, sourcePort));
    assert.deepEqual(edge.points.at(-1), portFace(target.box, targetPort));
    assert.ok(
      edge.points.every(
        ({ x, y }) =>
          x >= bounds.x &&
          y >= bounds.y &&
          x <= bounds.x + bounds.width &&
          y <= bounds.y + bounds.height,
      ),
    );
    for (let index = 1; index < edge.points.length; index++) {
      const previous = edge.points[index - 1];
      const current = edge.points[index];
      assert.ok(
        previous.x === current.x || previous.y === current.y,
        `${context} route ${edge.edge.from} → ${edge.edge.to} has a diagonal segment: ${JSON.stringify(
          {
            segment: [previous, current],
            source: source.box,
            sourcePort,
            target: target.box,
            targetPort,
          },
        )}`,
      );
    }
  }
}

test("byCodeUnit orders strings by code unit, unlike a locale-aware comparison", () => {
  const kinds = ["succession", "Transition", "binding", "ownership"];
  assert.deepEqual([...kinds].sort(byCodeUnit), ["Transition", "binding", "ownership", "succession"]);
  assert.notDeepEqual([...kinds].sort(byCodeUnit), [...kinds].sort((a, b) => a.localeCompare(b, "en")));
  assert.equal(byCodeUnit("a", "a"), 0);
});

test("layoutCanvas draws a pinned node's edge straight until the router has loaded", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
        node("b", "b", { x: 400, y: 200, width: 100, height: 40 }),
        node("c", "c", { x: 180, y: -20, width: 60, height: 80 }),
      ],
      [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
    ),
  );
  assert.equal(layout.edges[0].rerouted, false);
  assert.equal(layout.edges[0].points.length, 2);
});

test("layoutCanvas places unplaced roots in a near-square grid, in order, from the margin", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a"),
      node("b", "b"),
      node("c", "c"),
      node("d", "d"),
      node("e", "e"),
    ]),
  );
  const boxes = ["a", "b", "c", "d", "e"].map(
    (id) => layout.nodes.get(id)!.box,
  );
  // Five nodes take three columns; every unplaced label box is the minimum width
  // and two lines («part», name) high.
  assert.deepEqual(
    boxes.map((box) => [box.x, box.y]),
    [
      [MARGIN, MARGIN],
      [MARGIN + 96 + GAP, MARGIN],
      [MARGIN + 2 * (96 + GAP), MARGIN],
      [MARGIN, MARGIN + 52 + GAP],
      [MARGIN + 96 + GAP, MARGIN + 52 + GAP],
    ],
  );
  assert.ok(boxes.every((box) => box.width === 96 && box.height === 52));
  assert.ok(
    boxes.every(
      (_, i) => !layout.nodes.get(["a", "b", "c", "d", "e"][i])!.pinned,
    ),
  );
  assert.equal(layout.width, MARGIN + 3 * 96 + 2 * GAP + MARGIN);
  assert.equal(layout.height, MARGIN + 2 * 52 + GAP + MARGIN);
  assert.equal(layout.placeable, true);
});

test("layoutCanvas is deterministic: the same rendering lays out the same twice", () => {
  const result = rendering(
    [
      node("a", "a"),
      node("b", "b", { parent: "a" }),
      node("c", "c", { parent: "a" }),
    ],
    [{ from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc" }],
  );
  const first = layoutCanvas(result);
  const second = layoutCanvas(result);
  assert.deepEqual(
    [...first.nodes.values()].map((entry) => entry.box),
    [...second.nodes.values()].map((entry) => entry.box),
  );
  assert.deepEqual(
    first.edges.map((edge) => edge.points),
    second.edges.map((edge) => edge.points),
  );
});

test("layoutCanvas keeps the model's geometry exactly and marks the node pinned", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 300.5, y: 12, width: 150, height: 70 }),
      node("b", "b"),
    ]),
  );
  const a = layout.nodes.get("a")!;
  assert.deepEqual(a.box, { x: 300.5, y: 12, width: 150, height: 70 });
  assert.equal(a.pinned, true);
  // The pinned node keeps its grid slot, so its sibling stays where it would be anyway.
  assert.deepEqual(
    [layout.nodes.get("b")!.box.x, layout.nodes.get("b")!.box.y],
    [MARGIN + 150 + GAP, MARGIN],
  );
  // The canvas grows to hold what the model placed.
  assert.equal(layout.width, 300.5 + 150 + MARGIN);
});

test("layoutCanvas nests children inside their owner and sizes the owner around them", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a"),
      node("b", "b", { parent: "a" }),
      node("c", "c", { parent: "a" }),
    ]),
  );
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  const c = layout.nodes.get("c")!;
  assert.deepEqual(
    layout.roots.map((root) => root.node.id),
    ["a"],
  );
  assert.equal(b.parent, a);
  // Children start under the owner's label, inside its padding.
  assert.deepEqual([b.box.x, b.box.y], [a.box.x + 16, a.box.y + 52]);
  assert.deepEqual([c.box.x, c.box.y], [b.box.x + 96 + GAP, b.box.y]);
  assert.ok(a.box.x + a.box.width >= c.box.x + c.box.width + 16);
  assert.ok(a.box.y + a.box.height >= c.box.y + c.box.height + 16);
});

test("layoutCanvas grows an unsized owner to hold a child the model put beyond it", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a"),
      node("b", "b", { parent: "a", x: 500, y: 400 }),
    ]),
  );
  const a = layout.nodes.get("a")!;
  assert.ok(a.box.x + a.box.width >= 500 + 96 + 16);
  assert.ok(a.box.y + a.box.height >= 400 + 40 + 16);
});

test("layoutCanvas extends the canvas to a placed child beyond its sized owner", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 0, y: 0, width: 100, height: 100 }),
      node("b", "b", { parent: "a", x: 500, y: 400 }),
    ]),
  );
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  assert.deepEqual(a.box, { x: 0, y: 0, width: 100, height: 100 });
  assert.deepEqual([b.box.x, b.box.y], [500, 400]);
  assert.equal(layout.width, 500 + b.box.width + MARGIN);
  assert.equal(layout.height, 400 + b.box.height + MARGIN);
  assert.deepEqual(layout.origin, { x: 0, y: 0 });
});

test("layoutCanvas moves the canvas's corner up and left to geometry the model puts before the origin", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: -120, y: 40, width: 100, height: 50 }),
      node("b", "b", { x: 200, y: -30, width: 100, height: 50 }),
    ]),
  );
  assert.deepEqual(layout.nodes.get("a")!.box, {
    x: -120,
    y: 40,
    width: 100,
    height: 50,
  });
  assert.deepEqual(layout.origin, { x: -120 - MARGIN, y: -30 - MARGIN });
  assert.equal(layout.origin.x + layout.width, 300 + MARGIN);
  assert.equal(layout.origin.y + layout.height, 90 + MARGIN);
});

test("layoutCanvas moves a root's siblings past an owner grown around a placed child", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a"),
      node("b", "b", { parent: "a", x: 500, y: 400 }),
      node("c", "c"),
    ]),
  );
  const a = layout.nodes.get("a")!;
  const c = layout.nodes.get("c")!;
  assert.equal(a.box.x + a.box.width, 500 + 96 + 16);
  assert.equal(c.box.x, a.box.x + a.box.width + GAP);
  assert.equal(c.pinned, false);
});

test("layoutCanvas moves nested siblings past a container grown around a placed grandchild", () => {
  const layout = layoutCanvas(
    rendering([
      node("root", "root"),
      node("a", "a", { parent: "root" }),
      node("b", "b", { parent: "a", x: 300, y: 300 }),
      node("c", "c", { parent: "root" }),
      node("d", "d", { parent: "root" }),
      node("e", "e", { parent: "root" }),
    ]),
  );
  const root = layout.nodes.get("root")!;
  const a = layout.nodes.get("a")!;
  const c = layout.nodes.get("c")!;
  const d = layout.nodes.get("d")!;
  assert.deepEqual(
    [a.box.x + a.box.width, a.box.y + a.box.height],
    [300 + 96 + 16, 300 + 52 + 16],
  );
  // Four children take two columns and two rows: a's column is as wide as a, its row as tall.
  assert.equal(c.box.x, a.box.x + a.box.width + GAP);
  assert.equal(d.box.y, a.box.y + a.box.height + GAP);
  assert.ok(root.box.x + root.box.width >= c.box.x + c.box.width + 16);
  assert.ok(root.box.y + root.box.height >= d.box.y + d.box.height + 16);
});

test("layoutCanvas with unplaced takes the auto layout over the model's Layout and Route, pinning nothing", () => {
  const drawing = rendering(
    [node("a", "a", { x: 10, y: 10 }), node("b", "b", { x: 400, y: 10 })],
    [{ from: "a", to: "b", label: "", kind: "connection", route: [{ x: 500, y: 900 }] }],
  );
  const auto: AutoLayout = {
    nodes: new Map([
      ["a", { x: 24, y: 24, width: 96, height: 52 }],
      ["b", { x: 52, y: 200, width: 96, height: 52 }],
    ]),
    routes: new Map([[0, [{ x: 72, y: 76 }, { x: 72, y: 140 }, { x: 100, y: 140 }, { x: 100, y: 200 }]]]),
    ports: new Map(),
  };
  const stated = layoutCanvas(drawing, {}, auto);
  assert.deepEqual([stated.nodes.get("a")!.box.x, stated.nodes.get("b")!.box.x], [10, 400]);
  assert.deepEqual(stated.edges[0].route, [{ x: 500, y: 900 }]);
  const free = layoutCanvas(drawing, { unplaced: true }, auto);
  assert.deepEqual([free.nodes.get("a")!.box, free.nodes.get("b")!.box], [auto.nodes.get("a"), auto.nodes.get("b")]);
  assert.deepEqual([free.nodes.get("a")!.pinned, free.nodes.get("b")!.pinned], [false, false]);
  // The model's Route set aside, the edge follows the auto layout's bent route, not a straight line.
  assert.deepEqual(free.edges[0].points, auto.routes.get(0));
  assert.deepEqual(free.edges[0].route, [{ x: 72, y: 140 }, { x: 100, y: 140 }]);
  // A gesture's own override still wins over the auto layout.
  const dragged = layoutCanvas(drawing, { unplaced: true, nodes: new Map([["a", { x: 300, y: 300, width: 96, height: 52 }]]) }, auto);
  assert.deepEqual(dragged.nodes.get("a")!.box, { x: 300, y: 300, width: 96, height: 52 });
  assert.equal(dragged.nodes.get("a")!.pinned, true);
});

test("movedNode alone writes the dragged node and nothing of the auto layout under it", () => {
  const drawing = rendering(
    [node("box", "box", { x: 10, y: 10 }), node("a", "a", { parent: "box", x: 30, y: 40 }), node("b", "b", { parent: "box" })],
    [{ from: "a", to: "b", label: "", kind: "connection", route: [{ x: 500, y: 900 }] }],
  );
  const auto: AutoLayout = {
    nodes: new Map([
      ["box", { x: 24, y: 24, width: 300, height: 200 }],
      ["a", { x: 48, y: 60, width: 96, height: 52 }],
      ["b", { x: 48, y: 150, width: 96, height: 52 }],
    ]),
    routes: new Map([[0, [{ x: 96, y: 112 }, { x: 96, y: 130 }, { x: 120, y: 130 }, { x: 120, y: 150 }]]]),
    ports: new Map(),
  };
  const free = layoutCanvas(drawing, { unplaced: true }, auto);
  const held = movedNode(free, "box", 20, 0)!;
  assert.ok(held.nodes.length > 1 || held.edges.length > 0);
  const alone = movedNode(free, "box", 20, 0, true)!;
  assert.deepEqual(alone.nodes.map((placement) => placement.id), ["box"]);
  assert.equal(alone.nodes[0].layout.x, 44);
  assert.deepEqual(alone.edges, []);
});

test("layoutCanvas hides a sequence diagram's messages of a hidden kind and closes the rows over them", () => {
  const drawing = rendering(
    [node("a", "a"), node("b", "b")],
    [
      { from: "a", to: "b", label: "", kind: "flow" },
      { from: "b", to: "a", label: "", kind: "succession" },
      { from: "a", to: "b", label: "", kind: "flow" },
    ],
    { kind: "sequence" },
  );
  const all = layoutCanvas(drawing);
  const filtered = layoutCanvas(drawing, { hiddenKinds: new Set(["flow"]) });
  assert.deepEqual(filtered.edges.map((edge) => edge.hidden), [true, false, true]);
  assert.equal(filtered.edges[1].points[0].y, all.edges[0].points[0].y);
  assert.ok(filtered.height < all.height);
});

test("layoutCanvas hides the edges of a hidden kind, which then count for nothing", () => {
  const drawing = rendering(
    [node("a", "a", { x: 10, y: 10 }), node("b", "b", { x: 200, y: 10 })],
    [
      { from: "a", to: "b", label: "", kind: "typing", route: [{ x: 500, y: 900 }] },
      { from: "a", to: "b", label: "", kind: "connection" },
    ],
  );
  const all = layoutCanvas(drawing);
  assert.deepEqual(all.edges.map((edge) => edge.hidden), [false, false]);
  const filtered = layoutCanvas(drawing, { hiddenKinds: new Set(["typing"]) });
  assert.deepEqual(filtered.edges.map((edge) => edge.hidden), [true, false]);
  // The hidden edge's far-flung waypoint no longer stretches the canvas.
  assert.ok(filtered.height < all.height);
  assert.deepEqual(filtered.nodes.get("a")!.box, all.nodes.get("a")!.box);
});

// The server folds a collapsed node, so what it sends beneath one is the node's
// boundary: its ports and pins. The panel draws each of them inside the node's
// box and routes every edge at them, as the CLI does.
test("layoutCanvas draws a collapsed node's port children as pins on its border and routes the edges at them", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 10, y: 10, collapsed: true }),
        node("p1", "p1", { kind: "port", parent: "a" }),
        node("p2", "p2", { kind: "port", parent: "a" }),
        node("d", "d", { x: 300, y: 5 }),
      ],
      [
        { from: "d", to: "p1", label: "", kind: "connection" },
        { from: "p2", to: "d", label: "", kind: "connection" },
        { from: "a", to: "d", label: "", kind: "connection" },
      ],
    ),
  );
  const a = layout.nodes.get("a")!;
  assert.equal(a.collapsed, true);
  assert.deepEqual(
    a.children.map((child) => child.node.id),
    ["p1", "p2"],
  );
  let bottom = a.box.y + a.box.height;
  for (const id of ["p1", "p2"]) {
    const port = layout.nodes.get(id)!;
    assert.ok(port.snapped, `${id} is a pin`);
    pinnedOn(port, a);
    bottom = Math.max(bottom, port.box.y + port.box.height);
  }
  assert.equal(layout.edges.length, 3);
  for (const edge of layout.edges) {
    assert.ok(edge.points.length >= 2, `edge ${edge.edge.from} -> ${edge.edge.to} is routed`);
    assert.equal(edge.hidden, false);
  }
  assert.equal(layout.height, bottom + MARGIN);
});

test("layoutCanvas honours the view's canvas size as a minimum", () => {
  const layout = layoutCanvas(
    rendering([node("a", "a")], [], {
      canvas: { unit: "px", width: 800, height: 600 },
    }),
  );
  assert.deepEqual([layout.width, layout.height], [800, 600]);
});

test("layoutCanvas routes an edge from border to border through its waypoints", () => {
  const edge: RenderEdge = {
    from: "a",
    to: "b",
    label: "wire",
    kind: "connection",
    fqn: "M::ab",
    route: [{ x: 200, y: 200 }],
  };
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
        node("b", "b", { x: 300, y: 300, width: 100, height: 40 }),
      ],
      [edge],
    ),
  );
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
  const layout = layoutCanvas(
    rendering(
      [node("a", "a", { x: 0, y: 0, width: 100, height: 60 })],
      [{ from: "a", to: "a", label: "", kind: "transition" }],
    ),
  );
  const [placed] = layout.edges;
  assert.equal(placed.points.length, 4);
  assert.ok(placed.points.slice(1, 3).every((point) => point.x > 100));
  assert.deepEqual(placed.route, []);
  assert.equal(steerable(layout, placed), false);
});

test("layoutCanvas lays a sequence out as lifelines in a row with messages down them", () => {
  const layout = layoutCanvas(
    rendering(
      [node("a", "a"), node("b", "b")],
      [
        { from: "a", to: "b", label: "ask", kind: "flow" },
        { from: "b", to: "a", label: "answer", kind: "flow" },
      ],
      { kind: "sequence" },
    ),
  );
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

test("simpleNames keeps the qualifier that tells nodes of one name apart", () => {
  const nodes = [
    node("a", "Plant::pump"),
    node("b", "Backup::pump"),
    node("c", "Plant::tank"),
    node("d", "'Lab Use Cases'::'Acquire Image'"),
  ];
  const names = simpleNames(nodes);
  assert.deepEqual(
    [...names.entries()],
    [
      ["a", "Plant::pump"],
      ["b", "Backup::pump"],
      ["c", "tank"],
      ["d", "'Acquire Image'"],
    ],
  );
  assert.deepEqual(labelLines(nodes[0], [], names), ["Plant::pump"]);
  assert.deepEqual(labelLines(nodes[2], [], names), ["tank"]);
  assert.deepEqual(labelLines(nodes[0], ["qualifiers"], names), [
    "Plant::pump",
  ]);
});

test("simpleNames reads an escaped quote inside a quoted segment", () => {
  const nodes = [
    node("a", "Quoted::'It\\'s'::pump"),
    node("b", "Quoted::'It\\'s'"),
    node("c", "Plant::'It\\'s'"),
  ];
  assert.deepEqual(
    [...simpleNames(nodes).entries()],
    [
      ["a", "pump"],
      ["b", "Quoted::'It\\'s'"],
      ["c", "Plant::'It\\'s'"],
    ],
  );
  assert.equal(simpleName("Quoted::'It\\'s'::pump"), "pump");
  assert.equal(simpleName("'It\\'s'"), "'It\\'s'");
});

test("labelLines and shapeOf follow the graphical notation", () => {
  assert.deepEqual(
    labelLines(node("a", "tank", { type: "Tank", detail: "[2]" })),
    ["«part»", "tank : Tank", "[2]"],
  );
  assert.deepEqual(
    labelLines(node("a", "Plant::tank", { type: "Tank", detail: "[2]" }), [
      "port-symbols",
      "compartments",
    ]),
    ["tank", "[2]"],
  );
  assert.deepEqual(
    labelLines(
      node("a", "'Lab Use Cases'::'Acquire Image'", {
        kind: "use case",
        type: "Case",
      }),
      ["keywords", "typings"],
    ),
    ["«use case»", "'Acquire Image' : Case"],
  );
  assert.deepEqual(
    labelLines(node("a", "P::tank", { type: "Tank", detail: "[2]" }), [
      "keywords",
      "typings",
      "compartments",
      "qualifiers",
    ]),
    ["«part»", "P::tank : Tank", "[2]"],
  );
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
  const placeable = layoutCanvas(
    rendering(
      [declared, imported],
      [
        { from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" },
        { from: "b", to: "a", label: "", kind: "connection" },
      ],
    ),
  );
  assert.equal(movable(placeable, placeable.nodes.get("a")!), true);
  assert.equal(movable(placeable, placeable.nodes.get("b")!), false);
  assert.equal(steerable(placeable, placeable.edges[0]), true);
  assert.equal(steerable(placeable, placeable.edges[1]), false);
  const table = layoutCanvas(rendering([declared], [], { kind: "table" }));
  assert.equal(movable(table, table.nodes.get("a")!), false);
});

test("movable and steerable take a target another document declares, named or not, and no library's", () => {
  const foreignOrigin = { ...origin, uri: "file:///work/parts.sysml" };
  const declaration = {
    start: { line: 2, character: 4 },
    end: { line: 2, character: 30 },
  };
  const library = layoutCanvas(
    rendering(
      [
        node("a", "a", { declaredHere: true }),
        node("b", "b", { origin: foreignOrigin }),
        node("c", "", { fqn: undefined, declaration, origin: foreignOrigin }),
        node("d", "d", {
          fqn: undefined,
          origin: {
            ...origin,
            uri: "sysml-stdlib:///Systems%20Library/Parts.sysml",
          },
        }),
      ],
      [
        {
          from: "a",
          to: "b",
          label: "",
          kind: "connection",
          fqn: "Parts::ab",
          origin: foreignOrigin,
        },
        {
          from: "a",
          to: "d",
          label: "",
          kind: "connection",
          origin: foreignOrigin,
        },
      ],
    ),
  );
  assert.equal(movable(library, library.nodes.get("b")!), true);
  assert.equal(movable(library, library.nodes.get("c")!), true);
  assert.equal(movable(library, library.nodes.get("d")!), false);
  assert.equal(steerable(library, library.edges[0]), true);
  assert.equal(steerable(library, library.edges[1]), false);
});

test("movable and steerable accept a target reached by its declaration alone", () => {
  const declaration = {
    start: { line: 2, character: 4 },
    end: { line: 2, character: 30 },
  };
  const unnamed = layoutCanvas(
    rendering(
      [node("a", "a"), node("b", "", { fqn: undefined, declaration })],
      [{ from: "a", to: "b", label: "", kind: "transition", declaration }],
    ),
  );
  assert.equal(movable(unnamed, unnamed.nodes.get("b")!), true);
  assert.equal(steerable(unnamed, unnamed.edges[0]), true);
});

test("nodeUnder is the innermost drawn node holding the point, the later sibling of two that overlap", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 0, y: 0, width: 300, height: 200 }),
      node("b", "b", { parent: "a", x: 20, y: 60, width: 100, height: 50 }),
      node("c", "c", { x: 250, y: 100, width: 100, height: 50 }),
      node("d", "d", { x: 600, y: 600, width: 100, height: 50 }),
    ]),
  );
  assert.equal(nodeUnder(layout, { x: 10, y: 10 })?.node.id, "a");
  assert.equal(nodeUnder(layout, { x: 50, y: 80 })?.node.id, "b");
  // Where a's and c's boxes overlap, c is drawn later, on top; a border counts as inside.
  assert.equal(nodeUnder(layout, { x: 280, y: 120 })?.node.id, "c");
  assert.equal(nodeUnder(layout, { x: 700, y: 650 })?.node.id, "d");
  assert.equal(nodeUnder(layout, { x: 500, y: 500 }), undefined);
});

test("nodeUnder passes over the dragged subtree and reads the ports a collapsed node keeps", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 0, y: 0, width: 300, height: 200 }),
      node("b", "b", { parent: "a", x: 20, y: 60, width: 100, height: 50 }),
      node("e", "e", { parent: "b", x: 30, y: 80, width: 40, height: 20 }),
      node("c", "c", {
        x: 400,
        y: 0,
        width: 200,
        height: 200,
        collapsed: true,
      }),
      node("f", "f", { kind: "port", parent: "c", x: 420, y: 50, width: 40, height: 20 }),
    ]),
  );
  // The dragged node b is held over its own place: what is under the pointer is its owner.
  assert.equal(nodeUnder(layout, { x: 40, y: 85 }, "b")?.node.id, "a");
  assert.equal(nodeUnder(layout, { x: 40, y: 85 })?.node.id, "e");
  assert.equal(nodeUnder(layout, { x: 430, y: 60 })?.node.id, "f");
  assert.equal(nodeUnder(layout, { x: 550, y: 150 })?.node.id, "c");
});

test("nodeUnder reads the layout it is given, so a node placed aside no longer covers its old point", () => {
  const result = rendering([
    node("a", "a", { x: 0, y: 0, width: 100, height: 50 }),
    node("b", "b", { x: 200, y: 0, width: 100, height: 50 }),
  ]);
  const shown = layoutCanvas(
    result,
    overridesOf(movedNode(layoutCanvas(result), "a", 200, 0)!),
  );
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
  const layout = layoutCanvas(
    rendering([
      node("a", "a", {
        x: 100,
        y: 100,
        width: 200,
        height: 80,
        collapsed: true,
      }),
    ]),
  );
  assert.deepEqual(movedNode(layout, "a", 5, 5)!.nodes, [
    {
      id: "a",
      layout: { x: 105, y: 105, width: 200, height: 80, collapsed: true },
    },
  ]);
});

test("movedNode carries the placed descendants and inner routes along, leaving unplaced ones to follow", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0 }),
        node("b", "b", { parent: "a", x: 20, y: 60 }),
        node("c", "c", { parent: "a" }),
        node("d", "d"),
      ],
      [
        {
          from: "b",
          to: "c",
          label: "",
          kind: "connection",
          fqn: "M::bc",
          route: [{ x: 50, y: 50 }],
        },
        {
          from: "b",
          to: "d",
          label: "",
          kind: "connection",
          fqn: "M::bd",
          route: [{ x: 70, y: 70 }],
        },
      ],
    ),
  );
  const placements = movedNode(layout, "a", 100, 0)!;
  assert.deepEqual(placements.nodes, [
    { id: "a", layout: { x: 100, y: 0 } },
    { id: "b", layout: { x: 120, y: 60 } },
  ]);
  // Only the route between two nodes of the moved subtree moves with it.
  assert.deepEqual(placements.edges, [
    { index: 0, route: [{ x: 150, y: 50 }] },
  ]);
});

test("liftedEdges moves an edge within the lifted subtree whole and keeps a crossing edge's waypoints, re-anchored at its lifted end", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0 }),
        node("b", "b", { parent: "a", x: 20, y: 60 }),
        node("c", "c", { parent: "a" }),
        node("d", "d"),
      ],
      [
        {
          from: "b",
          to: "c",
          label: "",
          kind: "connection",
          fqn: "M::bc",
          route: [{ x: 50, y: 50 }],
        },
        {
          from: "b",
          to: "d",
          label: "",
          kind: "connection",
          fqn: "M::bd",
          route: [{ x: 70, y: 70 }],
        },
        { from: "d", to: "d", label: "", kind: "connection", fqn: "M::dd" },
      ],
    ),
  );
  const lifted = liftedEdges(layout, "a", 100, 30);
  // The edge between two nodes outside the subtree is not touched.
  assert.deepEqual(
    lifted.map((edge) => edge.index),
    [0, 1],
  );
  const [inner, crossing] = lifted;
  const shift = (points: { x: number; y: number }[]) =>
    points.map((p) => ({ x: p.x + 100, y: p.y + 30 }));
  assert.deepEqual(inner.points, shift(layout.edges[0].points));
  assert.deepEqual(inner.route, [{ x: 150, y: 80 }]);
  assert.deepEqual(inner.label, {
    x: layout.edges[0].label.x + 100,
    y: layout.edges[0].label.y + 30,
  });
  // The crossing edge keeps the model's waypoint and its anchor on d; its anchor on b moves with b.
  assert.deepEqual(crossing.route, [{ x: 70, y: 70 }]);
  assert.deepEqual(crossing.points.at(-1), layout.edges[1].points.at(-1));
  const b = layout.nodes.get("b")!.box;
  assert.deepEqual(
    crossing.points[0],
    anchor({ ...b, x: b.x + 100, y: b.y + 30 }, { x: 70, y: 70 }),
  );
  // A node the layout does not hold lifts no edge.
  assert.deepEqual(liftedEdges(layout, "z", 1, 1), []);
});

test("movedNode refuses a node no Layout can name", () => {
  const layout = layoutCanvas(rendering([node("a", "a", { fqn: undefined })]));
  assert.equal(movedNode(layout, "a", 1, 1), undefined);
  assert.equal(movedNode(layout, "missing", 1, 1), undefined);
});

test("movedWaypoint, insertedWaypoint and removedWaypoint edit one edge's route", () => {
  const edge: RenderEdge = {
    from: "a",
    to: "b",
    label: "",
    kind: "connection",
    fqn: "M::ab",
    route: [
      { x: 10, y: 10 },
      { x: 20, y: 20 },
    ],
  };
  const layout = layoutCanvas(
    rendering([node("a", "a"), node("b", "b")], [edge]),
  );
  assert.deepEqual(movedWaypoint(layout, 0, 1, { x: 30.6, y: 40.2 }), {
    nodes: [],
    edges: [
      {
        index: 0,
        route: [
          { x: 10, y: 10 },
          { x: 31, y: 40 },
        ],
      },
    ],
  });
  assert.equal(movedWaypoint(layout, 0, 2, { x: 0, y: 0 }), undefined);
  // Segment 1 runs from the first waypoint to the second; the new one goes between them.
  assert.deepEqual(insertedWaypoint(layout, 0, 1, { x: 15, y: 15 })!.edges, [
    {
      index: 0,
      route: [
        { x: 10, y: 10 },
        { x: 15, y: 15 },
        { x: 20, y: 20 },
      ],
    },
  ]);
  assert.deepEqual(insertedWaypoint(layout, 0, 0, { x: 5, y: 5 })!.edges, [
    {
      index: 0,
      route: [
        { x: 5, y: 5 },
        { x: 10, y: 10 },
        { x: 20, y: 20 },
      ],
    },
  ]);
  assert.equal(insertedWaypoint(layout, 0, 3, { x: 5, y: 5 }), undefined);
  assert.deepEqual(removedWaypoint(layout, 0, 0)!.edges, [
    { index: 0, route: [{ x: 20, y: 20 }] },
  ]);
  const single = layoutCanvas(
    rendering(
      [node("a", "a"), node("b", "b")],
      [{ ...edge, route: [{ x: 10, y: 10 }] }],
    ),
  );
  // Removing the last waypoint clears the route rather than leaving an empty one.
  assert.deepEqual(removedWaypoint(single, 0, 0)!.edges, [
    { index: 0, route: undefined },
  ]);
});

test("overridesOf previews a gesture: the moved node and route show where the drag has them", () => {
  const edge: RenderEdge = {
    from: "a",
    to: "b",
    label: "",
    kind: "connection",
    fqn: "M::ab",
    route: [{ x: 10, y: 10 }],
  };
  const result = rendering([node("a", "a"), node("b", "b")], [edge]);
  const layout = layoutCanvas(result);
  const preview = layoutCanvas(
    result,
    overridesOf({
      nodes: [{ id: "a", layout: { x: 400, y: 300 } }],
      edges: [{ index: 0, route: undefined }],
    }),
  );
  assert.deepEqual(
    [preview.nodes.get("a")!.box.x, preview.nodes.get("a")!.box.y],
    [400, 300],
  );
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
    routes: new Map([
      [
        0,
        [
          { x: 240, y: 80 },
          { x: 300, y: 80 },
          { x: 300, y: 200 },
          { x: 300, y: 230 },
        ],
      ],
    ]),
    ports: new Map(),
  };
  const result = rendering(
    [
      node("a", "a"),
      node("b", "b"),
      node("c", "c", { x: 50, y: 400, width: 90, height: 50 }),
    ],
    [
      { from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" },
      { from: "a", to: "c", label: "", kind: "connection", fqn: "M::ac" },
    ],
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
    const [from, to] = [
      layout.edges[1].points[i - 1],
      layout.edges[1].points[i],
    ];
    assert.ok(from.x === to.x || from.y === to.y);
  }
  // A gesture wins over both; held by the layout it started from, the pinned
  // end's edge stays straight for the drag and is routed again on the drop.
  const overrides = {
    ...overridesOf({
      nodes: [{ id: "a", layout: { x: 10, y: 10 } }],
      edges: [],
    }),
    held: layout,
  };
  const preview = layoutCanvas(result, overrides, auto);
  assert.deepEqual(
    [preview.nodes.get("a")!.box.x, preview.nodes.get("a")!.box.y],
    [10, 10],
  );
  assert.equal(preview.nodes.get("a")!.pinned, true);
  assert.equal(preview.edges[0].points.length, 2);
  assert.equal(preview.edges[0].rerouted, false);
});

function onBorder(
  point: { x: number; y: number },
  box: { x: number; y: number; width: number; height: number },
): boolean {
  const epsilon = 1e-7;
  const onVertical =
    (Math.abs(point.x - box.x) < epsilon ||
      Math.abs(point.x - (box.x + box.width)) < epsilon) &&
    point.y >= box.y - epsilon &&
    point.y <= box.y + box.height + epsilon;
  const onHorizontal =
    (Math.abs(point.y - box.y) < epsilon ||
      Math.abs(point.y - (box.y + box.height)) < epsilon) &&
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
      Math.max(Math.min(a.x, b.x), box.x) <
        Math.min(Math.max(a.x, b.x), box.x + box.width)
    );
  }
  return (
    a.x > box.x &&
    a.x < box.x + box.width &&
    Math.max(Math.min(a.y, b.y), box.y) <
      Math.min(Math.max(a.y, b.y), box.y + box.height)
  );
}

// lengthMidpoint is the point halfway along a polyline's length.
function lengthMidpoint(points: { x: number; y: number }[]): {
  x: number;
  y: number;
} {
  let total = 0;
  for (let i = 1; i < points.length; i++) {
    total += Math.hypot(
      points[i].x - points[i - 1].x,
      points[i].y - points[i - 1].y,
    );
  }
  let remaining = total / 2;
  for (let i = 1; i < points.length; i++) {
    const length = Math.hypot(
      points[i].x - points[i - 1].x,
      points[i].y - points[i - 1].y,
    );
    if (remaining <= length || i === points.length - 1) {
      const t = length === 0 ? 0 : remaining / length;
      return {
        x: points[i - 1].x + (points[i].x - points[i - 1].x) * t,
        y: points[i - 1].y + (points[i].y - points[i - 1].y) * t,
      };
    }
    remaining -= length;
  }
  return points[0] ?? { x: 0, y: 0 };
}

test("layoutCanvas routes a pinned node's edge orthogonally around the box between its ends", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
        node("b", "b", { x: 400, y: 200, width: 100, height: 40 }),
        node("c", "c", { x: 180, y: -20, width: 60, height: 80 }),
      ],
      [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
    ),
  );
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  assert.ok(edge.points.length > 2);
  assert.deepEqual(edge.route, edge.points.slice(1, -1));
  assert.ok(onBorder(edge.points[0], layout.nodes.get("a")!.box));
  assert.ok(onBorder(edge.points.at(-1)!, layout.nodes.get("b")!.box));
  const c = layout.nodes.get("c")!.box;
  const inflated = {
    x: c.x - CLEARANCE,
    y: c.y - CLEARANCE,
    width: c.width + 2 * CLEARANCE,
    height: c.height + 2 * CLEARANCE,
  };
  for (let i = 1; i < edge.points.length; i++) {
    const [from, to] = [edge.points[i - 1], edge.points[i]];
    assert.ok(
      from.x === to.x || from.y === to.y,
      `non-orthogonal segment: ${JSON.stringify([from, to])}`,
    );
    assert.ok(
      !crossesInterior(from, to, inflated),
      `segment crosses c: ${JSON.stringify([from, to])}`,
    );
  }
  assert.deepEqual(edge.label, lengthMidpoint(edge.points));
});

test("layoutCanvas leaves a stated route and an overridden straight edge alone", () => {
  const edge: RenderEdge = {
    from: "a",
    to: "b",
    label: "",
    kind: "connection",
    fqn: "M::ab",
    route: [{ x: 200, y: 200 }],
  };
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
  const straight = layoutCanvas(result, { routes: new Map([[0, undefined]]) })
    .edges[0];
  assert.equal(straight.rerouted, false);
  assert.equal(straight.points.length, 2);
  assert.deepEqual(straight.route, []);
});

test("liftedEdges draws a crossing rerouted edge straight and shifts an inner one whole", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
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
    ),
  );
  assert.ok(layout.edges.every((edge) => edge.rerouted));
  const lifted = liftedEdges(layout, "a", 50, 0);
  assert.deepEqual(
    lifted.map((edge) => edge.index),
    [0, 1],
  );
  const [inner, crossing] = lifted;
  // The inner edge keeps its rerouted waypoints, shifted with the subtree.
  assert.deepEqual(
    inner.route,
    layout.edges[0].route.map((p) => ({ x: p.x + 50, y: p.y })),
  );
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

test("held keeps the canvas it started from under a preview that draws less, and lets it grow", async () => {
  await loadAvoid(WASM);
  const result = rendering(
    [node("a", "a", { x: 0, y: 0, width: 100, height: 40 }), node("b", "b", { x: 400, y: 200, width: 100, height: 40 })],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  );
  const layout = layoutCanvas(result);
  const inward = overridesOf(movedNode(layout, "b", -200, -100)!);
  const shrunk = layoutCanvas(result, inward);
  assert.ok(shrunk.width < layout.width && shrunk.height < layout.height, "the moved node alone draws a smaller canvas");
  const preview = layoutCanvas(result, { ...inward, held: layout });
  assert.deepEqual(
    { origin: preview.origin, width: preview.width, height: preview.height },
    { origin: layout.origin, width: layout.width, height: layout.height },
    "the preview's canvas is the held layout's",
  );
  const outward = layoutCanvas(result, { ...overridesOf(movedNode(layout, "a", -50, -50)!), held: layout });
  assert.ok(outward.origin.x < layout.origin.x && outward.origin.y < layout.origin.y, "a preview beyond the held canvas grows it");
  assert.equal(outward.origin.x + outward.width, layout.origin.x + layout.width, "and still covers it");
});

test("movedNode writes no route for a rerouted edge, and movedWaypoint writes one", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0 }),
        node("b", "b", { parent: "a", x: 20, y: 60, width: 100, height: 40 }),
        node("c", "c", { parent: "a", x: 200, y: 200, width: 100, height: 40 }),
      ],
      [{ from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc" }],
    ),
  );
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
    return Math.max(
      0,
      Math.min(Math.max(a0.x, a1.x), Math.max(b0.x, b1.x)) -
        Math.max(Math.min(a0.x, a1.x), Math.min(b0.x, b1.x)),
    );
  }
  if (a0.x === a1.x && b0.x === b1.x && a0.x === b0.x) {
    return Math.max(
      0,
      Math.min(Math.max(a0.y, a1.y), Math.max(b0.y, b1.y)) -
        Math.max(Math.min(a0.y, a1.y), Math.min(b0.y, b1.y)),
    );
  }
  return 0;
}

test("layoutCanvas separates three edges into one target: no shared collinear segment over a pixel", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
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
    ),
  );
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
  const preview = layoutCanvas(result, {
    ...overridesOf(movedWaypoint(layout, 0, 0, { x: 33.4, y: 44.6 })!),
    held: layout,
  });
  assert.equal(preview.edges[0].rerouted, false);
  assert.deepEqual(preview.edges[0].route, [
    { x: 33, y: 45 },
    ...edge.route.slice(1),
  ]);
});

test("layoutCanvas routes a pinned node's edge around a populated container between its ends", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
        node("b", "b", { x: 600, y: 0, width: 100, height: 40 }),
        node("c", "c", { x: 220, y: -40, width: 240, height: 160 }),
        node("k", "k", { parent: "c", x: 240, y: -20, width: 80, height: 40 }),
      ],
      [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
    ),
  );
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  const c = layout.nodes.get("c")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(
      !crossesInterior(edge.points[i - 1], edge.points[i], c),
      `segment crosses c: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`,
    );
  }
});

test("layoutCanvas routes an edge out of its container and around another populated one", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
      [
        node("c", "c", { x: 0, y: 0 }),
        node("x", "x", { parent: "c", x: 20, y: 60, width: 100, height: 40 }),
        node("d", "d", { x: 500, y: 0, width: 200, height: 200 }),
        node("w", "w", { parent: "d", x: 520, y: 20, width: 60, height: 40 }),
        node("b", "b", { x: 800, y: 300, width: 100, height: 40 }),
      ],
      [{ from: "x", to: "b", label: "", kind: "connection", fqn: "M::xb" }],
    ),
  );
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  assert.ok(onBorder(edge.points[0], layout.nodes.get("x")!.box));
  assert.ok(onBorder(edge.points.at(-1)!, layout.nodes.get("b")!.box));
  const d = layout.nodes.get("d")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(
      !crossesInterior(edge.points[i - 1], edge.points[i], d),
      `segment crosses d: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`,
    );
  }
});

test("layoutCanvas keeps a child sticking out of its sized container an obstacle", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
        node("b", "b", { x: 600, y: 0, width: 100, height: 40 }),
        node("c", "c", { x: 200, y: 300, width: 100, height: 100 }),
        node("k", "k", { parent: "c", x: 300, y: 0, width: 100, height: 100 }),
      ],
      [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
    ),
  );
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  const k = layout.nodes.get("k")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(
      !crossesInterior(edge.points[i - 1], edge.points[i], k),
      `segment crosses k: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`,
    );
  }
});

test("layoutCanvas keeps a child partly protruding from its sized container an obstacle", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
        node("b", "b", { x: 600, y: 0, width: 100, height: 40 }),
        node("c", "c", { x: 200, y: 300, width: 200, height: 200 }),
        node("k", "k", { parent: "c", x: 300, y: 0, width: 100, height: 100 }),
      ],
      [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
    ),
  );
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  const k = layout.nodes.get("k")!.box;
  const c = layout.nodes.get("c")!.box;
  for (let i = 1; i < edge.points.length; i++) {
    assert.ok(
      !crossesInterior(edge.points[i - 1], edge.points[i], k),
      `segment crosses k: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`,
    );
    assert.ok(
      !crossesInterior(edge.points[i - 1], edge.points[i], c),
      `segment crosses c: ${JSON.stringify([edge.points[i - 1], edge.points[i]])}`,
    );
  }
});

test("layoutCanvas places edge ports on the sides facing the opposite endpoint", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", {
          x: 0,
          y: 0,
          width: 100,
          height: 60,
          ports: [{ id: "a.api", name: "api" }],
        }),
        node("b", "b", {
          x: 300,
          y: 0,
          width: 100,
          height: 60,
          ports: [{ id: "b.api", name: "api" }],
        }),
      ],
      [
        {
          from: "a",
          to: "b",
          fromPort: "a.api",
          toPort: "b.api",
          label: "",
          kind: "connection",
          fqn: "M::ab",
        },
      ],
    ),
  );
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  assert.deepEqual([a.ports[0].side, a.ports[0].offset], ["east", 0.5]);
  assert.deepEqual([b.ports[0].side, b.ports[0].offset], ["west", 0.5]);
  assert.deepEqual(
    [layout.edges[0].points[0], layout.edges[0].points.at(-1)],
    [portFace(a.box, a.ports[0]), portFace(b.box, b.ports[0])],
  );
  assert.deepEqual(portCenter(a.box, a.ports[0]), {
    x: a.box.x + a.box.width,
    y: a.box.y + a.box.height / 2,
  });
  const square = portBox(a.box, a.ports[0]);
  assert.equal(square.width, PORT_SIZE);
  assert.equal(square.height, PORT_SIZE);
});

test("clampNodeToBounds keeps port squares, labels, and pin exit legs inside its bounds", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", {
          x: 0,
          y: 0,
          width: 100,
          height: 60,
          ports: [{ id: "a.i", name: "i" }],
        }),
        node("b", "b", {
          x: 300,
          y: 0,
          width: 100,
          height: 60,
          ports: [{ id: "b.i", name: "i" }],
        }),
      ],
      [
        {
          from: "a",
          to: "b",
          fromPort: "a.i",
          toPort: "b.i",
          label: "",
          kind: "connection",
          fqn: "M::ab",
        },
      ],
    ),
  );
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
  const layout = layoutCanvas(
    rendering(
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
        {
          from: "west",
          to: "target",
          toPort: "target.in",
          label: "",
          kind: "connection",
          fqn: "M::westTarget",
        },
        {
          from: "target",
          to: "east",
          fromPort: "target.out",
          label: "",
          kind: "connection",
          fqn: "M::targetEast",
        },
      ],
    ),
  );
  const entry = layout.nodes.get("target")!;
  const west = entry.ports.find((port) => port.port.id === "target.in")!;
  const east = entry.ports.find((port) => port.port.id === "target.out")!;
  assert.equal(west.side, "west");
  assert.equal(east.side, "east");
  const exitLeg = (port: (typeof entry.ports)[number]) =>
    port === west ? 40 : 60;
  const extent = nodeExtent(entry, exitLeg);
  assert.equal(extent.x, portFace(entry.box, west).x - 40);
  assert.equal(extent.x + extent.width, portFace(entry.box, east).x + 60);

  const bounds = { x: 10, y: 0, width: 600, height: 300 };
  const at = clampNodeToBounds(
    entry,
    { x: -1000, y: entry.box.y },
    bounds,
    exitLeg,
  );
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

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), {
    x: moving.box.x,
    y: moving.box.y,
  });
});

test("freePlacement moves an overlap on the left to its nearest free side", () => {
  const moving = placedNode("moving", 100, 100);
  const other = placedNode("other", 150, 100);
  const bounds: Box = { x: 0, y: 0, width: 500, height: 300 };

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), {
    x: 38,
    y: 100,
  });
});

test("freePlacement respects a wall and chooses the other side of an overlap", () => {
  const moving = placedNode("moving", 100, 20);
  const other = placedNode("other", 70, 20);
  const bounds: Box = { x: 0, y: 0, width: 300, height: 80 };

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), {
    x: 182,
    y: 20,
  });
});

test("freePlacement includes each node's port exit leg in its extent", () => {
  const moving = placedNode("moving", 100, 100, 80, 40, [
    { id: "moving.out", name: "out" },
  ]);
  moving.ports[0].side = "east";
  const other = placedNode("other", 230, 100);
  const bounds: Box = { x: 0, y: 0, width: 500, height: 300 };
  const withNoExit = freePlacement(moving, moving.box, [other], bounds, 0);
  const withExit = freePlacement(
    moving,
    moving.box,
    [other],
    bounds,
    (entry, port) => {
      assert.equal(entry.node.id, "moving");
      assert.equal(port.port.id, "moving.out");
      return 40;
    },
  );

  assert.deepEqual(withNoExit, { x: 88, y: moving.box.y });
  assert.ok(withExit);
  assert.notDeepEqual(withExit, moving.box);
});

test("freePlacement returns its clamped position when no free spot exists", () => {
  const moving = placedNode("moving", 100, 20);
  const other = placedNode("other", 0, 0, 200, 80);
  const bounds: Box = { x: 0, y: 0, width: 200, height: 80 };

  assert.deepEqual(freePlacement(moving, moving.box, [other], bounds, 0), {
    x: moving.box.x,
    y: moving.box.y,
  });
});

test("freePlacement jumps past an overlap in the requested direction", () => {
  const moving = placedNode("moving", 100, 20, 40, 40);
  const other = placedNode("other", 110, 20, 40, 40);
  const bounds: Box = { x: 0, y: 0, width: 300, height: 100 };

  assert.deepEqual(
    freePlacement(moving, { x: 100, y: 20 }, [other], bounds, 0, {
      x: 1,
      y: 0,
    }),
    { x: 182, y: 20 },
  );
});

test("freePlacement leaves a directional step unchanged when that direction is blocked", () => {
  const moving = placedNode("moving", 100, 20, 40, 40);
  const other = placedNode("other", 110, 20, 40, 40);
  const bounds: Box = { x: 0, y: 0, width: 200, height: 100 };

  assert.equal(
    freePlacement(moving, { x: 100, y: 20 }, [other], bounds, 0, {
      x: 1,
      y: 0,
    }),
    undefined,
  );
});

test("alignedPlacement lines up a short east-to-west offset in either direction", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 400 };
  for (const offset of [5, -5]) {
    const movingNode = portNode("moving", 80, 100);
    const targetNode = portNode("target", 500, 100 + offset);
    const layout = layoutWithPorts(
      [movingNode, targetNode],
      [connectedEdge("moving", "target")],
      fixedPorts(["moving.api", "east", 0.5], ["target.api", "west", 0.5]),
      bounds,
    );
    const moving = layout.nodes.get("moving")!;
    const target = layout.nodes.get("target")!;
    const at = { x: moving.box.x, y: moving.box.y };
    const placed = alignedPlacement(moving, at, layout, bounds, 0);
    const ownCenter = portCenter({ ...moving.box, ...placed }, moving.ports[0]);
    const targetCenter = portCenter(target.box, target.ports[0]);

    assert.equal(placed.x, at.x);
    assert.equal(placed.y, at.y + offset);
    assert.equal(ownCenter.y, targetCenter.y);
  }
});

test("alignedPlacement ignores east ports wired to other east ports", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 400 };
  const layout = layoutWithPorts(
    [portNode("moving", 80, 100), portNode("target", 500, 105)],
    [connectedEdge("moving", "target")],
    fixedPorts(["moving.api", "east", 0.5], ["target.api", "east", 0.5]),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };

  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
});

test("alignedPlacement ignores a west port to the left of an east port", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 400 };
  const layout = layoutWithPorts(
    [portNode("moving", 500, 100), portNode("target", 100, 105)],
    [connectedEdge("moving", "target")],
    fixedPorts(["moving.api", "east", 0.5], ["target.api", "west", 0.5]),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };

  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
});

test("alignedPlacement ignores south-to-north ports that face apart", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 500 };
  const layout = layoutWithPorts(
    [portNode("moving", 80, 300), portNode("target", 85, 100)],
    [connectedEdge("moving", "target")],
    fixedPorts(["moving.api", "south", 0.5], ["target.api", "north", 0.5]),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };

  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
});

test("alignedPlacement leaves zero and exactly MIN_JOG offsets unchanged", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 400 };
  for (const offset of [0, MIN_JOG]) {
    const layout = layoutWithPorts(
      [portNode("moving", 80, 100), portNode("target", 500, 100 + offset)],
      [connectedEdge("moving", "target")],
      fixedPorts(["moving.api", "east", 0.5], ["target.api", "west", 0.5]),
      bounds,
    );
    const moving = layout.nodes.get("moving")!;
    const at = { x: moving.box.x, y: moving.box.y };

    assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
  }
});

test("alignedPlacement rejects an alignment blocked inside the clearance buffers", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 500 };
  const movingNode = portNode("moving", 80, 100);
  const targetNode = portNode("target", 500, 105);
  const edges = [connectedEdge("moving", "target")];
  const ports = fixedPorts(
    ["moving.api", "east", 0.5],
    ["target.api", "west", 0.5],
  );
  const initial = layoutWithPorts(
    [movingNode, targetNode],
    edges,
    ports,
    bounds,
  );
  const movingExtent = nodeExtent(initial.nodes.get("moving")!);
  const blocker = node("blocker", "blocker", {
    x: movingNode.x,
    y: movingExtent.y + movingExtent.height + 2 * CLEARANCE,
    width: movingNode.width,
    height: 40,
  });
  const layout = layoutWithPorts(
    [movingNode, targetNode, blocker],
    edges,
    ports,
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };
  const others = [...layout.nodes.values()].filter(
    (entry) => entry.node.id !== "moving",
  );

  assert.deepEqual(freePlacement(moving, at, others, bounds, 0), at);
  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
});

test("alignedPlacement rejects an alignment that would cross the bounds", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 260 };
  const layout = layoutWithPorts(
    [portNode("moving", 80, 190), portNode("target", 500, 195)],
    [connectedEdge("moving", "target")],
    fixedPorts(["moving.api", "east", 0.5], ["target.api", "west", 0.5]),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };
  const others = [...layout.nodes.values()].filter(
    (entry) => entry.node.id !== "moving",
  );

  assert.deepEqual(freePlacement(moving, at, others, bounds, 0), at);
  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
});

test("alignedPlacement minimizes remaining jogs before choosing the smaller shift", () => {
  const bounds: Box = { x: 0, y: 0, width: 1000, height: 400 };
  const layout = layoutWithPorts(
    [
      portNode("moving", 80, 100),
      portNode("plus", 450, 104),
      portNode("minus", 700, 94),
    ],
    [connectedEdge("moving", "plus"), connectedEdge("moving", "minus")],
    fixedPorts(
      ["moving.api", "east", 0.5],
      ["plus.api", "west", 0.5],
      ["minus.api", "west", 0.5],
    ),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };

  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), {
    x: at.x,
    y: at.y + 4,
  });
});

test("alignedPlacement lines up north-to-south ports on x", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 500 };
  const layout = layoutWithPorts(
    [portNode("moving", 80, 300), portNode("target", 85, 100)],
    [connectedEdge("moving", "target")],
    fixedPorts(["moving.api", "north", 0.5], ["target.api", "south", 0.5]),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };
  const placed = alignedPlacement(moving, at, layout, bounds, 0);

  assert.deepEqual(placed, { x: at.x + 5, y: at.y });
  assert.equal(
    portCenter({ ...moving.box, ...placed }, moving.ports[0]).x,
    portCenter(
      layout.nodes.get("target")!.box,
      layout.nodes.get("target")!.ports[0],
    ).x,
  );
});

test("alignedPlacement ignores mixed-side port pairs", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 400 };
  const layout = layoutWithPorts(
    [portNode("moving", 80, 100), portNode("target", 500, 105)],
    [connectedEdge("moving", "target")],
    fixedPorts(["moving.api", "east", 0.5], ["target.api", "north", 0.5]),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };

  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
});

test("alignedPlacement ignores hidden edges", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 400 };
  const makeLayout = (): CanvasLayout =>
    layoutWithPorts(
      [portNode("moving", 80, 100), portNode("target", 500, 105)],
      [connectedEdge("moving", "target")],
      fixedPorts(["moving.api", "east", 0.5], ["target.api", "west", 0.5]),
      bounds,
    );
  const hiddenEdgeLayout = makeLayout();
  hiddenEdgeLayout.edges[0].hidden = true;
  const hiddenEdgeMoving = hiddenEdgeLayout.nodes.get("moving")!;
  const hiddenEdgeAt = { x: hiddenEdgeMoving.box.x, y: hiddenEdgeMoving.box.y };
  assert.deepEqual(
    alignedPlacement(
      hiddenEdgeMoving,
      hiddenEdgeAt,
      hiddenEdgeLayout,
      bounds,
      0,
    ),
    hiddenEdgeAt,
  );

});

test("alignedPlacement preserves jog-length libavoid routes across seeded short offsets", async () => {
  await loadAvoid(WASM);
  const bounds: Box = { x: 0, y: 0, width: 800, height: 450 };
  const edges = [connectedEdge("moving", "target")];
  const ports = fixedPorts(
    ["moving.api", "east", 0.5],
    ["target.api", "west", 0.5],
  );
  let seed = 0x91b4c7d3;
  const randomInt = (max: number): number => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    return seed % (max + 1);
  };
  for (let sample = 0; sample < 200; sample++) {
    const offset = (randomInt(1) === 0 ? -1 : 1) * (1 + randomInt(MIN_JOG - 2));
    const targetY = 60 + randomInt(260);
    const nodes = [
      portNode("moving", 80, targetY - offset),
      portNode("target", 500, targetY),
    ];
    const initial = layoutWithPorts(nodes, edges, ports, bounds);
    const moving = initial.nodes.get("moving")!;
    const others = [...initial.nodes.values()].filter(
      (entry) => entry.node.id !== "moving",
    );
    const at = freePlacement(
      moving,
      moving.box,
      others,
      bounds,
      portExitReach(1),
    )!;
    const placed = alignedPlacement(
      moving,
      at,
      initial,
      bounds,
      portExitReach(1),
    );
    const layout = layoutCanvas(
      rendering(nodes, edges),
      { nodes: new Map([["moving", { ...moving.box, ...placed }]]), bounds },
      { nodes: new Map(), routes: new Map(), ports },
    );
    const edge = layout.edges[0];
    const source = layout.nodes.get("moving")!;
    const target = layout.nodes.get("target")!;

    assert.equal(
      portCenter(source.box, source.ports[0]).y,
      portCenter(target.box, target.ports[0]).y,
      `seeded scene ${sample}: ${JSON.stringify({ offset, targetY, at, placed, source: source.box, target: target.box })}`,
    );
    assert.equal(edge.rerouted, true);
    for (let index = 1; index < edge.points.length; index++) {
      assert.ok(
        Math.hypot(
          edge.points[index].x - edge.points[index - 1].x,
          edge.points[index].y - edge.points[index - 1].y,
        ) >= MIN_JOG,
        `seeded scene ${sample} had a short route segment: ${JSON.stringify(edge.points)}`,
      );
    }
  }
});

test("freePlacement finds the nearest free position in the off-grid review scene", () => {
  const bounds: Box = { x: 0, y: 0, width: 180, height: 120 };
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
    assert.ok(
      freeAt(moving, result, others),
      `placement is occupied in seeded scene ${sample}`,
    );
    assert.ok(
      Math.hypot(result.x - at.x, result.y - at.y) <=
        Math.hypot(bruteForce.x - at.x, bruteForce.y - at.y) + 1,
      `placement is not nearest in seeded scene ${sample}`,
    );
  }
  assert.ok(checked > 100, `only ${checked} scenes had a free spot`);
});

test("reattachRoute moves a start perpendicular without changing its exit leg", () => {
  const route = [
    { x: 0, y: 0 },
    { x: 20, y: 0 },
    { x: 20, y: 40 },
    { x: 80, y: 40 },
  ];
  const start = { x: 0, y: 5 };
  const end = route.at(-1)!;
  const reattached = reattachRoute(route, start, end);

  assert.ok(orthogonal(reattached));
  assertPointNear(reattached[0], start, "start");
  assertPointNear(reattached.at(-1)!, end, "end");
  assert.equal(
    Math.hypot(
      reattached[1].x - reattached[0].x,
      reattached[1].y - reattached[0].y,
    ),
    20,
  );
});

test("reattachRoute moves the first two bends along the start axis to preserve its exit leg", () => {
  const route = [
    { x: 0, y: 0 },
    { x: 20, y: 0 },
    { x: 20, y: 40 },
    { x: 80, y: 40 },
  ];
  const start = { x: 5, y: 0 };
  const end = route.at(-1)!;
  const reattached = reattachRoute(route, start, end);

  assert.ok(orthogonal(reattached));
  assertPointNear(reattached[0], start, "start");
  assertPointNear(reattached.at(-1)!, end, "end");
  assert.equal(
    Math.hypot(
      reattached[1].x - reattached[0].x,
      reattached[1].y - reattached[0].y,
    ),
    20,
  );
});

test("reattachRoute makes a straight route into an orthogonal Z when its ends are no longer level", () => {
  const horizontal = reattachRoute(
    [
      { x: 0, y: 0 },
      { x: 100, y: 0 },
    ],
    { x: 10, y: 20 },
    { x: 100, y: 0 },
  );
  assert.ok(orthogonal(horizontal));
  assertPointNear(horizontal[0], { x: 10, y: 20 }, "horizontal start");
  assertPointNear(horizontal.at(-1)!, { x: 100, y: 0 }, "horizontal end");
  assert.equal(horizontal.length, 4);

  const vertical = reattachRoute(
    [
      { x: 10, y: 0 },
      { x: 10, y: 100 },
    ],
    { x: 0, y: 10 },
    { x: 30, y: 100 },
  );
  assert.ok(orthogonal(vertical));
  assertPointNear(vertical[0], { x: 0, y: 10 }, "vertical start");
  assertPointNear(vertical.at(-1)!, { x: 30, y: 100 }, "vertical end");
  assert.equal(vertical.length, 4);
});

test("reattachRoute keeps an L orthogonal when its start moves perpendicular", () => {
  const start = { x: 0, y: 10 };
  const end = { x: 40, y: 60 };
  const reattached = reattachRoute(
    [{ x: 0, y: 0 }, { x: 40, y: 0 }, end],
    start,
    end,
  );

  assert.ok(orthogonal(reattached));
  assertPointNear(reattached[0], start, "start");
  assertPointNear(reattached.at(-1)!, end, "end");
});

test("reattachRoute keeps both moved ends orthogonal", () => {
  const start = { x: 0, y: 10 };
  const end = { x: 70, y: 80 };
  const reattached = reattachRoute(
    [
      { x: 0, y: 0 },
      { x: 20, y: 0 },
      { x: 20, y: 30 },
      { x: 60, y: 30 },
      { x: 60, y: 70 },
    ],
    start,
    end,
  );

  assert.ok(orthogonal(reattached));
  assertPointNear(reattached[0], start, "start");
  assertPointNear(reattached.at(-1)!, end, "end");
});

test("reattachRoute leaves a non-orthogonal input unchanged", () => {
  const route = [
    { x: 0, y: 0 },
    { x: 10, y: 5 },
    { x: 20, y: 0 },
  ];
  assert.equal(reattachRoute(route, { x: 0, y: 10 }, { x: 20, y: 10 }), route);
});

test("reattachRoute keeps a three-point horizontal exit pointing out after the start passes its bend", () => {
  const route = [
    { x: 100, y: 100 },
    { x: 120, y: 100 },
    { x: 120, y: 200 },
  ];
  const start = { x: 150, y: 100 };
  const end = route.at(-1)!;
  const reattached = reattachRoute(route, start, end);

  assertRouteDirections(
    reattached,
    start,
    end,
    { x: 1, y: 0 },
    { x: 0, y: -1 },
  );
});

test("reattachRoute keeps a three-point vertical exit pointing out after the start passes its bend", () => {
  const route = [
    { x: 100, y: 100 },
    { x: 100, y: 120 },
    { x: 200, y: 120 },
  ];
  const start = { x: 100, y: 150 };
  const end = route.at(-1)!;
  const reattached = reattachRoute(route, start, end);

  assertRouteDirections(
    reattached,
    start,
    end,
    { x: 0, y: 1 },
    { x: -1, y: 0 },
  );
});

test("reattachRoute keeps a three-point end pointing into its port after the end passes its bend", () => {
  const route = [
    { x: 100, y: 100 },
    { x: 100, y: 200 },
    { x: 120, y: 200 },
  ];
  const start = route[0];
  const end = { x: 80, y: 200 };
  const reattached = reattachRoute(route, start, end);

  assertRouteDirections(
    reattached,
    start,
    end,
    { x: 0, y: 1 },
    { x: -1, y: 0 },
  );
});

test("reattachRoute preserves outward directions when a four-point Z start passes its middle bend", () => {
  const route = [
    { x: 100, y: 100 },
    { x: 120, y: 100 },
    { x: 120, y: 150 },
    { x: 180, y: 150 },
  ];
  const start = { x: 130, y: 100 };
  const end = route.at(-1)!;
  const reattached = reattachRoute(route, start, end);

  assertRouteDirections(
    reattached,
    start,
    end,
    { x: 1, y: 0 },
    { x: -1, y: 0 },
  );
});

test("reattachRoute goes around vertically when a straight route's start passes its end", () => {
  const start = { x: 250, y: 120 };
  const end = { x: 200, y: 100 };
  const reattached = reattachRoute(
    [
      { x: 100, y: 100 },
      { x: 200, y: 100 },
    ],
    start,
    end,
  );

  assertRouteDirections(
    reattached,
    start,
    end,
    { x: 1, y: 0 },
    { x: -1, y: 0 },
  );
  const a = { x: start.x + CLEARANCE, y: start.y };
  const b = { x: end.x - CLEARANCE, y: end.y };
  const middleY = (a.y + b.y) / 2;
  assert.deepEqual(reattached, [
    start,
    a,
    { x: a.x, y: middleY },
    { x: b.x, y: middleY },
    b,
    end,
  ]);
});

test("keepOrthogonalRoutes replaces only diagonals with matching visible orthogonal routes", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 500 };
  const nodes = [
    portNode("a", 80, 100),
    portNode("b", 500, 100),
    portNode("c", 500, 300),
  ];
  const nextEdges = [
    connectedEdge("a", "b"),
    connectedEdge("a", "b"),
    connectedEdge("a", "b"),
    connectedEdge("a", "b"),
    connectedEdge("a", "b"),
    connectedEdge("a", "c"),
  ];
  const previousEdges = [...nextEdges];
  previousEdges[5] = connectedEdge("a", "b");
  const ports = fixedPorts(
    ["a.api", "east", 0.5],
    ["b.api", "west", 0.5],
    ["c.api", "west", 0.5],
  );
  const previous = layoutWithPorts(nodes, previousEdges, ports, bounds);
  const next = layoutWithPorts(nodes, nextEdges, ports, bounds);
  const endpoints = (
    layout: CanvasLayout,
    index: number,
  ): [RenderPoint, RenderPoint] => {
    const edge = layout.edges[index];
    const source = layout.nodes.get(edge.edge.from)!;
    const target = layout.nodes.get(edge.edge.to)!;
    return [
      portFace(source.box, source.ports[0]),
      portFace(target.box, target.ports[0]),
    ];
  };
  const setPoints = (
    layout: CanvasLayout,
    index: number,
    points: RenderPoint[],
    hidden?: boolean,
  ): void => {
    layout.edges[index] = {
      ...layout.edges[index],
      points,
      route: points.slice(1, -1),
      hidden: hidden ?? layout.edges[index].hidden,
    };
  };
  const [aToBStart, aToBEnd] = endpoints(next, 0);
  const [aToCStart, aToCEnd] = endpoints(next, 5);
  const diagonal = (start: RenderPoint, end: RenderPoint): RenderPoint[] => [
    start,
    { x: (start.x + end.x) / 2, y: start.y + 20 },
    end,
  ];

  setPoints(previous, 0, [aToBStart, aToBEnd]);
  setPoints(next, 0, diagonal(aToBStart, aToBEnd));
  setPoints(previous, 1, [aToBStart, aToBEnd]);
  setPoints(next, 1, [
    aToBStart,
    { x: 300, y: aToBStart.y },
    { x: 300, y: 180 },
    { x: aToBEnd.x, y: 180 },
    aToBEnd,
  ]);
  setPoints(previous, 2, diagonal(aToBStart, aToBEnd));
  setPoints(next, 2, diagonal(aToBStart, aToBEnd));
  setPoints(previous, 3, [aToBStart, aToBEnd]);
  setPoints(next, 3, diagonal(aToBStart, aToBEnd), true);
  setPoints(previous, 4, [aToBStart, aToBEnd], true);
  setPoints(next, 4, diagonal(aToBStart, aToBEnd));
  setPoints(previous, 5, [aToBStart, aToBEnd]);
  setPoints(next, 5, diagonal(aToCStart, aToCEnd));

  const kept = keepOrthogonalRoutes(next, previous);
  const replacement = kept.edges[0];
  assert.ok(orthogonal(replacement.points));
  assert.deepEqual(replacement.points, [aToBStart, aToBEnd]);
  assert.deepEqual(replacement.route, []);
  assert.deepEqual(replacement.label, {
    x: (aToBStart.x + aToBEnd.x) / 2,
    y: aToBStart.y,
  });
  assert.equal(replacement.rerouted, true);
  for (const index of [1, 2, 3, 4, 5]) {
    assert.equal(
      kept.edges[index],
      next.edges[index],
      `edge ${index} should stay unchanged`,
    );
  }
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
    [
      {
        from: "source",
        to: "target",
        fromPort: "source.out",
        toPort: "target.in",
        label: "",
        kind: "connection",
        fqn: "M::edge",
      },
    ],
  );
  const unbounded = layoutCanvas(result);
  assert.ok(
    unbounded.edges[0].points.some(
      ({ x, y }) =>
        x < bounds.x ||
        y < bounds.y ||
        x > bounds.x + bounds.width ||
        y > bounds.y + bounds.height,
    ),
  );

  const bounded = layoutCanvas(result, { bounds });
  const route = bounded.edges[0].points;
  const source = bounded.nodes.get("source")!;
  const target = bounded.nodes.get("target")!;
  assert.equal(bounded.edges[0].rerouted, true);
  assert.deepEqual(route[0], portFace(source.box, source.ports[0]));
  assert.deepEqual(route.at(-1), portFace(target.box, target.ports[0]));
  assert.ok(
    route.every(
      ({ x, y }) =>
        x >= bounds.x &&
        y >= bounds.y &&
        x <= bounds.x + bounds.width &&
        y <= bounds.y + bounds.height,
    ),
  );
  for (let index = 1; index < route.length; index++) {
    assert.ok(
      route[index - 1].x === route[index].x ||
        route[index - 1].y === route[index].y,
    );
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
      {
        from: "source",
        to: "moving",
        fromPort: "source.out",
        toPort: "moving.in",
        label: "",
        kind: "connection",
        fqn: "M::sourceMoving",
      },
      {
        from: "moving",
        to: "target",
        fromPort: "moving.out",
        toPort: "target.in",
        label: "",
        kind: "connection",
        fqn: "M::movingTarget",
      },
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

  const after = layoutCanvas(result, {
    nodes: new Map([["moving", at]]),
    bounds,
  });
  for (const edge of after.edges) {
    const source = after.nodes.get(edge.edge.from)!;
    const target = after.nodes.get(edge.edge.to)!;
    const sourcePort = source.ports.find(
      (port) => port.port.id === edge.edge.fromPort,
    )!;
    const targetPort = target.ports.find(
      (port) => port.port.id === edge.edge.toPort,
    )!;
    assert.equal(edge.rerouted, true);
    assert.deepEqual(edge.points[0], portFace(source.box, sourcePort));
    assert.deepEqual(edge.points.at(-1), portFace(target.box, targetPort));
    for (let index = 1; index < edge.points.length; index++) {
      assert.ok(
        edge.points[index - 1].x === edge.points[index].x ||
          edge.points[index - 1].y === edge.points[index].y,
      );
    }
  }
});

test("landing drag keeps right-angled routes at the captured Pilot obstruction", async () => {
  await loadAvoid(WASM);
  const fixture = JSON.parse(
    readFileSync("src/landing/stack.json", "utf8"),
  ) as LandingFixture;
  const model = landingModel(
    sceneTarget(STACK_SCENE),
    fixture.hash,
    renderResult(renderedViewFromJson(fixture.render)),
    fixture.instances,
  );
  const result = presentedStack(model);
  const auto = await autoLayout(result);
  assert.ok(auto, "ELK should lay out the saved landing render");

  const bounds: Box = { x: -600, y: -900, width: 3000, height: 1800 };
  // Pilot's port faces Flexo across OpenSysML, which sits between them.
  const positions: Record<string, RenderPoint> = {
    opensysml: { x: 500, y: 200 },
    pilot: { x: -240, y: 80 },
    flexo: { x: 1331, y: 180 },
  };
  const idOf = (feature: string): string => model.parts.get(feature)!.id;
  const layoutAt = (pilotX: number): CanvasLayout => {
    const nodes = new Map<string, LayoutGeometry>();
    for (const [feature, position] of Object.entries(positions)) {
      const geometry = auto.nodes.get(idOf(feature));
      assert.ok(geometry, `ELK should place landing box ${feature}`);
      nodes.set(idOf(feature), {
        ...geometry,
        x: feature === "pilot" ? pilotX : position.x,
        y: position.y,
      });
    }
    return layoutCanvas(
      result,
      { nodes: carried(result.nodes, auto, nodes), bounds },
      auto,
    );
  };
  const next = layoutAt(positions.pilot.x);
  const pilotEdge = next.edges.find(
    ({ edge }) => edge.from === idOf("pilot") && edge.to === idOf("flexo"),
  );
  assert.ok(pilotEdge);
  assert.ok(
    !orthogonal(pilotEdge.points),
    "the captured Pilot position should reproduce its diagonal route",
  );

  const previous = layoutAt(positions.pilot.x - 40);
  for (const edge of previous.edges) {
    assert.ok(
      orthogonal(edge.points),
      `the prior Pilot position should route ${edge.edge.from} → ${edge.edge.to} orthogonally`,
    );
  }

  const kept = keepOrthogonalRoutes(next, previous);
  for (const edge of kept.edges) {
    assert.ok(
      orthogonal(edge.points),
      `kept route ${edge.edge.from} → ${edge.edge.to} should be orthogonal`,
    );
    const source = kept.nodes.get(edge.edge.from)!;
    const target = kept.nodes.get(edge.edge.to)!;
    const sourcePort = source.ports.find(
      (port) => port.port.id === edge.edge.fromPort,
    );
    const targetPort = target.ports.find(
      (port) => port.port.id === edge.edge.toPort,
    );
    assert.ok(sourcePort && targetPort);
    assertPointNear(
      edge.points[0],
      portFace(source.box, sourcePort),
      `${edge.edge.from} port face`,
    );
    assertPointNear(
      edge.points.at(-1)!,
      portFace(target.box, targetPort),
      `${edge.edge.to} port face`,
    );
  }
});

test("landing stacked-drop repro starts with diagonal routes", async () => {
  await loadAvoid(WASM);
  const boxes: Record<string, Box> = {
    n1: { x: 133, y: 240, width: 159, height: 70 },
    n7: { x: -206.9174346923828, y: 331.2325134277344, width: 189, height: 70 },
    n13: { x: 36, y: 36, width: 256, height: 70 },
    n19: {
      x: -169.9174346923828,
      y: 417.2325134277344,
      width: 234,
      height: 70,
    },
  };
  const names: Record<string, string> = {
    n1: "OpenSysML",
    n7: "sysml-toolkit",
    n13: "SysML v2 Pilot Implementation",
    n19: "Flexo MMS",
  };
  const nodes = Object.entries(boxes).map(([id, box]) =>
    node(id, names[id], { ...box, ports: [{ id: `${id}.api`, name: "api" }] }),
  );
  const edges = [
    {
      from: "n13",
      to: "n19",
      fromPort: "n13.api",
      toPort: "n19.api",
      label: "",
      kind: "connection",
      fqn: "M::n13n19",
    },
    {
      from: "n1",
      to: "n19",
      fromPort: "n1.api",
      toPort: "n19.api",
      label: "",
      kind: "connection",
      fqn: "M::n1n19",
    },
    {
      from: "n7",
      to: "n19",
      fromPort: "n7.api",
      toPort: "n19.api",
      label: "",
      kind: "connection",
      fqn: "M::n7n19",
    },
  ] as const;
  const ports: AutoLayout["ports"] = new Map();
  ports.set("n1.api", { side: "east", offset: 0.5 });
  ports.set("n7.api", { side: "east", offset: 0.5 });
  ports.set("n13.api", { side: "east", offset: 0.5 });
  ports.set("n19.api", { side: "west", offset: 0.5 });
  const auto: AutoLayout = { nodes: new Map(), routes: new Map(), ports };
  const bounds: Box = {
    x: -213.08255948571724,
    y: -413.64820438139134,
    width: 1098.1650895737687,
    height: 907.2445130634958,
  };
  const result = rendering(nodes, [...edges]);
  const stacked = layoutCanvas(result, { bounds }, auto);
  const exitReach = (entry: PlacedNode, port: PlacedPort): number => {
    const sharing = edges.filter(
      (edge) =>
        (edge.from === entry.node.id && edge.fromPort === port.port.id) ||
        (edge.to === entry.node.id && edge.toPort === port.port.id),
    ).length;
    return portExitReach(sharing);
  };

  assert.ok(
    stacked.edges.some(({ points }) =>
      points.some(
        (point, index) =>
          index > 0 &&
          point.x !== points[index - 1].x &&
          point.y !== points[index - 1].y,
      ),
    ),
    "the stacked layout should reproduce at least one diagonal fallback route",
  );

  const settled = new Map<string, LayoutGeometry>();
  for (const id of ["n7", "n19"]) {
    const current = layoutCanvas(result, { nodes: settled, bounds }, auto);
    const moving = current.nodes.get(id)!;
    const others = [...current.nodes.values()].filter(
      (entry) => entry.node.id !== id,
    );
    const at = freePlacement(
      moving,
      moving.box,
      others,
      insetBox(bounds, CLEARANCE),
      exitReach,
    );

    assert.ok(at, `expected ${id} to have a free placement`);
    assert.ok(
      freeAt(moving, at, others, exitReach),
      `expected ${id} to be clear of both routing buffers`,
    );
    settled.set(id, at);
  }
  assertOrthogonalRoutes(
    layoutCanvas(result, { nodes: settled, bounds }, auto),
    bounds,
  );
});

test("freePlacement keeps a seeded four-node chain routable after drops", async () => {
  await loadAvoid(WASM);
  const bounds: Box = { x: 0, y: 0, width: 700, height: 400 };
  const result = rendering(
    [
      node("a", "a", {
        x: 40,
        y: 180,
        width: 80,
        height: 40,
        ports: [{ id: "a.out", name: "out" }],
      }),
      node("b", "b", {
        x: 220,
        y: 180,
        width: 80,
        height: 40,
        ports: [
          { id: "b.in", name: "in" },
          { id: "b.out", name: "out" },
        ],
      }),
      node("c", "c", {
        x: 400,
        y: 180,
        width: 80,
        height: 40,
        ports: [
          { id: "c.in", name: "in" },
          { id: "c.out", name: "out" },
        ],
      }),
      node("d", "d", {
        x: 580,
        y: 180,
        width: 80,
        height: 40,
        ports: [{ id: "d.in", name: "in" }],
      }),
    ],
    [
      {
        from: "a",
        to: "b",
        fromPort: "a.out",
        toPort: "b.in",
        label: "",
        kind: "connection",
        fqn: "M::ab",
      },
      {
        from: "b",
        to: "c",
        fromPort: "b.out",
        toPort: "c.in",
        label: "",
        kind: "connection",
        fqn: "M::bc",
      },
      {
        from: "c",
        to: "d",
        fromPort: "c.out",
        toPort: "d.in",
        label: "",
        kind: "connection",
        fqn: "M::cd",
      },
    ],
  );
  const auto: AutoLayout = {
    nodes: new Map(),
    routes: new Map(),
    ports: new Map([
      ["a.out", { side: "east", offset: 0.5 }],
      ["b.in", { side: "west", offset: 0.5 }],
      ["b.out", { side: "east", offset: 0.5 }],
      ["c.in", { side: "west", offset: 0.5 }],
      ["c.out", { side: "east", offset: 0.5 }],
      ["d.in", { side: "west", offset: 0.5 }],
    ]),
  };
  let seed = 0x7a4d39b1;
  const randomInt = (max: number): number => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    return seed % (max + 1);
  };
  const inset = insetBox(bounds, CLEARANCE);
  const exitReach = () => portExitReach(1);
  let checked = 0;
  for (let sample = 0; sample < 200; sample++) {
    const settled = new Map<string, LayoutGeometry>();
    let current = layoutCanvas(result, { bounds }, auto);
    let allFree = true;
    for (const id of ["a", "b", "c", "d"]) {
      const moving = current.nodes.get(id)!;
      const others = [...current.nodes.values()].filter(
        (entry) => entry.node.id !== id,
      );
      const at = { x: randomInt(620), y: randomInt(320) };
      const placed = freePlacement(moving, at, others, inset, exitReach);
      if (!placed || !freeAt(moving, placed, others, exitReach)) {
        allFree = false;
        break;
      }
      settled.set(id, placed);
      current = layoutCanvas(result, { nodes: settled, bounds }, auto);
    }
    if (allFree) {
      checked++;
      const boxes = [...current.nodes.values()].map(
        ({ node: { id }, box }) => ({ id, box }),
      );
      assertOrthogonalRoutes(
        current,
        bounds,
        `seeded scene ${sample}: ${JSON.stringify(boxes)}`,
      );
    }
  }
  assert.ok(
    checked > 0,
    "expected seeded scenes where every drop found a free position",
  );
});

test("layoutCanvas routes a lower-left sender into a west port from outside its face", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
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
      [
        {
          from: "sender",
          to: "receiver",
          fromPort: "sender.out1",
          toPort: "receiver.in1",
          label: "wire",
          kind: "connection",
          fqn: "M::sender_receiver",
        },
      ],
    ),
  );
  const receiver = layout.nodes.get("receiver")!;
  const edge = layout.edges[0];
  const face = portFace(receiver.box, receiver.ports[0]);
  assert.equal(receiver.ports[0].side, "west");
  assert.deepEqual(edge.points.at(-1), face);
  const beforeFace = edge.points.at(-2)!;
  assert.equal(beforeFace.y, face.y);
  assert.ok(
    beforeFace.x < face.x,
    `receiver port is not approached from the left: ${JSON.stringify(edge.points)}`,
  );
  for (let i = 1; i < edge.points.length; i++) {
    for (const [borderStart, borderEnd] of [
      [
        { x: receiver.box.x, y: receiver.box.y },
        { x: receiver.box.x + receiver.box.width, y: receiver.box.y },
      ],
      [
        { x: receiver.box.x + receiver.box.width, y: receiver.box.y },
        {
          x: receiver.box.x + receiver.box.width,
          y: receiver.box.y + receiver.box.height,
        },
      ],
      [
        {
          x: receiver.box.x + receiver.box.width,
          y: receiver.box.y + receiver.box.height,
        },
        { x: receiver.box.x, y: receiver.box.y + receiver.box.height },
      ],
      [
        { x: receiver.box.x, y: receiver.box.y + receiver.box.height },
        { x: receiver.box.x, y: receiver.box.y },
      ],
    ] as const) {
      assert.ok(
        overlapLength(
          edge.points[i - 1],
          edge.points[i],
          borderStart,
          borderEnd,
        ) <= 1,
        `route runs along receiver border: ${JSON.stringify(edge.points)}`,
      );
    }
  }
});

test("layoutCanvas spreads defaulted ports on a side in their source order", () => {
  const ports = ["first", "second", "third"].map((id) => ({ id, name: id }));
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0, width: 100, height: 100, ports }),
        node("b", "b", { x: 300, y: 0, width: 80, height: 40 }),
        node("c", "c", { x: 300, y: 100, width: 80, height: 40 }),
        node("d", "d", { x: 300, y: 200, width: 80, height: 40 }),
      ],
      [
        {
          from: "a",
          to: "b",
          fromPort: "first",
          label: "",
          kind: "connection",
          fqn: "M::ab",
        },
        {
          from: "a",
          to: "c",
          fromPort: "second",
          label: "",
          kind: "connection",
          fqn: "M::ac",
        },
        {
          from: "a",
          to: "d",
          fromPort: "third",
          label: "",
          kind: "connection",
          fqn: "M::ad",
        },
      ],
    ),
  );
  assert.deepEqual(
    layout.nodes.get("a")!.ports.map(({ side, offset }) => [side, offset]),
    [
      ["east", 0.25],
      ["east", 0.5],
      ["east", 0.75],
    ],
  );
});

test("layoutCanvas falls back to the node anchor for a missing endpoint port", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", {
          x: 0,
          y: 0,
          width: 100,
          height: 60,
          ports: [{ id: "a.api", name: "api" }],
        }),
        node("b", "b", { x: 300, y: 0, width: 100, height: 60 }),
      ],
      [
        {
          from: "a",
          to: "b",
          fromPort: "missing",
          label: "",
          kind: "connection",
          fqn: "M::ab",
          route: [{ x: 150, y: 30 }],
        },
      ],
    ),
  );
  const source = layout.nodes.get("a")!.box;
  const target = layout.nodes.get("b")!.box;
  assert.deepEqual(
    layout.edges[0].points[0],
    anchor(source, { x: 150, y: 30 }),
  );
  assert.deepEqual(
    layout.edges[0].points.at(-1),
    anchor(target, { x: 150, y: 30 }),
  );
});

test("layoutCanvas preserves an automatic port placement when its node moves", () => {
  const result = rendering(
    [
      node("a", "a", {
        x: 0,
        y: 0,
        width: 100,
        height: 60,
        ports: [{ id: "a.api", name: "api" }],
      }),
      node("b", "b", { x: 300, y: 0, width: 100, height: 60 }),
    ],
    [
      {
        from: "a",
        to: "b",
        fromPort: "a.api",
        label: "",
        kind: "connection",
        fqn: "M::ab",
      },
    ],
  );
  const auto: AutoLayout = {
    nodes: new Map(),
    routes: new Map(),
    ports: new Map([["a.api", { side: "north", offset: 0.2 }]]),
  };
  const layout = layoutCanvas(
    result,
    { nodes: new Map([["a", { x: 400, y: 250 }]]) },
    auto,
  );
  const source = layout.nodes.get("a")!;
  assert.deepEqual(
    [source.ports[0].side, source.ports[0].offset],
    ["north", 0.2],
  );
  assert.deepEqual(
    layout.edges[0].points[0],
    portFace(source.box, source.ports[0]),
  );
});

test("liftedEdges recalculates a port anchor from the shifted node box", () => {
  const result = rendering(
    [
      node("a", "a", {
        x: 0,
        y: 0,
        width: 100,
        height: 60,
        ports: [{ id: "a.api", name: "api" }],
      }),
      node("b", "b", {
        x: 300,
        y: 0,
        width: 100,
        height: 60,
        ports: [{ id: "b.api", name: "api" }],
      }),
    ],
    [
      {
        from: "a",
        to: "b",
        fromPort: "a.api",
        toPort: "b.api",
        label: "",
        kind: "connection",
        fqn: "M::ab",
      },
    ],
  );
  const layout = layoutCanvas(result);
  const source = layout.nodes.get("a")!;
  const lifted = liftedEdges(layout, "a", 40, 25)[0];
  const shiftedBox = {
    ...source.box,
    x: source.box.x + 40,
    y: source.box.y + 25,
  };
  assert.deepEqual(lifted.points[0], portFace(shiftedBox, source.ports[0]));
});

test("obstacles leaves a moving project clear of the boxes drawn inside it", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 100, y: 100, width: 200, height: 100 }),
      node("c", "c", { parent: "a", x: 116, y: 116, width: 60, height: 30 }),
      node("b", "b", { x: 500, y: 100, width: 80, height: 40 }),
    ]),
  );
  const a = layout.nodes.get("a")!;
  const bounds = { x: 0, y: 0, width: 1000, height: 600 };
  const clear = obstacles(layout.nodes.values(), "a");
  assert.deepEqual(
    clear.map((entry) => entry.node.id),
    ["b"],
  );
  assert.deepEqual(freePlacement(a, { x: 110, y: 100 }, clear, bounds, 0), {
    x: 110,
    y: 100,
  });
  const everyOther = [...layout.nodes.values()].filter(
    (entry) => entry.node.id !== "a",
  );
  assert.notDeepEqual(
    freePlacement(a, { x: 110, y: 100 }, everyOther, bounds, 0),
    { x: 110, y: 100 },
  );
});

test("obstacles keeps another project's box that sticks out past its project", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 100, y: 100, width: 200, height: 100 }),
      node("b", "b", { x: 500, y: 100, width: 80, height: 40 }),
      node("inside", "inside", {
        parent: "b",
        x: 510,
        y: 110,
        width: 20,
        height: 10,
      }),
      node("exposed", "exposed", {
        parent: "b",
        x: 560,
        y: 110,
        width: 60,
        height: 10,
      }),
    ]),
  );
  const ids = obstacles(layout.nodes.values(), "a").map(
    (entry) => entry.node.id,
  );
  assert.deepEqual(ids, ["b", "exposed"]);
});

test("alignedPlacement lines up a project whose own boxes are not obstacles", () => {
  const bounds: Box = { x: 0, y: 0, width: 900, height: 400 };
  const movingNode = portNode("moving", 80, 100);
  const inner = node("inner", "inner", {
    parent: "moving",
    x: 90,
    y: 110,
    width: 20,
    height: 10,
  });
  const targetNode = portNode("target", 500, 105);
  const layout = layoutWithPorts(
    [movingNode, inner, targetNode],
    [connectedEdge("moving", "target")],
    fixedPorts(["moving.api", "east", 0.5], ["target.api", "west", 0.5]),
    bounds,
  );
  const moving = layout.nodes.get("moving")!;
  const at = { x: moving.box.x, y: moving.box.y };
  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0), at);
  const clear = obstacles(layout.nodes.values(), "moving");
  assert.deepEqual(alignedPlacement(moving, at, layout, bounds, 0, clear), {
    x: at.x,
    y: at.y + 5,
  });
});

test("resettle carries a re-clamped project's boxes, so later projects stay clear of them", () => {
  const geometry = {
    a: { x: 100, y: 100, width: 100, height: 50 },
    exposed: { x: 190, y: 110, width: 40, height: 20 },
    b: { x: 400, y: 100, width: 80, height: 40 },
  };
  const result = rendering([
    node("a", "a", geometry.a),
    node("exposed", "exposed", { parent: "a", ...geometry.exposed }),
    node("b", "b", geometry.b),
  ]);
  const auto: AutoLayout = {
    nodes: new Map(Object.entries(geometry)),
    routes: new Map(),
    ports: new Map(),
  };
  const settled = new Map(layoutCanvas(result, undefined, auto).nodes);
  resettle(settled, result.nodes, auto, "a", { x: 140, y: 100 });
  assert.deepEqual(settled.get("a")!.box, { ...geometry.a, x: 140 });
  assert.deepEqual(settled.get("exposed")!.box, {
    ...geometry.exposed,
    x: 230,
  });
  const clear = obstacles(settled.values(), "b");
  assert.deepEqual(
    clear.map((entry) => entry.node.id),
    ["a", "exposed"],
  );
  const bounds = { x: 0, y: 0, width: 1000, height: 600 };
  const dropped = freePlacement(
    settled.get("b")!,
    { x: 265, y: 100 },
    clear,
    bounds,
    0,
  );
  assert.ok(
    dropped && dropped.x > 270,
    `b should be pushed clear of the carried box: ${JSON.stringify(dropped)}`,
  );
});

// A route the model states as a diagonal: drawn verbatim under `model`, routed around the boxes under `router`.
function statedDiagonal(): RenderResult {
  return rendering(
    [
      node("a", "a", { x: 0, y: 0, width: 100, height: 40 }),
      node("b", "b", { x: 400, y: 200, width: 100, height: 40 }),
      node("c", "c", { x: 180, y: -20, width: 60, height: 80 }),
    ],
    [
      {
        from: "a",
        to: "b",
        label: "",
        kind: "connection",
        fqn: "M::ab",
        route: [{ x: 250, y: 150 }],
      },
    ],
  );
}

test("layoutCanvas draws a stated route verbatim under model routing and orthogonally under the router", async () => {
  await loadAvoid(WASM);
  const result = statedDiagonal();
  const stated = layoutCanvas(result).edges[0];
  assert.equal(stated.rerouted, false);
  assert.deepEqual(stated.route, [{ x: 250, y: 150 }]);
  assert.equal(orthogonal(stated.points), false);
  assert.equal(layoutCanvas(result).routing, "model");
  assert.deepEqual(layoutCanvas(result, { routing: "model" }).edges[0], stated);

  const layout = layoutCanvas(result, { routing: "router" });
  assert.equal(layout.routing, "router");
  const [routed] = layout.edges;
  assert.equal(routed.rerouted, true);
  assert.ok(orthogonal(routed.points), `router route is not orthogonal: ${JSON.stringify(routed.points)}`);
  assert.ok(!routed.points.some((point) => point.x === 250 && point.y === 150));
  assert.deepEqual(routed.route, routed.points.slice(1, -1));
  // A gesture's own override routes the same way: the router draws, the model's points do not.
  const overridden = layoutCanvas(result, { routing: "router", routes: new Map([[0, [{ x: 10, y: 300 }]]]) }).edges[0];
  assert.equal(overridden.rerouted, true);
  assert.ok(orthogonal(overridden.points));
});

test("the router leaves the auto layout's routes, hidden edges and self-loops as they are", async () => {
  await loadAvoid(WASM);
  const auto: AutoLayout = {
    nodes: new Map([
      ["a", { x: 100, y: 50, width: 140, height: 60 }],
      ["b", { x: 300, y: 200, width: 140, height: 60 }],
    ]),
    routes: new Map([
      [0, [{ x: 240, y: 80 }, { x: 300, y: 80 }, { x: 300, y: 230 }]],
    ]),
    ports: new Map(),
  };
  const elk = rendering(
    [node("a", "a"), node("b", "b")],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  );
  for (const routing of ["model", "router"] as const) {
    const [edge] = layoutCanvas(elk, { routing }, auto).edges;
    assert.equal(edge.rerouted, false, routing);
    assert.deepEqual(edge.points, auto.routes.get(0), routing);
  }

  const filtered = rendering(
    [
      node("a", "a", { x: 10, y: 10 }),
      node("d", "d", { x: 200, y: 5 }),
    ],
    [
      { from: "a", to: "d", label: "", kind: "flow", route: [{ x: 900, y: 900 }] },
      { from: "d", to: "a", label: "", kind: "flow" },
      { from: "d", to: "d", label: "", kind: "connection", route: [{ x: 700, y: 700 }] },
    ],
  );
  const hidden = layoutCanvas(filtered, { routing: "router", hiddenKinds: new Set(["flow"]) });
  assert.deepEqual(hidden.edges.map((edge) => edge.hidden), [true, true, false]);
  assert.deepEqual(hidden.edges.map((edge) => edge.rerouted), [false, false, false]);
  // A self-loop's stated route is advisory too: the loop swings out to the right as an unrouted one does.
  const loop = hidden.edges[2];
  const d = hidden.nodes.get("d")!.box;
  assert.deepEqual(loop.route, []);
  assert.equal(loop.points.length, 4);
  assert.ok(loop.points.slice(1, -1).every((point) => point.x > d.x + d.width));
  assert.equal(hidden.height, 10 + 52 + MARGIN);
});

test("the router hides the waypoint handles and keeps a moved subtree's stated routes with it", async () => {
  await loadAvoid(WASM);
  const result = statedDiagonal();
  const layout = layoutCanvas(result, { routing: "router" });
  const [edge] = layout.edges;
  assert.equal(steerable(layoutCanvas(result), layoutCanvas(result).edges[0]), true);
  assert.equal(steerable(layout, edge), false);
  assert.equal(movedWaypoint(layout, 0, 0, { x: 1, y: 1 }), undefined);

  const nested = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 0, y: 0 }),
        node("b", "b", { parent: "a", x: 20, y: 60 }),
        node("c", "c", { parent: "a", x: 300, y: 60 }),
      ],
      [{ from: "b", to: "c", label: "", kind: "connection", fqn: "M::bc", route: [{ x: 150, y: 200 }] }],
    ),
    { routing: "router" },
  );
  assert.equal(nested.edges[0].rerouted, true);
  // The model's route, undrawn, still follows the nodes it runs between, so it is not left behind.
  assert.deepEqual(movedNode(nested, "a", 100, 0)!.edges, [{ index: 0, route: [{ x: 250, y: 200 }] }]);
});

test("a container drag under the router writes only the model's stated routes, a self-loop's included", async () => {
  await loadAvoid(WASM);
  const auto: AutoLayout = {
    nodes: new Map([
      ["p", { x: 0, y: 0, width: 400, height: 200 }],
      ["a", { x: 20, y: 60, width: 100, height: 40 }],
      ["b", { x: 280, y: 60, width: 100, height: 40 }],
    ]),
    routes: new Map([[0, [{ x: 120, y: 80 }, { x: 200, y: 80 }, { x: 200, y: 90 }, { x: 280, y: 90 }]]]),
    ports: new Map(),
  };
  const elk = rendering(
    [node("p", "p"), node("a", "a", { parent: "p" }), node("b", "b", { parent: "p" })],
    [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }],
  );
  const underModel = layoutCanvas(elk, { routing: "model" }, auto);
  assert.deepEqual(movedNode(underModel, "p", 10, 0)!.edges, [
    { index: 0, route: [{ x: 210, y: 80 }, { x: 210, y: 90 }] },
  ]);
  const underRouter = layoutCanvas(elk, { routing: "router" }, auto);
  assert.equal(underRouter.edges[0].rerouted, false);
  assert.deepEqual(movedNode(underRouter, "p", 10, 0)!.edges, []);

  const loop = layoutCanvas(
    rendering(
      [node("a", "a", { x: 20, y: 0 })],
      [{ from: "a", to: "a", label: "", kind: "connection", fqn: "M::aa", route: [{ x: 150, y: 10 }, { x: 150, y: 30 }] }],
    ),
    { routing: "router" },
  );
  assert.equal(loop.edges[0].rerouted, false);
  assert.deepEqual(loop.edges[0].route, []);
  assert.deepEqual(movedNode(loop, "a", 100, 0)!.edges, [{ index: 0, route: [{ x: 250, y: 10 }, { x: 250, y: 30 }] }]);
});

// sizedOwner is a part `a` the model places in a 120×60 box, two ports under it, a part `b` with a port
// `q`, and the connector `a.p1 → b.q` the interconnection kind draws.
function sizedOwner(kind: "tree" | "interconnection"): RenderResult {
  return rendering(
    [
      node("rig", "Rig", { kind: "part def" }),
      node("a", "a", { parent: "rig", x: 10, y: 10, width: 120, height: 60 }),
      node("p1", "p1", { kind: "port", type: "Fuel", parent: "a" }),
      node("p2", "p2", { kind: "port", type: "Fuel", parent: "a" }),
      node("b", "b", { parent: "rig" }),
      node("q", "q", { kind: "port", type: "Fuel", parent: "b" }),
    ],
    kind === "interconnection" ? [{ from: "p1", to: "q", label: "", kind: "connection", fqn: "M::c" }] : [],
    { kind },
  );
}

// pinnedOn asserts a port node is drawn as a pin straddling its owner's border: its square
// centred on the border, neither inside nor outside the box.
function pinnedOn(port: PlacedNode, owner: PlacedNode): void {
  assert.ok(port.snapped, `${port.node.id} should be pinned on ${owner.node.id}`);
  assert.deepEqual([port.box.width, port.box.height], [PORT_SIZE, PORT_SIZE]);
  const centre = { x: port.box.x + port.box.width / 2, y: port.box.y + port.box.height / 2 };
  assert.ok(onBorder(centre, owner.box), `${port.node.id} at ${JSON.stringify(centre)} should lie on ${JSON.stringify(owner.box)}`);
  assert.deepEqual(centre, portCenter(owner.box, port.snapped));
}

for (const kind of ["tree", "interconnection"] as const) {
  test(`layoutCanvas keeps a stated box its size and pins its ports on its border (${kind})`, () => {
    const layout = layoutCanvas(sizedOwner(kind));
    const a = layout.nodes.get("a")!;
    assert.deepEqual(a.box, { x: 10, y: 10, width: 120, height: 60 });
    pinnedOn(layout.nodes.get("p1")!, a);
    pinnedOn(layout.nodes.get("p2")!, a);
    assert.notDeepEqual(layout.nodes.get("p1")!.box, layout.nodes.get("p2")!.box);
    assert.deepEqual(layout.notices, []);
  });
}

test("layoutCanvas pins the ports of an unsized owner the same way, its box fitted to its label alone", () => {
  const layout = layoutCanvas(sizedOwner("interconnection"));
  const b = layout.nodes.get("b")!;
  const q = layout.nodes.get("q")!;
  pinnedOn(q, b);
  assert.deepEqual([b.box.width, b.box.height], [labelSize(b.lines).width, labelSize(b.lines).height]);
  assert.equal(b.children.length, 1);
});

test("layoutCanvas anchors a connector at the faces of the pins it joins", () => {
  const layout = layoutCanvas(sizedOwner("interconnection"));
  const a = layout.nodes.get("a")!;
  const b = layout.nodes.get("b")!;
  const p1 = layout.nodes.get("p1")!;
  const q = layout.nodes.get("q")!;
  assert.equal(p1.snapped!.side, "east", "p1 faces b, to its right");
  assert.equal(q.snapped!.side, "west", "q faces a, to its left");
  const [edge] = layout.edges;
  assert.equal(edge.hidden, false);
  assert.deepEqual(edge.points[0], portFace(a.box, p1.snapped!));
  assert.deepEqual(edge.points.at(-1), portFace(b.box, q.snapped!));
  assert.deepEqual(nodeUnder(layout, portCenter(a.box, p1.snapped!))?.node.id, "p1");
});

test("layoutCanvas leaves undrawn an unplaced child a stated size has no room for, with a notice", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 10, y: 10, width: 120, height: 60 }),
        node("inner", "inner", { parent: "a" }),
        node("deep", "deep", { parent: "inner" }),
        node("c", "c", { x: 400, y: 10 }),
      ],
      [{ from: "inner", to: "c", label: "", kind: "connection", fqn: "M::e" }],
    ),
  );
  const a = layout.nodes.get("a")!;
  const inner = layout.nodes.get("inner")!;
  assert.deepEqual(a.box, { x: 10, y: 10, width: 120, height: 60 });
  assert.ok(inner.box.y + inner.box.height > a.box.y + a.box.height, "the child does not fit the box");
  assert.equal(inner.clipped, true);
  assert.equal(layout.nodes.get("deep")!.clipped, true);
  assert.equal(layout.edges[0].hidden, true);
  assert.deepEqual(listedEdges(layout), []);
  assert.equal(nodeUnder(layout, { x: inner.box.x + 1, y: inner.box.y + inner.box.height - 1 }), undefined);
  assert.equal(layout.height, 10 + 60 + MARGIN, "the canvas covers the drawn boxes alone");
  assert.deepEqual(layout.notices, ["2 node(s) with no room in the stated size of the box above them, left undrawn"]);
});

test("layoutCanvas draws an unplaced child a stated size has room for", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 10, y: 10, width: 300, height: 200 }),
      node("inner", "inner", { parent: "a" }),
    ]),
  );
  const a = layout.nodes.get("a")!;
  const inner = layout.nodes.get("inner")!;
  assert.equal(inner.clipped, false);
  assert.ok(inner.box.x >= a.box.x && inner.box.x + inner.box.width <= a.box.x + a.box.width);
  assert.ok(inner.box.y >= a.box.y && inner.box.y + inner.box.height <= a.box.y + a.box.height);
  assert.deepEqual(layout.notices, []);
});

test("layoutCanvas keeps a port the model places in its stated box, and one with members drawn under it inside its owner", () => {
  const layout = layoutCanvas(
    rendering([
      node("a", "a", { x: 10, y: 10, width: 120, height: 60 }),
      node("placed", "placed", { kind: "port", parent: "a", x: 200, y: 200 }),
      node("b", "b"),
      node("nested", "nested", { kind: "port", parent: "b" }),
      node("sub", "sub", { kind: "port", parent: "nested" }),
    ]),
  );
  const placed = layout.nodes.get("placed")!;
  assert.equal(placed.snapped, undefined);
  assert.equal(placed.pinned, true);
  assert.deepEqual([placed.box.x, placed.box.y], [200, 200]);
  const b = layout.nodes.get("b")!;
  const nested = layout.nodes.get("nested")!;
  assert.equal(nested.snapped, undefined);
  assert.ok(nested.box.x >= b.box.x && nested.box.x + nested.box.width <= b.box.x + b.box.width);
  assert.ok(nested.box.y >= b.box.y && nested.box.y + nested.box.height <= b.box.y + b.box.height);
  pinnedOn(layout.nodes.get("sub")!, nested);
});

test("layoutCanvas draws a placed member of a clipped child where the model puts it, with its edges", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 10, y: 10, width: 120, height: 60 }),
        node("inner", "inner", { parent: "a" }),
        node("sensor", "sensor", { parent: "inner", x: 400, y: 100 }),
        node("c", "c", { x: 400, y: 300 }),
      ],
      [{ from: "sensor", to: "c", label: "", kind: "connection", fqn: "M::sc" }],
    ),
  );
  const inner = layout.nodes.get("inner")!;
  const sensor = layout.nodes.get("sensor")!;
  assert.equal(inner.clipped, true);
  assert.equal(sensor.clipped, false);
  assert.deepEqual([sensor.box.x, sensor.box.y], [400, 100]);
  assert.equal(layout.edges[0].hidden, false);
  assert.equal(nodeUnder(layout, { x: 401, y: 101 })?.node.id, "sensor");
  assert.ok(layout.width >= 400 + sensor.box.width + MARGIN);
  assert.deepEqual(layout.notices, ["1 node(s) with no room in the stated size of the box above them, left undrawn"]);
});

test("liftedEdges anchors a connector at a lifted owner's pin, and on a pin lifted off its border", () => {
  const layout = layoutCanvas(sizedOwner("interconnection"));
  const a = layout.nodes.get("a")!;
  const p1 = layout.nodes.get("p1")!;
  const [lifted] = liftedEdges(layout, "a", 100, 30);
  const movedA = { ...a.box, x: a.box.x + 100, y: a.box.y + 30 };
  assert.deepEqual(lifted.points[0], portFace(movedA, p1.snapped!));
  const [pinLifted] = liftedEdges(layout, "p1", 50, 0);
  const start = pinLifted.points[0];
  assert.ok(start.x >= p1.box.x + 50 && start.x <= p1.box.x + 50 + PORT_SIZE, `anchored on the lifted pin, not at ${JSON.stringify(start)}`);
});

test("layoutCanvas routes around an owner a placed member of its clipped child lies beyond", () => {
  const layout = layoutCanvas(
    rendering(
      [
        node("a", "a", { x: 100, y: 80, width: 120, height: 60 }),
        node("inner", "inner", { parent: "a" }),
        node("sensor", "sensor", { parent: "inner", x: 400, y: 100, width: 40, height: 20 }),
        node("far", "far", { x: -200, y: 100, width: 40, height: 20 }),
      ],
      [{ from: "sensor", to: "far", label: "", kind: "connection", fqn: "M::sf" }],
    ),
  );
  const [edge] = layout.edges;
  assert.equal(edge.rerouted, true);
  const a = layout.nodes.get("a")!.box;
  const crosses = edge.points.some((p, i) => i > 0 && intersectsSegment(edge.points[i - 1], p, a));
  assert.equal(crosses, false, `route ${JSON.stringify(edge.points)} crosses a's box ${JSON.stringify(a)}`);
});

// intersectsSegment reports an axis-aligned segment passing through the inside of a box.
function intersectsSegment(p: { x: number; y: number }, q: { x: number; y: number }, box: Box): boolean {
  const [x1, x2] = [Math.min(p.x, q.x), Math.max(p.x, q.x)];
  const [y1, y2] = [Math.min(p.y, q.y), Math.max(p.y, q.y)];
  return x1 < box.x + box.width && x2 > box.x && y1 < box.y + box.height && y2 > box.y;
}

// The membership edges a mixed rendering carries run from an owner to a member drawn
// inside it, and some edges join two siblings inside one box: both stay inside the box.

function overlapping(a: Box, b: Box): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

function insideBox(point: RenderPoint, box: Box): boolean {
  return point.x >= box.x && point.x <= box.x + box.width && point.y >= box.y && point.y <= box.y + box.height;
}

function onBorderOf(point: RenderPoint, box: Box): boolean {
  const onX = point.x === box.x || point.x === box.x + box.width;
  const onY = point.y === box.y || point.y === box.y + box.height;
  return insideBox(point, box) && (onX || onY);
}

function describes(layout: CanvasLayout, edge: PlacedEdge): string {
  const name = (id: string) => layout.nodes.get(id)?.node.name ?? id;
  return `${name(edge.edge.from)} -> ${name(edge.edge.to)}${edge.edge.label ? ` (${edge.edge.label})` : ""}: ${edge.points.map((p) => `${p.x},${p.y}`).join(" ")}`;
}

// scopeOf is the box both ends of an edge are drawn inside: the owner of a membership
// edge, the shared parent of two siblings; none for an edge between unrelated nodes.
function scopeOf(layout: CanvasLayout, edge: PlacedEdge): PlacedNode | undefined {
  const from = layout.nodes.get(edge.edge.from)!;
  const to = layout.nodes.get(edge.edge.to)!;
  if (from.snapped || to.snapped) {
    return undefined; // a pin is on the border, not inside the box
  }
  const descends = (entry: PlacedNode, of: PlacedNode): boolean => {
    for (let parent = entry.parent; parent; parent = parent.parent) {
      if (parent === of) {
        return true;
      }
    }
    return false;
  };
  if (descends(to, from)) {
    return from;
  }
  if (descends(from, to)) {
    return to;
  }
  return from.parent !== undefined && from.parent === to.parent ? from.parent : undefined;
}

function assertLabelsClearHeaders(layout: CanvasLayout): void {
  for (const edge of layout.edges) {
    const label = edgeLabelBox(edge);
    if (edge.hidden || !label) {
      continue;
    }
    for (const entry of layout.nodes.values()) {
      assert.ok(
        !overlapping(label, headerBox(entry)),
        `the label of ${describes(layout, edge)} covers ${entry.node.name}'s header`,
      );
    }
  }
}

function assertScopedEdgesInside(layout: CanvasLayout): number {
  let scoped = 0;
  for (const edge of layout.edges) {
    const scope = edge.hidden ? undefined : scopeOf(layout, edge);
    if (!scope) {
      continue;
    }
    scoped++;
    for (const point of edge.points) {
      assert.ok(insideBox(point, scope.box), `${describes(layout, edge)} leaves ${scope.node.name}`);
    }
  }
  return scoped;
}

function assertOrthogonal(layout: CanvasLayout): void {
  for (const edge of layout.edges) {
    if (edge.hidden || !edge.rerouted) {
      continue;
    }
    edge.points.forEach((point, i) => {
      assert.ok(i === 0 || point.x === edge.points[i - 1].x || point.y === edge.points[i - 1].y, `${describes(layout, edge)} is orthogonal`);
    });
  }
}

// crossesInto reports an orthogonal segment running through a box's interior.
function crossesInto(a: RenderPoint, b: RenderPoint, box: Box): boolean {
  const left = Math.min(a.x, b.x);
  const right = Math.max(a.x, b.x);
  const top = Math.min(a.y, b.y);
  const bottom = Math.max(a.y, b.y);
  return left < box.x + box.width && right > box.x && top < box.y + box.height && bottom > box.y;
}

function pumpRendering(edges: RenderEdge[]): RenderResult {
  return rendering(
    [
      node("pump", "Pump", { kind: "part def" }),
      node("pressure", "pressure", { kind: "attribute", parent: "pump", type: "Pressure" }),
      node("impeller", "impeller", { parent: "pump", type: "Impeller" }),
      node("spare", "spare", { kind: "ref part", parent: "pump", type: "Impeller" }),
      node("prime", "prime", { kind: "action", parent: "pump" }),
      node("impellerDef", "Impeller", { kind: "part def" }),
    ],
    edges,
    { kind: "mixed" },
  );
}

function membership(from: string, to: string, kind: string, label = ""): RenderEdge {
  return { from, to, label, kind, fqn: `M::${to}` };
}

test("layoutCanvas routes an owner → member edge inside the owner, from its header line to the member", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    pumpRendering([
      membership("pump", "pressure", "reference"),
      membership("pump", "impeller", "composition"),
      membership("pump", "spare", "reference"),
      membership("pump", "prime", "composition"),
      membership("impeller", "impellerDef", "typing"),
    ]),
  );
  const pump = layout.nodes.get("pump")!;
  const header = headerBox(pump);
  for (const edge of layout.edges.slice(0, 4)) {
    const member = layout.nodes.get(edge.edge.to)!;
    assert.ok(edge.rerouted, `${describes(layout, edge)} is routed`);
    for (const point of edge.points) {
      assert.ok(insideBox(point, pump.box), `${describes(layout, edge)} leaves Pump`);
    }
    const start = edge.points[0];
    assert.equal(start.y, header.y + header.height, `${describes(layout, edge)} starts on Pump's header line`);
    assert.ok(start.x > pump.box.x && start.x < pump.box.x + pump.box.width);
    assert.ok(onBorderOf(edge.points.at(-1)!, member.box), `${describes(layout, edge)} ends on ${member.node.name}'s border`);
  }
  const typing = layout.edges[4];
  assert.ok(typing.points.some((point) => !insideBox(point, pump.box)), "the typing edge leaves Pump for the definition");
});

test("layoutCanvas keeps an edge between siblings inside their container", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    pumpRendering([
      membership("pump", "prime", "composition"),
      membership("spare", "impeller", "reference"),
      membership("pressure", "prime", "reference"),
    ]),
  );
  assert.equal(assertScopedEdgesInside(layout), 3);
});

test("layoutCanvas places a membership edge's keyword where it covers no node's header", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    pumpRendering([
      membership("pump", "pressure", "require", "require"),
      membership("pump", "impeller", "assume", "assume"),
      membership("pump", "spare", "assert", "assert"),
      membership("pump", "prime", "frame", "frame"),
    ]),
  );
  assertLabelsClearHeaders(layout);
  for (const edge of layout.edges) {
    const label = edgeLabelBox(edge)!;
    const onRoute = edge.points.some((a, i) => {
      const b = edge.points[i + 1];
      return b !== undefined && (a.x === b.x ? edge.label.x === a.x && edge.label.y >= Math.min(a.y, b.y) && edge.label.y <= Math.max(a.y, b.y)
        : edge.label.y === a.y && edge.label.x >= Math.min(a.x, b.x) && edge.label.x <= Math.max(a.x, b.x));
    });
    assert.ok(onRoute, `the label of ${describes(layout, edge)} sits on its route`);
    assert.ok(label.y >= 0);
  }
});

// The server's mixed rendering of testdata/mixed-memberships.sysml: every member kind
// nested under its owner, each with its membership edge.
test("layoutCanvas routes the mixed rendering's memberships inside their owners, labels off the headers", async () => {
  await loadAvoid(WASM);
  const result = JSON.parse(readFileSync("src/webview/testdata/mixed-memberships.json", "utf8")) as RenderResult;
  const layout = layoutCanvas(result);
  assert.equal(layout.edges.length, 25);
  assert.equal(assertScopedEdgesInside(layout), 19);
  assertLabelsClearHeaders(layout);
  assertOrthogonal(layout);
  // The edge from outlet, a pin on Pump's east border, to Library's FluidPort has no room
  // inside the package between Pump and its border, so the engine routes it around, as before.
  const outlet = layout.edges.find((edge) => layout.nodes.get(edge.edge.from)?.node.name === "outlet")!;
  assert.ok(outlet.rerouted && outlet.points.length > 2, describes(layout, outlet));
});

test("layoutCanvas leaves a container through its border below the header, never through the header", async () => {
  await loadAvoid(WASM);
  const layout = layoutCanvas(
    rendering(
      [
        node("sys", "System", { kind: "part def", x: 24, y: 200 }),
        node("a", "a", { parent: "sys" }),
        node("b", "b", { parent: "sys" }),
        node("ext", "External", { kind: "part def", x: 600, y: 24, width: 96, height: 52 }),
      ],
      [
        membership("a", "ext", "typing"),
        membership("sys", "a", "composition"),
        membership("sys", "b", "composition"),
      ],
      { kind: "mixed" },
    ),
  );
  const sys = layout.nodes.get("sys")!;
  const header = headerBox(sys);
  assert.ok(header.height < sys.box.height, "System has a header strip above its members");
  const typing = layout.edges[0];
  assert.ok(typing.rerouted);
  for (let i = 1; i < typing.points.length; i++) {
    assert.ok(
      !crossesInto(typing.points[i - 1], typing.points[i], header),
      `${describes(layout, typing)} crosses System's header`,
    );
  }
  assert.ok(typing.points.some((point) => !insideBox(point, sys.box)), "the typing edge reaches External outside System");
});

// The server's rendering of GeneralViews::requirementView (examples/general-views-demo/vehicle.sysml):
// fifteen edges between unnested nodes whose labels the router has to keep off the boxes.
test("layoutCanvas keeps the requirement view's edge labels off its nodes", async () => {
  await loadAvoid(WASM);
  const result = JSON.parse(readFileSync("src/webview/testdata/requirement-view.json", "utf8")) as RenderResult;
  const layout = layoutCanvas(result);
  assert.equal(layout.edges.length, 15);
  assertLabelsClearHeaders(layout);
});

test("layoutCanvas keeps a sized owner's members where they fit, lane or no lane", async () => {
  await loadAvoid(WASM);
  const sized = layoutCanvas(
    rendering(
      [
        node("owner", "Owner", { kind: "part def", x: 24, y: 24, width: 160, height: 112 }),
        node("member", "member", { parent: "owner" }),
      ],
      [membership("owner", "member", "composition")],
      { kind: "mixed" },
    ),
  );
  const member = sized.nodes.get("member")!;
  assert.equal(member.clipped, false, "the member is still drawn inside the sized owner");
  assert.equal(member.box.y, 24 + headerBox(sized.nodes.get("owner")!).height, "the member sits where it did without the edge");
  assert.equal(sized.edges[0].hidden, false);
  const points = sized.edges[0].points;
  assert.ok(points.length >= 2 && points.every((point, i) => i === 0 || point.x === points[i - 1].x || point.y === points[i - 1].y));

  const hidden = layoutCanvas(
    rendering(
      [node("owner", "Owner", { kind: "part def" }), node("member", "member", { parent: "owner" })],
      [membership("owner", "member", "composition")],
      { kind: "mixed" },
    ),
    { hiddenKinds: new Set(["composition"]) },
  );
  const owner = hidden.nodes.get("owner")!;
  assert.equal(hidden.nodes.get("member")!.box.y, owner.box.y + headerBox(owner).height, "a hidden edge asks for no lane");
});
