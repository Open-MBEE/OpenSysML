// What releasing a dragged node over another one does, judged by the admission
// rule Move to… applies, so the canvas marks exactly the destinations the menu lists.
import { ancestors, moveDestinations, type Rendering } from "../edits";
import type { RenderNode, RenderResult } from "../protocol";

/** Drop is what a release over `target` would do, and what the status line says meanwhile. */
export interface Drop {
  target: RenderNode;
  /** The target admits the dragged declaration, so releasing moves it there. */
  admits: boolean;
  message: string;
}

/** dropOn decides a release of `node` over `target`: a move when Move to… would offer the target, else nothing, with the reason. */
export function dropOn(node: RenderNode, target: RenderNode, result: RenderResult): Drop {
  const rendering = renderingOf(result);
  if (rendering.palette !== undefined && moveDestinations(node, rendering).some((destination) => destination.node === target)) {
    return { target, admits: true, message: `Release to move ${label(node)} into ${label(target)}.` };
  }
  return { target, admits: false, message: refusal(node, target, rendering) };
}

/** dragHint says how a dragged node is moved into another, when the rendering draws one that admits it. */
export function dragHint(node: RenderNode, result: RenderResult): string | undefined {
  if (result.palette === undefined || !moveDestinations(node, renderingOf(result)).some((destination) => destination.node)) {
    return undefined;
  }
  return `Hold Shift and release over a node to move ${label(node)} into it.`;
}

// refusal says why target does not take node, nearest cause first.
function refusal(node: RenderNode, target: RenderNode, rendering: Rendering): string {
  if (rendering.palette === undefined) {
    return "The language server does not serve model edits, so nothing is moved.";
  }
  if (node.fqn === undefined) {
    return `${label(node)} is not declared in this document, so it cannot be moved.`;
  }
  if (node.notation === undefined) {
    return `${label(node)} cannot be moved from the diagram.`;
  }
  if (target.fqn === undefined) {
    return `${label(target)} is not declared in this document, so nothing can be moved into it.`;
  }
  if (node.owners?.[0]?.fqn === target.fqn) {
    return `${label(node)} is already declared in ${label(target)}.`;
  }
  if (target.fqn === node.fqn || target.owners?.some((owner) => owner.fqn === node.fqn) || ancestors(target, rendering.nodes).includes(node)) {
    return `${label(target)} is declared inside ${label(node)}, which cannot be moved into it.`;
  }
  return `A ${node.notation} cannot be declared in ${label(target)}.`;
}

// renderingOf is the rendering an edit is judged on, as the extension reads it.
function renderingOf(result: RenderResult): Rendering {
  return { nodes: result.nodes ?? [], edges: result.edges, view: result.view, version: result.version, palette: result.palette };
}

// label names a node as the canvas does: its name, or its kind when it has none.
function label(node: RenderNode): string {
  return node.name === "" ? `the ${node.kind}` : node.name;
}
