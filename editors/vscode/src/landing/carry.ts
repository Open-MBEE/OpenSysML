import type { LayoutGeometry, RenderNode, RenderPoint } from "../protocol";
import type { AutoLayout } from "../webview/autolayout";
import type { Box, PlacedNode } from "../webview/layout";

// obstacles is what a moving top-level box must stay clear of: the other shown top-level boxes,
// and any box of theirs that sticks out past its project. The moving box carries its own.
export function obstacles(entries: Iterable<PlacedNode>, moving: string): PlacedNode[] {
  const all = [...entries];
  const byId = new Map(all.map((entry) => [entry.node.id, entry]));
  const projectOf = (entry: PlacedNode): PlacedNode => {
    const parent = entry.node.parent === undefined ? undefined : byId.get(entry.node.parent);
    return parent === undefined ? entry : projectOf(parent);
  };
  const within = (inner: Box, outer: Box): boolean =>
    inner.x >= outer.x &&
    inner.y >= outer.y &&
    inner.x + inner.width <= outer.x + outer.width &&
    inner.y + inner.height <= outer.y + outer.height;
  return all.filter((entry) => {
    if (entry.hidden) {
      return false;
    }
    const project = projectOf(entry);
    if (project.node.id === moving) {
      return false;
    }
    return project === entry || project.hidden || !within(entry.box, project.box);
  });
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

// resettle puts a project at `at` in `settled` and carries what is drawn inside it, so a later
// clearance check sees the project's boxes where the rebuilt layout will draw them.
export function resettle(
  settled: Map<string, PlacedNode>,
  nodes: RenderNode[],
  auto: AutoLayout | undefined,
  id: string,
  at: RenderPoint,
): void {
  const entry = settled.get(id);
  if (!entry) {
    return;
  }
  for (const [nodeId, geometry] of carried(nodes, auto, new Map([[id, { ...entry.box, ...at }]]))) {
    const placed = settled.get(nodeId);
    if (placed) {
      settled.set(nodeId, { ...placed, box: { ...placed.box, x: geometry.x, y: geometry.y } });
    }
  }
}
