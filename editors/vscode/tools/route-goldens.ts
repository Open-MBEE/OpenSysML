// Records the panel's libavoid routes for the parity scenes as JSON goldens that
// the Go router (internal/ir/view/route) is tested against point for point.
// Run through tools/route-goldens.mjs, which bundles this file like the tests.
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { confinedPath } from "./golden-dir";
import { avoidRoutes, CLEARANCE, loadAvoid, type AvoidEdge, type AvoidPort, type AvoidShape } from "../src/webview/avoid";
import type { Box } from "../src/webview/geometry";
import type { RenderPoint } from "../src/protocol";

interface Scene {
  name: string;
  shapes: Array<{ id: string; box: Box; ports?: AvoidPort[] }>;
  edges: AvoidEdge[];
  bounds?: Box;
}

interface Golden extends Scene {
  routes: Record<string, RenderPoint[]>;
}

const SCENE_SEED = 0x6d2b79f5;
const extraPairSeed = (sample: number): number => (0x9e3779b9 ^ Math.imul(sample + 1, 0x85ebca6b)) >>> 0;
const sharedBoxSeed = (sample: number): number => (0x243f6a88 ^ Math.imul(sample + 1, 0x9e3779b1)) >>> 0;

type Integer = (min: number, max: number) => number;

function seededRandom(seed: number): () => number {
  let state = seed >>> 0;
  return () => {
    state = (Math.imul(state, 1664525) + 1013904223) >>> 0;
    return state / 0x100000000;
  };
}

function integerOf(seed: number): Integer {
  const random = seededRandom(seed);
  return (min, max) => min + Math.floor(random() * (max - min + 1));
}

function separated(a: Box, b: Box): boolean {
  const dx = Math.max(0, a.x - (b.x + b.width), b.x - (a.x + a.width));
  const dy = Math.max(0, a.y - (b.y + b.height), b.y - (a.y + a.height));
  return Math.hypot(dx, dy) >= 2 * CLEARANCE;
}

function placeBoxes(boxes: Box[], count: number, integer: Integer): Box[] {
  for (let i = boxes.length; i < count; i++) {
    let placed = false;
    for (let attempt = 0; attempt < 20_000 && !placed; attempt++) {
      const width = integer(80, 200);
      const height = integer(30, 60);
      const candidate = { x: integer(0, 1200 - width), y: integer(0, 600 - height), width, height };
      if (boxes.every((box) => separated(box, candidate))) {
        boxes.push(candidate);
        placed = true;
      }
    }
    if (!placed) {
      throw new Error(`could not place box ${i + 1} in a scene of ${count}`);
    }
  }
  return boxes;
}

interface Pair {
  source: number;
  target: number;
}

function randomPair(boxCount: number, integer: Integer): Pair {
  const source = integer(0, boxCount - 1);
  let target = integer(0, boxCount - 2);
  if (target >= source) {
    target++;
  }
  return { source, target };
}

function randomScenes(samples: number): Array<{ boxes: Box[]; pair: Pair }> {
  const integer = integerOf(SCENE_SEED);
  const scenes: Array<{ boxes: Box[]; pair: Pair }> = [];
  for (let i = 0; i < samples; i++) {
    const boxes = placeBoxes([], integer(2, 8), integer);
    scenes.push({ boxes, pair: randomPair(boxes.length, integer) });
  }
  return scenes;
}

function randomPairs(boxCount: number, count: number, seed: number): Pair[] {
  const integer = integerOf(seed);
  return Array.from({ length: count }, () => randomPair(boxCount, integer));
}

function randomSharedTargetPairs(boxCount: number, seed: number): Pair[] {
  const random = seededRandom(seed);
  const indices = Array.from({ length: boxCount }, (_, index) => index);
  for (let i = indices.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [indices[i], indices[j]] = [indices[j], indices[i]];
  }
  const target = indices[0];
  return indices.slice(1, 4).map((source) => ({ source, target }));
}

function boxScene(name: string, boxes: Box[], pairs: Pair[], bounds?: Box): Scene {
  return {
    name,
    shapes: boxes.map((box, index) => ({ id: `node-${index}`, box })),
    edges: pairs.map(({ source, target }, index) => ({ index, from: `node-${source}`, to: `node-${target}` })),
    bounds,
  };
}

const samples = 500;

// Single random pairs, plus a second pair in the same scene, as the panel's seeded-scene test does.
function randomPairScenes(): Scene[] {
  const scenes: Scene[] = [];
  randomScenes(samples).forEach(({ boxes, pair }, sample) => {
    const extra = randomPairs(boxes.length, 1, extraPairSeed(sample));
    scenes.push(boxScene(`random-pair-${sample}`, boxes, [pair, ...extra]));
  });
  return scenes;
}

