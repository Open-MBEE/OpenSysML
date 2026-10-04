// Routes edges orthogonally around the shapes with libavoid's WASM router; every
// route it finds is the panel's own, never written to the model by drawing it.
import { AvoidLib } from "libavoid-js";

import { GAP, portFace, PORT_SIZE, snap, type Box, type PortPosition } from "./geometry";
import type { RenderPoint } from "../protocol";

export const CLEARANCE = GAP / 2;
export const NUDGING = 8;
export const MIN_JOG = 16;
export const EXCLUSIVE_PIN_LIMIT = 12;

export function portExitReach(sharing: number): number {
  return CLEARANCE + NUDGING * Math.max(0, sharing - 1);
}

type Avoid = ReturnType<typeof AvoidLib.getInstance>;
type ShapeRef = InstanceType<Avoid["ShapeRef"]>;
type ConnRef = InstanceType<Avoid["ConnRef"]>;

const PIN_CLASS = 1;

let avoid: Avoid | undefined;
let loading: Promise<void> | undefined;
let reported = false;

// reportAvoidFailure logs the router's first failure and no more.
export function reportAvoidFailure(context: string, error: unknown): void {
  if (reported) {
    return;
  }
  reported = true;
  console.error(`libavoid ${context}:`, error);
}

// loadAvoid brings the WASM router up once; repeat calls share the one load.
export function loadAvoid(wasmUrl: string): Promise<void> {
  loading ??= AvoidLib.load(wasmUrl).then(() => {
    avoid = AvoidLib.getInstance();
  });
  return loading;
}

export interface AvoidEdge {
  index: number;
  from: string;
  to: string;
  fromPort?: string;
  toPort?: string;
}

export interface AvoidPort extends PortPosition {
  id: string;
}

export interface AvoidShape {
  box: Box;
  ports?: AvoidPort[];
}

interface ShapePins {
  shape: ShapeRef;
  ports: Map<string, number>;
}

