import assert from "node:assert/strict";
import path from "node:path";
import { test } from "node:test";

import type { RenderEdge, RenderNode, RenderPoint, RenderResult } from "../protocol";
import {
  avoidRoutes,
  CLEARANCE,
  loadAvoid,
  MIN_JOG,
  NUDGING,
  portExitReach,
  routingObstacle,
  straightenJogs,
  type AvoidPort,
  type AvoidShape,
  type RoutingObstacle,
} from "./avoid";
import { portFace, PORT_SIZE, type Box, type Side } from "./geometry";
import { clampNodeToBounds, layoutCanvas } from "./layout";

const WASM = path.resolve("node_modules/libavoid-js/dist/libavoid.wasm");
const SCENE_SEED = 0x6d2b79f5;
const origin = {
  uri: "file:///m.sysml",
  range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } },
  digest: "d0",
};

function node(id: string, x: number, y: number, ports: RenderNode["ports"] = []): RenderNode {
  return {
    id,
    kind: "part",
    name: id,
    type: "",
    detail: "",
    fqn: `M::${id}`,
    origin,
    x,
    y,
    width: 80,
    height: 40,
    ports,
  };
}

function rendering(nodes: RenderNode[], edges: RenderEdge[]): RenderResult {
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
  };
}

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

function separated(a: Box, b: Box, clearance = CLEARANCE): boolean {
  const dx = Math.max(0, a.x - (b.x + b.width), b.x - (a.x + a.width));
  const dy = Math.max(0, a.y - (b.y + b.height), b.y - (a.y + a.height));
  return Math.hypot(dx, dy) >= 2 * clearance;
}

