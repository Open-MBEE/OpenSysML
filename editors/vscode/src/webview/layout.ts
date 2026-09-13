// Lays a rendering out on the canvas: where the model states a node's place it goes
// exactly there, and where it does not the node takes a slot in a grid under its
// owner; a sequence is lifelines in a row with its messages down them. Pure
// geometry, in the canvas's pixels with y down; the SVG is drawn from it.
import type {
  EdgePlacement,
  LayoutGeometry,
  NodePlacement,
  RenderEdge,
  RenderNode,
  RenderPoint,
  RenderResult,
} from "../protocol";

export interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** How a node is drawn: the label box of an element, or the symbol of a control node. */
export type Shape = "box" | "point" | "circle" | "ring" | "diamond" | "bar" | "history";

export interface PlacedNode {
  node: RenderNode;
  box: Box;
  /** The label's lines: head, «kind», then the detail; empty for a symbol. */
  lines: string[];
  shape: Shape;
  /** The model, or a gesture in progress, states where the node goes. */
  pinned: boolean;
  children: PlacedNode[];
  parent?: PlacedNode;
  /** In a sequence, the y the node's lifeline runs down to from its box. */
  lifeline?: number;
}

export interface PlacedEdge {
  edge: RenderEdge;
  /** The edge's index in the rendering's edges, which a route edit names it by. */
  index: number;
  /** The polyline drawn: the anchor on the source, the route's waypoints, the anchor on the target. */
  points: RenderPoint[];
  /** The waypoints the edge is steered through, `points` without its two anchors. */
  route: RenderPoint[];
  /** Where the label sits, on the polyline's midpoint. */
  label: RenderPoint;
}

export interface CanvasLayout {
  roots: PlacedNode[];
  nodes: Map<string, PlacedNode>;
  edges: PlacedEdge[];
  width: number;
  height: number;
  /** The rendering's kind reads DiagramLayout back, so gestures on it can be kept. */
  placeable: boolean;
}

/** The rendering kinds whose renderers position nodes and route edges from DiagramLayout. */
const PLACEABLE_KINDS = new Set(["tree", "interconnection", "state", "action"]);

/** Geometry a gesture in progress shows in place of the model's, before the model says so. */
export interface Overrides {
  nodes?: Map<string, LayoutGeometry>;
  /** By edge index; an entry of no points shows the edge straight. */
  routes?: Map<number, RenderPoint[] | undefined>;
}

/** What one completed gesture puts in the model. */
export interface Placements {
  nodes: NodePlacement[];
  edges: EdgePlacement[];
}

export const FONT_SIZE = 13;
const LINE_HEIGHT = 18;
/** The width of an average glyph at FONT_SIZE in the workbench's proportional font, generously. */
const GLYPH_WIDTH = 7.5;
const BOLD_GLYPH_WIDTH = 8;
const LABEL_PAD_X = 12;
const LABEL_PAD_Y = 8;
const MIN_WIDTH = 96;
const MIN_HEIGHT = 40;
/** Between slots of one grid, and between a container's border and its slots. */
export const GAP = 32;
const CONTAINER_PAD = 16;
/** The canvas's margin around the outermost boxes. */
export const MARGIN = 24;
const POINT_SIZE = 12;
const SYMBOL_SIZE = 28;
const BAR_WIDTH = 48;
const BAR_HEIGHT = 6;
/** How far a self-loop swings out from its node. */
const LOOP_REACH = 36;
/** Between the messages of a sequence, and below its last one. */
const MESSAGE_GAP = 40;
/** How far a message to its own lifeline reaches out. */
const SELF_MESSAGE_REACH = 28;

/** How a gesture's positions are written, so the model reads back in whole pixels. */
export function snap(value: number): number {
  return Math.round(value);
}

