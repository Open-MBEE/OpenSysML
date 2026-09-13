// How a diagram action becomes an opensysml/applyModelEdit request: owner,
// endpoint spelling, the version it is pinned to and refusal wording. No VS
// Code here, so it is unit-tested.
import {
  admits,
  type ApplyModelEditParams,
  type EdgePlacement,
  type EditPalette,
  type ModelEditOperation,
  type ModelEditRefusal,
  type NodePlacement,
  type RenderEdge,
  type RenderNode,
  type RenderOwner,
} from "./protocol";

/**
 * Rendering is the diagram an action is taken on: its nodes and edges, the view
 * they were drawn for (empty for a pseudo-view), what they offer to add, and the
 * document version they draw.
 */
export interface Rendering {
  nodes: RenderNode[];
  edges?: RenderEdge[];
  view?: string;
  version: number;
  palette?: EditPalette;
}

/**
 * placementOperations turns where a gesture left nodes and edges into the layout
 * and route operations of one edit: a Layout in the view's body for a rendering
 * of a declared view, inline on the element for a pseudo-view. An element no
 * qualified name reaches is targeted by its declaration and placed inline, since
 * a view body cannot name it. Undefined when something placed is not declared by
 * the document, since no annotation can reach it.
 */
export function placementOperations(
  rendering: Rendering,
  nodes: NodePlacement[],
  edges: EdgePlacement[],
): ModelEditOperation[] | undefined {
  const view = rendering.view || undefined;
  const operations: ModelEditOperation[] = [];
  for (const placement of nodes) {
    const node = rendering.nodes.find((candidate) => candidate.id === placement.id);
    if (node?.fqn) {
      operations.push({ kind: "setLayout", target: node.fqn, view, layout: placement.layout });
    } else if (node?.declaration) {
      operations.push({ kind: "setLayout", declaration: node.declaration, layout: placement.layout });
    } else {
      return undefined;
    }
  }
  for (const placement of edges) {
    const edge = rendering.edges?.[placement.index];
    if (edge?.fqn) {
      operations.push({ kind: "setRoute", target: edge.fqn, view, route: placement.route });
    } else if (edge?.declaration) {
      operations.push({ kind: "setRoute", declaration: edge.declaration, route: placement.route });
    } else {
      return undefined;
    }
  }
  return operations;
}

/** A node's ancestors, nearest first, ending at a root. */
export function ancestors(node: RenderNode, nodes: RenderNode[]): RenderNode[] {
  const byID = new Map(nodes.map((candidate) => [candidate.id, candidate]));
  const out: RenderNode[] = [];
  const seen = new Set<string>([node.id]);
  for (let parent = node.parent; parent && !seen.has(parent); parent = byID.get(parent)?.parent) {
    seen.add(parent);
    const found = byID.get(parent);
    if (!found) {
      break;
    }
    out.push(found);
  }
  return out;
}

/** ownerOf is the node's own declaration, else its nearest ancestor the document declares. */
export function ownerOf(node: RenderNode | undefined, nodes: RenderNode[]): RenderNode | undefined {
  if (!node) {
    return undefined;
  }
  return [node, ...ancestors(node, nodes)].find((candidate) => candidate.fqn);
}

/** rootOwner is the rendering's one declared root, or nothing when there are several. */
export function rootOwner(nodes: RenderNode[]): RenderNode | undefined {
  const roots = nodes.filter((node) => !node.parent && node.fqn);
  return roots.length === 1 ? roots[0] : undefined;
}

/** DOCUMENT_ROOT stands for the document itself, which owns its top-level declarations; an edit names it by the empty owner. */
export const DOCUMENT_ROOT: RenderOwner = { fqn: "", feature: false };

/** ownersOf: the namespaces declaring node, nearest first, the document last; undefined for a node the document does not declare. */
export function ownersOf(node: RenderNode): RenderOwner[] | undefined {
  return node.fqn === undefined ? undefined : [...(node.owners ?? []), DOCUMENT_ROOT];
}

/** Destination is a namespace a move may put a node into: a drawn node, or the document itself, which no node draws. */
export interface Destination {
  fqn: string;
  node?: RenderNode;
}

/**
 * moveDestinations lists where node may be moved, in drawing order with the document last:
 * every declared node that admits its notation, but itself, what it declares and its present
 * owner; the document when it admits the notation and does not already own the node.
 */
export function moveDestinations(node: RenderNode, rendering: Rendering): Destination[] {
  if (node.fqn === undefined || node.notation === undefined) {
    return [];
  }
  const notation = node.notation;
  const owner = node.owners?.[0] ?? DOCUMENT_ROOT;
  const offered = new Set<string>([node.fqn, owner.fqn]);
  const out: Destination[] = [];
  for (const candidate of rendering.nodes) {
    if (candidate.fqn === undefined || offered.has(candidate.fqn) || !admits(rendering.palette, notation, candidate)) {
      continue;
    }
    if (
      candidate.owners?.some((owner) => owner.fqn === node.fqn) ||
      ancestors(candidate, rendering.nodes).includes(node)
    ) {
      continue;
    }
    offered.add(candidate.fqn);
    out.push({ fqn: candidate.fqn, node: candidate });
  }
  if (!offered.has(DOCUMENT_ROOT.fqn) && rendering.palette?.owners?.[notation] === undefined) {
    out.push({ fqn: DOCUMENT_ROOT.fqn });
  }
  return out;
}

