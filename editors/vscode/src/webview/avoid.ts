// Routes edges orthogonally around the shapes with libavoid's WASM router; every
// route it finds is the panel's own, never written to the model by drawing it.
import { AvoidLib } from "libavoid-js";

import { GAP, portFace, PORT_SIZE, snap, type Box, type PortPosition } from "./geometry";
import type { RenderPoint } from "../protocol";

export const CLEARANCE = GAP / 2;
export const EXCLUSIVE_PIN_LIMIT = 12;

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
export function avoidRoutes(shapes: Map<string, AvoidShape>, edges: AvoidEdge[]): Map<number, RenderPoint[]> | undefined {
  if (!avoid) {
    return undefined;
  }
    const api = avoid;
    const router = new api.Router(api.OrthogonalRouting);
    try {
    router.setRoutingParameter(api.idealNudgingDistance, 8);
    router.setRoutingParameter(api.segmentPenalty, 50);
    router.setRoutingOption(api.nudgeSharedPathsWithCommonEndPoint, true);
    router.setRoutingOption(api.performUnifyingNudgingPreprocessingStep, true);

    const usable = edges.filter((edge) => shapes.has(edge.from) && shapes.has(edge.to));
    const hasPortEnds = usable.some((edge) =>
      shapes.get(edge.from)?.ports?.some((port) => port.id === edge.fromPort) ||
      shapes.get(edge.to)?.ports?.some((port) => port.id === edge.toPort),
    );
    // Port faces project half a glyph beyond the node border.
    router.setRoutingParameter(api.shapeBufferDistance, CLEARANCE - (hasPortEnds ? PORT_SIZE / 2 : 0));
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
      const hasPort =
        (edge.fromPort !== undefined && source.ports.has(edge.fromPort)) ||
        (edge.toPort !== undefined && target.ports.has(edge.toPort));
      const route = cleanRoute(points, hasPort);
      if (route) {
        routes.set(index, route);
      }
    }
    return routes;
  } catch (error) {
    reportAvoidFailure("routing failed", error);
    return undefined;
  } finally {
    api.destroy(router);
  }
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
  const center = new api.Point(box.x + box.width / 2, box.y + box.height / 2);
  const rectangle = new api.Rectangle(center, box.width, box.height);
  let shape: ShapeRef;
  try {
    shape = new api.ShapeRef(router, rectangle);
  } finally {
    api.destroy(rectangle);
    api.destroy(center);
  }

  const offsets = [0.25, 0.5, 0.75] as const;
  const pins = [
    ...offsets.map((offset) => [offset, 0, api.ConnDirUp] as const),
    ...offsets.map((offset) => [1, offset, api.ConnDirRight] as const),
    ...offsets.map((offset) => [offset, 1, api.ConnDirDown] as const),
    ...offsets.map((offset) => [0, offset, api.ConnDirLeft] as const),
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
        x = (face.x - box.x) / box.width;
        y = 0;
        direction = api.ConnDirUp;
        break;
      case "east":
        x = 1;
        y = (face.y - box.y) / box.height;
        direction = api.ConnDirRight;
        break;
      case "south":
        x = (face.x - box.x) / box.width;
        y = 1;
        direction = api.ConnDirDown;
        break;
      case "west":
        x = 0;
        y = (face.y - box.y) / box.height;
        direction = api.ConnDirLeft;
        break;
    }
    const pin = new api.ShapeConnectionPin(shape, classId, x, y, true, -PORT_SIZE / 2, direction);
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

function collinear(a: RenderPoint, b: RenderPoint, c: RenderPoint): boolean {
  return (a.x === b.x && b.x === c.x) || (a.y === b.y && b.y === c.y);
}
