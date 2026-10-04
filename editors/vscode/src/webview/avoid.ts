// Routes edges orthogonally around the shapes with libavoid's WASM router; every
// route it finds is the panel's own, never written to the model by drawing it.
import { AvoidLib } from "libavoid-js";

import { GAP, snap, type Box } from "./geometry";
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
}

/** The route libavoid finds for each edge, by edge index; undefined when the router is not loaded or the pass fails. */
export function avoidRoutes(shapes: Map<string, Box>, edges: AvoidEdge[]): Map<number, RenderPoint[]> | undefined {
  if (!avoid) {
    return undefined;
  }
  const api = avoid;
  const router = new api.Router(api.OrthogonalRouting);
  try {
    router.setRoutingParameter(api.shapeBufferDistance, CLEARANCE);
    router.setRoutingParameter(api.idealNudgingDistance, 8);
    router.setRoutingParameter(api.segmentPenalty, 50);
    router.setRoutingOption(api.nudgeOrthogonalSegmentsConnectedToShapes, true);
    router.setRoutingOption(api.nudgeSharedPathsWithCommonEndPoint, true);
    router.setRoutingOption(api.performUnifyingNudgingPreprocessingStep, true);

    const usable = edges.filter((edge) => shapes.has(edge.from) && shapes.has(edge.to));
    const counts = new Map([...shapes.keys()].map((id) => [id, 0]));
    for (const edge of usable) {
      counts.set(edge.from, counts.get(edge.from)! + 1);
      counts.set(edge.to, counts.get(edge.to)! + 1);
    }
    const refs = new Map<string, ShapeRef>();
    for (const [id, box] of shapes) {
      refs.set(id, addShape(api, router, box, (counts.get(id) ?? 0) <= EXCLUSIVE_PIN_LIMIT));
    }

    const connectors: Array<{ index: number; connector: ConnRef }> = [];
    for (const edge of usable) {
      const sourceEnd = new api.ConnEnd(refs.get(edge.from)!, PIN_CLASS);
      const targetEnd = new api.ConnEnd(refs.get(edge.to)!, PIN_CLASS);
      let connector: ConnRef;
      try {
        connector = new api.ConnRef(router, sourceEnd, targetEnd);
      } finally {
        api.destroy(sourceEnd);
        api.destroy(targetEnd);
      }
      connector.setRoutingType(api.ConnType_Orthogonal);
      connectors.push({ index: edge.index, connector });
    }

    router.processTransaction();

    const routes = new Map<number, RenderPoint[]>();
    for (const { index, connector } of connectors) {
      // The display route's points are borrowed from the WASM heap, never destroyed.
      const displayRoute = connector.displayRoute();
      const points: RenderPoint[] = [];
      for (let i = 0; i < displayRoute.size(); i++) {
        const point = displayRoute.get_ps(i);
        points.push({ x: point.x, y: point.y });
      }
      const route = cleanRoute(points);
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
  box: Box,
  exclusive: boolean,
): ShapeRef {
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
  return shape;
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
