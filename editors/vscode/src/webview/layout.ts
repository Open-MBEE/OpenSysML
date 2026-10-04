// Lays a rendering out on the canvas: where the model states a node's place it goes
// exactly there, and where it does not the node takes a slot in a grid under its
// owner; a sequence is lifelines in a row with its messages down them. Pure
// geometry, in the canvas's pixels with y down; the SVG is drawn from it.
import {
  reachable,
  type EdgePlacement,
  type LayoutGeometry,
  type NodePlacement,
  type RenderEdge,
  type RenderNode,
  type RenderPort,
  type RenderPoint,
  type RenderResult,
} from "../protocol";
import type { AutoLayout } from "./autolayout";
import {
  GAP,
  portBox,
  portCenter,
  portFace,
  PORT_SIZE,
  snap,
  type Box,
  type PortPosition,
  type Side,
} from "./geometry";
import { avoidRoutes, CLEARANCE, type AvoidShape } from "./avoid";

export { GAP, portBox, portCenter, portFace, PORT_SIZE, snap } from "./geometry";
export type { Box, PortPosition, Side } from "./geometry";

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
  /** The node is drawn without its children. */
  collapsed: boolean;
  ports: PlacedPort[];
  /** An owner is collapsed, so the node is not drawn, nor any edge at it. */
  hidden: boolean;
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
  /** An end is under a collapsed owner, so the edge is not drawn. */
  hidden: boolean;
  /** The route was found around the other boxes at layout time; the model states none, so a move does not write it. */
  rerouted: boolean;
}

export interface PlacedPort extends PortPosition {
  port: RenderPort;
}

export interface PortLabelPlacement {
  x: number;
  y: number;
  anchor: "start" | "end";
  bounds: Box;
}

export interface CanvasLayout {
  roots: PlacedNode[];
  nodes: Map<string, PlacedNode>;
  edges: PlacedEdge[];
  /** The canvas's top-left corner: the origin, or above and left of geometry the model puts there. */
  origin: RenderPoint;
  /** The canvas's size from its origin. */
  width: number;
  height: number;
  /** The rendering's kind reads DiagramLayout back, so gestures on it can be kept. */
  placeable: boolean;
}

/** The rendering kinds whose renderers position nodes and route edges from DiagramLayout. */
export const PLACEABLE_KINDS = new Set(["tree", "interconnection", "state", "action"]);

