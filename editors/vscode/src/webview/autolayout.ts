// Places the nodes a model does not place, and routes the edges between them,
// with the ELK layered algorithm: boxes land in layers, edges run orthogonally
// around them, and containers grow to hold their children.
import ELK from "elkjs/lib/elk.bundled.js";
import type { ElkExtendedEdge, ElkNode } from "elkjs/lib/elk-api";

import type { LayoutGeometry, RenderNode, RenderPoint, RenderResult } from "../protocol";
import {
  CONTAINER_PAD,
  GAP,
  glyphSize,
  labelLines,
  labelSize,
  MARGIN,
  PLACEABLE_KINDS,
  PORT_SIZE,
  shapeOf,
  snap,
  symbolSize,
  type Side,
} from "./layout";

export interface AutoLayout {
  /** Absolute canvas geometry (x, y, width, height) for every node ELK placed. */
  nodes: Map<string, LayoutGeometry>;
  /** By edge index: ELK's orthogonal polyline in absolute canvas coordinates, source anchor first, target anchor last. */
  routes: Map<number, RenderPoint[]>;
  /** ELK's port positions as sides and proportional offsets on their nodes. */
  ports: Map<string, { side: Side; offset: number }>;
}

/** Above this many nodes the grid stays: laying out a migrated model must not hang the panel. */
export const AUTO_LAYOUT_LIMIT = 600;

const elk = new ELK();

// PACK_ASPECT_RATIO is the width to height a container packs its unwired children toward.
const PACK_ASPECT_RATIO = 2.5;
const ORDER_STEP = 100000;

/**
 * autoLayout lays the rendering out with ELK, or returns undefined when the kind
 * has no editable canvas, the rendering is too large, or ELK fails.
 */
export async function autoLayout(result: RenderResult): Promise<AutoLayout | undefined> {
  if (!PLACEABLE_KINDS.has(result.kind) || (result.nodes?.length ?? 0) === 0 || result.nodes!.length > AUTO_LAYOUT_LIMIT) {
    return undefined;
  }
  try {
    return await layOut(result);
  } catch (err) {
    console.warn(`auto layout failed: ${err instanceof Error ? err.message : String(err)}`);
    return undefined;
  }
}

