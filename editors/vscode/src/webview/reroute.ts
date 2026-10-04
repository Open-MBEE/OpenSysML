// Routes an edge orthogonally around the other boxes: a grid over every border
// and port, searched with a bend penalty; the route is the panel's, never the model's.
import { GAP, snap, type Box } from "./geometry";
import type { RenderPoint } from "../protocol";

export const CLEARANCE = GAP / 2;
export const BEND_COST = 4 * GAP;
export const GRID_BUDGET = 40_000;

/** An axis-aligned polyline from a point on `source`'s border to a point on `target`'s border that keeps `clearance` off every obstacle, or undefined when none exists. */
export function orthogonalRoute(
  source: Box,
  target: Box,
  obstacles: Box[],
  clearance = CLEARANCE,
): RenderPoint[] | undefined {
  if (
    !validBox(source) ||
    !validBox(target) ||
    !Number.isFinite(clearance) ||
    clearance < 0 ||
    overlaps(source, target)
  ) {
    return undefined;
  }

  const sourceInflated = inflate(source, clearance);
  const targetInflated = inflate(target, clearance);
  if (contains(sourceInflated, target) || contains(targetInflated, source)) {
    return undefined;
  }

  const sourcePorts = ports(source, clearance);
  const targetPorts = ports(target, clearance);
  const blocked = [
    sourceInflated,
    targetInflated,
    ...obstacles.filter(validBox).map((box) => inflate(box, clearance)),
  ];
  const xs = uniqueSorted([
    ...blocked.flatMap((box) => [box.x, box.x + box.width]),
    source.x + source.width / 2,
    target.x + target.width / 2,
    ...sourcePorts.flatMap((port) => [port.stub.x]),
    ...targetPorts.flatMap((port) => [port.stub.x]),
  ]);
  const ys = uniqueSorted([
    ...blocked.flatMap((box) => [box.y, box.y + box.height]),
    source.y + source.height / 2,
    target.y + target.height / 2,
    ...sourcePorts.flatMap((port) => [port.stub.y]),
    ...targetPorts.flatMap((port) => [port.stub.y]),
  ]);
  if (xs.length * ys.length > GRID_BUDGET) {
    return undefined;
  }
  const height = ys.length;
  const pointCount = xs.length * height;
  const points = new Array<RenderPoint | undefined>(pointCount);
  const neighbors = new Array<Neighbor[]>(pointCount);
  for (let ix = 0; ix < xs.length; ix++) {
    for (let iy = 0; iy < ys.length; iy++) {
      const point = { x: xs[ix], y: ys[iy] };
      if (blocked.some((box) => inside(point, box))) {
        continue;
      }
      const id = ix * height + iy;
      points[id] = point;
      neighbors[id] = [];
    }
  }

  // Neighbors join along each grid line where the stretch between is clear.
  for (let ix = 0; ix < xs.length; ix++) {
    let previous = -1;
    for (let iy = 0; iy < ys.length; iy++) {
      const id = ix * height + iy;
      if (!points[id]) {
        continue;
      }
      if (previous >= 0 && clearSegment(points[previous]!, points[id]!, blocked)) {
        connect(neighbors, points, previous, id, 1);
      }
      previous = id;
    }
  }
  for (let iy = 0; iy < ys.length; iy++) {
    let previous = -1;
    for (let ix = 0; ix < xs.length; ix++) {
      const id = ix * height + iy;
      if (!points[id]) {
        continue;
      }
      if (previous >= 0 && clearSegment(points[previous]!, points[id]!, blocked)) {
        connect(neighbors, points, previous, id, 0);
      }
      previous = id;
    }
  }

  const sourceStubBlockers = blocked.slice(1);
  const targetStubBlockers = blocked.filter((_, index) => index !== 1);
  const sourceIds = sourcePorts.map((port) =>
    clearSegment(port.border, port.stub, sourceStubBlockers)
      ? gridId(port.stub, xs, ys, height, points)
      : undefined,
  );
  const targets = targetPorts
    .map((port) => ({
      port,
      id: clearSegment(port.border, port.stub, targetStubBlockers)
        ? gridId(port.stub, xs, ys, height, points)
        : undefined,
    }))
    .filter((goal): goal is GoalPort => goal.id !== undefined);
  if (targets.length === 0) {
    return undefined;
  }

  const goalsById = new Map<number, GoalPort[]>();
  for (const goal of targets) {
    const at = goalsById.get(goal.id) ?? [];
    at.push(goal);
    goalsById.set(goal.id, at);
  }

  // A* over (grid point, direction) states: length costs pixels, a turn BEND_COST.
  const stateCount = pointCount * 2;
  const distances = new Float64Array(stateCount);
  distances.fill(Infinity);
  const previous = new Int32Array(stateCount);
  previous.fill(-1);
  const originPort = new Int8Array(stateCount);
  originPort.fill(-1);
  const queue = new MinHeap();
  for (let i = 0; i < sourcePorts.length; i++) {
    const id = sourceIds[i];
    if (id === undefined) {
      continue;
    }
    const state = id * 2 + sourcePorts[i].axis;
    const distance = clearance;
    if (distance >= distances[state]) {
      continue;
    }
    distances[state] = distance;
    originPort[state] = i;
    queue.push({ state, cost: distance, priority: distance + heuristic(id, targets, points) });
  }

  let bestCost = Infinity;
  let bestState = -1;
  let bestGoal: GoalPort | undefined;
  while (queue.length > 0) {
    const current = queue.pop()!;
    if (current.cost !== distances[current.state]) {
      continue;
    }
    if (current.priority > bestCost) {
      break;
    }
    const id = current.state >> 1;
    const direction = current.state & 1;
    for (const goal of goalsById.get(id) ?? []) {
      const cost = current.cost + clearance + (direction === goal.port.axis ? 0 : BEND_COST);
      if (cost < bestCost) {
        bestCost = cost;
        bestState = current.state;
        bestGoal = goal;
      }
    }
    for (const next of neighbors[id] ?? []) {
      const state = next.id * 2 + next.axis;
      const cost = current.cost + next.length + (direction === next.axis ? 0 : BEND_COST);
      if (cost >= distances[state]) {
        continue;
      }
      distances[state] = cost;
      previous[state] = current.state;
      originPort[state] = originPort[current.state];
      queue.push({
        state,
        cost,
        priority: cost + heuristic(next.id, targets, points),
      });
    }
  }

  if (bestState < 0 || bestGoal === undefined) {
    return undefined;
  }
  const gridPath: RenderPoint[] = [];
  for (let state = bestState; state >= 0; state = previous[state]) {
    gridPath.push(points[state >> 1]!);
  }
  gridPath.reverse();
  const start = sourcePorts[originPort[bestState]];
  if (!start) {
    return undefined;
  }
  return cleanRoute([start.border, start.stub, ...gridPath, bestGoal.port.stub, bestGoal.port.border]);
}

