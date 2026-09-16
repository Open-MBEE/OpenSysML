// The custom methods the OpenSysML language server adds for diagrams, and the
// payloads they carry. They mirror internal/lsp/render.go.

export const RENDER_METHOD = "opensysml/render";
export const VIEWS_METHOD = "opensysml/views";
export const RENDER_CHANGED_METHOD = "opensysml/renderChanged";
export const DOCUMENTS_METHOD = "opensysml/documents";
export const RENDER_DOCUMENT_METHOD = "opensysml/renderDocument";

/** The capability the server advertises when it serves the render methods. */
export const RENDER_CAPABILITY = "openSysmlRender";

/** The capability the server advertises when it serves document rendering. */
export const RENDER_DOCUMENT_CAPABILITY = "openSysmlRenderDocument";

/** Turns diagram actions into a WorkspaceEdit; mirrors internal/lsp/modeledit.go. */
export const APPLY_MODEL_EDIT_METHOD = "opensysml/applyModelEdit";

/** The capability the server advertises when it serves model edits. */
export const APPLY_MODEL_EDIT_CAPABILITY = "openSysmlApplyModelEdit";

/** The capability each side advertises when it speaks the cross-document diagram contract: renderings naming other documents' declarations, `declaredHere`, and layouts pinned with `declaredIn`. */
export const CROSS_DOCUMENT_CAPABILITY = "openSysmlCrossDocumentLayout";

/** The URI scheme the server locates standard-library declarations in. */
export const STDLIB_SCHEME = "sysml-stdlib";

/** Serves the text of a `sysml-stdlib:` document; mirrors internal/lsp/stdlib.go. */
export const STDLIB_CONTENT_METHOD = "opensysml/stdlibContent";

/** The capability the server advertises when it serves the content request. */
export const STDLIB_CONTENT_CAPABILITY = "openSysmlStdlibContent";

export interface StdlibContentParams {
  uri: string;
}

export interface StdlibContentResult {
  text: string;
}

export interface Position {
  line: number;
  character: number;
}

export interface Range {
  start: Position;
  end: Position;
}

/**
 * Where an element was declared: `range` is the whole declaration, `selectionRange`
 * the declared identifier alone, which is where clicking a node goes. `digest`
 * fingerprints the text the ranges are of; an operation naming a range of another
 * document hands it back, and is answered stale once that text has changed.
 */
export interface RenderOrigin {
  uri: string;
  range: Range;
  selectionRange?: Range;
  digest: string;
}

export interface RenderNode {
  id: string;
  kind: string;
  name: string;
  type: string;
  detail: string;
  parent?: string;
  /** The qualified name a model edit targets the declaration by, in whichever workspace document declares it; absent for a node with none, and for a library's. */
  fqn?: string;
  /** The requested document declares the node, so every edit reaches it; absent, only a layout does. */
  declaredHere?: boolean;
  /** The keyword the declaration was written with (`part def`, `port`); with `fqn`. A move asks its new owner to admit it. */
  notation?: string;
  /** The namespaces declaring the node, nearest first, drawn or not; absent with `fqn`, and for a top-level declaration. */
  owners?: RenderOwner[];
  /** The range of the node's declaration, in the document `origin` names, when no qualified name reaches it; a layout edit targets that instead of `fqn`. */
  declaration?: Range;
  /** Where the node was declared; absent for one drawn from no workspace document. */
  origin?: RenderOrigin;
  /** Where a `DiagramLayout::Layout` puts the node, in pixels, y down; absent when the model does not place it. */
  x?: number;
  y?: number;
  /** The stated size, both or neither. */
  width?: number;
  height?: number;
  /** The node is drawn closed, its children hidden. */
  collapsed?: boolean;
}

/** One waypoint or corner, in the canvas's pixels, y down. */
export interface RenderPoint {
  x: number;
  y: number;
}

/** The drawing surface a view states with a `DiagramLayout::Canvas`. */
export interface RenderCanvas {
  unit?: string;
  width?: number;
  height?: number;
}

/** RenderOwner is a namespace declaring a node: its qualified name, and whether it is a feature an end path chains through with `.`. */
export interface RenderOwner {
  fqn: string;
  feature: boolean;
}