// layOut hands ELK the nodes — nested as the model owns them — and the edges, and
// reads back absolute canvas geometry: node coordinates are relative to their
// parent in ELK's JSON, and edge section coordinates relative to the root.
async function layOut(result: RenderResult): Promise<AutoLayout> {
  const nodes = result.nodes ?? [];
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const children = new Map<string | undefined, RenderNode[]>();
  for (const node of nodes) {
    const parent = node.parent !== undefined && node.parent !== node.id && byId.has(node.parent) ? node.parent : undefined;
    const siblings = children.get(parent) ?? [];
    siblings.push(node);
    children.set(parent, siblings);
  }
  // A node whose owner is collapsed is not in the graph, so no edge routes to it.
  const inGraph = (id: string): boolean => {
    let node = byId.get(id);
    while (node) {
      const parent = node.parent;
      if (parent === undefined || parent === node.id || !byId.has(parent)) {
        return true;
      }
      node = byId.get(parent);
      if (node?.collapsed) {
        return false;
      }
    }
    return false;
  };
  const options = spacingOptions(result.kind);
  const edges: ElkExtendedEdge[] = [];
  const portEndpoints: Array<{ nodeId: string; portId: string }> = [];
  (result.edges ?? []).forEach((edge, index) => {
    if (edge.from === edge.to || !inGraph(edge.from) || !inGraph(edge.to)) {
      return;
    }
    const from = byId.get(edge.from)!;
    const to = byId.get(edge.to)!;
    const fromPort = from.ports?.some((port) => port.id === edge.fromPort) ? edge.fromPort : undefined;
    const toPort = to.ports?.some((port) => port.id === edge.toPort) ? edge.toPort : undefined;
    edges.push({
      id: `e${index}`,
      sources: [fromPort ?? edge.from],
      targets: [toPort ?? edge.to],
    });
    if (fromPort !== undefined) {
      portEndpoints.push({ nodeId: from.id, portId: fromPort });
    }
    if (toPort !== undefined) {
      portEndpoints.push({ nodeId: to.id, portId: toPort });
    }
  });
  const connectedPorts = new Map<string, Set<string>>();
  for (const { nodeId, portId } of portEndpoints) {
    const ports = connectedPorts.get(nodeId) ?? new Set<string>();
    ports.add(portId);
    connectedPorts.set(nodeId, ports);
  }
  const wired = new Set(edges.flatMap((edge) => [...edge.sources, ...edge.targets]));
  for (const { nodeId } of portEndpoints) {
    wired.add(nodeId);
  }
  // unwired reports whether no edge reaches a node or anything inside it.
  const unwired = (node: RenderNode): boolean =>
    !wired.has(node.id) && (children.get(node.id) ?? []).every(unwired);
  // packed is each container of unwired children laid out alone by rectpacking: layered
  // stacks them in one column, and cannot size such a container while its ports have no side.
  const packed = new Map<string, ElkNode>();
  const elkNode = (node: RenderNode, packing = false): ElkNode => {
    const size = symbolSize(shapeOf(node.kind)) ?? labelSize(labelLines(node));
    const kids = node.collapsed ? [] : (children.get(node.id) ?? []);
    const alone = packed.get(node.id);
    const order = (children.get(node.parent) ?? []).indexOf(node);
    const out: ElkNode = alone
      ? { id: node.id, width: alone.width, height: alone.height }
      : { id: node.id, width: size.width, height: size.height };
    // Interactive crossing minimization sorts siblings by the centers of their input
    // positions, so the seeds stand far enough apart that no node's size can reorder them.
    out.x = out.y = order * ORDER_STEP;
    const ports = (node.ports ?? []).filter((port) => connectedPorts.get(node.id)?.has(port.id));
    if (ports.length > 0) {
      out.ports = ports.map((port) => {
        const label = glyphSize(port.name);
        return {
          id: port.id,
          width: PORT_SIZE,
          height: PORT_SIZE,
          layoutOptions: { "elk.port.borderOffset": String(-PORT_SIZE / 2) },
          labels: [{ text: port.name, width: label.width, height: label.height }],
        };
      });
    }
    if (kids.length > 0 && !alone) {
      out.children = kids.map((kid) => elkNode(kid, packing));
      // A container's label sits above its children, so the top padding is its height.
      out.layoutOptions = {
        ...options,
        ...(packing ? { "elk.algorithm": "rectpacking", "elk.aspectRatio": `${PACK_ASPECT_RATIO}` } : {}),
        "elk.padding": `[top=${size.height},left=${CONTAINER_PAD},bottom=${CONTAINER_PAD},right=${CONTAINER_PAD}]`,
        "elk.nodeSize.constraints": "MINIMUM_SIZE",
        "elk.nodeSize.minimum": `(${size.width},${size.height})`,
      };
    }
    if (ports.length > 0) {
      out.layoutOptions = {
        ...out.layoutOptions,
        "elk.portConstraints": "FREE",
        "elk.portLabels.placement": "OUTSIDE",
      };
    }
    return out;
  };
  const packable = (node: RenderNode): boolean => {
    const kids = node.collapsed ? [] : (children.get(node.id) ?? []);
    return kids.length > 0 && kids.every(unwired);
  };
  for (const node of nodes) {
    const parent = node.parent === undefined ? undefined : byId.get(node.parent);
    if (packable(node) && !(parent && packable(parent))) {
      // The container's own ports stay out of this pass; the layered pass places them.
      const { ports: _ports, ...graph } = elkNode(node, true);
      packed.set(node.id, await elk.layout(graph));
    }
  }
  const laid = await elk.layout({
    id: "__root__",
    layoutOptions: {
      ...options,
      "elk.algorithm": "layered",
      "elk.hierarchyHandling": "INCLUDE_CHILDREN",
      // Edge section coordinates come back relative to the root, like a node's
      // geometry relative to its parent, so they read as canvas coordinates.
      "org.eclipse.elk.json.edgeCoords": "ROOT",
    },
    children: (children.get(undefined) ?? []).map((node) => elkNode(node)),
    edges,
  });
  const placed = new Map<string, LayoutGeometry>();
  const ports = new Map<string, { side: Side; offset: number }>();
  const walk = (node: ElkNode, ox: number, oy: number): void => {
    for (const child of node.children ?? []) {
      const x = ox + (child.x ?? 0);
      const y = oy + (child.y ?? 0);
      const geometry: LayoutGeometry = {
        x: snap(x + MARGIN),
        y: snap(y + MARGIN),
        width: snap(child.width ?? 0),
        height: snap(child.height ?? 0),
      };
      if (byId.get(child.id)?.collapsed) {
        geometry.collapsed = true;
      }
      placed.set(child.id, geometry);
      for (const port of child.ports ?? []) {
        const width = child.width ?? 0;
        const height = child.height ?? 0;
        const x = (port.x ?? 0) + (port.width ?? PORT_SIZE) / 2;
        const y = (port.y ?? 0) + (port.height ?? PORT_SIZE) / 2;
        const sides: Array<{ side: Side; distance: number; along: number; extent: number }> = [
          { side: "north", distance: Math.abs(y), along: x, extent: width },
          { side: "east", distance: Math.abs(width - x), along: y, extent: height },
          { side: "south", distance: Math.abs(height - y), along: x, extent: width },
          { side: "west", distance: Math.abs(x), along: y, extent: height },
        ];
        const nearest = sides.reduce((best, candidate) => candidate.distance < best.distance ? candidate : best, sides[0]);
        const offset = nearest.extent > 0 ? Math.max(0, Math.min(1, nearest.along / nearest.extent)) : 0.5;
        ports.set(port.id, { side: nearest.side, offset });
      }
      walk(packed.get(child.id) ?? child, x, y);
    }
  };
  walk(laid, 0, 0);
  const routes = new Map<number, RenderPoint[]>();
  for (const edge of laid.edges ?? []) {
    const index = Number(edge.id.slice(1));
    if ((result.edges?.[index]?.route?.length ?? 0) > 0) {
      continue;
    }
    const points: RenderPoint[] = [];
    for (const section of edge.sections ?? []) {
      points.push(section.startPoint, ...(section.bendPoints ?? []), section.endPoint);
    }
    if (points.length >= 2) {
      routes.set(index, points.map((point) => ({ x: snap(point.x + MARGIN), y: snap(point.y + MARGIN) })));
    }
  }
  reconcile(result, children, placed, routes);
  return { nodes: placed, routes, ports };
}

