// Draws a laid-out rendering as SVG. Every node is a group carrying its rendering
// id, every edge a group carrying its index, and the handles a route is edited by
// carry the edge and the waypoint or segment they stand on; the panel script
// reads those attributes off whatever the pointer lands on.
import type { RenderPoint } from "../protocol";
import { CanvasLayout, FONT_SIZE, liftedEdges, MARGIN, movable, PlacedEdge, PlacedNode, steerable } from "./layout";

const SVG = "http://www.w3.org/2000/svg";
const LINE_HEIGHT = 18;
const LABEL_PAD_X = 12;
const LABEL_PAD_Y = 8;
const CORNER = 6;
/** The radius of a waypoint handle, and of the smaller handle a segment grows one from. */
export const WAYPOINT_RADIUS = 5;
const SEGMENT_RADIUS = 3.5;

const CLASSIFIER_KINDS = new Set([
  "class", "classifier", "subclassifier", "datatype", "struct", "assoc",
  "assoc struct", "behavior", "function", "predicate", "metaclass", "type",
]);

/**
 * drawCanvas is the SVG of a layout: nodes in tree order under their owners, then
 * edges, then handles. An edge with an end under a collapsed owner is not drawn.
 */
export function drawCanvas(layout: CanvasLayout): SVGSVGElement {
  const svg = element("svg", {
    xmlns: SVG,
    width: String(layout.width),
    height: String(layout.height),
    viewBox: `${layout.origin.x} ${layout.origin.y} ${layout.width} ${layout.height}`,
    class: "opensysml-canvas",
    "font-size": String(FONT_SIZE),
  });
  svg.append(markers());
  const nodes = element("g", { class: "nodes" });
  for (const root of layout.roots) {
    drawNode(nodes, root, layout);
  }
  svg.append(nodes);
  const edges = element("g", { class: "edges" });
  const handles = element("g", { class: "handles" });
  for (const edge of layout.edges) {
    if (edge.hidden) {
      continue;
    }
    edges.append(drawEdge(edge));
    if (steerable(layout, edge)) {
      handles.append(drawHandles(edge));
    }
  }
  svg.append(edges, handles);
  return svg;
}

// liftNode floats a node, its descendants and the edges at them by (dx, dy) over a canvas that
// stays put, the groups moved marked `lifted`; the canvas grows right and down to keep them in view.
export function liftNode(svg: SVGSVGElement, layout: CanvasLayout, id: string, dx: number, dy: number): void {
  const entry = layout.nodes.get(id);
  if (!entry) {
    return;
  }
  let right = layout.origin.x + layout.width;
  let bottom = layout.origin.y + layout.height;
  const lift = (current: PlacedNode): void => {
    const group = svg.querySelector<SVGGElement>(`g.opensysml-node[data-opensysml-id="${cssEscape(current.node.id)}"]`);
    if (group) {
      group.setAttribute("transform", `translate(${dx} ${dy})`);
      group.classList.add("lifted");
      group.parentElement?.append(group);
      right = Math.max(right, current.box.x + current.box.width + dx + MARGIN);
      bottom = Math.max(bottom, current.box.y + current.box.height + dy + MARGIN);
    }
    for (const child of current.children) {
      lift(child);
    }
  };
  lift(entry);
  for (const edge of liftedEdges(layout, id, dx, dy)) {
    const drawn = drawEdge(edge);
    drawn.classList.add("lifted");
    svg.querySelector(`g.opensysml-edge[data-edge="${edge.index}"]`)?.replaceWith(drawn);
    if (steerable(layout, edge)) {
      const handles = drawHandles(edge);
      handles.classList.add("lifted");
      svg.querySelector(`g.edge-handles[data-edge="${edge.index}"]`)?.replaceWith(handles);
    }
    for (const point of edge.points) {
      right = Math.max(right, point.x + MARGIN);
      bottom = Math.max(bottom, point.y + MARGIN);
    }
  }
  const width = right - layout.origin.x;
  const height = bottom - layout.origin.y;
  svg.setAttribute("width", String(width));
  svg.setAttribute("height", String(height));
  svg.setAttribute("viewBox", `${layout.origin.x} ${layout.origin.y} ${width} ${height}`);
}

