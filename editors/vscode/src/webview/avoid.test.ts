import assert from "node:assert/strict";
import path from "node:path";
import { test } from "node:test";

import { avoidRoutes, CLEARANCE, loadAvoid } from "./avoid";
import type { Box } from "./geometry";
import type { RenderPoint } from "../protocol";

const WASM = path.resolve("node_modules/libavoid-js/dist/libavoid.wasm");
const SCENE_SEED = 0x6d2b79f5;

interface Pair {
  source: number;
  target: number;
}

type Random = () => number;

function seededRandom(seed: number): Random {
  let state = seed >>> 0;
  return () => {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
    return state / 0x100000000;
  };
}

function separated(a: Box, b: Box): boolean {
  const dx = Math.max(0, a.x - (b.x + b.width), b.x - (a.x + a.width));
  const dy = Math.max(0, a.y - (b.y + b.height), b.y - (a.y + a.height));
  return Math.hypot(dx, dy) >= 2 * CLEARANCE;
}

function placeBoxes(boxes: Box[], count: number, integer: (min: number, max: number) => number): Box[] {
  for (let i = boxes.length; i < count; i++) {
    let placed = false;
    for (let attempt = 0; attempt < 20_000 && !placed; attempt++) {
      const width = integer(80, 200);
      const height = integer(30, 60);
      const candidate = {
        x: integer(0, 1200 - width),
        y: integer(0, 600 - height),
        width,
        height,
      };
      if (boxes.every((box) => separated(box, candidate))) {
        boxes.push(candidate);
        placed = true;
      }
    }
    assert.ok(placed, `could not place box ${i + 1} in a scene of ${count}`);
  }
  return boxes;
}

function randomPair(boxCount: number, integer: (min: number, max: number) => number): Pair {
  const source = integer(0, boxCount - 1);
  let target = integer(0, boxCount - 2);
  if (target >= source) {
    target++;
  }
  return { source, target };
}

function randomScenes(samples: number): Array<{ boxes: Box[]; pair: Pair }> {
  const random = seededRandom(SCENE_SEED);
  const integer = (min: number, max: number) => min + Math.floor(random() * (max - min + 1));
  const scenes: Array<{ boxes: Box[]; pair: Pair }> = [];
  for (let i = 0; i < samples; i++) {
    const boxes = placeBoxes([], integer(2, 8), integer);
    scenes.push({ boxes, pair: randomPair(boxes.length, integer) });
  }
  return scenes;
}

function randomPairs(boxCount: number, count: number, seed: number): Pair[] {
  const random = seededRandom(seed);
  const integer = (min: number, max: number) => min + Math.floor(random() * (max - min + 1));
  return Array.from({ length: count }, () => randomPair(boxCount, integer));
}

function completeScene(boxes: Box[], minimumCount: number, seed: number): Box[] {
  const random = seededRandom(seed);
  const integer = (min: number, max: number) => min + Math.floor(random() * (max - min + 1));
  return placeBoxes([...boxes], minimumCount, integer);
}

function randomSharedTargetPairs(boxCount: number, seed: number): Pair[] {
  assert.ok(boxCount >= 4, "shared-target routing needs at least four boxes");
  const random = seededRandom(seed);
  const indices = Array.from({ length: boxCount }, (_, index) => index);
  for (let i = indices.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [indices[i], indices[j]] = [indices[j], indices[i]];
  }
  const target = indices[0];
  return indices.slice(1, 4).map((source) => ({ source, target }));
}

function onBorder(point: RenderPoint, box: Box): boolean {
  return (
    ((point.x === box.x || point.x === box.x + box.width) && point.y >= box.y && point.y <= box.y + box.height) ||
    ((point.y === box.y || point.y === box.y + box.height) && point.x >= box.x && point.x <= box.x + box.width)
  );
}