// Three sources sharing one target over seeded scenes completed to at least four boxes.
function sharedTargetScenes(): Scene[] {
  const scenes: Scene[] = [];
  randomScenes(samples).forEach(({ boxes }, sample) => {
    const complete = placeBoxes([...boxes], 4, integerOf(sharedBoxSeed(sample)));
    scenes.push(boxScene(`shared-target-${sample}`, complete, randomSharedTargetPairs(complete.length, sharedBoxSeed(sample))));
  });
  return scenes;
}

function ringScene(): Scene {
  const boxes: Box[] = Array.from({ length: 13 }, (_, index) => {
    const angle = (2 * Math.PI * index) / 13;
    return { x: Math.round(570 + 250 * Math.cos(angle)), y: Math.round(285 + 250 * Math.sin(angle)), width: 60, height: 30 };
  });
  boxes.push({ x: 560, y: 280, width: 80, height: 40 });
  return boxScene("thirteen-connectors-past-the-exclusive-pin-limit", boxes, Array.from({ length: 13 }, (_, source) => ({ source, target: 13 })));
}

const canvas: Box = { x: 0, y: 0, width: 1200, height: 600 };

// Ported shapes: edges that meet named ports, flush header pins, and routes kept inside bounds.
function portScenes(): Scene[] {
  const port = (id: string, side: AvoidPort["side"], offset: number, flush?: boolean): AvoidPort => ({ id, side, offset, flush });
  return [
    {
      name: "same-side-edges-to-distinct-port-faces",
      shapes: [
        { id: "a", box: { x: 100, y: 100, width: 120, height: 60 }, ports: [port("a.p1", "east", 0.25), port("a.p2", "east", 0.75)] },
        { id: "b", box: { x: 400, y: 60, width: 120, height: 60 }, ports: [port("b.in", "west", 0.5)] },
        { id: "c", box: { x: 400, y: 220, width: 120, height: 60 } },
      ],
      edges: [
        { index: 0, from: "a", to: "b", fromPort: "a.p1", toPort: "b.in" },
        { index: 1, from: "a", to: "c", fromPort: "a.p2" },
      ],
    },
    {
      name: "three-shared-port-lanes-inside-bounds",
      shapes: [
        { id: "hub", box: { x: 40, y: 40, width: 120, height: 60 }, ports: [port("hub.out", "south", 0.5)] },
        { id: "x", box: { x: 40, y: 300, width: 100, height: 50 }, ports: [port("x.in", "north", 0.5)] },
        { id: "y", box: { x: 260, y: 300, width: 100, height: 50 }, ports: [port("y.in", "north", 0.5)] },
        { id: "z", box: { x: 480, y: 300, width: 100, height: 50 }, ports: [port("z.in", "west", 0.5)] },
      ],
      edges: [
        { index: 0, from: "hub", to: "x", fromPort: "hub.out", toPort: "x.in" },
        { index: 1, from: "hub", to: "y", fromPort: "hub.out", toPort: "y.in" },
        { index: 2, from: "hub", to: "z", fromPort: "hub.out", toPort: "z.in" },
      ],
      bounds: { x: 0, y: 0, width: 640, height: 400 },
    },
    {
      name: "bounded-port-route-past-an-obstacle",
      shapes: [
        { id: "left", box: { x: 20, y: 200, width: 100, height: 50 }, ports: [port("left.out", "south", 0.8)] },
        { id: "wall", box: { x: 180, y: 120, width: 60, height: 220 } },
        { id: "right", box: { x: 320, y: 200, width: 100, height: 50 }, ports: [port("right.in", "south", 0.2)] },
      ],
      edges: [{ index: 0, from: "left", to: "right", fromPort: "left.out", toPort: "right.in" }],
      bounds: { x: 0, y: 0, width: 440, height: 300 },
    },
    {
      name: "generic-pin-beside-a-ported-side",
      shapes: [
        { id: "a", box: { x: 60, y: 60, width: 100, height: 50 }, ports: [port("a.p", "south", 0.5)] },
        { id: "b", box: { x: 300, y: 60, width: 100, height: 50 } },
        { id: "c", box: { x: 60, y: 260, width: 100, height: 50 }, ports: [port("c.p", "north", 0.5)] },
      ],
      edges: [
        { index: 0, from: "a", to: "b" },
        { index: 1, from: "a", to: "c", fromPort: "a.p", toPort: "c.p" },
      ],
    },
    {
      name: "unported-lanes-in-a-ported-batch",
      shapes: [
        { id: "s", box: { x: 40, y: 150, width: 100, height: 50 } },
        { id: "t", box: { x: 400, y: 150, width: 100, height: 50 } },
        { id: "u", box: { x: 400, y: 20, width: 100, height: 50 }, ports: [port("u.in", "west", 0.5)] },
        { id: "v", box: { x: 400, y: 300, width: 100, height: 50 } },
      ],
      edges: [
        { index: 0, from: "s", to: "t" },
        { index: 1, from: "s", to: "t" },
        { index: 2, from: "s", to: "u", toPort: "u.in" },
        { index: 3, from: "s", to: "v" },
      ],
    },
    {
      name: "flush-header-pin-to-member-port",
      shapes: [
        { id: "owner", box: { x: 40, y: 40, width: 300, height: 28 }, ports: [port("owner.header", "south", 0.5, true)] },
        { id: "member", box: { x: 80, y: 160, width: 100, height: 50 }, ports: [port("member.in", "west", 0.5)] },
        { id: "other", box: { x: 230, y: 160, width: 100, height: 50 } },
      ],
      edges: [
        { index: 0, from: "owner", to: "member", fromPort: "owner.header", toPort: "member.in" },
        { index: 1, from: "owner", to: "other", fromPort: "owner.header" },
      ],
      bounds: canvas,
    },
    {
      name: "fractional-west-port-box",
      shapes: [
        { id: "a", box: { x: 10.5, y: 20.25, width: 90.5, height: 44.75 }, ports: [port("a.w", "west", 0.3), port("a.e", "east", 0.6)] },
        { id: "b", box: { x: 300.25, y: 200.5, width: 80, height: 40 }, ports: [port("b.n", "north", 0.5)] },
      ],
      edges: [
        { index: 0, from: "a", to: "b", fromPort: "a.e", toPort: "b.n" },
        { index: 1, from: "b", to: "a", toPort: "a.w" },
      ],
    },
  ];
}