/** layoutCanvas places every node and routes every edge of the rendering. */
export function layoutCanvas(result: RenderResult, overrides: Overrides = {}): CanvasLayout {
  const placed = new Map<string, PlacedNode>();
  const roots: PlacedNode[] = [];
  for (const node of result.nodes ?? []) {
    const entry: PlacedNode = {
      node,
      box: { x: 0, y: 0, width: 0, height: 0 },
      lines: labelLines(node),
      shape: shapeOf(node.kind),
      pinned: false,
      children: [],
    };
    placed.set(node.id, entry);
  }
  for (const entry of placed.values()) {
    const parent = entry.node.parent ? placed.get(entry.node.parent) : undefined;
    if (parent && parent !== entry) {
      entry.parent = parent;
      parent.children.push(entry);
    } else {
      roots.push(entry);
    }
  }
  if (result.kind === "sequence") {
    return layoutSequence(result, roots, placed);
  }
  const geometry = (entry: PlacedNode): LayoutGeometry | undefined => {
    const override = overrides.nodes?.get(entry.node.id);
    if (override) {
      return override;
    }
    const { x, y, width, height, collapsed } = entry.node;
    return x !== undefined && y !== undefined ? { x, y, width, height, collapsed } : undefined;
  };
  const sizes = new Map<PlacedNode, { width: number; height: number }>();
  for (const root of roots) {
    measure(root, geometry, sizes);
  }
  const grid = gridOf(roots, sizes, { x: MARGIN, y: MARGIN });
  roots.forEach((root, i) => place(root, grid[i], geometry, sizes));

  let width = MARGIN;
  let height = MARGIN;
  for (const root of roots) {
    width = Math.max(width, root.box.x + root.box.width);
    height = Math.max(height, root.box.y + root.box.height);
  }
  const edges = (result.edges ?? []).map((edge, index) => routeEdge(edge, index, placed, overrides.routes));
  for (const edge of edges) {
    for (const point of edge.points) {
      width = Math.max(width, point.x);
      height = Math.max(height, point.y);
    }
  }
  return {
    roots,
    nodes: placed,
    edges,
    width: Math.max(width + MARGIN, result.canvas?.width ?? 0),
    height: Math.max(height + MARGIN, result.canvas?.height ?? 0),
    placeable: PLACEABLE_KINDS.has(result.kind),
  };
}

// layoutSequence lays lifelines out in a row, in the order rendered, and its
// messages down them in the order the model states; nothing here reads the model's
// geometry, since a sequence rendering carries none.
function layoutSequence(result: RenderResult, roots: PlacedNode[], placed: Map<string, PlacedNode>): CanvasLayout {
  const edges = result.edges ?? [];
  const bottom = MARGIN + headHeight(roots) + MESSAGE_GAP * (edges.length + 1);
  let x = MARGIN;
  for (const root of roots) {
    const size = labelSize(root.lines);
    root.box = { x, y: MARGIN, width: size.width, height: size.height };
    root.lifeline = bottom;
    x += size.width + GAP;
  }
  const routed = edges.map((edge, index): PlacedEdge => {
    const y = MARGIN + headHeight(roots) + MESSAGE_GAP * (index + 1);
    const from = placed.get(edge.from)?.box;
    const to = placed.get(edge.to)?.box;
    const fromX = from ? from.x + from.width / 2 : MARGIN;
    const toX = to ? to.x + to.width / 2 : MARGIN;
    const points = edge.from === edge.to
      ? [{ x: fromX, y }, { x: fromX + SELF_MESSAGE_REACH, y }, { x: fromX + SELF_MESSAGE_REACH, y: y + MESSAGE_GAP / 2 }, { x: fromX, y: y + MESSAGE_GAP / 2 }]
      : [{ x: fromX, y }, { x: toX, y }];
    return { edge, index, points, route: [], label: midpoint(points) };
  });
  return {
    roots,
    nodes: placed,
    edges: routed,
    width: Math.max(x - GAP, MARGIN) + MARGIN,
    height: bottom + MARGIN,
    placeable: false,
  };
}

// headHeight is the tallest lifeline box, so every lifeline starts under the same line.
function headHeight(roots: PlacedNode[]): number {
  return roots.reduce((max, root) => Math.max(max, labelSize(root.lines).height), 0);
}

// measure is the size a node takes when nothing places it wider: the model's
// size when stated, else its label around the grid of its children.
function measure(
  entry: PlacedNode,
  geometry: (entry: PlacedNode) => LayoutGeometry | undefined,
  sizes: Map<PlacedNode, { width: number; height: number }>,
): { width: number; height: number } {
  const stated = geometry(entry);
  const shown = shownChildren(entry, stated);
  for (const child of shown) {
    measure(child, geometry, sizes);
  }
  let size: { width: number; height: number };
  if (stated?.width !== undefined && stated.height !== undefined) {
    size = { width: stated.width, height: stated.height };
  } else {
    size = symbolSize(entry.shape) ?? labelSize(entry.lines);
    if (shown.length > 0) {
      const grid = gridOf(shown, sizes, { x: 0, y: 0 });
      const extent = gridExtent(grid, shown, sizes);
      size = {
        width: Math.max(size.width, extent.width + 2 * CONTAINER_PAD),
        height: size.height + extent.height + CONTAINER_PAD,
      };
    }
  }
  sizes.set(entry, size);
  return size;
}