/** The route libavoid finds for each edge, by edge index; undefined when the router is not loaded or the pass fails. */
export function avoidRoutes(
  shapes: Map<string, AvoidShape>,
  edges: AvoidEdge[],
  /** Routes stay inside bounds. */
  bounds?: Box,
): Map<number, RenderPoint[]> | undefined {
  if (!avoid) {
    return undefined;
  }
  const api = avoid;
  const router = new api.Router(api.OrthogonalRouting);
  const frameRefs: ShapeRef[] = [];
  try {
    router.setRoutingParameter(api.idealNudgingDistance, NUDGING);
    router.setRoutingParameter(api.segmentPenalty, 50);
    router.setRoutingOption(api.nudgeSharedPathsWithCommonEndPoint, true);
    router.setRoutingOption(api.performUnifyingNudgingPreprocessingStep, true);

    const usable = edges.filter((edge) => shapes.has(edge.from) && shapes.has(edge.to));
    const hasPortEnds = usable.some((edge) =>
      shapes.get(edge.from)?.ports?.some((port) => port.id === edge.fromPort) ||
      shapes.get(edge.to)?.ports?.some((port) => port.id === edge.toPort),
    );
    router.setRoutingParameter(api.shapeBufferDistance, CLEARANCE);
    // Shape-connected nudging moves crowded port endpoints away from their faces.
    router.setRoutingOption(api.nudgeOrthogonalSegmentsConnectedToShapes, !hasPortEnds);
    const counts = new Map([...shapes.keys()].map((id) => [id, 0]));
    for (const edge of usable) {
      counts.set(edge.from, counts.get(edge.from)! + 1);
      counts.set(edge.to, counts.get(edge.to)! + 1);
    }
    const refs = new Map<string, ShapePins>();
    let nextPortClass = PIN_CLASS + 1;
    for (const [id, shape] of shapes) {
      refs.set(
        id,
        addShape(api, router, shape, (counts.get(id) ?? 0) <= EXCLUSIVE_PIN_LIMIT, () => nextPortClass++),
      );
    }
    if (bounds) {
      frameRefs.push(...addBoundsFrames(api, router, bounds));
    }

    const connectors: Array<{ index: number; connector: ConnRef; edge: AvoidEdge }> = [];
    for (const edge of usable) {
      const source = refs.get(edge.from)!;
      const target = refs.get(edge.to)!;
      const sourceEnd = new api.ConnEnd(source.shape, source.ports.get(edge.fromPort ?? "") ?? PIN_CLASS);
      const targetEnd = new api.ConnEnd(target.shape, target.ports.get(edge.toPort ?? "") ?? PIN_CLASS);
      let connector: ConnRef;
      try {
        connector = new api.ConnRef(router, sourceEnd, targetEnd);
      } finally {
        api.destroy(sourceEnd);
        api.destroy(targetEnd);
      }
      connector.setRoutingType(api.ConnType_Orthogonal);
      connectors.push({ index: edge.index, connector, edge });
    }

    router.processTransaction();

    const routes = new Map<number, RenderPoint[]>();
    for (const { index, connector, edge } of connectors) {
      // The display route's points are borrowed from the WASM heap, never destroyed.
      const displayRoute = connector.displayRoute();
      const points: RenderPoint[] = [];
      for (let i = 0; i < displayRoute.size(); i++) {
        const point = displayRoute.get_ps(i);
        points.push({ x: point.x, y: point.y });
      }
      const source = refs.get(edge.from)!;
      const target = refs.get(edge.to)!;
      const sourceShapeData = shapes.get(edge.from)!;
      const targetShapeData = shapes.get(edge.to)!;
      const hasPort =
        (edge.fromPort !== undefined && source.ports.has(edge.fromPort)) ||
        (edge.toPort !== undefined && target.ports.has(edge.toPort));
      const route = cleanRoute(points, hasPort);
      if (route) {
        const sourcePort = sourceShapeData.ports?.find((port) => port.id === edge.fromPort);
        const targetPort = targetShapeData.ports?.find((port) => port.id === edge.toPort);
        if (sourcePort) {
          alignPortFace(route, 0, 1, sourceShapeData.box, sourcePort);
        }
        if (targetPort) {
          alignPortFace(route, route.length - 1, route.length - 2, targetShapeData.box, targetPort);
        }
        routes.set(index, route);
      }
    }
    return straightenJogs(routes, [...shapes.values()].map(({ box }) => box), bounds);
  } catch (error) {
    reportAvoidFailure("routing failed", error);
    return undefined;
  } finally {
    api.destroy(router);
  }
}

interface OrthogonalSegment {
  axis: "horizontal" | "vertical";
  direction: -1 | 1;
  length: number;
}

interface JogShift {
  points: [number, number];
  movedSegment: number;
  changedSegment: number;
  mergedSegment: [number, number];
  delta: RenderPoint;
}

/** Straightens short Z-jogs when the replacement route remains clear. */
export function straightenJogs(
  routes: Map<number, RenderPoint[]>,
  obstacles: Box[],
  bounds?: Box,
): Map<number, RenderPoint[]> {
  const straightened = new Map<number, RenderPoint[]>();
  for (const [index, route] of routes) {
    straightened.set(index, route.map(({ x, y }) => ({ x, y })));
  }

  while (true) {
    let changed = false;
    for (const [index, route] of straightened) {
      for (let jog = 1; jog + 2 < route.length; jog++) {
        const shifts = jogShifts(route, jog);
        const ordered = shifts.sort(
          (first, second) =>
            segmentLength(route[first.movedSegment], route[first.movedSegment + 1]) -
            segmentLength(route[second.movedSegment], route[second.movedSegment + 1]),
        );
        for (const shift of ordered) {
          const candidate = shiftedRoute(route, shift);
          if (!candidate || !legalShift(index, route, candidate, shift, straightened, obstacles, bounds)) {
            continue;
          }
          straightened.set(index, compactRoute(candidate));
          changed = true;
          break;
        }
        if (changed) {
          break;
        }
      }
      if (changed) {
        break;
      }
    }
    if (!changed) {
      return straightened;
    }
  }
}