// Randomized port endpoints on every side, as the panel's perpendicular-final-segment test.
function randomPortScenes(): Scene[] {
  const sides: AvoidPort["side"][] = ["north", "east", "south", "west"];
  const scenes: Scene[] = [];
  for (let sample = 0; sample < 100; sample++) {
    const integer = integerOf(0x51ed270b ^ Math.imul(sample + 1, 0x27d4eb2f));
    const boxes = placeBoxes([], integer(2, 5), integer);
    const shapes = boxes.map((box, index) => ({
      id: `node-${index}`,
      box,
      ports: Array.from({ length: integer(0, 2) }, (_, p) => ({
        id: `node-${index}.p${p}`,
        side: sides[integer(0, 3)],
        offset: [0.25, 0.5, 0.75][integer(0, 2)],
      })),
    }));
    const edges: AvoidEdge[] = [];
    const count = integer(1, 3);
    for (let index = 0; index < count; index++) {
      const { source, target } = randomPair(boxes.length, integer);
      const from = shapes[source];
      const to = shapes[target];
      edges.push({
        index,
        from: from.id,
        to: to.id,
        fromPort: from.ports.length > 0 && integer(0, 1) === 1 ? from.ports[integer(0, from.ports.length - 1)].id : undefined,
        toPort: to.ports.length > 0 && integer(0, 1) === 1 ? to.ports[integer(0, to.ports.length - 1)].id : undefined,
      });
    }
    scenes.push({ name: `random-ports-${sample}`, shapes, edges, bounds: sample % 2 === 0 ? canvas : undefined });
  }
  return scenes;
}

function record(scene: Scene): Golden {
  const shapes = new Map<string, AvoidShape>(scene.shapes.map(({ id, box, ports }) => [id, ports ? { box, ports } : { box }]));
  const routes = avoidRoutes(shapes, scene.edges, scene.bounds);
  if (!routes) {
    throw new Error(`${scene.name}: avoidRoutes failed`);
  }
  const recorded: Record<string, RenderPoint[]> = {};
  for (const [index, points] of routes) {
    recorded[String(index)] = points;
  }
  return { ...scene, routes: recorded };
}

const groups: Record<string, () => Scene[]> = {
  "random-pairs": randomPairScenes,
  "shared-targets": sharedTargetScenes,
  "pin-limit": () => [ringScene()],
  ports: portScenes,
  "random-ports": randomPortScenes,
};

// tools/ and the bundle under out/ both sit one level below the extension.
const extensionDir = fileURLToPath(new URL("..", import.meta.url));
const repoRoot = path.resolve(extensionDir, "..", "..");
const outDir = confinedPath(process.argv[2] ?? path.join(repoRoot, "internal/ir/view/route/testdata/parity"), repoRoot);
mkdirSync(outDir, { recursive: true });
await loadAvoid(path.join(extensionDir, "node_modules/libavoid-js/dist/libavoid.wasm"));
let scenes = 0;
let routes = 0;
for (const [group, build] of Object.entries(groups)) {
  const goldens = build().map(record);
  scenes += goldens.length;
  routes += goldens.reduce((sum, golden) => sum + Object.keys(golden.routes).length, 0);
  // Shards keep each golden well under the repository's fixture size cap.
  const shardSize = 125;
  for (let shard = 0; shard * shardSize < goldens.length; shard++) {
    const name = goldens.length > shardSize ? `${group}-${shard}.json` : `${group}.json`;
    const slice = goldens.slice(shard * shardSize, (shard + 1) * shardSize);
    // Checked per file so that a symlinked golden cannot redirect the write outside the checkout.
    writeFileSync(confinedPath(path.join(outDir, name), repoRoot), JSON.stringify({ scenes: slice }) + "\n");
  }
}
console.log(`${scenes} scenes, ${routes} routes written to ${outDir}`);