// place puts a node at the model's position or its slot, then its children in
// the grid below its label; a box the model does not size grows to hold a child
// the model put beyond it.
function place(
  entry: PlacedNode,
  slot: RenderPoint,
  geometry: (entry: PlacedNode) => LayoutGeometry | undefined,
  sizes: Map<PlacedNode, { width: number; height: number }>,
): void {
  const stated = geometry(entry);
  const size = sizes.get(entry) ?? { width: MIN_WIDTH, height: MIN_HEIGHT };
  entry.pinned = stated !== undefined;
  entry.box = { x: stated?.x ?? slot.x, y: stated?.y ?? slot.y, width: size.width, height: size.height };
  const shown = shownChildren(entry, stated);
  if (shown.length === 0) {
    return;
  }
  const header = symbolSize(entry.shape) ?? labelSize(entry.lines);
  const grid = gridOf(shown, sizes, { x: entry.box.x + CONTAINER_PAD, y: entry.box.y + header.height });
  shown.forEach((child, i) => place(child, grid[i], geometry, sizes));
  if (stated?.width !== undefined && stated.height !== undefined) {
    return;
  }
  for (const child of shown) {
    entry.box.width = Math.max(entry.box.width, child.box.x + child.box.width + CONTAINER_PAD - entry.box.x);
    entry.box.height = Math.max(entry.box.height, child.box.y + child.box.height + CONTAINER_PAD - entry.box.y);
  }
}

// shownChildren is the children drawn inside a node: none when it is collapsed.
function shownChildren(entry: PlacedNode, stated: LayoutGeometry | undefined): PlacedNode[] {
  return stated?.collapsed ? [] : entry.children;
}

// gridOf assigns every entry a slot in a near-square grid from origin, in order,
// each column as wide and each row as tall as its widest and tallest entry. A
// pinned entry keeps its slot empty, so its siblings do not shift when it moves.
function gridOf(
  entries: PlacedNode[],
  sizes: Map<PlacedNode, { width: number; height: number }>,
  origin: RenderPoint,
): RenderPoint[] {
  const columns = Math.max(1, Math.ceil(Math.sqrt(entries.length)));
  const columnWidths: number[] = [];
  const rowHeights: number[] = [];
  entries.forEach((entry, i) => {
    const size = sizes.get(entry) ?? { width: MIN_WIDTH, height: MIN_HEIGHT };
    const column = i % columns;
    const row = Math.floor(i / columns);
    columnWidths[column] = Math.max(columnWidths[column] ?? 0, size.width);
    rowHeights[row] = Math.max(rowHeights[row] ?? 0, size.height);
  });
  const columnOffsets = offsets(columnWidths, origin.x);
  const rowOffsets = offsets(rowHeights, origin.y);
  return entries.map((_, i) => ({ x: columnOffsets[i % columns], y: rowOffsets[Math.floor(i / columns)] }));
}

function offsets(extents: number[], start: number): number[] {
  const out: number[] = [];
  let at = start;
  for (const extent of extents) {
    out.push(at);
    at += extent + GAP;
  }
  return out;
}

// gridExtent is the size a grid of slots spans from its origin.
function gridExtent(
  grid: RenderPoint[],
  entries: PlacedNode[],
  sizes: Map<PlacedNode, { width: number; height: number }>,
): { width: number; height: number } {
  let width = 0;
  let height = 0;
  entries.forEach((entry, i) => {
    const size = sizes.get(entry) ?? { width: MIN_WIDTH, height: MIN_HEIGHT };
    width = Math.max(width, grid[i].x + size.width);
    height = Math.max(height, grid[i].y + size.height);
  });
  return { width, height };
}

/** labelLines is a node's label as the graphical notation orders it: head, «kind», detail. */
export function labelLines(node: RenderNode): string[] {
  if (symbolSize(shapeOf(node.kind))) {
    return [];
  }
  const head = node.name === "" ? node.kind : node.type === "" ? node.name : `${node.name} : ${node.type}`;
  const lines = [head];
  if (node.name !== "") {
    lines.push(`«${node.kind}»`);
  }
  if (node.detail !== "") {
    lines.push(node.detail);
  }
  return lines;
}