interface Port {
  border: RenderPoint;
  stub: RenderPoint;
  axis: 0 | 1;
}

interface Neighbor {
  id: number;
  axis: 0 | 1;
  length: number;
}

interface GoalPort {
  port: Port;
  id: number;
}

interface QueueEntry {
  state: number;
  cost: number;
  priority: number;
}

// ports are the four places a route leaves a box: the border point and the stub
// a clearance out from it, on the axis the stub runs along.
function ports(box: Box, clearance: number): Port[] {
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  return [
    { border: { x: cx, y: box.y }, stub: { x: cx, y: box.y - clearance }, axis: 1 },
    {
      border: { x: box.x + box.width, y: cy },
      stub: { x: box.x + box.width + clearance, y: cy },
      axis: 0,
    },
    {
      border: { x: cx, y: box.y + box.height },
      stub: { x: cx, y: box.y + box.height + clearance },
      axis: 1,
    },
    { border: { x: box.x, y: cy }, stub: { x: box.x - clearance, y: cy }, axis: 0 },
  ];
}

function connect(
  graph: Neighbor[][],
  points: Array<RenderPoint | undefined>,
  from: number,
  to: number,
  axis: 0 | 1,
): void {
  const a = points[from]!;
  const b = points[to]!;
  const length = Math.abs(axis === 0 ? b.x - a.x : b.y - a.y);
  graph[from].push({ id: to, axis, length });
  graph[to].push({ id: from, axis, length });
}

// gridId is a port stub's grid point, undefined when the stub fell on a blocked point.
function gridId(
  point: RenderPoint,
  xs: number[],
  ys: number[],
  height: number,
  points: Array<RenderPoint | undefined>,
): number | undefined {
  const ix = coordinateIndex(xs, point.x);
  const iy = coordinateIndex(ys, point.y);
  if (ix < 0 || iy < 0) {
    return undefined;
  }
  const id = ix * height + iy;
  return points[id] ? id : undefined;
}

function coordinateIndex(coordinates: number[], value: number): number {
  let low = 0;
  let high = coordinates.length - 1;
  while (low <= high) {
    const middle = (low + high) >> 1;
    if (Math.abs(coordinates[middle] - value) < 1e-6) {
      return middle;
    }
    if (coordinates[middle] < value) {
      low = middle + 1;
    } else {
      high = middle - 1;
    }
  }
  return -1;
}