function jogShifts(route: RenderPoint[], jog: number): JogShift[] {
  const previous = orthogonalSegment(route[jog - 1], route[jog]);
  const short = orthogonalSegment(route[jog], route[jog + 1]);
  const next = orthogonalSegment(route[jog + 1], route[jog + 2]);
  if (
    !previous ||
    !short ||
    !next ||
    short.length >= MIN_JOG ||
    previous.axis !== next.axis ||
    previous.axis === short.axis ||
    previous.direction !== next.direction
  ) {
    return [];
  }

  const delta = { x: route[jog + 1].x - route[jog].x, y: route[jog + 1].y - route[jog].y };
  const shifts: JogShift[] = [];
  if (jog > 1) {
    shifts.push({
      points: [jog - 1, jog],
      movedSegment: jog - 1,
      changedSegment: jog - 2,
      mergedSegment: [jog - 1, jog + 2],
      delta,
    });
  }
  if (jog + 2 < route.length - 1) {
    shifts.push({
      points: [jog + 1, jog + 2],
      movedSegment: jog + 1,
      changedSegment: jog + 2,
      mergedSegment: [jog - 1, jog + 2],
      delta: { x: -delta.x, y: -delta.y },
    });
  }
  return shifts;
}

function shiftedRoute(route: RenderPoint[], shift: JogShift): RenderPoint[] {
  return route.map((point, index) =>
    shift.points.includes(index)
      ? { x: point.x + shift.delta.x, y: point.y + shift.delta.y }
      : { x: point.x, y: point.y },
  );
}

function legalShift(
  index: number,
  route: RenderPoint[],
  candidate: RenderPoint[],
  shift: JogShift,
  routes: Map<number, RenderPoint[]>,
  obstacles: Box[],
  bounds?: Box,
): boolean {
  if (bounds && shift.points.some((point) => !inside(candidate[point], bounds))) {
    return false;
  }
  for (const segmentIndex of [shift.movedSegment, shift.changedSegment]) {
    const before = orthogonalSegment(route[segmentIndex], route[segmentIndex + 1]);
    const after = orthogonalSegment(candidate[segmentIndex], candidate[segmentIndex + 1]);
    if (!before || !after || before.axis !== after.axis || before.direction !== after.direction) {
      return false;
    }
    if (
      (segmentIndex === 0 || segmentIndex === route.length - 2) &&
      after.length < Math.min(before.length, CLEARANCE)
    ) {
      return false;
    }
    if (obstacles.some((box) => crossesInterior(candidate[segmentIndex], candidate[segmentIndex + 1], box))) {
      return false;
    }
    for (const [otherIndex, other] of routes) {
      if (
        otherIndex !== index &&
        other.slice(1).some((point, otherSegment) =>
          lanesTooClose(
            candidate[segmentIndex],
            candidate[segmentIndex + 1],
            other[otherSegment],
            point,
          ),
        )
      ) {
        return false;
      }
    }
  }
  const [mergeStart, mergeEnd] = shift.mergedSegment;
  const merged = orthogonalSegment(candidate[mergeStart], candidate[mergeEnd]);
  if (!merged) {
    return false;
  }
  if (
    bounds &&
    (!inside(candidate[mergeStart], bounds) || !inside(candidate[mergeEnd], bounds))
  ) {
    return false;
  }
  if (obstacles.some((box) => crossesInterior(candidate[mergeStart], candidate[mergeEnd], box))) {
    return false;
  }
  if (
    (mergeStart === 0 &&
      merged.length < Math.min(segmentLength(route[0], route[1]), CLEARANCE)) ||
    (mergeEnd === route.length - 1 &&
      merged.length < Math.min(segmentLength(route.at(-2)!, route.at(-1)!), CLEARANCE))
  ) {
    return false;
  }
  for (const [otherIndex, other] of routes) {
    if (
      otherIndex !== index &&
      other.slice(1).some((point, otherSegment) =>
        lanesTooClose(
          candidate[mergeStart],
          candidate[mergeEnd],
          other[otherSegment],
          point,
        ),
      )
    ) {
      return false;
    }
  }
  return true;
}

function orthogonalSegment(a: RenderPoint, b: RenderPoint): OrthogonalSegment | undefined {
  if (a.x === b.x && a.y !== b.y) {
    return { axis: "vertical", direction: Math.sign(b.y - a.y) as -1 | 1, length: Math.abs(b.y - a.y) };
  }
  if (a.y === b.y && a.x !== b.x) {
    return { axis: "horizontal", direction: Math.sign(b.x - a.x) as -1 | 1, length: Math.abs(b.x - a.x) };
  }
  return undefined;
}

