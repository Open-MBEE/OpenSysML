import type {
  RenderCanvas as PbRenderCanvas,
  RenderEdge as PbRenderEdge,
  RenderGeometry as PbRenderGeometry,
  RenderNode as PbRenderNode,
  RenderNote as PbRenderNote,
  RenderPoint as PbRenderPoint,
  RenderPort as PbRenderPort,
  RenderRow as PbRenderRow,
  RenderStyle as PbRenderStyle,
  Span as PbSpan,
  RenderViewResponse,
} from "../generated/sysml_pb.js";

export interface RenderSpan {
  file: string;
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
}

export interface RenderPort {
  id: string;
  name: string;
  type: string;
  direction: string;
}

export interface RenderGeometry {
  x: number;
  y: number;
  width: number;
  height: number;
  hasSize: boolean;
  collapsed: boolean;
}

export interface RenderStyle {
  fill: string;
  line: string;
  text: string;
  font: string;
  fontSize: number;
  bold: boolean;
  italic: boolean;
}

export interface RenderPoint {
  x: number;
  y: number;
}

export interface RenderNode {
  id: string;
  kind: string;
  name: string;
  nameSynthesized: boolean;
  type: string;
  detail: string;
  text: string;
  standIn: boolean;
  parent: string;
  ports: readonly RenderPort[];
  origin?: RenderSpan;
  geometry?: RenderGeometry;
  style?: RenderStyle;
}

export interface RenderEdge {
  from: string;
  to: string;
  fromPort: string;
  toPort: string;
  label: string;
  name: string;
  kind: string;
  origin?: RenderSpan;
  route: readonly RenderPoint[];
  style?: RenderStyle;
}

export interface RenderCanvas {
  unit: string;
  width: number;
  height: number;
  hasSize: boolean;
}

export interface RenderRow {
  cells: readonly string[];
  origin?: RenderSpan;
}

export interface RenderNote {
  text: string;
  anchor: string;
  edgeFrom: string;
  edgeTo: string;
  x: number;
  y: number;
  width: number;
  height: number;
  hasSize: boolean;
  origin?: RenderSpan;
}

export interface RenderedView {
  view: string;
  kind: string;
  stated: string;
  nodes: readonly RenderNode[];
  edges: readonly RenderEdge[];
  columns: readonly string[];
  rows: readonly RenderRow[];
  canvas?: RenderCanvas;
  notes: readonly RenderNote[];
  notices: readonly string[];
}

function spanOf(span: PbSpan | undefined): RenderSpan | undefined {
  return span === undefined
    ? undefined
    : {
        file: span.file,
        startLine: span.startLine,
        startCol: span.startCol,
        endLine: span.endLine,
        endCol: span.endCol,
      };
}

function geometryOf(geometry: PbRenderGeometry | undefined): RenderGeometry | undefined {
  return geometry === undefined
    ? undefined
    : {
        x: geometry.x,
        y: geometry.y,
        width: geometry.width,
        height: geometry.height,
        hasSize: geometry.hasSize,
        collapsed: geometry.collapsed,
      };
}

function styleOf(style: PbRenderStyle | undefined): RenderStyle | undefined {
  return style === undefined
    ? undefined
    : {
        fill: style.fill,
        line: style.line,
        text: style.text,
        font: style.font,
        fontSize: style.fontSize,
        bold: style.bold,
        italic: style.italic,
      };
}

function portOf(port: PbRenderPort): RenderPort {
  return { id: port.id, name: port.name, type: port.type, direction: port.direction };
}

function pointOf(point: PbRenderPoint): RenderPoint {
  return { x: point.x, y: point.y };
}

function nodeOf(node: PbRenderNode): RenderNode {
  const origin = spanOf(node.origin);
  const geometry = geometryOf(node.geometry);
  const style = styleOf(node.style);
  return {
    id: node.id,
    kind: node.kind,
    name: node.name,
    nameSynthesized: node.nameSynthesized,
    type: node.type,
    detail: node.detail,
    text: node.text,
    standIn: node.standIn,
    parent: node.parent,
    ports: node.ports.map(portOf),
    ...(origin === undefined ? {} : { origin }),
    ...(geometry === undefined ? {} : { geometry }),
    ...(style === undefined ? {} : { style }),
  };
}

function edgeOf(edge: PbRenderEdge): RenderEdge {
  const origin = spanOf(edge.origin);
  const style = styleOf(edge.style);
  return {
    from: edge.from,
    to: edge.to,
    fromPort: edge.fromPort,
    toPort: edge.toPort,
    label: edge.label,
    name: edge.name,
    kind: edge.kind,
    route: edge.route.map(pointOf),
    ...(origin === undefined ? {} : { origin }),
    ...(style === undefined ? {} : { style }),
  };
}

function rowOf(row: PbRenderRow): RenderRow {
  const origin = spanOf(row.origin);
  return {
    cells: [...row.cells],
    ...(origin === undefined ? {} : { origin }),
  };
}

function noteOf(note: PbRenderNote): RenderNote {
  const origin = spanOf(note.origin);
  return {
    text: note.text,
    anchor: note.anchor,
    edgeFrom: note.edgeFrom,
    edgeTo: note.edgeTo,
    x: note.x,
    y: note.y,
    width: note.width,
    height: note.height,
    hasSize: note.hasSize,
    ...(origin === undefined ? {} : { origin }),
  };
}

function canvasOf(canvas: PbRenderCanvas | undefined): RenderCanvas | undefined {
  return canvas === undefined
    ? undefined
    : { unit: canvas.unit, width: canvas.width, height: canvas.height, hasSize: canvas.hasSize };
}

export function renderedViewOf(response: RenderViewResponse): RenderedView {
  const canvas = canvasOf(response.canvas);
  return {
    view: response.view,
    kind: response.kind,
    stated: response.stated,
    nodes: response.nodes.map(nodeOf),
    edges: response.edges.map(edgeOf),
    columns: [...response.columns],
    rows: response.rows.map(rowOf),
    notes: response.notes.map(noteOf),
    notices: [...response.notices],
    ...(canvas === undefined ? {} : { canvas }),
  };
}