function heuristic(
  id: number,
  targets: GoalPort[],
  points: Array<RenderPoint | undefined>,
): number {
  const at = points[id]!;
  let best = Infinity;
  for (const goal of targets) {
    const end = points[goal.id]!;
    best = Math.min(best, Math.abs(at.x - end.x) + Math.abs(at.y - end.y));
  }
  return best;
}

// clearSegment is an axis-aligned segment that enters no box's interior.
function clearSegment(a: RenderPoint, b: RenderPoint, boxes: Box[]): boolean {
  if (a.x !== b.x && a.y !== b.y) {
    return false;
  }
  for (const box of boxes) {
    if (a.y === b.y) {
      if (a.y > box.y && a.y < box.y + box.height && overlapsInterval(a.x, b.x, box.x, box.x + box.width)) {
        return false;
      }
    } else if (
      a.x > box.x &&
      a.x < box.x + box.width &&
      overlapsInterval(a.y, b.y, box.y, box.y + box.height)
    ) {
      return false;
    }
  }
  return true;
}

function overlapsInterval(a0: number, a1: number, b0: number, b1: number): boolean {
  return Math.max(Math.min(a0, a1), b0) < Math.min(Math.max(a0, a1), b1);
}

// cleanRoute snaps to whole pixels, then drops doubled points and mid-segment bends.
function cleanRoute(points: RenderPoint[]): RenderPoint[] | undefined {
  const snapped: RenderPoint[] = [];
  for (const point of points) {
    const next = { x: snap(point.x), y: snap(point.y) };
    const last = snapped.at(-1);
    if (!last || last.x !== next.x || last.y !== next.y) {
      snapped.push(next);
    }
  }
  const clean: RenderPoint[] = [];
  for (const point of snapped) {
    while (clean.length >= 2 && collinear(clean.at(-2)!, clean.at(-1)!, point)) {
      clean.pop();
    }
    const last = clean.at(-1);
    if (!last || last.x !== point.x || last.y !== point.y) {
      clean.push(point);
    }
  }
  return clean.length >= 2 ? clean : undefined;
}

function collinear(a: RenderPoint, b: RenderPoint, c: RenderPoint): boolean {
  return (a.x === b.x && b.x === c.x) || (a.y === b.y && b.y === c.y);
}

function validBox(box: Box): boolean {
  return (
    Number.isFinite(box.x) &&
    Number.isFinite(box.y) &&
    Number.isFinite(box.width) &&
    Number.isFinite(box.height) &&
    box.width > 0 &&
    box.height > 0
  );
}

function inflate(box: Box, clearance: number): Box {
  return {
    x: box.x - clearance,
    y: box.y - clearance,
    width: box.width + clearance * 2,
    height: box.height + clearance * 2,
  };
}

function inside(point: RenderPoint, box: Box): boolean {
  return (
    point.x > box.x &&
    point.x < box.x + box.width &&
    point.y > box.y &&
    point.y < box.y + box.height
  );
}

function overlaps(a: Box, b: Box): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

function contains(outer: Box, inner: Box): boolean {
  return (
    inner.x >= outer.x &&
    inner.y >= outer.y &&
    inner.x + inner.width <= outer.x + outer.width &&
    inner.y + inner.height <= outer.y + outer.height
  );
}

function uniqueSorted(values: number[]): number[] {
  const sorted = values.sort((a, b) => a - b);
  const unique: number[] = [];
  for (const value of sorted) {
    if (unique.length === 0 || Math.abs(value - unique.at(-1)!) > 1e-7) {
      unique.push(value);
    }
  }
  return unique;
}

class MinHeap {
  private readonly items: QueueEntry[] = [];

  get length(): number {
    return this.items.length;
  }

  push(entry: QueueEntry): void {
    this.items.push(entry);
    let child = this.items.length - 1;
    while (child > 0) {
      const parent = (child - 1) >> 1;
      if (this.items[parent].priority <= entry.priority) {
        break;
      }
      this.items[child] = this.items[parent];
      child = parent;
    }
    this.items[child] = entry;
  }

  pop(): QueueEntry | undefined {
    if (this.items.length === 0) {
      return undefined;
    }
    const first = this.items[0];
    const last = this.items.pop()!;
    if (this.items.length > 0) {
      let parent = 0;
      while (true) {
        const left = parent * 2 + 1;
        if (left >= this.items.length) {
          break;
        }
        const right = left + 1;
        const child =
          right < this.items.length && this.items[right].priority < this.items[left].priority ? right : left;
        if (this.items[child].priority >= last.priority) {
          break;
        }
        this.items[parent] = this.items[child];
        parent = child;
      }
      this.items[parent] = last;
    }
    return first;
  }
}
