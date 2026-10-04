import assert from "node:assert/strict";
import { test } from "node:test";

import type { Box } from "./geometry";
import { CLEARANCE, GRID_BUDGET, orthogonalRoute } from "./reroute";

function box(x: number, y: number, width: number, height: number): Box {
  return { x, y, width, height };
}

function onBorder(point: { x: number; y: number }, at: Box): boolean {
  const epsilon = 1e-7;
  const onVertical =
    (Math.abs(point.x - at.x) < epsilon || Math.abs(point.x - (at.x + at.width)) < epsilon) &&
    point.y >= at.y - epsilon &&
    point.y <= at.y + at.height + epsilon;
  const onHorizontal =
    (Math.abs(point.y - at.y) < epsilon || Math.abs(point.y - (at.y + at.height)) < epsilon) &&
    point.x >= at.x - epsilon &&
    point.x <= at.x + at.width + epsilon;
  return onVertical || onHorizontal;
}

function crossesInterior(a: { x: number; y: number }, b: { x: number; y: number }, at: Box): boolean {
  if (a.y === b.y) {
    return (
      a.y > at.y &&
      a.y < at.y + at.height &&
      Math.max(Math.min(a.x, b.x), at.x) < Math.min(Math.max(a.x, b.x), at.x + at.width)
    );
  }
  return (
    a.x > at.x &&
    a.x < at.x + at.width &&
    Math.max(Math.min(a.y, b.y), at.y) < Math.min(Math.max(a.y, b.y), at.y + at.height)
  );
}

function assertOrthogonal(route: { x: number; y: number }[], source: Box, target: Box, obstacles: Box[]): void {
  assert.ok(onBorder(route[0], source), `first point is not on source border: ${JSON.stringify(route[0])}`);
  assert.ok(onBorder(route.at(-1)!, target), `last point is not on target border: ${JSON.stringify(route.at(-1))}`);
  for (let i = 1; i < route.length; i++) {
    const a = route[i - 1];
    const b = route[i];
    assert.ok(a.x === b.x || a.y === b.y, `non-orthogonal segment: ${JSON.stringify([a, b])}`);
    assert.ok(a.x !== b.x || a.y !== b.y, `zero-length segment at ${JSON.stringify(a)}`);
    for (const obstacle of obstacles) {
      assert.ok(!crossesInterior(a, b, obstacle), `segment crosses obstacle interior: ${JSON.stringify([a, b])}`);
    }
  }
}

test("orthogonalRoute is a straight line between aligned, unobstructed boxes", () => {
  const route = orthogonalRoute(box(0, 0, 100, 40), box(300, 0, 100, 40), []);
  assert.deepEqual(route, [{ x: 100, y: 20 }, { x: 300, y: 20 }]);
});

test("orthogonalRoute bends around a blocking box, keeping its clearance", () => {
  const source = box(0, 0, 100, 40);
  const target = box(400, 0, 100, 40);
  const obstacle = box(200, -20, 60, 80);
  const route = orthogonalRoute(source, target, [obstacle])!;
  assert.ok(route !== undefined);
  assert.ok(route.length > 2);
  const inflated = box(obstacle.x - CLEARANCE, obstacle.y - CLEARANCE, obstacle.width + 2 * CLEARANCE, obstacle.height + 2 * CLEARANCE);
  assertOrthogonal(route, source, target, [inflated]);
});

test("orthogonalRoute declines overlapping boxes and a target inside the source's clearance", () => {
  assert.equal(orthogonalRoute(box(0, 0, 100, 40), box(50, 10, 100, 40), []), undefined);
  assert.equal(orthogonalRoute(box(0, 0, 100, 40), box(104, 10, 8, 8), []), undefined);
});

test("orthogonalRoute gives up when the grid its obstacles would draw exceeds GRID_BUDGET", () => {
  const source = box(0, 0, 100, 40);
  const target = box(300, 0, 100, 40);
  const obstacles = (count: number) =>
    Array.from({ length: count }, (_, i) => box(1000 + 40 * i, 1000 + 40 * i, 10, 10));
  // Each obstacle adds two x and two y lines, so 101 boxes blow the 40,000-point budget.
  assert.ok((2 * 101 + 8) ** 2 > GRID_BUDGET);
  assert.equal(orthogonalRoute(source, target, obstacles(101)), undefined);
  assert.ok(orthogonalRoute(source, target, obstacles(10)) !== undefined);
});

test("orthogonalRoute stays axis-aligned and clear of every box over 500 seeded scenes", () => {
  let seed = 0x6d2b79f5;
  const random = () => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    return seed / 0x100000000;
  };
  const integer = (min: number, max: number) => min + Math.floor(random() * (max - min + 1));
  const separated = (a: Box, b: Box) => {
    const dx = Math.max(0, a.x - (b.x + b.width), b.x - (a.x + a.width));
    const dy = Math.max(0, a.y - (b.y + b.height), b.y - (a.y + a.height));
    return Math.hypot(dx, dy) >= 2 * CLEARANCE;
  };
  const scene = (count: number): Box[] => {
    const boxes: Box[] = [];
    for (let i = 0; i < count; i++) {
      let placed = false;
      for (let attempt = 0; attempt < 20_000 && !placed; attempt++) {
        const width = integer(80, 200);
        const height = integer(30, 60);
        const candidate = box(integer(0, 1200 - width), integer(0, 600 - height), width, height);
        if (boxes.every((other) => separated(other, candidate))) {
          boxes.push(candidate);
          placed = true;
        }
      }
      assert.ok(placed, `could not place box ${i + 1} in a scene of ${count}`);
    }
    return boxes;
  };

  let routed = 0;
  const samples = 500;
  for (let sample = 0; sample < samples; sample++) {
    const boxes = scene(integer(2, 8));
    const sourceIndex = integer(0, boxes.length - 1);
    let targetIndex = integer(0, boxes.length - 2);
    if (targetIndex >= sourceIndex) {
      targetIndex++;
    }
    const source = boxes[sourceIndex];
    const target = boxes[targetIndex];
    const obstacles = boxes.filter((_, index) => index !== sourceIndex && index !== targetIndex);
    const route = orthogonalRoute(source, target, obstacles);
    if (!route) {
      continue;
    }
    routed++;
    assertOrthogonal(route, source, target, [...obstacles, source, target]);
  }
  assert.ok(routed >= samples * 0.95, `only ${routed}/${samples} scenes were routed`);
});