// labelSize is the box a label needs, its head in bold glyphs.
function labelSize(lines: string[]): { width: number; height: number } {
  let width = 0;
  lines.forEach((line, i) => {
    width = Math.max(width, [...line].length * (i === 0 ? BOLD_GLYPH_WIDTH : GLYPH_WIDTH));
  });
  return {
    width: Math.max(MIN_WIDTH, Math.ceil(width + 2 * LABEL_PAD_X)),
    height: Math.max(MIN_HEIGHT, lines.length * LINE_HEIGHT + 2 * LABEL_PAD_Y),
  };
}

/** shapeOf is the symbol a control node's kind is drawn as, or a label box. */
export function shapeOf(kind: string): Shape {
  switch (kind) {
    case "start":
      return "point";
    case "initial":
      return "circle";
    case "final":
      return "ring";
    case "fork":
    case "join":
      return "bar";
    case "decision":
    case "merge":
    case "choice":
    case "junction":
      return "diamond";
    case "shallow history":
    case "deep history":
      return "history";
    default:
      return "box";
  }
}

// symbolSize is a symbol's fixed size; undefined for a label box.
function symbolSize(shape: Shape): { width: number; height: number } | undefined {
  switch (shape) {
    case "point":
      return { width: POINT_SIZE, height: POINT_SIZE };
    case "circle":
    case "ring":
    case "diamond":
    case "history":
      return { width: SYMBOL_SIZE, height: SYMBOL_SIZE };
    case "bar":
      return { width: BAR_WIDTH, height: BAR_HEIGHT };
    case "box":
      return undefined;
  }
}

// routeEdge is an edge's polyline: from the border of its source, through the
// route's waypoints, to the border of its target; a self-loop swings out to the right.
function routeEdge(
  edge: RenderEdge,
  index: number,
  placed: Map<string, PlacedNode>,
  routes: Map<number, RenderPoint[] | undefined> | undefined,
): PlacedEdge {
  const route = (routes?.has(index) ? routes.get(index) : edge.route) ?? [];
  const from = placed.get(edge.from)?.box ?? { x: 0, y: 0, width: 0, height: 0 };
  const to = placed.get(edge.to)?.box ?? { x: 0, y: 0, width: 0, height: 0 };
  let inner = route;
  if (route.length === 0 && edge.from === edge.to) {
    inner = [
      { x: from.x + from.width + LOOP_REACH, y: from.y + from.height / 3 },
      { x: from.x + from.width + LOOP_REACH, y: from.y + (2 * from.height) / 3 },
    ];
  }
  const start = anchor(from, inner[0] ?? center(to));
  const end = anchor(to, inner[inner.length - 1] ?? center(from));
  const points = [start, ...inner, end];
  return { edge, index, points, route, label: midpoint(points) };
}