export interface RenderEdge {
  from: string;
  to: string;
  label: string;
  kind: string;
  /** The qualified name a model edit targets the declaring connection by, in whichever workspace document declares it; absent for one with none. */
  fqn?: string;
  /** The range of the connection's declaration, in the document `origin` names, when no qualified name reaches it, as on a node. */
  declaration?: Range;
  /** Where the connection was declared; absent for one drawn from no workspace document. */
  origin?: RenderOrigin;
  /** The waypoints a `DiagramLayout::Route` steers the edge through, source to target. */
  route?: RenderPoint[];
}

export interface RenderRow {
  cells: string[];
  origin?: RenderOrigin;
}

export interface RenderParams {
  textDocument: { uri: string };
  view?: string;
  form?: string;
}

export interface RenderResult {
  view: string;
  kind: string;
  stated: string;
  form: string;
  artifact: string;
  nodes: RenderNode[];
  edges: RenderEdge[];
  rows?: RenderRow[];
  columns?: string[];
  notices: string[];
  canvas?: RenderCanvas;
  /** What a diagram of this kind offers to add; absent when the rendering is not editable. */
  palette?: EditPalette;
  version: number;
}

/** The geometry a `setLayout` writes; `width` and `height` go together. */
export interface LayoutGeometry {
  x: number;
  y: number;
  width?: number;
  height?: number;
  collapsed?: boolean;
}

/** The member and connection kinds a rendering's kind offers, in the document's language. */
export interface EditPalette {
  members: string[];
  connections: string[];
  /** The members that take a type. */
  typed: string[];
  /** For each member only some bodies offer (`subject`), and each drawn notation that is one, the ids of the nodes that open one. */
  owners?: Record<string, string[]>;
}

/** admits: whether a member may go into node — any node, unless the palette confines the kind to some. */
export function admits(palette: EditPalette | undefined, memberKind: string, node: RenderNode): boolean {
  const owners = palette?.owners?.[memberKind];
  return owners ? owners.includes(node.id) : true;
}

/** declaredHere: whether the requested document declares a node, which every edit reaches; another document's takes a layout alone. */
export function declaredHere(node: RenderNode): node is RenderNode & { fqn: string } {
  return node.declaredHere === true && node.fqn !== undefined;
}

/** ownDeclarations: nodes as a server predating `declaredHere` means them — every named one is the requested document's own. */
export function ownDeclarations(nodes: RenderNode[]): RenderNode[] {
  return nodes.map((node) => (node.fqn === undefined ? node : { ...node, declaredHere: true }));
}

/** reachable: whether a layout edit can reach a node or edge — by qualified name, or by declaration when none reaches it. */
export function reachable(element: RenderNode | RenderEdge): boolean {
  return element.fqn !== undefined || element.declaration !== undefined;
}

/** One edit.Operation on the wire; `kind` selects which of the other fields are read. */
export type ModelEditOperation =
  | { kind: "setValue"; target: string; value: string }
  | { kind: "rename"; target: string; newName: string }
  | { kind: "addMember"; owner: string; memberKind: string; name: string; type?: string; multiplicity?: string; value?: string; specializes?: string[] }
  | { kind: "addConnection"; owner: string; memberKind: string; from: string; to: string; name?: string; type?: string }
  | { kind: "delete"; target: string; cascade?: boolean }
  | { kind: "move"; target: string; owner: string }
  /** Places `target` in `view`'s body, or inline in its own declaration without a view; no `layout` clears the annotation. `declaredIn` names the document declaring `target`, whose text `digest` fingerprints as its origin reported, so that a namesake declared since is answered stale rather than placed. */
  | { kind: "setLayout"; target: string; declaredIn?: string; digest?: string; view?: string; layout?: LayoutGeometry }
  /** Places the node declared at `declaration`, which no qualified name reaches, inline: a view body cannot name it. The range is one of the document `declaredIn` names — whose text `digest` fingerprints, as its origin reported — or of the edited document without it. */
  | { kind: "setLayout"; declaration: Range; declaredIn?: string; digest?: string; layout?: LayoutGeometry }
  /** Steers the connection `target` through `route`, per view or inline as above; an empty or absent route clears it. */
  | { kind: "setRoute"; target: string; declaredIn?: string; digest?: string; view?: string; route?: RenderPoint[] }
  /** Steers the connection declared at `declaration` inline, as `setLayout` by declaration places a node. */
  | { kind: "setRoute"; declaration: Range; declaredIn?: string; digest?: string; route?: RenderPoint[] }
  /** Sizes the drawing surface of the view `target`; no `canvas` clears it. */
  | { kind: "setCanvas"; target: string; canvas?: RenderCanvas };