function segmentLength(a: RenderPoint, b: RenderPoint): number {
  return Math.abs(b.x - a.x) + Math.abs(b.y - a.y);
}

function inside(point: RenderPoint, bounds: Box): boolean {
  return (
    point.x >= bounds.x &&
    point.y >= bounds.y &&
    point.x <= bounds.x + bounds.width &&
    point.y <= bounds.y + bounds.height
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

function lanesTooClose(a: RenderPoint, b: RenderPoint, c: RenderPoint, d: RenderPoint): boolean {
  const first = orthogonalSegment(a, b);
  const second = orthogonalSegment(c, d);
  if (!first || !second || first.axis !== second.axis) {
    return false;
  }
  if (first.axis === "horizontal") {
    return (
      Math.abs(a.y - c.y) < NUDGING &&
      Math.min(Math.max(a.x, b.x), Math.max(c.x, d.x)) >
        Math.max(Math.min(a.x, b.x), Math.min(c.x, d.x))
    );
  }
  return (
    Math.abs(a.x - c.x) < NUDGING &&
    Math.min(Math.max(a.y, b.y), Math.max(c.y, d.y)) >
      Math.max(Math.min(a.y, b.y), Math.min(c.y, d.y))
  );
}

function compactRoute(route: RenderPoint[]): RenderPoint[] {
  const compacted: RenderPoint[] = [];
  for (const point of route) {
    if (compacted.at(-1)?.x === point.x && compacted.at(-1)?.y === point.y) {
      continue;
    }
    while (compacted.length >= 2) {
      const previous = orthogonalSegment(compacted.at(-2)!, compacted.at(-1)!);
      const next = orthogonalSegment(compacted.at(-1)!, point);
      if (!previous || !next || previous.axis !== next.axis || previous.direction !== next.direction) {
        break;
      }
      compacted.pop();
    }
    compacted.push(point);
  }
  return compacted;
}

function addBoundsFrames(
  api: Avoid,
  router: InstanceType<Avoid["Router"]>,
  bounds: Box,
): ShapeRef[] {
  const thickness = Math.max(bounds.width, bounds.height, CLEARANCE);
  const horizontalWidth = bounds.width + 2 * thickness;
  const verticalHeight = bounds.height + 2 * thickness;
  const frames: Box[] = [
    {
      x: bounds.x - thickness,
      y: bounds.y - CLEARANCE - thickness,
      width: horizontalWidth,
      height: thickness,
    },
    {
      x: bounds.x - thickness,
      y: bounds.y + bounds.height + CLEARANCE,
      width: horizontalWidth,
      height: thickness,
    },
    {
      x: bounds.x - CLEARANCE - thickness,
      y: bounds.y - thickness,
      width: thickness,
      height: verticalHeight,
    },
    {
      x: bounds.x + bounds.width + CLEARANCE,
      y: bounds.y - thickness,
      width: thickness,
      height: verticalHeight,
    },
  ];
  return frames.map((box) => {
    const center = new api.Point(box.x + box.width / 2, box.y + box.height / 2);
    const rectangle = new api.Rectangle(center, box.width, box.height);
    try {
      return new api.ShapeRef(router, rectangle);
    } finally {
      api.destroy(rectangle);
      api.destroy(center);
    }
  });
}

// addShape is a box as a routing obstacle, with twelve proportional pins spread
// over its sides for connectors to attach at.
function addShape(
  api: Avoid,
  router: InstanceType<Avoid["Router"]>,
  shapeData: AvoidShape,
  exclusive: boolean,
  allocatePortClass: () => number,
): ShapePins {
  const box = shapeData.box;
  const portsOn = (side: PortPosition["side"]) => shapeData.ports?.some((port) => port.side === side) ?? false;
  const westGrowth = portsOn("west") ? PORT_SIZE / 2 : 0;
  const eastGrowth = portsOn("east") ? PORT_SIZE / 2 : 0;
  const northGrowth = portsOn("north") ? PORT_SIZE / 2 : 0;
  const southGrowth = portsOn("south") ? PORT_SIZE / 2 : 0;
  const routingBox = {
    x: box.x - westGrowth,
    y: box.y - northGrowth,
    width: box.width + westGrowth + eastGrowth,
    height: box.height + northGrowth + southGrowth,
  };
  const center = new api.Point(routingBox.x + routingBox.width / 2, routingBox.y + routingBox.height / 2);
  const rectangle = new api.Rectangle(center, routingBox.width, routingBox.height);
  let shape: ShapeRef;
  try {
    shape = new api.ShapeRef(router, rectangle);
  } finally {
    api.destroy(rectangle);
    api.destroy(center);
  }

  const offsets = [0.25, 0.5, 0.75] as const;
  const position = (x: number, y: number, direction: number) =>
    [(x - routingBox.x) / routingBox.width, (y - routingBox.y) / routingBox.height, direction] as const;
  const pins = [
    ...offsets.map((offset) => position(box.x + box.width * offset, box.y, api.ConnDirUp)),
    ...offsets.map((offset) => position(box.x + box.width, box.y + box.height * offset, api.ConnDirRight)),
    ...offsets.map((offset) => position(box.x + box.width * offset, box.y + box.height, api.ConnDirDown)),
    ...offsets.map((offset) => position(box.x, box.y + box.height * offset, api.ConnDirLeft)),
  ];
  for (const [x, y, direction] of pins) {
    const pin = new api.ShapeConnectionPin(shape, PIN_CLASS, x, y, true, 0, direction);
    pin.setExclusive(exclusive);
  }
  const ports = new Map<string, number>();
  (shapeData.ports ?? []).forEach((port) => {
    const classId = allocatePortClass();
    const face = portFace(box, port);
    let x: number;
    let y: number;
    let direction: number;
    switch (port.side) {
      case "north":
        x = (face.x - routingBox.x) / routingBox.width;
        y = 0;
        direction = api.ConnDirUp;
        break;
      case "east":
        x = 1;
        y = (face.y - routingBox.y) / routingBox.height;
        direction = api.ConnDirRight;
        break;
      case "south":
        x = (face.x - routingBox.x) / routingBox.width;
        y = 1;
        direction = api.ConnDirDown;
        break;
      case "west":
        x = 0;
        y = (face.y - routingBox.y) / routingBox.height;
        direction = api.ConnDirLeft;
        break;
    }
    const pin = new api.ShapeConnectionPin(shape, classId, x, y, true, 0, direction);
    pin.setExclusive(false);
    ports.set(port.id, classId);
  });
  return { shape, ports };
}

// cleanRoute keeps port-face coordinates exact and drops doubled points and mid-segment bends.
function cleanRoute(points: RenderPoint[], preservePortFaces = false): RenderPoint[] | undefined {
  const normalized: RenderPoint[] = [];
  for (const point of points) {
    const next = preservePortFaces ? { x: point.x, y: point.y } : { x: snap(point.x), y: snap(point.y) };
    const last = normalized.at(-1);
    if (!last || last.x !== next.x || last.y !== next.y) {
      normalized.push(next);
    }
  }
  const clean: RenderPoint[] = [];
  for (const point of normalized) {
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

function alignPortFace(route: RenderPoint[], pointIndex: number, adjacentIndex: number, box: Box, port: AvoidPort): void {
  const face = portFace(box, port);
  route[pointIndex] = face;
  const adjacent = route[adjacentIndex];
  const vertical = port.side === "north" || port.side === "south";
  const actual = vertical ? adjacent.x : adjacent.y;
  const expected = vertical ? face.x : face.y;
  const tolerance = Number.EPSILON * Math.max(1, Math.abs(actual), Math.abs(expected)) * 4;
  if (Math.abs(actual - expected) <= tolerance) {
    route[adjacentIndex] = vertical ? { ...adjacent, x: expected } : { ...adjacent, y: expected };
  }
}

function collinear(a: RenderPoint, b: RenderPoint, c: RenderPoint): boolean {
  return (a.x === b.x && b.x === c.x) || (a.y === b.y && b.y === c.y);
}