/** moveOperation is the one operation that puts node into the namespace owner names; "" is the document. */
export function moveOperation(node: RenderNode, owner: string): ModelEditOperation | undefined {
  return node.fqn === undefined ? undefined : { kind: "move", target: node.fqn, owner };
}

/** nameSegments splits a qualified name at `::` outside quotes: `'P::Q'::x` is two segments. */
export function nameSegments(text: string): string[] {
  const out: string[] = [];
  let start = 0;
  let quoted = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (quoted && c === "\\") {
      i++;
    } else if (c === "'") {
      quoted = !quoted;
    } else if (!quoted && text.startsWith("::", i)) {
      out.push(text.slice(start, i));
      start = i + 2;
      i++;
    }
  }
  out.push(text.slice(start));
  return out;
}

/**
 * connectionOwner is the nearest namespace declaring both nodes, drawn or not, down to
 * the document itself; undefined when either is not declared in the document.
 */
export function connectionOwner(from: RenderNode, to: RenderNode): RenderOwner | undefined {
  const fromOwners = ownersOf(from);
  const toOwners = ownersOf(to);
  if (!fromOwners || !toOwners) {
    return undefined;
  }
  const shared = new Set(toOwners.map((owner) => owner.fqn));
  return fromOwners.find((owner) => shared.has(owner.fqn));
}

/** describeOwner names an owner for a message: its qualified name, or the document for DOCUMENT_ROOT. */
export function describeOwner(owner: RenderOwner): string {
  return owner.fqn === "" ? "the document" : owner.fqn;
}

/** localName is the last name of a qualified one, as the notation spells it: `'fuel::out'` of `'x::y'::'fuel::out'`. */
function localName(fqn: string): string {
  const segments = nameSegments(fqn);
  return segments[segments.length - 1];
}

/**
 * endpointPath spells a node from owner's scope, chaining through a feature by `.` and
 * into any other namespace by `::` (`tank.fuelOut`; from the document, `Car::tank.fuelOut`);
 * undefined for the owner itself and for a node outside it.
 */
export function endpointPath(node: RenderNode, owner: RenderOwner): string | undefined {
  const owners = ownersOf(node);
  const below = owners?.findIndex((step) => step.fqn === owner.fqn) ?? -1;
  if (!owners || below < 0 || node.fqn === undefined) {
    return undefined;
  }
  const steps: RenderOwner[] = [{ fqn: node.fqn, feature: false }, ...owners.slice(0, below)].reverse();
  return steps.map((step, i) => (i === 0 ? "" : steps[i - 1].feature ? "." : "::") + localName(step.fqn)).join("");
}

/** What the user is told when an action names a rendering that has been replaced. */
export const REDRAWN_MESSAGE =
  "The document changed after the diagram was drawn; it is redrawn now, so repeat the action on it.";

/** offeredOn reports whether an action taken on rendering version `offered` still names `rendering`. */
export function offeredOn(rendering: Rendering, offered: number): boolean {
  return offered === rendering.version;
}

// editParams pins the request to the version the operations were read from: a later
// version may spell the same names for other declarations, so it is that text or none.
export function editParams(uri: string, rendering: Rendering, operations: ModelEditOperation[]): ApplyModelEditParams {
  return { textDocument: { uri }, version: rendering.version, operations };
}

/** describeRefusal is the one-line message a refused request is reported with. */
export function describeRefusal(refused: ModelEditRefusal[]): string {
  return refused.map(describeOne).join("\n");
}

function describeOne(refusal: ModelEditRefusal): string {
  const parts = [refusal.message];
  for (const diagnostic of refusal.diagnostics ?? []) {
    parts.push(`${diagnostic.range.start.line + 1}:${diagnostic.range.start.character + 1}: ${diagnostic.message}`);
  }
  if (refusal.referring?.length) {
    parts.push(`Referenced by ${refusal.referring.join(", ")}.`);
  }
  return parts.join("\n");
}

/** Characters a backslash may escape in an unrestricted name (KerML §8.2.2). */
const ESCAPABLE = new Set(["b", "t", "n", "f", "r", '"', "'", "\\"]);

/**
 * validName says why a name cannot be declared, or nothing when it can. It
 * checks the token shape only; the server decides reserved words.
 */
export function validName(name: string): string | undefined {
  const text = name.trim();
  if (text === "") {
    return "A name is required.";
  }
  if (/^[A-Za-z_][A-Za-z0-9_]*$/.test(text)) {
    return undefined;
  }
  if (!text.startsWith("'")) {
    return "A name is an identifier, or any text in single quotes.";
  }
  for (let i = 1; i < text.length; i++) {
    const c = text[i];
    if (c === "\\") {
      i++;
      if (i === text.length || !ESCAPABLE.has(text[i])) {
        return "A backslash in a quoted name escapes one of b t n f r \" ' \\.";
      }
    } else if (c === "\n" || c === "\r") {
      break;
    } else if (c === "'") {
      if (i === 1) {
        return "A quoted name needs some text between the quotes.";
      }
      return i === text.length - 1
        ? undefined
        : "A quoted name ends at its closing quote; escape a quote inside as \\'.";
    }
  }
  return "A quoted name needs a closing quote on the same line.";
}