function placeBoxes(boxes: Box[], count: number, integer: (min: number, max: number) => number, clearance = CLEARANCE): Box[] {
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
      if (boxes.every((box) => separated(box, candidate, clearance))) {
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

function randomScenes(samples: number, clearance = CLEARANCE): Array<{ boxes: Box[]; pair: Pair }> {
  const random = seededRandom(SCENE_SEED);
  const integer = (min: number, max: number) => min + Math.floor(random() * (max - min + 1));
  const scenes: Array<{ boxes: Box[]; pair: Pair }> = [];
  for (let i = 0; i < samples; i++) {
    const boxes = placeBoxes([], integer(2, 8), integer, clearance);
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

test("portExitReach reserves the lane reach for each shared connection", () => {
  assert.equal(NUDGING, 8);
  assert.deepEqual([0, 1, 2, 3].map(portExitReach), [
    CLEARANCE,
    CLEARANCE,
    CLEARANCE + NUDGING,
    CLEARANCE + 2 * NUDGING,
  ]);
});

test("straightenJogs removes a short Z jog without moving its endpoints", () => {
  const route = [
    { x: -40, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 24 },
    { x: 8, y: 24 },
    { x: 8, y: 64 },
    { x: 40, y: 64 },
  ];
  const routes = new Map([[0, route]]);
  const straightened = straightenJogs(routes, new Map(), new Map());
  const result = straightened.get(0)!;

  assert.equal(MIN_JOG, 16);
  assert.notEqual(straightened, routes);
  assert.notEqual(result, route);
  assert.equal(result.length, 4);
  assert.deepEqual(result[0], route[0]);
  assert.deepEqual(result.at(-1), route.at(-1));
  assert.deepEqual(result[1], { x: 8, y: 0 });
  assert.deepEqual(route[2], { x: 0, y: 24 });
  for (let index = 1; index < result.length; index++) {
    assert.ok(result[index - 1].x === result[index].x || result[index - 1].y === result[index].y);
  }
});

test("straightenJogs keeps a jog of exactly one grid square", () => {
  const route = [
    { x: -40, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 16, y: 40 },
    { x: 16, y: 80 },
    { x: 40, y: 80 },
  ];
  assert.deepEqual(straightenJogs(new Map([[0, route]]), new Map(), new Map()).get(0), route);
});

test("straightenJogs keeps a short jog forced by facing endpoints", () => {
  const route = [
    { x: 0, y: 0 },
    { x: 16, y: 0 },
    { x: 16, y: 6 },
    { x: 100, y: 6 },
  ];
  assert.deepEqual(straightenJogs(new Map([[0, route]]), new Map(), new Map()).get(0), route);
});

test("straightenJogs leaves a U-turn unchanged", () => {
  const route = [
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 8, y: 40 },
    { x: 8, y: 0 },
    { x: 40, y: 0 },
  ];
  assert.deepEqual(straightenJogs(new Map([[0, route]]), new Map(), new Map()).get(0), route);
});

test("straightenJogs tries the other shift when the shorter one crosses an obstacle", () => {
  const route = [
    { x: -40, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 8, y: 40 },
    { x: 8, y: 80 },
    { x: 40, y: 80 },
  ];
  const box = { x: 7, y: 10, width: 2, height: 20 };
  const obstacles = new Map([["obstacle", { routing: box, raw: box }]]);
  const result = straightenJogs(new Map([[0, route]]), obstacles, new Map()).get(0)!;

  assert.ok(result.length < route.length);
  assert.deepEqual(result[0], route[0]);
  assert.deepEqual(result.at(-1), route.at(-1));
  assert.equal(result[1].x, 0);
});

test("straightenJogs keeps a jog when both shifts cross obstacles", () => {
  const route = [
    { x: -40, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 8, y: 40 },
    { x: 8, y: 80 },
    { x: 40, y: 80 },
  ];
  const first = { x: 7, y: 10, width: 2, height: 20 };
  const second = { x: -1, y: 50, width: 2, height: 20 };
  const obstacles = new Map([
    ["first", { routing: first, raw: first }],
    ["second", { routing: second, raw: second }],
  ]);
  assert.deepEqual(straightenJogs(new Map([[0, route]]), obstacles, new Map()).get(0), route);
});

test("straightenJogs preserves the routing clearance from shape buffers", () => {
  const route = [
    { x: -40, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 15, y: 40 },
    { x: 15, y: 100 },
    { x: 45, y: 100 },
  ];
  const raw = { x: 16, y: -100, width: 40, height: 100 };
  const obstacle = routingObstacle({ box: raw });
  const routes = new Map([[0, route]]);
  const result = straightenJogs(
    routes,
    new Map<string, RoutingObstacle>([["obstacle", obstacle]]),
    new Map<number, [string, string]>([[0, ["source", "target"]]]),
  ).get(0)!;

  assert.ok(result.length < route.length);
  assert.deepEqual(result[1], { x: 0, y: 0 });
  assert.deepEqual(result[2], { x: 0, y: 100 });
});

test("straightenJogs keeps the port exit leg at least its clearance", () => {
  const route = [
    { x: 0, y: 0 },
    { x: 16, y: 0 },
    { x: 16, y: 40 },
    { x: 8, y: 40 },
    { x: 8, y: 80 },
    { x: 40, y: 80 },
  ];
  const result = straightenJogs(new Map([[0, route]]), new Map(), new Map()).get(0)!;

  assert.deepEqual(result[0], route[0]);
  assert.deepEqual(result.at(-1), route.at(-1));
  assert.ok(Math.abs(result[1].x - result[0].x) + Math.abs(result[1].y - result[0].y) >= CLEARANCE);
  assert.ok(result.length < route.length);
});

test("straightenJogs keeps a lane away from a nearby parallel route", () => {
  const route = [
    { x: -40, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 8, y: 40 },
    { x: 8, y: 80 },
    { x: 40, y: 80 },
  ];
  const nearby = [{ x: 4, y: -20 }, { x: 4, y: 100 }];
  const routes = new Map([
    [0, route],
    [1, nearby],
  ]);
  assert.deepEqual(straightenJogs(routes, new Map(), new Map()).get(0), route);
});

test("straightenJogs rejects a shift that crosses another route when the other shift is legal", () => {
  const route = [
    { x: -30, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 15, y: 40 },
    { x: 15, y: 80 },
    { x: 45, y: 80 },
  ];
  const crossing = [{ x: 12, y: 20 }, { x: 30, y: 20 }];
  const routes = new Map([
    [0, route],
    [1, crossing],
  ]);
  const result = straightenJogs(routes, new Map(), new Map()).get(0)!;

  assert.ok(result.length < route.length);
  assert.deepEqual(result.slice(1, 3), [{ x: 0, y: 0 }, { x: 0, y: 80 }]);
});

test("straightenJogs keeps a jog when both shifts would add crossings", () => {
  const route = [
    { x: -30, y: 0 },
    { x: 0, y: 0 },
    { x: 0, y: 40 },
    { x: 15, y: 40 },
    { x: 15, y: 80 },
    { x: 45, y: 80 },
  ];
  const crossingAbove = [{ x: 12, y: 20 }, { x: 30, y: 20 }];
  const crossingBelow = [{ x: -10, y: 60 }, { x: 10, y: 60 }];
  const routes = new Map([
    [0, route],
    [1, crossingAbove],
    [2, crossingBelow],
  ]);

  assert.deepEqual(straightenJogs(routes, new Map(), new Map()).get(0), route);
});

test("straightenJogs rejects a shift that leaves the routing bounds", () => {
  const route = [
    { x: 8, y: 0 },
    { x: 8, y: 20 },
    { x: 0, y: 20 },
    { x: 0, y: 60 },
    { x: 40, y: 60 },
  ];
  const bounds: Box = { x: 0, y: 0, width: 4, height: 80 };
  assert.deepEqual(straightenJogs(new Map([[0, route]]), new Map(), new Map(), bounds).get(0), route);
});

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

function boxBorders(box: Box): Array<[RenderPoint, RenderPoint]> {
  return [
    [{ x: box.x, y: box.y }, { x: box.x + box.width, y: box.y }],
    [{ x: box.x + box.width, y: box.y }, { x: box.x + box.width, y: box.y + box.height }],
    [{ x: box.x + box.width, y: box.y + box.height }, { x: box.x, y: box.y + box.height }],
    [{ x: box.x, y: box.y + box.height }, { x: box.x, y: box.y }],
  ];
}

function assertPortApproach(outside: RenderPoint, face: RenderPoint, port: AvoidPort, context: string): void {
  switch (port.side) {
    case "north":
      assert.equal(outside.x, face.x, `port approach is not perpendicular: ${context}`);
      assert.ok(outside.y < face.y, `route does not approach north port from outside: ${context}`);
      break;
    case "east":
      assert.equal(outside.y, face.y, `port approach is not perpendicular: ${context}`);
      assert.ok(outside.x > face.x, `route does not approach east port from outside: ${context}`);
      break;
    case "south":
      assert.equal(outside.x, face.x, `port approach is not perpendicular: ${context}`);
      assert.ok(outside.y > face.y, `route does not approach south port from outside: ${context}`);
      break;
    case "west":
      assert.equal(outside.y, face.y, `port approach is not perpendicular: ${context}`);
      assert.ok(outside.x < face.x, `route does not approach west port from outside: ${context}`);
      break;
  }
}

function routeScene(boxes: Box[], pairs: Pair[]): Array<RenderPoint[] | undefined> {
  const shapes = new Map<string, AvoidShape>(boxes.map((box, index) => [`node-${index}`, { box }]));
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

function assertStraighteningIdempotent(
  boxes: Box[],
  pairs: Pair[],
  routes: Array<RenderPoint[] | undefined>,
): void {
  const current = new Map<number, RenderPoint[]>();
  routes.forEach((route, index) => {
    if (route) {
      current.set(index, route);
    }
  });
  const obstacles = new Map<string, RoutingObstacle>();
  boxes.forEach((box, index) => {
    obstacles.set(`node-${index}`, routingObstacle({ box }));
  });
  const ends = new Map<number, [string, string]>();
  pairs.forEach(({ source, target }, index) => {
    ends.set(index, [`node-${source}`, `node-${target}`]);
  });
  const straightened = straightenJogs(current, obstacles, ends);
  for (const [index, route] of current) {
    const result = straightened.get(index)!;
    assert.deepEqual(result, route, `straightening changed seeded route ${index} a second time`);
    assert.deepEqual(result[0], route[0], `straightening moved route ${index}'s first endpoint`);
    assert.deepEqual(result.at(-1), route.at(-1), `straightening moved route ${index}'s last endpoint`);
    for (let segment = 1; segment < result.length; segment++) {
      assert.ok(
        result[segment - 1].x === result[segment].x || result[segment - 1].y === result[segment].y,
        `straightening made seeded route ${index} non-orthogonal`,
      );
    }
  }
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
    const routes = routeScene(boxes, pairs);
    routed += assertRoutes(boxes, pairs, routes);
    assertStraighteningIdempotent(boxes, pairs, routes);
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
    const routes = routeScene(grown, pairs);
    routed += assertRoutes(grown, pairs, routes);
    assertStraighteningIdempotent(grown, pairs, routes);
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

test("avoidRoutes sends same-side edges to their distinct port faces", async () => {
  await loadAvoid(WASM);
  const boxes: Box[] = [
    { x: 0, y: 30, width: 80, height: 40 },
    { x: 320, y: 0, width: 120, height: 120 },
  ];
  const ports: AvoidPort[] = [
    { id: "in-low", side: "west", offset: 0.25 },
    { id: "in-high", side: "west", offset: 0.75 },
  ];
  const shapes = new Map<string, AvoidShape>([
    ["source", { box: boxes[0] }],
    ["target", { box: boxes[1], ports }],
  ]);
  const routes = avoidRoutes(shapes, [
    { index: 0, from: "source", to: "target", toPort: ports[0].id },
    { index: 1, from: "source", to: "target", toPort: ports[1].id },
  ]);
  assert.ok(routes);
  const faces = ports.map((port) => portFace(boxes[1], port));
  const first = routes.get(0)!;
  const second = routes.get(1)!;
  assert.deepEqual(first.at(-1), faces[0]);
  assert.deepEqual(second.at(-1), faces[1]);
  assert.notDeepEqual(faces[0], faces[1]);
  for (const route of [first, second]) {
    const before = route.at(-2)!;
    const face = route.at(-1)!;
    assert.equal(before.y, face.y);
    assert.ok(before.x < face.x);
  }
});

test("avoidRoutes keeps three shared-port lanes inside the bounds after clamping", async () => {
  await loadAvoid(WASM);
  const edges: RenderEdge[] = Array.from({ length: 3 }, (_, index) => ({
    from: `source-${index}`,
    to: "target",
    toPort: "target.in",
    label: "",
    kind: "connection",
    fqn: `M::edge${index}`,
  }));
  const layout = layoutCanvas(rendering([
    node("source-0", 300, 20),
    node("source-1", 300, 140),
    node("source-2", 300, 260),
    node("target", 600, 140, [{ id: "target.in", name: "in" }]),
  ], edges));
  const target = layout.nodes.get("target")!;
  const port = target.ports[0];
  assert.equal(port.side, "west");
  const bounds: Box = { x: 0, y: 0, width: 700, height: 400 };
  const at = clampNodeToBounds(target, { ...target.box, x: -1000 }, bounds, portExitReach(3));
  const targetBox = { ...target.box, ...at };
  assert.equal(portFace(targetBox, port).x - portExitReach(3), bounds.x);
  const shapes = new Map<string, AvoidShape>(
    [...layout.nodes].map(([id, entry]) => [
      id,
      {
        box: id === "target" ? targetBox : entry.box,
        ...(id === "target"
          ? { ports: target.ports.map(({ port: placed, side, offset }) => ({ id: placed.id, side, offset })) }
          : {}),
      },
    ]),
  );
  const routes = avoidRoutes(
    shapes,
    edges.map((edge, index) => ({
      index,
      from: edge.from,
      to: edge.to,
      toPort: edge.toPort,
    })),
  );
  assert.ok(routes);
  for (const index of edges.keys()) {
    const route = routes.get(index);
    assert.ok(route && route.length >= 2);
    assert.ok(route.every(({ x }) => x >= bounds.x), `route is clipped by the hero edge: ${JSON.stringify(route)}`);
  }
});

test("avoidRoutes bounds an orthogonal port route that otherwise escapes below an obstacle", async () => {
  await loadAvoid(WASM);
  const bounds: Box = { x: 0, y: 0, width: 500, height: 260 };
  const source = { x: 50, y: 190, width: 80, height: 40 };
  const target = { x: 370, y: 190, width: 80, height: 40 };
  const obstacle = { x: 210, y: 80, width: 80, height: 172 };
  const sourcePort: AvoidPort = { id: "source.out", side: "east", offset: 0.5 };
  const targetPort: AvoidPort = { id: "target.in", side: "west", offset: 0.5 };
  const shapes = new Map<string, AvoidShape>([
    ["source", { box: source, ports: [sourcePort] }],
    ["target", { box: target, ports: [targetPort] }],
    ["obstacle", { box: obstacle }],
  ]);
  const edges = [{ index: 0, from: "source", to: "target", fromPort: sourcePort.id, toPort: targetPort.id }];
  const unbounded = avoidRoutes(shapes, edges);
  assert.ok(unbounded);
  const unboundedRoute = unbounded.get(0);
  assert.ok(unboundedRoute);
  assert.ok(
    unboundedRoute.some(({ x, y }) => x < bounds.x || y < bounds.y || x > bounds.x + bounds.width || y > bounds.y + bounds.height),
    `expected an unbounded route to escape: ${JSON.stringify(unboundedRoute)}`,
  );

  const bounded = avoidRoutes(shapes, edges, bounds);
  assert.ok(bounded);
  const route = bounded.get(0);
  assert.ok(route && route.length >= 2);
  assert.deepEqual(route[0], portFace(source, sourcePort));
  assert.deepEqual(route.at(-1), portFace(target, targetPort));
  assert.ok(route.every(({ x, y }) => x >= bounds.x && y >= bounds.y && x <= bounds.x + bounds.width && y <= bounds.y + bounds.height));
  for (let index = 1; index < route.length; index++) {
    assert.ok(route[index - 1].x === route[index].x || route[index - 1].y === route[index].y);
  }
});

test("avoidRoutes keeps the generic pin on the node box when another side has a port", async () => {
  await loadAvoid(WASM);
  const source = { x: 0, y: 0, width: 100, height: 60 };
  const target = { x: 320, y: 0, width: 100, height: 60 };
  const routes = avoidRoutes(new Map<string, AvoidShape>([
    ["source", { box: source, ports: [{ id: "unused", side: "east", offset: 0.5 }] }],
    ["target", { box: target }],
  ]), [{ index: 0, from: "source", to: "target" }]);
  assert.ok(routes);
  const route = routes.get(0);
  assert.ok(route);
  assertRoutes([source, target], [{ source: 0, target: 1 }], [route]);
  assert.equal(route[0].x, source.x + source.width);
});

test("avoidRoutes keeps an unported route when a distant ported connection is added", async () => {
  await loadAvoid(WASM);
  const source = { x: 0, y: 40, width: 80, height: 40 };
  const target = { x: 320, y: 40, width: 80, height: 40 };
  const obstacle = { x: 160, y: 20, width: 60, height: 80 };
  const baseShapes = new Map<string, AvoidShape>([
    ["source", { box: source }],
    ["target", { box: target }],
    ["obstacle", { box: obstacle }],
  ]);
  const unported = { index: 0, from: "source", to: "target" };
  const baseline = avoidRoutes(baseShapes, [unported]);
  assert.ok(baseline);
  const baselineRoute = baseline.get(0);
  assert.ok(baselineRoute);
  assertRoutes([source, target, obstacle], [{ source: 0, target: 1 }], [baselineRoute]);

  const remotePort = { id: "remote.out", side: "east", offset: 0.5 } as const;
  const mixedShapes = new Map<string, AvoidShape>(baseShapes);
  mixedShapes.set("remote-source", { box: { x: 1000, y: 0, width: 80, height: 40 }, ports: [remotePort] });
  mixedShapes.set("remote-target", { box: { x: 1300, y: 0, width: 80, height: 40 } });
  const withRemoteConnection = avoidRoutes(mixedShapes, [
    unported,
    { index: 1, from: "remote-source", to: "remote-target", fromPort: remotePort.id },
  ]);
  assert.ok(withRemoteConnection);
  assert.deepEqual(withRemoteConnection.get(0), baselineRoute);
  assert.ok(withRemoteConnection.get(1));
});

test("avoidRoutes keeps distinct lanes for unported edges in a ported batch", async () => {
  await loadAvoid(WASM);
  const sourceA = { x: 0, y: 25, width: 80, height: 40 };
  const sourceB = { x: 0, y: 100, width: 80, height: 40 };
  const target = { x: 320, y: 0, width: 120, height: 160 };
  const remoteSource = { x: 1000, y: 0, width: 80, height: 40 };
  const remoteTarget = { x: 1300, y: 0, width: 80, height: 40 };
  const remotePort = { id: "remote.out", side: "east", offset: 0.5 } as const;
  const routes = avoidRoutes(new Map<string, AvoidShape>([
    ["source-a", { box: sourceA }],
    ["source-b", { box: sourceB }],
    ["target", { box: target }],
    ["remote-source", { box: remoteSource, ports: [remotePort] }],
    ["remote-target", { box: remoteTarget }],
  ]), [
    { index: 0, from: "source-a", to: "target" },
    { index: 1, from: "source-b", to: "target" },
    { index: 2, from: "remote-source", to: "remote-target", fromPort: remotePort.id },
  ]);
  assert.ok(routes);
  const unported = [routes.get(0), routes.get(1)];
  assertRoutes(
    [sourceA, sourceB, target],
    [{ source: 0, target: 2 }, { source: 1, target: 2 }],
    unported,
  );
  assert.equal(unported[0]!.at(-1)!.x, target.x);
  assert.equal(unported[1]!.at(-1)!.x, target.x);
});

test("avoidRoutes keeps randomized port endpoints on their faces with perpendicular final segments", async () => {
  await loadAvoid(WASM);
  const sides: Side[] = ["north", "east", "south", "west"];
  let sample = 0;
  for (const { boxes, pair } of randomScenes(samples, CLEARANCE + PORT_SIZE / 2)) {
    const pairs = [pair, ...randomPairs(boxes.length, 2, extraPairSeed(sample))];
    const random = seededRandom(sharedBoxSeed(sample));
    const portsByNode = boxes.map(() => [] as AvoidPort[]);
    const portPairs = pairs.map(({ source, target }, index) => {
      const sourcePort: AvoidPort = {
        id: `edge-${index}-source`,
        side: sides[Math.floor(random() * sides.length)],
        offset: 0.1 + random() * 0.8,
      };
      const targetPort: AvoidPort = {
        id: `edge-${index}-target`,
        side: sides[Math.floor(random() * sides.length)],
        offset: 0.1 + random() * 0.8,
      };
      portsByNode[source].push(sourcePort);
      portsByNode[target].push(targetPort);
      return { source, target, sourcePort, targetPort };
    });
    const shapes = new Map<string, AvoidShape>(
      boxes.map((box, index) => [`node-${index}`, { box, ports: portsByNode[index] }]),
    );
    const routes = avoidRoutes(
      shapes,
      portPairs.map(({ source, target, sourcePort, targetPort }, index) => ({
        index,
        from: `node-${source}`,
        to: `node-${target}`,
        fromPort: sourcePort.id,
        toPort: targetPort.id,
      })),
    );
    assert.ok(routes, `no route map in scene ${sample}`);
    for (let index = 0; index < portPairs.length; index++) {
      const { source, target, sourcePort, targetPort } = portPairs[index];
      const route: RenderPoint[] | undefined = routes.get(index);
      assert.ok(route && route.length >= 2, `edge ${index} was not routed in scene ${sample}`);
      const startFace = portFace(boxes[source], sourcePort);
      const endFace = portFace(boxes[target], targetPort);
      const context: string = JSON.stringify({ sample, index, source, target, sourcePort, targetPort, sourceBox: boxes[source], targetBox: boxes[target], route });
      assert.deepEqual(route[0], startFace, `wrong source face: ${context}`);
      assert.deepEqual(route.at(-1), endFace, `wrong target face: ${context}`);
      assertPortApproach(route[1], startFace, sourcePort, `source of edge ${index} in scene ${sample}: ${context}`);
      assertPortApproach(route.at(-2)!, endFace, targetPort, `target of edge ${index} in scene ${sample}: ${context}`);
      for (let point = 1; point < route.length; point++) {
        const before = route[point - 1];
        const after = route[point];
        assert.ok(before.x === after.x || before.y === after.y, `diagonal segment in scene ${sample}`);
        for (const box of boxes) {
          assert.ok(
            !crossesInterior(before, after, box),
            `route crosses box interior in scene ${sample}: ${JSON.stringify({ index, before, after, box, context })}`,
          );
          for (const [borderStart, borderEnd] of boxBorders(box)) {
            assert.ok(
              overlapLength(before, after, borderStart, borderEnd) <= 1,
              `route runs along a box border in scene ${sample}: ${JSON.stringify({ index, before, after, box, context })}`,
            );
          }
        }
      }
    }
    sample++;
  }
});
