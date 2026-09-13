// How a diagram action becomes an opensysml/applyModelEdit request: owner,
// endpoint spelling, the version it is pinned to and refusal wording. No VS
// Code here, so it is unit-tested.
import type { ApplyModelEditParams, ModelEditOperation, ModelEditRefusal, RenderNode } from "./protocol";

/** Rendering is the diagram an action is taken on: its nodes and the document version they draw. */
export interface Rendering {
  nodes: RenderNode[];
  version: number;
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

/** connectionOwner is the nearest declared common ancestor of two nodes, either included. */
export function connectionOwner(from: RenderNode, to: RenderNode, nodes: RenderNode[]): RenderNode | undefined {
  const fromChain = [from, ...ancestors(from, nodes)];
  const toChain = new Set([to, ...ancestors(to, nodes)].map((node) => node.id));
  return fromChain.find((node) => node.fqn && toChain.has(node.id));
}

/** endpointPath spells a node from owner's scope (`tank.fuelOut`); undefined when it cannot be. */
export function endpointPath(node: RenderNode, owner: RenderNode, nodes: RenderNode[]): string | undefined {
  const steps: string[] = [];
  for (const step of [node, ...ancestors(node, nodes)]) {
    if (step.id === owner.id) {
      return steps.length === 0 ? undefined : steps.reverse().join(".");
    }
    if (!step.name || step.name.includes("::")) {
      return undefined;
    }
    steps.push(step.name);
  }
  return undefined;
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