function crossesInterior(a: RenderPoint, b: RenderPoint, box: Box): boolean {
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

function overlapLength(a0: RenderPoint, a1: RenderPoint, b0: RenderPoint, b1: RenderPoint): number {
  if (a0.y === a1.y && b0.y === b1.y && a0.y === b0.y) {
    return Math.max(0, Math.min(Math.max(a0.x, a1.x), Math.max(b0.x, b1.x)) - Math.max(Math.min(a0.x, a1.x), Math.min(b0.x, b1.x)));
  }
  if (a0.x === a1.x && b0.x === b1.x && a0.x === b0.x) {
    return Math.max(0, Math.min(Math.max(a0.y, a1.y), Math.max(b0.y, b1.y)) - Math.max(Math.min(a0.y, a1.y), Math.min(b0.y, b1.y)));
  }
  return 0;
}

function routeScene(boxes: Box[], pairs: Pair[]): Array<RenderPoint[] | undefined> {
  const shapes = new Map<string, Box>(boxes.map((box, index) => [`node-${index}`, box]));
  const routes = avoidRoutes(
    shapes,
    pairs.map(({ source, target }, index) => ({ index, from: `node-${source}`, to: `node-${target}` })),
  );
  assert.ok(routes !== undefined, "avoidRoutes returned no map with the router loaded");
  return pairs.map((_, index) => routes.get(index));
}

// assertRoutes checks every routed edge of a scene: orthogonal, on the ends'
// borders, through no other box's interior, no two edges sharing a segment.
function assertRoutes(boxes: Box[], pairs: Pair[], routes: Array<RenderPoint[] | undefined>): number {
  let routed = 0;
  for (let index = 0; index < pairs.length; index++) {
    const route = routes[index];
    assert.ok(route !== undefined && route.length >= 2, `edge ${index} was not routed`);
    routed++;
    const { source, target } = pairs[index];
    assert.ok(onBorder(route[0], boxes[source]), `first point is not on source border: ${JSON.stringify(route[0])}`);
    assert.ok(onBorder(route.at(-1)!, boxes[target]), `last point is not on target border: ${JSON.stringify(route.at(-1))}`);
    const obstacles = boxes.filter((_, i) => i !== source && i !== target);
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
  for (let first = 0; first < routes.length; first++) {
    for (let second = first + 1; second < routes.length; second++) {
      const a = routes[first]!;
      const b = routes[second]!;
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
  return routed;
}

const samples = 500;
const extraPairSeed = (sample: number): number => (0x9e3779b9 ^ Math.imul(sample + 1, 0x85ebca6b)) >>> 0;
const sharedBoxSeed = (sample: number): number => (0x243f6a88 ^ Math.imul(sample + 1, 0x9e3779b1)) >>> 0;

test("avoidRoutes routes every edge orthogonally over 500 seeded scenes", async () => {
  await loadAvoid(WASM);
  let routed = 0;
  let sample = 0;
  for (const { boxes, pair } of randomScenes(samples)) {
    const pairs = [pair, ...randomPairs(boxes.length, 2, extraPairSeed(sample))];
    routed += assertRoutes(boxes, pairs, routeScene(boxes, pairs));
    sample++;
  }
  assert.equal(routed, samples * 3);
});

test("avoidRoutes separates three sources sharing one target over 500 seeded scenes", async () => {
  await loadAvoid(WASM);
  let routed = 0;
  let sample = 0;
  for (const { boxes } of randomScenes(samples)) {
    const grown = completeScene(boxes, 4, sharedBoxSeed(sample));
    const pairs = randomSharedTargetPairs(grown.length, extraPairSeed(sample));
    routed += assertRoutes(grown, pairs, routeScene(grown, pairs));
    sample++;
  }
  assert.equal(routed, samples * 3);
});

test("avoidRoutes routes all thirteen connectors at one shape past the exclusive pin limit", async () => {
  await loadAvoid(WASM);
  const boxes: Box[] = Array.from({ length: 13 }, (_, index) => {
    const angle = (2 * Math.PI * index) / 13;
    return {
      x: Math.round(570 + 250 * Math.cos(angle)),
      y: Math.round(285 + 250 * Math.sin(angle)),
      width: 60,
      height: 30,
    };
  });
  boxes.push({ x: 560, y: 280, width: 80, height: 40 });
  const pairs = Array.from({ length: 13 }, (_, source) => ({ source, target: 13 }));
  assertRoutes(boxes, pairs, routeScene(boxes, pairs));
});