// reconcile moves each placed node's subtree to the model's place (outermost first), keeps routes
// inside it and drops those crossing its border; then unplaced containers grow to cover their
// children, their own routes dropped.
function reconcile(
  result: RenderResult,
  children: Map<string | undefined, RenderNode[]>,
  placed: Map<string, LayoutGeometry>,
  routes: Map<number, RenderPoint[]>,
): void {
  const nodes = result.nodes ?? [];
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const depth = (node: RenderNode): number => {
    let d = 0;
    let at = node;
    while (at.parent !== undefined && at.parent !== at.id && byId.has(at.parent)) {
      d++;
      at = byId.get(at.parent)!;
    }
    return d;
  };
  const subtree = (id: string): Set<string> => {
    const inside = new Set([id]);
    const stack = [id];
    while (stack.length > 0) {
      for (const child of children.get(stack.pop()!) ?? []) {
        if (placed.has(child.id) && !inside.has(child.id)) {
          inside.add(child.id);
          stack.push(child.id);
        }
      }
    }
    return inside;
  };
  for (const node of nodes
    .filter((n) => n.x !== undefined && n.y !== undefined && placed.has(n.id))
    .sort((a, b) => depth(a) - depth(b))) {
    const geometry = placed.get(node.id)!;
    const dx = node.x! - geometry.x;
    const dy = node.y! - geometry.y;
    const inside = subtree(node.id);
    for (const id of inside) {
      if (id === node.id) {
        continue;
      }
      const moved = placed.get(id)!;
      moved.x += dx;
      moved.y += dy;
    }
    geometry.x = node.x!;
    geometry.y = node.y!;
    if (node.width !== undefined) {
      geometry.width = node.width;
    }
    if (node.height !== undefined) {
      geometry.height = node.height;
    }
    for (const [index, points] of routes) {
      const edge = result.edges![index];
      const from = inside.has(edge.from);
      const to = inside.has(edge.to);
      if (from && to) {
        routes.set(index, points.map((point) => ({ x: snap(point.x + dx), y: snap(point.y + dy) })));
      } else if (from || to) {
        routes.delete(index);
      }
    }
  }
  for (const node of nodes
    .filter((n) => placed.has(n.id) && (n.x === undefined || n.y === undefined) && !n.collapsed)
    .sort((a, b) => depth(b) - depth(a))) {
    const kids = (children.get(node.id) ?? []).filter((child) => placed.has(child.id));
    if (kids.length === 0) {
      continue;
    }
    const geometry = placed.get(node.id)!;
    const header = symbolSize(shapeOf(node.kind)) ?? labelSize(labelLines(node));
    let left = geometry.x;
    let top = geometry.y;
    let right = geometry.x + (geometry.width ?? 0);
    let bottom = geometry.y + (geometry.height ?? 0);
    for (const child of kids) {
      const box = placed.get(child.id)!;
      left = Math.min(left, box.x - CONTAINER_PAD);
      top = Math.min(top, box.y - header.height);
      right = Math.max(right, box.x + (box.width ?? 0) + CONTAINER_PAD);
      bottom = Math.max(bottom, box.y + (box.height ?? 0) + CONTAINER_PAD);
    }
    if (left === geometry.x && top === geometry.y && right - left === geometry.width && bottom - top === geometry.height) {
      continue;
    }
    geometry.x = left;
    geometry.y = top;
    geometry.width = right - left;
    geometry.height = bottom - top;
    // The box moved, so routes anchored on its old border are dropped; layoutCanvas re-routes them around the boxes.
    for (const [index] of routes) {
      const edge = result.edges![index];
      if (edge.from === node.id || edge.to === node.id) {
        routes.delete(index);
      }
    }
  }
}

// spacingOptions is shared by the root and every compound node so nested
// layers have the same direction, edge routing, and label-friendly gaps.
function spacingOptions(kind: string): Record<string, string> {
  return {
    "elk.edgeRouting": "ORTHOGONAL",
    "elk.direction": kind === "tree" || kind === "action" ? "DOWN" : "RIGHT",
    // Siblings in a layer keep the model's declaration order (seeded through the input positions)
    // rather than whatever crossing minimization settles on, so a diagram reads as the model is written.
    "elk.layered.crossingMinimization.strategy": "INTERACTIVE",
    "elk.spacing.nodeNode": `${GAP}`,
    "elk.layered.spacing.nodeNodeBetweenLayers": `${GAP * 2}`,
    "elk.spacing.edgeNode": `${GAP / 2}`,
    "elk.layered.spacing.edgeNodeBetweenLayers": `${GAP / 2}`,
    "elk.spacing.componentComponent": `${GAP}`,
  };
}
