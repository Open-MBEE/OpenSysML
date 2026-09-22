import assert from "node:assert/strict";
import { test } from "node:test";

import type { RenderEdge, RenderNode, RenderPoint, RenderResult } from "../protocol";
import { AUTO_LAYOUT_LIMIT, autoLayout, type AutoLayout } from "./autolayout";
import { GAP, type Box } from "./layout";

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };

function node(id: string, name: string, extra: Partial<RenderNode> = {}): RenderNode {
  return { id, kind: "part", name, type: "", detail: "", fqn: `M::${name}`, origin, ...extra };
}

function edge(from: string, to: string, extra: Partial<RenderEdge> = {}): RenderEdge {
  return { from, to, label: "", kind: "connection", fqn: `M::${from}${to}`, ...extra };
}

function rendering(nodes: RenderNode[], edges: RenderEdge[] = [], extra: Partial<RenderResult> = {}): RenderResult {
  return {
    view: "M::V",
    kind: "tree",
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

// boxOf is a node's placed geometry as a box; ELK always states a size.
function boxOf(laid: AutoLayout, id: string): Box {
  const geometry = laid.nodes.get(id);
  assert.ok(geometry?.width !== undefined && geometry.height !== undefined);
  return { x: geometry.x, y: geometry.y, width: geometry.width, height: geometry.height };
}

function disjoint(a: Box, b: Box): boolean {
  return a.x + a.width <= b.x || b.x + b.width <= a.x || a.y + a.height <= b.y || b.y + b.height <= a.y;
}

// onBorder is the point lying on the box's edge, give or take a pixel of snapping.
function onBorder(point: RenderPoint, box: Box): boolean {
  const withinX = point.x >= box.x - 1 && point.x <= box.x + box.width + 1;
  const withinY = point.y >= box.y - 1 && point.y <= box.y + box.height + 1;
  const onX = Math.abs(point.x - box.x) <= 1 || Math.abs(point.x - box.x - box.width) <= 1;
  const onY = Math.abs(point.y - box.y) <= 1 || Math.abs(point.y - box.y - box.height) <= 1;
  return (withinX && onY) || (withinY && onX);
}

function orthogonal(points: RenderPoint[]): boolean {
  return points.every((point, i) => i === 0 || point.x === points[i - 1].x || point.y === points[i - 1].y);
}

test("autoLayout lays a chain out in layers downward and routes its edges orthogonally", async () => {
  const result = rendering([node("a", "a"), node("b", "b"), node("c", "c")], [edge("a", "b"), edge("b", "c")]);
  const laid = await autoLayout(result);
  assert.ok(laid);
  const a = boxOf(laid, "a");
  const b = boxOf(laid, "b");
  const c = boxOf(laid, "c");
  assert.ok(a.y < b.y && b.y < c.y);
  for (const box of [a, b, c]) {
    assert.ok(box.width > 0 && box.height > 0);
  }
  assert.ok(disjoint(a, b) && disjoint(b, c) && disjoint(a, c));
  const routes = [laid.routes.get(0)!, laid.routes.get(1)!];
  assert.ok(routes.every((route) => route.length >= 2));
  assert.ok(routes.every(orthogonal));
  assert.ok(onBorder(routes[0][0], a) && onBorder(routes[0].at(-1)!, b));
  assert.ok(onBorder(routes[1][0], b) && onBorder(routes[1].at(-1)!, c));
});

test("autoLayout holds a container's children inside it and reports absolute edge points", async () => {
  const result = rendering(
    [node("p", "p"), node("a", "a", { parent: "p" }), node("b", "b", { parent: "p" }), node("c", "c")],
    [edge("a", "c"), edge("b", "c")],
  );
  const laid = await autoLayout(result);
  assert.ok(laid);
  const p = boxOf(laid, "p");
  const a = boxOf(laid, "a");
  const b = boxOf(laid, "b");
  const c = boxOf(laid, "c");
  for (const child of [a, b]) {
    assert.ok(child.x >= p.x && child.x + child.width <= p.x + p.width);
    assert.ok(child.y >= p.y && child.y + child.height <= p.y + p.height);
  }
  const horizontalGap = Math.max(b.x - (a.x + a.width), a.x - (b.x + b.width));
  const verticalGap = Math.max(b.y - (a.y + a.height), a.y - (b.y + b.height));
  assert.ok(horizontalGap >= GAP || verticalGap >= GAP);
  // The routes are absolute canvas coordinates: they end on c's border directly.
  for (const route of [laid.routes.get(0)!, laid.routes.get(1)!]) {
    assert.ok(onBorder(route.at(-1)!, c));
  }
});

test("autoLayout leaves a routed edge and a self-loop alone", async () => {
  const route = [{ x: 40, y: 40 }];
  const result = rendering(
    [node("a", "a"), node("b", "b"), node("c", "c")],
    [edge("a", "b", { route }), edge("b", "c"), edge("c", "c")],
  );
  const laid = await autoLayout(result);
  assert.ok(laid);
  assert.equal(laid.routes.has(0), false);
  assert.ok(laid.routes.has(1));
  assert.equal(laid.routes.has(2), false);
});

test("autoLayout declines a kind with no canvas and a rendering over the limit", async () => {
  assert.equal(await autoLayout(rendering([node("a", "a")], [], { kind: "sequence" })), undefined);
  const many = Array.from({ length: AUTO_LAYOUT_LIMIT + 1 }, (_, i) => node(`n${i}`, `n${i}`));
  assert.equal(await autoLayout(rendering(many)), undefined);
});

test("autoLayout lays an interconnection out left to right", async () => {
  const result = rendering([node("a", "a"), node("b", "b")], [edge("a", "b")], { kind: "interconnection" });
  const laid = await autoLayout(result);
  assert.ok(laid);
  assert.ok(boxOf(laid, "a").x < boxOf(laid, "b").x);
});

test("autoLayout moves an unplaced subtree with the container the model places", async () => {
  const result = rendering(
    [node("p", "p", { x: 500, y: 400, width: 300, height: 200 }), node("a", "a", { parent: "p" }), node("b", "b", { parent: "p" })],
    [edge("a", "b")],
    { kind: "interconnection" },
  );
  const laid = await autoLayout(result);
  assert.ok(laid);
  const p = boxOf(laid, "p");
  assert.deepEqual(p, { x: 500, y: 400, width: 300, height: 200 });
  for (const child of [boxOf(laid, "a"), boxOf(laid, "b")]) {
    assert.ok(child.x >= p.x && child.x + child.width <= p.x + p.width);
    assert.ok(child.y >= p.y && child.y + child.height <= p.y + p.height);
  }
  for (const point of laid.routes.get(0)!) {
    assert.ok(point.x >= p.x && point.x <= p.x + p.width && point.y >= p.y && point.y <= p.y + p.height);
  }
});

test("autoLayout grows an unplaced container around a child the model places elsewhere", async () => {
  const result = rendering(
    [node("q", "q"), node("a", "a", { parent: "q", x: 700, y: 50 }), node("b", "b"), node("c", "c")],
    [edge("q", "b"), edge("a", "b"), edge("c", "b")],
  );
  const laid = await autoLayout(result);
  assert.ok(laid);
  const q = boxOf(laid, "q");
  const a = boxOf(laid, "a");
  assert.deepEqual([a.x, a.y], [700, 50]);
  assert.ok(a.x >= q.x && a.x + a.width <= q.x + q.width);
  assert.ok(a.y >= q.y && a.y + a.height <= q.y + q.height);
  // The grown box's own route is dropped so the edge anchors on its new border.
  assert.equal(laid.routes.has(0), false);
  // So is a route at the placed child, which the model positions.
  assert.equal(laid.routes.has(1), false);
  assert.ok(laid.routes.has(2));
});

test("autoLayout drops the route of an edge crossing a placed container's border", async () => {
  const result = rendering(
    [node("p", "p", { x: 500, y: 400, width: 300, height: 200 }), node("a", "a", { parent: "p" }), node("c", "c")],
    [edge("a", "c")],
  );
  const laid = await autoLayout(result);
  assert.ok(laid);
  assert.equal(laid.routes.has(0), false);
});
