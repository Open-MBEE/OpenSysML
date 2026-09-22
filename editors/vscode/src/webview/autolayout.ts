// Places the nodes a model does not place, and routes the edges between them,
// with the ELK layered algorithm: boxes land in layers, edges run orthogonally
// around them, and containers grow to hold their children.
import ELK from "elkjs/lib/elk.bundled.js";
import type { ElkExtendedEdge, ElkNode } from "elkjs/lib/elk-api";

import type { LayoutGeometry, RenderNode, RenderPoint, RenderResult } from "../protocol";
import {
  CONTAINER_PAD,
  GAP,
  labelLines,
  labelSize,
  MARGIN,
  PLACEABLE_KINDS,
  shapeOf,
  snap,
  symbolSize,
} from "./layout";

export interface AutoLayout {
  /** Absolute canvas geometry (x, y, width, height) for every node ELK placed. */
  nodes: Map<string, LayoutGeometry>;
  /** By edge index: ELK's orthogonal polyline in absolute canvas coordinates, source anchor first, target anchor last. */
  routes: Map<number, RenderPoint[]>;
}

/** Above this many nodes the grid stays: laying out a migrated model must not hang the panel. */
export const AUTO_LAYOUT_LIMIT = 600;

const elk = new ELK();

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
  const elkNode = (node: RenderNode): ElkNode => {
    const size = symbolSize(shapeOf(node.kind)) ?? labelSize(labelLines(node));
    const kids = node.collapsed ? [] : (children.get(node.id) ?? []);
    const out: ElkNode = { id: node.id, width: size.width, height: size.height };
    if (kids.length > 0) {
      out.children = kids.map(elkNode);
      // A container's label sits above its children, so the top padding is its height.
      out.layoutOptions = {
        ...options,
        "elk.padding": `[top=${size.height},left=${CONTAINER_PAD},bottom=${CONTAINER_PAD},right=${CONTAINER_PAD}]`,
        "elk.nodeSize.constraints": "MINIMUM_SIZE",
        "elk.nodeSize.minimum": `(${size.width},${size.height})`,
      };
    }
    return out;
  };
  const edges: ElkExtendedEdge[] = [];
  (result.edges ?? []).forEach((edge, index) => {
    if (edge.from === edge.to || !inGraph(edge.from) || !inGraph(edge.to)) {
      return;
    }
    edges.push({ id: `e${index}`, sources: [edge.from], targets: [edge.to] });
  });
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
    children: (children.get(undefined) ?? []).map(elkNode),
    edges,
  });
  const placed = new Map<string, LayoutGeometry>();
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
      walk(child, x, y);
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
  return { nodes: placed, routes };
}

// spacingOptions is shared by the root and every compound node so nested
// layers have the same direction, edge routing, and label-friendly gaps.
function spacingOptions(kind: string): Record<string, string> {
  return {
    "elk.edgeRouting": "ORTHOGONAL",
    "elk.direction": kind === "tree" || kind === "action" ? "DOWN" : "RIGHT",
    "elk.spacing.nodeNode": `${GAP}`,
    "elk.layered.spacing.nodeNodeBetweenLayers": `${GAP * 2}`,
    "elk.spacing.edgeNode": `${GAP / 2}`,
    "elk.layered.spacing.edgeNodeBetweenLayers": `${GAP / 2}`,
    "elk.spacing.componentComponent": `${GAP}`,
  };
}