function center(box: Box): RenderPoint {
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

// anchor is where the line from a box's center toward a point leaves the box.
export function anchor(box: Box, toward: RenderPoint): RenderPoint {
  const c = center(box);
  const dx = toward.x - c.x;
  const dy = toward.y - c.y;
  if ((dx === 0 && dy === 0) || box.width === 0 || box.height === 0) {
    return c;
  }
  const scale = Math.min(
    dx === 0 ? Infinity : box.width / 2 / Math.abs(dx),
    dy === 0 ? Infinity : box.height / 2 / Math.abs(dy),
  );
  return { x: c.x + dx * scale, y: c.y + dy * scale };
}

// midpoint is the point halfway along a polyline's length.
function midpoint(points: RenderPoint[]): RenderPoint {
  let total = 0;
  for (let i = 1; i < points.length; i++) {
    total += Math.hypot(points[i].x - points[i - 1].x, points[i].y - points[i - 1].y);
  }
  let remaining = total / 2;
  for (let i = 1; i < points.length; i++) {
    const length = Math.hypot(points[i].x - points[i - 1].x, points[i].y - points[i - 1].y);
    if (remaining <= length || i === points.length - 1) {
      const t = length === 0 ? 0 : remaining / length;
      return { x: points[i - 1].x + (points[i].x - points[i - 1].x) * t, y: points[i - 1].y + (points[i].y - points[i - 1].y) * t };
    }
    remaining -= length;
  }
  return points[0] ?? { x: 0, y: 0 };
}

/** movable reports whether a node can be dragged: the document declares it, so a Layout can name it. */
export function movable(layout: CanvasLayout, entry: PlacedNode): boolean {
  return layout.placeable && entry.node.fqn !== undefined;
}

/** steerable reports whether an edge's route can be edited: the document declares the connection. */
export function steerable(layout: CanvasLayout, edge: PlacedEdge): boolean {
  return layout.placeable && edge.edge.fqn !== undefined;
}

/**
 * movedNode is what dragging a node by (dx, dy) puts in the model: the node itself,
 * every descendant the model already places, and every stated route between nodes
 * of that subtree — all shifted together, so the subtree keeps its shape.
 */
export function movedNode(layout: CanvasLayout, id: string, dx: number, dy: number): Placements | undefined {
  const entry = layout.nodes.get(id);
  if (!entry || !movable(layout, entry)) {
    return undefined;
  }
  const subtree = new Set<string>();
  const nodes: NodePlacement[] = [];
  const visit = (current: PlacedNode, dragged: boolean): void => {
    subtree.add(current.node.id);
    if ((dragged || current.node.x !== undefined) && movable(layout, current)) {
      nodes.push({ id: current.node.id, layout: shifted(current, dx, dy) });
    }
    for (const child of current.children) {
      visit(child, false);
    }
  };
  visit(entry, true);
  const edges: EdgePlacement[] = [];
  for (const edge of layout.edges) {
    if (edge.route.length > 0 && steerable(layout, edge) && subtree.has(edge.edge.from) && subtree.has(edge.edge.to)) {
      edges.push({ index: edge.index, route: edge.route.map((p) => ({ x: snap(p.x + dx), y: snap(p.y + dy) })) });
    }
  }
  return { nodes, edges };
}

// shifted is a node's stated geometry moved by (dx, dy): its size and collapse
// stay as the model has them, so a drag changes position and nothing else.
function shifted(entry: PlacedNode, dx: number, dy: number): LayoutGeometry {
  const { width, height, collapsed } = entry.node;
  const out: LayoutGeometry = { x: snap(entry.box.x + dx), y: snap(entry.box.y + dy) };
  if (width !== undefined && height !== undefined) {
    out.width = width;
    out.height = height;
  }
  if (collapsed) {
    out.collapsed = true;
  }
  return out;
}

/** movedWaypoint is the route with waypoint `point` of edge `index` dragged to `to`. */
export function movedWaypoint(layout: CanvasLayout, index: number, point: number, to: RenderPoint): Placements | undefined {
  const edge = layout.edges[index];
  if (!edge || !steerable(layout, edge) || point < 0 || point >= edge.route.length) {
    return undefined;
  }
  const route = edge.route.map((p, i) => (i === point ? { x: snap(to.x), y: snap(to.y) } : p));
  return { nodes: [], edges: [{ index, route }] };
}

/** insertedWaypoint is the route with a new waypoint at `at`, splitting segment `segment` of the drawn polyline. */
export function insertedWaypoint(layout: CanvasLayout, index: number, segment: number, at: RenderPoint): Placements | undefined {
  const edge = layout.edges[index];
  if (!edge || !steerable(layout, edge) || segment < 0 || segment >= edge.points.length - 1) {
    return undefined;
  }
  // Segment i of the polyline runs from the anchor or waypoint i-1 to waypoint i, so the new one takes index i.
  const route = [...edge.route.slice(0, segment), { x: snap(at.x), y: snap(at.y) }, ...edge.route.slice(segment)];
  return { nodes: [], edges: [{ index, route }] };
}

/** removedWaypoint is the route without waypoint `point`; no route at all once none is left. */
export function removedWaypoint(layout: CanvasLayout, index: number, point: number): Placements | undefined {
  const edge = layout.edges[index];
  if (!edge || !steerable(layout, edge) || point < 0 || point >= edge.route.length) {
    return undefined;
  }
  const route = edge.route.filter((_, i) => i !== point);
  return { nodes: [], edges: [{ index, route: route.length > 0 ? route : undefined }] };
}

/** overridesOf shows placements on the canvas before the model has them. */
export function overridesOf(placements: Placements): Overrides {
  return {
    nodes: new Map(placements.nodes.map((p) => [p.id, p.layout])),
    routes: new Map(placements.edges.map((p) => [p.index, p.route])),
  };
}