export interface ApplyModelEditParams {
  textDocument: { uri: string };
  version: number;
  operations: ModelEditOperation[];
}

/**
 * Why an operation was refused; `operation` is its index, or -1 for the request as a whole.
 * `referring` names the declarations referring to a refused target, qualified by document
 * when that is another; `referrers` tells each from the document declaring it.
 */
export interface ModelEditRefusal {
  operation: number;
  failure: string;
  message: string;
  diagnostics?: { range: Range; message: string; severity?: number; code?: string | number; source?: string }[];
  referring?: string[];
  referrers?: ModelEditReferrer[];
}

export interface ModelEditReferrer {
  name: string;
  uri: string;
}

/**
 * A WorkspaceEdit as the protocol writes it; the language client converts it. Each document
 * change is pinned to the version the server computed it against, null for a document the
 * server read from disk.
 */
export interface WorkspaceEdit {
  changes?: Record<string, TextEdit[]>;
  documentChanges?: { textDocument: { uri: string; version: number | null }; edits: TextEdit[] }[];
}

export interface TextEdit {
  range: Range;
  newText: string;
}

/** Exactly one of `edit`, `refused` or `stale`; `version` is the document version answered at. */
export interface ApplyModelEditResult {
  edit?: WorkspaceEdit;
  refused?: ModelEditRefusal[];
  stale?: boolean;
  version: number;
}

export interface ViewsParams {
  textDocument: { uri: string };
}

export interface ViewInfo {
  name: string;
  kind: string;
  supported: boolean;
  reason?: string;
}

export interface ViewsResult {
  views: ViewInfo[];
  pseudoViews?: string[];
}

/** One document definition of the workspace, by qualified name. */
export interface DocumentInfo {
  name: string;
  uri: string;
}

export interface DocumentsResult {
  documents: DocumentInfo[];
}

export interface RenderDocumentParams {
  name: string;
}

export interface RenderDocumentResult {
  name: string;
  markdown: string;
}

export interface RenderChangedParams {
  textDocument: { uri: string };
  version: number;
}

/** One entry of the panel's view picker. */
export interface PickerEntry {
  /** What is sent as the render request's `view`. */
  value: string;
  label: string;
  /** False for a view whose rendering kind the server does not produce. */
  supported: boolean;
  reason?: string;
}

/** A message the extension sends the webview; `drawn` counts the panel's drawings and names this one. */
export type ToWebview =
  | { type: "render"; result: RenderResult; selected: string; drawn: number }
  | { type: "views"; views: PickerEntry[]; selected: string }
  | { type: "error"; message: string }
  | { type: "highlight"; id: string | undefined }
  /** A drop the model did not take: the canvas goes back to the model's layout, the status line says why. */
  | { type: "revert"; message?: string };

/** A diagram action naming nodes by rendering id; the extension asks for the rest. */
export type EditAction =
  | { kind: "addMember"; memberKind: string; typed: boolean; owner?: string }
  | { kind: "addConnection"; connectionKind: string; from?: string; to?: string }
  | { kind: "rename"; id: string }
  | { kind: "delete"; id: string }
  | { kind: "move"; id: string };

/** Where a gesture left a node. */
export interface NodePlacement {
  id: string;
  layout: LayoutGeometry;
}

/** Where a gesture left an edge, by its index in the rendering's edges: its waypoints, or none for a straight edge. */
export interface EdgePlacement {
  index: number;
  route?: RenderPoint[];
}

/** A message the webview sends the extension; `drawn` is the drawing a message's ids name. */
export type FromWebview =
  | { type: "ready" }
  | { type: "reveal"; id: string; drawn: number }
  | { type: "pick"; view: string }
  | { type: "edit"; action: EditAction; drawn: number }
  /** One completed gesture: everything it moved, applied as one edit. */
  | { type: "place"; nodes: NodePlacement[]; edges: EdgePlacement[]; drawn: number }
  /** A node dropped on another: moved into it as a declaration, and placed where it was released. */
  | { type: "reparent"; id: string; owner: string; nodes: NodePlacement[]; edges: EdgePlacement[]; drawn: number }
  | { type: "failed"; message: string };
