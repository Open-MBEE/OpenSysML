import type { LayoutGeometry, RenderNode, RenderPoint } from "../protocol";
import type { AutoLayout } from "../webview/autolayout";
import type { PlacedNode } from "../webview/layout";

// obstacles is what a moving top-level box must stay clear of: the other shown top-level
// boxes. A nested box is drawn inside its project's, so moving a project carries it along.
export function obstacles(entries: Iterable<PlacedNode>, moving: string): PlacedNode[] {
  return [...entries].filter((entry) => entry.node.id !== moving && !entry.hidden && entry.node.parent === undefined);
}

// carried is `moved` plus every descendant of a moved node, shifted as its nearest moved
// ancestor was: a box dragged across the diagram carries what is drawn inside it.
export function carried(
  nodes: RenderNode[],
  auto: AutoLayout | undefined,
  moved: Map<string, LayoutGeometry>,
): Map<string, LayoutGeometry> {
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const shiftOf = (id: string): RenderPoint | undefined => {
    const node = byId.get(id);
    const parent = node?.parent === undefined || node.parent === id ? undefined : byId.get(node.parent);
    if (!parent) {
      return undefined;
    }
    const at = moved.get(parent.id);
    const laid = auto?.nodes.get(parent.id);
    if (at && laid) {
      return { x: at.x - laid.x, y: at.y - laid.y };
    }
    return shiftOf(parent.id);
  };
  const out = new Map(moved);
  for (const node of nodes) {
    const laid = auto?.nodes.get(node.id);
    if (laid === undefined || moved.has(node.id)) {
      continue;
    }
    const shift = shiftOf(node.id);
    if (shift) {
      out.set(node.id, { ...laid, x: laid.x + shift.x, y: laid.y + shift.y });
    }
  }
  return out;
}
