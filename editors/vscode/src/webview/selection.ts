// A selection emphasises one node and what it is related to: its incident edges
// and the nodes at their other ends. Everything else is dimmed by the classes
// below, which drawing marks onto the groups the canvas already carries.

import type { RenderEdge, RenderResult } from "../protocol";

/** The groups a selection emphasises: node ids and edge indices into the rendering's edges. */
export interface Neighbourhood {
  nodes: Set<string>;
  edges: Set<number>;
}

/** The class on the SVG while a selection is shown, under which the dimming rules apply. */
export const HAS_FOCUS = "has-focus";
/** The class on the selected node's group. */
export const FOCUS = "focus";
/** The class on the groups of the selected node's incident edges and neighbouring nodes. */
export const RELATED = "related";

/**
 * neighbourhood is the selected node, the edges at it that are not hidden by kind,
 * and the nodes at their other ends. A node with no edges is a neighbourhood of one.
 */
export function neighbourhood(result: RenderResult, id: string, hiddenKinds: ReadonlySet<string> = new Set()): Neighbourhood {
  const nodes = new Set<string>([id]);
  const edges = new Set<number>();
  (result.edges ?? []).forEach((edge, index) => {
    if (hiddenKinds.has(edge.kind)) {
      return;
    }
    if (edge.from === id) {
      edges.add(index);
      nodes.add(edge.to);
    } else if (edge.to === id) {
      edges.add(index);
      nodes.add(edge.from);
    }
  });
  return { nodes, edges };
}

/** relatedByKind counts a node's incident edges by kind, hidden kinds included, so a filter can say what it hides. */
export function relatedByKind(edges: readonly RenderEdge[], id: string): Map<string, number> {
  const counts = new Map<string, number>();
  for (const edge of edges) {
    if (edge.from === id || edge.to === id) {
      counts.set(edge.kind, (counts.get(edge.kind) ?? 0) + 1);
    }
  }
  return counts;
}

/**
 * markSelection puts the selection's classes on an SVG's groups, or takes them all
 * off when there is no selection. Edge handles follow their edge.
 */
export function markSelection(svg: SVGSVGElement, selection: Neighbourhood | undefined): void {
  for (const marked of svg.querySelectorAll(`.${FOCUS}, .${RELATED}`)) {
    marked.classList.remove(FOCUS, RELATED);
  }
  svg.classList.toggle(HAS_FOCUS, selection !== undefined);
  if (!selection) {
    return;
  }
  for (const group of svg.querySelectorAll<SVGGElement>("g.opensysml-node[data-opensysml-id]")) {
    const id = group.dataset.opensysmlId ?? "";
    if (selection.nodes.has(id)) {
      group.classList.add(RELATED);
    }
  }
  for (const group of svg.querySelectorAll<SVGGElement>("g.opensysml-edge[data-edge], g.edge-handles[data-edge]")) {
    if (selection.edges.has(Number(group.dataset.edge))) {
      group.classList.add(RELATED);
    }
  }
  for (const group of svg.querySelectorAll<SVGGElement>(`g.opensysml-node.${RELATED}`)) {
    if (selection.nodes.size === 1 || isFocus(group, selection)) {
      group.classList.add(FOCUS);
    }
  }
}

// isFocus is whether the group is the selected node: the one the neighbourhood was built around, which is its first entry.
function isFocus(group: SVGGElement, selection: Neighbourhood): boolean {
  return group.dataset.opensysmlId === selection.nodes.values().next().value;
}