/** cssEscape quotes an id for an attribute selector, since CSS.escape is not in every webview host. */
export function cssEscape(value: string): string {
  return value.replace(/["\\]/g, String.raw`\$&`);
}

// markers are the arrowheads edges end in: a filled head for a transition or a
// succession, an open one for a flow. A connection ends in none.
function markers(): SVGDefsElement {
  const defs = element("defs", {});
  const filled = element("marker", {
    id: "arrow", viewBox: "0 0 10 10", refX: "9", refY: "5",
    markerWidth: "9", markerHeight: "9", orient: "auto-start-reverse",
  });
  filled.append(element("path", { d: "M 0 0 L 10 5 L 0 10 z", class: "arrow-fill" }));
  const open = element("marker", {
    id: "arrow-open", viewBox: "0 0 10 10", refX: "9", refY: "5",
    markerWidth: "9", markerHeight: "9", orient: "auto-start-reverse",
  });
  open.append(element("path", { d: "M 0 0 L 10 5 L 0 10", class: "arrow-line" }));
  defs.append(filled, open);
  return defs;
}

// drawNode appends a node's group to parent: its shape, its label, then its children
// so they draw over it. The group is marked movable when a Layout can name the node.
function drawNode(parent: SVGElement, entry: PlacedNode, layout: CanvasLayout): void {
  const group = element("g", {
    class: `opensysml-node${movable(layout, entry) ? " movable" : ""}`,
    "data-opensysml-id": entry.node.id,
    "data-kind": entry.node.kind,
  });
  const { box } = entry;
  if (entry.lifeline !== undefined) {
    group.append(element("line", {
      x1: String(box.x + box.width / 2), y1: String(box.y + box.height),
      x2: String(box.x + box.width / 2), y2: String(entry.lifeline), class: "lifeline",
    }));
  }
  group.append(shape(entry));
  if (entry.lines.length > 0) {
    const text = element("text", { x: String(box.x + LABEL_PAD_X), y: String(box.y + LABEL_PAD_Y), class: "label" });
    entry.lines.forEach((line, i) => {
      const span = element("tspan", {
        x: String(box.x + LABEL_PAD_X),
        dy: String(i === 0 ? LINE_HEIGHT * 0.8 : LINE_HEIGHT),
        class: labelLineClass(i, entry.node.name !== ""),
      });
      span.textContent = line;
      text.append(span);
    });
    group.append(text);
  }
  if (entry.collapsed && entry.children.length > 0) {
    const mark = element("text", { x: String(box.x + box.width - LABEL_PAD_X), y: String(box.y + LABEL_PAD_Y + LINE_HEIGHT * 0.8), class: "collapsed", "text-anchor": "end" });
    mark.textContent = "+";
    group.append(mark);
  }
  parent.append(group);
  if (!entry.collapsed) {
    for (const child of entry.children) {
      drawNode(parent, child, layout);
    }
  }
}

// labelLineClass styles the i-th label line: the head, then the «kind» of a
// named node, then detail.
function labelLineClass(i: number, named: boolean): string {
  if (i === 0) {
    return "head";
  }
  return i === 1 && named ? "keyword" : "detail";
}

// shape is the outline a node is drawn with: a box for an element, square-cornered
// for a definition; a symbol for a control node.
function shape(entry: PlacedNode): SVGElement {
  const { x, y, width, height } = entry.box;
  const cx = x + width / 2;
  const cy = y + height / 2;
  switch (entry.shape) {
    case "point":
      return element("circle", { cx: String(cx), cy: String(cy), r: String(width / 2), class: "shape filled" });
    case "circle":
      return element("circle", { cx: String(cx), cy: String(cy), r: String(width / 2), class: "shape filled" });
    case "ring": {
      const group = element("g", { class: "shape" });
      group.append(
        element("circle", { cx: String(cx), cy: String(cy), r: String(width / 2), class: "shape" }),
        element("circle", { cx: String(cx), cy: String(cy), r: String(width / 2 - 5), class: "shape filled" }),
      );
      return group;
    }
    case "diamond":
      return element("polygon", {
        points: `${cx},${y} ${x + width},${cy} ${cx},${y + height} ${x},${cy}`,
        class: "shape",
      });
    case "bar":
      return element("rect", { x: String(x), y: String(y), width: String(width), height: String(height), class: "shape filled" });
    case "history": {
      const group = element("g", { class: "shape" });
      const letter = element("text", { x: String(cx), y: String(cy + FONT_SIZE / 3), "text-anchor": "middle", class: "label head" });
      letter.textContent = entry.node.kind === "deep history" ? "H*" : "H";
      group.append(element("circle", { cx: String(cx), cy: String(cy), r: String(width / 2), class: "shape" }), letter);
      return group;
    }
    case "box": {
      const definition = entry.node.kind.endsWith(" def") || CLASSIFIER_KINDS.has(entry.node.kind);
      const attrs: Record<string, string> = {
        x: String(x), y: String(y), width: String(width), height: String(height),
        class: `shape${entry.children.length > 0 && !entry.collapsed ? " container" : ""}`,
      };
      if (!definition) {
        attrs.rx = String(CORNER);
      }
      return element("rect", attrs);
    }
  }
}

// drawEdge is an edge's polyline with the arrowhead its kind takes and its label
// at the midpoint.
function drawEdge(edge: PlacedEdge): SVGGElement {
  const group = element("g", { class: `opensysml-edge ${edge.edge.kind}`, "data-edge": String(edge.index) });
  const attrs: Record<string, string> = { points: polyline(edge.points), class: "line" };
  switch (edge.edge.kind) {
    case "flow":
      attrs["marker-end"] = "url(#arrow-open)";
      break;
    case "connection":
      break;
    default:
      attrs["marker-end"] = "url(#arrow)";
  }
  group.append(element("polyline", attrs));
  if (edge.edge.label !== "") {
    const text = element("text", {
      x: String(edge.label.x), y: String(edge.label.y - 6), "text-anchor": "middle", class: "edge-label",
    });
    text.textContent = edge.edge.label;
    group.append(text);
  }
  return group;
}

// drawHandles are the handles a steerable edge is edited by: one on each waypoint
// to drag it, and one on the middle of each segment that a drag grows a waypoint from.
function drawHandles(edge: PlacedEdge): SVGGElement {
  const group = element("g", { class: "edge-handles", "data-edge": String(edge.index) });
  for (let i = 1; i < edge.points.length; i++) {
    const a = edge.points[i - 1];
    const b = edge.points[i];
    group.append(element("circle", {
      cx: String((a.x + b.x) / 2), cy: String((a.y + b.y) / 2), r: String(SEGMENT_RADIUS),
      class: "segment", "data-edge": String(edge.index), "data-segment": String(i - 1),
    }));
  }
  edge.route.forEach((point, i) => {
    group.append(element("circle", {
      cx: String(point.x), cy: String(point.y), r: String(WAYPOINT_RADIUS),
      class: "waypoint", "data-edge": String(edge.index), "data-point": String(i),
    }));
  });
  return group;
}

function polyline(points: RenderPoint[]): string {
  return points.map((p) => `${p.x},${p.y}`).join(" ");
}

function element<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string>): SVGElementTagNameMap[K] {
  const out = document.createElementNS(SVG, tag);
  for (const [name, value] of Object.entries(attrs)) {
    out.setAttribute(name, value);
  }
  return out;
}