/** Geometry a gesture in progress shows in place of the model's, before the model says so. */
export interface Overrides {
  nodes?: Map<string, LayoutGeometry>;
  /** By edge index; an entry of no points shows the edge straight. */
  routes?: Map<number, RenderPoint[] | undefined>;
  /** Rerouted edges stay inside this box. */
  bounds?: Box;
  /** The layout a gesture started from: its rerouted edges whose ends have not moved are kept, the rest drawn straight, so a drag does not re-route. */
  held?: CanvasLayout;
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
export const CONTAINER_PAD = 16;
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

/**
 * layoutCanvas places every node and routes every edge of the rendering. `auto`
 * is what ELK laid out for the same rendering: it places a node the model does
 * not, without pinning it, and routes an edge whose ends are not placed.
 */
export function layoutCanvas(result: RenderResult, overrides: Overrides = {}, auto?: AutoLayout): CanvasLayout {
  const placed = new Map<string, PlacedNode>();
  const roots: PlacedNode[] = [];
  for (const node of result.nodes ?? []) {
    const entry: PlacedNode = {
      node,
      box: { x: 0, y: 0, width: 0, height: 0 },
      lines: labelLines(node),
      shape: shapeOf(node.kind),
      pinned: false,
      collapsed: false,
      hidden: false,
      children: [],
      ports: [],
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
    const layout = layoutSequence(result, roots, placed);
    placePorts(result.edges ?? [], placed, auto);
    return includePortExtent(layout);
  }
  const geometry = (entry: PlacedNode): NodeGeometry => {
    const override = overrides.nodes?.get(entry.node.id);
    if (override) {
      return { stated: override, pinned: true };
    }
    const { x, y, width, height, collapsed } = entry.node;
    if (x !== undefined && y !== undefined) {
      return { stated: { x, y, width, height, collapsed }, pinned: true };
    }
    // The auto layout's geometry is stated but not pinned: it is a guess, so
    // the node keeps a grid slot for a layout that runs without it.
    const laid = auto?.nodes.get(entry.node.id);
    return laid !== undefined ? { stated: { ...laid, collapsed }, pinned: false } : { pinned: false };
  };
  placeGrid(roots, { x: MARGIN, y: MARGIN }, geometry);
  placePorts(result.edges ?? [], placed, auto);

  // Every drawn box counts, since a placed child may lie beyond a sized owner,
  // and a placed node may lie left of or above the origin.
  const extent = { left: 0, top: 0, right: MARGIN, bottom: MARGIN };
  const reach = (x: number, y: number): void => {
    extent.left = Math.min(extent.left, x);
    extent.top = Math.min(extent.top, y);
    extent.right = Math.max(extent.right, x);
    extent.bottom = Math.max(extent.bottom, y);
  };
  for (const entry of placed.values()) {
    if (entry.hidden) {
      continue;
    }
    const nodeBounds = nodeExtent(entry);
    reach(nodeBounds.x, nodeBounds.y);
    reach(nodeBounds.x + nodeBounds.width, nodeBounds.y + nodeBounds.height);
  }
  const edges = (result.edges ?? []).map((edge, index) => routeEdge(edge, index, placed, overrides.routes, auto));
  rerouteAroundBoxes(edges, placed, roots, overrides, auto);
  for (const edge of edges) {
    if (edge.hidden) {
      continue;
    }
    for (const point of edge.points) {
      reach(point.x, point.y);
    }
  }
  const origin = {
    x: extent.left < 0 ? extent.left - MARGIN : 0,
    y: extent.top < 0 ? extent.top - MARGIN : 0,
  };
  return {
    roots,
    nodes: placed,
    edges,
    origin,
    width: Math.max(extent.right + MARGIN, result.canvas?.width ?? 0) - origin.x,
    height: Math.max(extent.bottom + MARGIN, result.canvas?.height ?? 0) - origin.y,
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
    return { edge, index, points, route: [], label: midpoint(points), hidden: false, rerouted: false };
  });
  return {
    roots,
    nodes: placed,
    edges: routed,
    origin: { x: 0, y: 0 },
    width: Math.max(x - GAP, MARGIN) + MARGIN,
    height: bottom + MARGIN,
    placeable: false,
  };
}

// headHeight is the tallest lifeline box, so every lifeline starts under the same line.
function headHeight(roots: PlacedNode[]): number {
  return roots.reduce((max, root) => Math.max(max, labelSize(root.lines).height), 0);
}

/** Where a node goes and whether the model or a gesture, rather than a guess, states it. */
interface NodeGeometry {
  stated?: LayoutGeometry;
  pinned: boolean;
}

type Geometry = (entry: PlacedNode) => NodeGeometry;

// placeGrid puts entries in a near-square grid from origin, in order, each column
// as wide and each row as tall as its widest and tallest entry. An unsized box's
// extent depends on where it stands (a child the model put beyond it is so much
// further out), so columns are settled left to right and rows top to bottom, each
// entry's extent taken once its slot is known. A pinned entry keeps its slot
// empty, so its siblings do not shift when it moves.
function placeGrid(entries: PlacedNode[], origin: RenderPoint, geometry: Geometry): void {
  const columns = Math.max(1, Math.ceil(Math.sqrt(entries.length)));
  spread(lanes(entries, columns, true), origin.x, (entry, x) => placeAcross(entry, x, geometry));
  spread(lanes(entries, columns, false), origin.y, (entry, y) => placeDown(entry, y, geometry));
}

// lanes groups a grid's entries by column, or by row.
function lanes(entries: PlacedNode[], columns: number, byColumn: boolean): PlacedNode[][] {
  const out: PlacedNode[][] = [];
  entries.forEach((entry, i) => {
    const lane = byColumn ? i % columns : Math.floor(i / columns);
    out[lane] ??= [];
    out[lane].push(entry);
  });
  return out;
}

// spread lays lanes out one after another from start, GAP apart, each as far
// as its longest entry reaches once placed at the lane's offset.
function spread(lanes: PlacedNode[][], start: number, placeAt: (entry: PlacedNode, at: number) => number): void {
  let at = start;
  for (const lane of lanes) {
    let extent = 0;
    for (const entry of lane) {
      extent = Math.max(extent, placeAt(entry, at));
    }
    at += extent + GAP;
  }
}

// placeAcross settles a node's x and width: the model's x, else the slot's; the
// model's width, else its label's, widened to hold every child shown. The columns
// of its children are settled first, from inside its padding.
function placeAcross(entry: PlacedNode, slot: number, geometry: Geometry): number {
  const { stated, pinned } = geometry(entry);
  entry.pinned = pinned;
  entry.collapsed = stated?.collapsed === true;
  if (entry.collapsed) {
    hide(entry.children);
  }
  entry.box.x = stated?.x ?? slot;
  const shown = shownChildren(entry, stated);
  const columns = Math.max(1, Math.ceil(Math.sqrt(shown.length)));
  spread(lanes(shown, columns, true), entry.box.x + CONTAINER_PAD, (child, x) => placeAcross(child, x, geometry));
  if (stated?.width !== undefined && stated.height !== undefined) {
    entry.box.width = stated.width;
    return entry.box.width;
  }
  let width = (symbolSize(entry.shape) ?? labelSize(entry.lines)).width;
  for (const child of shown) {
    width = Math.max(width, child.box.x + child.box.width + CONTAINER_PAD - entry.box.x);
  }
  entry.box.width = width;
  return width;
}

// placeDown settles a node's y and height as placeAcross does its x and width;
// the rows of its children start below its label.
function placeDown(entry: PlacedNode, slot: number, geometry: Geometry): number {
  const { stated } = geometry(entry);
  entry.box.y = stated?.y ?? slot;
  const shown = shownChildren(entry, stated);
  const columns = Math.max(1, Math.ceil(Math.sqrt(shown.length)));
  const header = symbolSize(entry.shape) ?? labelSize(entry.lines);
  spread(lanes(shown, columns, false), entry.box.y + header.height, (child, y) => placeDown(child, y, geometry));
  if (stated?.width !== undefined && stated.height !== undefined) {
    entry.box.height = stated.height;
    return entry.box.height;
  }
  let height = header.height;
  for (const child of shown) {
    height = Math.max(height, child.box.y + child.box.height + CONTAINER_PAD - entry.box.y);
  }
  entry.box.height = height;
  return height;
}

// shownChildren is the children drawn inside a node: none when it is collapsed.
function shownChildren(entry: PlacedNode, stated: LayoutGeometry | undefined): PlacedNode[] {
  return stated?.collapsed ? [] : entry.children;
}

// hide marks a collapsed node's subtree as not drawn.
function hide(entries: PlacedNode[]): void {
  for (const entry of entries) {
    entry.hidden = true;
    hide(entry.children);
  }
}

/** labelLines is a node's label as the graphical notation orders it: head, «kind», detail. */
export function labelLines(node: RenderNode): string[] {
  if (symbolSize(shapeOf(node.kind))) {
    return [];
  }
  const lines = [labelHead(node)];
  if (node.name !== "") {
    lines.push(`«${node.kind}»`);
  }
  if (node.detail !== "") {
    lines.push(node.detail);
  }
  return lines;
}

/** glyphSize uses the same average-glyph metrics as a node label, without box padding. */
export function glyphSize(text: string): { width: number; height: number } {
  return { width: Math.ceil([...text].length * GLYPH_WIDTH), height: LINE_HEIGHT };
}

/** portLabelPlacement puts a port name outside its box and clear of its edge path. */
export function portLabelPlacement(box: Box, port: PlacedPort): PortLabelPlacement {
  const center = portCenter(box, port);
  const square = portBox(box, port);
  const size = glyphSize(port.port.name);
  let x: number;
  let y: number;
  let anchor: "start" | "end" = "start";
  switch (port.side) {
    case "north":
      x = square.x + PORT_SIZE + 2;
      y = box.y - 2;
      break;
    case "east":
      x = square.x + PORT_SIZE + 2;
      y = center.y - PORT_SIZE / 2 - 1;
      break;
    case "south":
      x = square.x + PORT_SIZE + 2;
      y = box.y + box.height + PORT_SIZE / 2 + FONT_SIZE;
      break;
    case "west":
      x = square.x - 2;
      y = center.y - PORT_SIZE / 2 - 1;
      anchor = "end";
      break;
  }
  return {
    x,
    y,
    anchor,
    bounds: { x: anchor === "end" ? x - size.width : x, y: y - size.height, ...size },
  };
}

/** nodeExtent covers a node, its ports and labels, plus any outward port-pin leg requested. */
export function nodeExtent(
  node: PlacedNode,
  portExitLeg: number | ((port: PlacedPort) => number) = 0,
): Box {
  let left = node.box.x;
  let top = node.box.y;
  let right = node.box.x + node.box.width;
  let bottom = node.box.y + node.box.height;
  const reach = (x: number, y: number): void => {
    left = Math.min(left, x);
    top = Math.min(top, y);
    right = Math.max(right, x);
    bottom = Math.max(bottom, y);
  };
  for (const port of node.ports) {
    const square = portBox(node.box, port);
    const label = portLabelPlacement(node.box, port).bounds;
    reach(square.x, square.y);
    reach(square.x + square.width, square.y + square.height);
    reach(label.x, label.y);
    reach(label.x + label.width, label.y + label.height);
    const exitLeg = typeof portExitLeg === "function" ? portExitLeg(port) : portExitLeg;
    if (exitLeg > 0) {
      const face = portFace(node.box, port);
      switch (port.side) {
        case "north":
          reach(face.x, face.y - exitLeg);
          break;
        case "east":
          reach(face.x + exitLeg, face.y);
          break;
        case "south":
          reach(face.x, face.y + exitLeg);
          break;
        case "west":
          reach(face.x - exitLeg, face.y);
          break;
      }
    }
  }
  return { x: left, y: top, width: right - left, height: bottom - top };
}

/** clampNodeToBounds keeps a node's full visible and routing extent inside a box. */
export function clampNodeToBounds(
  node: PlacedNode,
  at: RenderPoint,
  bounds: Box,
  portExitLeg: number | ((port: PlacedPort) => number) = 0,
): RenderPoint {
  const extent = nodeExtent(node, portExitLeg);
  const left = extent.x - node.box.x;
  const top = extent.y - node.box.y;
  const right = extent.x + extent.width - node.box.x;
  const bottom = extent.y + extent.height - node.box.y;
  const minX = bounds.x - left;
  const minY = bounds.y - top;
  return {
    x: Math.min(Math.max(at.x, minX), Math.max(minX, bounds.x + bounds.width - right)),
    y: Math.min(Math.max(at.y, minY), Math.max(minY, bounds.y + bounds.height - bottom)),
  };
}

/** Finds the nearest free, in-bounds position for a node. */
export function freePlacement(
  node: PlacedNode,
  at: RenderPoint,
  others: PlacedNode[],
  bounds: Box,
  portExitLeg: number | ((node: PlacedNode, port: PlacedPort) => number),
  direction?: { x: number; y: number },
): RenderPoint | undefined {
  const exitLeg = (entry: PlacedNode, port: PlacedPort): number =>
    typeof portExitLeg === "number" ? portExitLeg : portExitLeg(entry, port);
  const clamp = (point: RenderPoint): RenderPoint =>
    clampNodeToBounds(node, point, bounds, (port) => exitLeg(node, port));
  const extentAt = (entry: PlacedNode, point: RenderPoint): Box =>
    nodeExtent({ ...entry, box: { ...entry.box, x: point.x, y: point.y } }, (port) => exitLeg(entry, port));
  const expandedAt = (point: RenderPoint): Box => inflate(extentAt(node, point), CLEARANCE);
  const otherExtents = others
    .filter((other) => other.node.id !== node.node.id && !other.hidden)
    .map((other) => inflate(extentAt(other, other.box), CLEARANCE));
  const isFree = (point: RenderPoint): boolean => {
    const extent = expandedAt(point);
    return otherExtents.every((other) => !intersectsBoxes(extent, other));
  };
  const clampedAt = clamp(at);
  if (isFree(clampedAt)) {
    return clampedAt;
  }

  const movingAtOrigin = expandedAt({ x: node.box.x, y: node.box.y });
  const leftOffset = movingAtOrigin.x - node.box.x;
  const rightOffset = movingAtOrigin.x + movingAtOrigin.width - node.box.x;
  const topOffset = movingAtOrigin.y - node.box.y;
  const bottomOffset = movingAtOrigin.y + movingAtOrigin.height - node.box.y;
  const baseExtent = extentAt(node, node.box);
  const minX = bounds.x - (baseExtent.x - node.box.x);
  const minY = bounds.y - (baseExtent.y - node.box.y);
  const maxX = Math.max(minX, bounds.x + bounds.width - (baseExtent.x + baseExtent.width - node.box.x));
  const maxY = Math.max(minY, bounds.y + bounds.height - (baseExtent.y + baseExtent.height - node.box.y));
  const xs = new Set([at.x, minX, maxX]);
  const ys = new Set([at.y, minY, maxY]);
  for (const extent of otherExtents) {
    xs.add(extent.x - rightOffset);
    xs.add(extent.x + extent.width - leftOffset);
    ys.add(extent.y - bottomOffset);
    ys.add(extent.y + extent.height - topOffset);
  }

  const candidates = new Map<string, RenderPoint>();
  for (const x of xs) {
    if (x < minX || x > maxX) {
      continue;
    }
    for (const y of ys) {
      if (y >= minY && y <= maxY) {
        candidates.set(`${x},${y}`, { x, y });
      }
    }
  }

  const ordered = [...candidates.values()].sort(
    (first, second) => Math.hypot(first.x - at.x, first.y - at.y) - Math.hypot(second.x - at.x, second.y - at.y),
  );
  for (const candidate of ordered) {
    if (direction && !movesInDirection(candidate, at, direction)) {
      continue;
    }
    if (isFree(candidate)) {
      return candidate;
    }
  }
  return direction ? undefined : clampedAt;
}

function movesInDirection(candidate: RenderPoint, at: RenderPoint, direction: RenderPoint): boolean {
  if (direction.x !== 0 && direction.y === 0) {
    return (candidate.x - at.x) * direction.x > 0 && candidate.y === at.y;
  }
  if (direction.y !== 0 && direction.x === 0) {
    return (candidate.y - at.y) * direction.y > 0 && candidate.x === at.x;
  }
  return false;
}

function inflate(box: Box, distance: number): Box {
  return {
    x: box.x - distance,
    y: box.y - distance,
    width: box.width + 2 * distance,
    height: box.height + 2 * distance,
  };
}

function intersectsBoxes(first: Box, second: Box): boolean {
  return (
    first.x < second.x + second.width &&
    first.x + first.width > second.x &&
    first.y < second.y + second.height &&
    first.y + first.height > second.y
  );
}

function placePorts(edges: RenderEdge[], placed: Map<string, PlacedNode>, auto?: AutoLayout): void {
  for (const entry of placed.values()) {
    if (entry.hidden) {
      continue;
    }
    const defaults = new Map<Side, PlacedPort[]>();
    entry.ports = (entry.node.ports ?? []).map((port) => {
      const laid = auto?.ports.get(port.id);
      if (laid) {
        return { port, side: laid.side, offset: laid.offset };
      }
      const side = defaultPortSide(entry, port, edges, placed);
      const positioned = { port, side, offset: 0 };
      const group = defaults.get(side) ?? [];
      group.push(positioned);
      defaults.set(side, group);
      return positioned;
    });
    for (const group of defaults.values()) {
      group.forEach((port, index) => {
        port.offset = (index + 1) / (group.length + 1);
      });
    }
  }
}

function defaultPortSide(entry: PlacedNode, port: RenderPort, edges: RenderEdge[], placed: Map<string, PlacedNode>): Side {
  const edge = edges.find((candidate) =>
    (candidate.from === entry.node.id && candidate.fromPort === port.id) ||
    (candidate.to === entry.node.id && candidate.toPort === port.id),
  );
  if (!edge) {
    return "south";
  }
  const other = placed.get(edge.from === entry.node.id ? edge.to : edge.from);
  if (!other || other.hidden) {
    return "south";
  }
  const here = center(entry.box);
  const there = center(other.box);
  const dx = there.x - here.x;
  const dy = there.y - here.y;
  if (Math.abs(dx) >= Math.abs(dy)) {
    return dx >= 0 ? "east" : "west";
  }
  return dy >= 0 ? "south" : "north";
}

function includePortExtent(layout: CanvasLayout): CanvasLayout {
  let left = layout.origin.x;
  let top = layout.origin.y;
  let right = layout.origin.x + layout.width;
  let bottom = layout.origin.y + layout.height;
  let hasPorts = false;
  for (const entry of layout.nodes.values()) {
    if (entry.hidden || entry.ports.length === 0) {
      continue;
    }
    hasPorts = true;
    const extent = nodeExtent(entry);
    left = Math.min(left, extent.x);
    top = Math.min(top, extent.y);
    right = Math.max(right, extent.x + extent.width);
    bottom = Math.max(bottom, extent.y + extent.height);
  }
  if (!hasPorts) {
    return layout;
  }
  const origin = {
    x: left < layout.origin.x ? left - MARGIN : layout.origin.x,
    y: top < layout.origin.y ? top - MARGIN : layout.origin.y,
  };
  return {
    ...layout,
    origin,
    width: Math.max(right + MARGIN, layout.origin.x + layout.width) - origin.x,
    height: Math.max(bottom + MARGIN, layout.origin.y + layout.height) - origin.y,
  };
}

// labelHead is a label's first line: the kind of an unnamed node, else the name
// with its type when it has one.
function labelHead(node: RenderNode): string {
  if (node.name === "") {
    return node.kind;
  }
  return node.type === "" ? node.name : `${node.name} : ${node.type}`;
}

// labelSize is the box a label needs, its head in bold glyphs.
export function labelSize(lines: string[]): { width: number; height: number } {
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
export function symbolSize(shape: Shape): { width: number; height: number } | undefined {
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
// The auto layout's route applies only where neither end is placed, since an end
// the model or a gesture moved is where the route was computed around it.
function routeEdge(
  edge: RenderEdge,
  index: number,
  placed: Map<string, PlacedNode>,
  routes: Map<number, RenderPoint[] | undefined> | undefined,
  auto?: AutoLayout,
): PlacedEdge {
  const stated = routes?.has(index) ? routes.get(index) : edge.route;
  const source = placed.get(edge.from);
  const target = placed.get(edge.to);
  const routed = autoRoute(edge, index, routes, auto, source, target);
  if (routed !== undefined) {
    // ELK's anchors already lie on the boxes' borders, so its polyline is drawn
    // verbatim and a drag edits only the inner waypoints.
    return {
      edge,
      index,
      points: routed,
      route: routed.slice(1, -1),
      label: midpoint(routed),
      hidden: source?.hidden === true || target?.hidden === true,
      rerouted: false,
    };
  }
  const route = stated ?? [];
  const from = source?.box ?? { x: 0, y: 0, width: 0, height: 0 };
  const to = target?.box ?? { x: 0, y: 0, width: 0, height: 0 };
  let inner = route;
  if (route.length === 0 && edge.from === edge.to) {
    inner = [
      { x: from.x + from.width + LOOP_REACH, y: from.y + from.height / 3 },
      { x: from.x + from.width + LOOP_REACH, y: from.y + (2 * from.height) / 3 },
    ];
  }
  const start = anchor(from, inner[0] ?? center(to));
  const end = anchor(to, inner.at(-1) ?? center(from));
  const fromPort = source?.ports.find((port) => port.port.id === edge.fromPort);
  const toPort = target?.ports.find((port) => port.port.id === edge.toPort);
  const startPoint = fromPort ? portFace(from, fromPort) : start;
  const endPoint = toPort ? portFace(to, toPort) : end;
  const points = [startPoint, ...inner, endPoint];
  return { edge, index, points, route, label: midpoint(points), hidden: source?.hidden === true || target?.hidden === true, rerouted: false };
}

function sameBox(a: Box | undefined, b: Box): boolean {
  return a !== undefined && a.x === b.x && a.y === b.y && a.width === b.width && a.height === b.height;
}

// autoRoute is the auto layout's route for an edge, usable only where neither end
// is placed: an end the model or a gesture moved is where the route was computed.
function autoRoute(
  edge: RenderEdge,
  index: number,
  routes: Map<number, RenderPoint[] | undefined> | undefined,
  auto: AutoLayout | undefined,
  source: PlacedNode | undefined,
  target: PlacedNode | undefined,
): RenderPoint[] | undefined {
  const stated = routes?.has(index) ? routes.get(index) : edge.route;
  const routed =
    stated === undefined && source?.pinned === false && target?.pinned === false ? auto?.routes.get(index) : undefined;
  return routed !== undefined && routed.length >= 2 ? routed : undefined;
}

// avoidable is an edge the panel routes around the boxes: nothing states its route,
// the auto layout has none for it, and both ends are drawn, distinct and not hidden.
function avoidable(
  edge: RenderEdge,
  index: number,
  placed: Map<string, PlacedNode>,
  routes: Map<number, RenderPoint[] | undefined> | undefined,
  auto: AutoLayout | undefined,
): boolean {
  const source = placed.get(edge.from);
  const target = placed.get(edge.to);
  return (
    !routes?.has(index) &&
    (edge.route?.length ?? 0) === 0 &&
    edge.from !== edge.to &&
    source !== undefined &&
    target !== undefined &&
    !source.hidden &&
    !target.hidden &&
    autoRoute(edge, index, routes, auto, source, target) === undefined
  );
}

// rerouteAroundBoxes replaces the drawn-straight avoidable edges with routes around
// the boxes: the held layout's kept where its ends have not moved during a gesture,
// else libavoid passes grouped by the containers each edge's ends lie inside. The
// route is the panel's, never the model's.
function rerouteAroundBoxes(
  edges: PlacedEdge[],
  placed: Map<string, PlacedNode>,
  roots: PlacedNode[],
  overrides: Overrides,
  auto: AutoLayout | undefined,
): void {
  const indices = edges
    .filter((edge) => avoidable(edge.edge, edge.index, placed, overrides.routes, auto))
    .map((edge) => edge.index);
  const held = overrides.held;
  if (held) {
    for (const index of indices) {
      const edge = edges[index];
      const kept = held.edges[index];
      const source = placed.get(edge.edge.from);
      const target = placed.get(edge.edge.to);
      if (
        kept?.rerouted === true &&
        source !== undefined &&
        target !== undefined &&
        sameBox(held.nodes.get(kept.edge.from)?.box, source.box) &&
        sameBox(held.nodes.get(kept.edge.to)?.box, target.box)
      ) {
        edges[index] = { ...edge, points: kept.points, route: kept.route, label: kept.label, rerouted: true };
      }
    }
    return;
  }
  if (indices.length === 0) {
    return;
  }
  // Every drawn leaf and populated container is an obstacle — except, per edge,
  // the containers its ends lie inside, which it must cross to leave. Edges
  // sharing their exempt containers route in one pass; passes see different
  // obstacles, so two edges in different passes are not nudged apart.
  const leaves = new Map<string, Box>();
  const containers = new Map<string, Box>();
  const visit = (entry: PlacedNode): void => {
    if (entry.hidden) {
      return;
    }
    const shown = entry.children.filter((child) => !child.hidden);
    (shown.length === 0 ? leaves : containers).set(entry.node.id, entry.box);
    for (const child of shown) {
      visit(child);
    }
  };
  for (const root of roots) {
    visit(root);
  }
  const groups = new Map<string, number[]>();
  for (const index of indices) {
    const edge = edges[index].edge;
    const exempt = new Set<string>();
    for (const end of [edge.from, edge.to]) {
      for (let entry = placed.get(end)?.parent; entry; entry = entry.parent) {
        exempt.add(entry.node.id);
      }
    }
    const key = [...exempt].sort().join(" ");
    const group = groups.get(key) ?? [];
    group.push(index);
    groups.set(key, group);
  }
  for (const [key, group] of groups) {
    const exempt = new Set(key === "" ? [] : key.split(" "));
    // A node a blocked container fully contains needs no shape of its own — the
    // container covers it, and a shape inside another is no obstacle at all; one
    // sticking out of its container keeps its shape, since the container does not.
    const covered = (entry: PlacedNode): boolean => {
      for (let parent = entry.parent; parent; parent = parent.parent) {
        const box = parent.box;
        if (
          containers.has(parent.node.id) &&
          !exempt.has(parent.node.id) &&
          entry.box.x >= box.x &&
          entry.box.y >= box.y &&
          entry.box.x + entry.box.width <= box.x + box.width &&
          entry.box.y + entry.box.height <= box.y + box.height
        ) {
          return true;
        }
      }
      return false;
    };
    const shapes = new Map<string, AvoidShape>();
    for (const [id] of [...leaves, ...containers]) {
      if (!covered(placed.get(id)!) && (!containers.has(id) || !exempt.has(id))) {
        shapes.set(id, avoidShape(placed.get(id)!));
      }
    }
    for (const index of group) {
      const edge = edges[index].edge;
      shapes.set(edge.from, avoidShape(placed.get(edge.from)!));
      shapes.set(edge.to, avoidShape(placed.get(edge.to)!));
    }
    const routes = avoidRoutes(
      shapes,
      group.map((index) => ({
        index,
        from: edges[index].edge.from,
        to: edges[index].edge.to,
        fromPort: edges[index].edge.fromPort,
        toPort: edges[index].edge.toPort,
      })),
      overrides.bounds,
    );
    if (!routes) {
      continue;
    }
    for (const index of group) {
      const points = routes.get(index);
      if (!points) {
        continue;
      }
      const edge = edges[index];
      edges[index] = { ...edge, points, route: points.slice(1, -1), label: midpoint(points), rerouted: true };
    }
  }
}

function avoidShape(entry: PlacedNode): AvoidShape {
  return {
    box: entry.box,
    ports: entry.ports.map(({ port, side, offset }) => ({ id: port.id, side, offset })),
  };
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

/** movable reports whether a node can be dragged: a workspace document declares it, so a Layout can reach it. */
export function movable(layout: CanvasLayout, entry: PlacedNode): boolean {
  return layout.placeable && reachable(entry.node);
}

/** steerable reports whether an edge's route can be edited: a workspace document declares the connection. */
export function steerable(layout: CanvasLayout, edge: PlacedEdge): boolean {
  return layout.placeable && reachable(edge.edge);
}

// nodeUnder is the node drawn on top at a point: the innermost, latest-drawn box holding it,
// passing over hidden nodes and the subtree of `except`, which a drag holds over the others.
export function nodeUnder(layout: CanvasLayout, at: RenderPoint, except?: string): PlacedNode | undefined {
  let found: PlacedNode | undefined;
  const visit = (entry: PlacedNode): void => {
    if (entry.hidden || entry.node.id === except) {
      return;
    }
    if (contains(entry.box, at)) {
      found = entry;
    }
    for (const child of entry.children) {
      visit(child);
    }
  };
  for (const root of layout.roots) {
    visit(root);
  }
  return found;
}

function contains(box: Box, at: RenderPoint): boolean {
  return at.x >= box.x && at.x <= box.x + box.width && at.y >= box.y && at.y <= box.y + box.height;
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
    if (edge.route.length > 0 && !edge.rerouted && steerable(layout, edge) && subtree.has(edge.edge.from) && subtree.has(edge.edge.to)) {
      edges.push({ index: edge.index, route: edge.route.map((p) => ({ x: snap(p.x + dx), y: snap(p.y + dy) })) });
    }
  }
  return { nodes, edges };
}

// liftedEdges routes the edges at the subtree under `id` lifted by (dx, dy): one within it moves
// whole, waypoints included, as movedNode writes it; one crossing its border keeps its waypoints.
export function liftedEdges(layout: CanvasLayout, id: string, dx: number, dy: number): PlacedEdge[] {
  const entry = layout.nodes.get(id);
  if (!entry) {
    return [];
  }
  const placed = new Map(layout.nodes);
  const subtree = new Set<string>();
  const visit = (current: PlacedNode): void => {
    subtree.add(current.node.id);
    placed.set(current.node.id, { ...current, box: { ...current.box, x: current.box.x + dx, y: current.box.y + dy } });
    for (const child of current.children) {
      visit(child);
    }
  };
  visit(entry);
  const out: PlacedEdge[] = [];
  for (const edge of layout.edges) {
    const from = subtree.has(edge.edge.from);
    const to = subtree.has(edge.edge.to);
    if (edge.hidden || (!from && !to)) {
      continue;
    }
    const route = from && to ? edge.route.map((p) => ({ x: p.x + dx, y: p.y + dy })) : edge.rerouted ? [] : edge.route;
    out.push(routeEdge(edge.edge, edge.index, placed, new Map([[edge.index, route]])));
  }
  return out;
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
