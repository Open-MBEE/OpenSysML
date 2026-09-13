// How a diagram action becomes an opensysml/applyModelEdit request: owner,
// endpoint spelling, retry and refusal wording. No VS Code here, so it is unit-tested.
import type { ApplyModelEditResult, ModelEditRefusal, RenderNode } from "./protocol";

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

/** withRetry re-sends once, at the buffer's version as it is then, when the server found the first stale. */
export async function withRetry(
  version: () => number,
  request: (version: number) => Promise<ApplyModelEditResult>,
): Promise<ApplyModelEditResult> {
  const first = await request(version());
  if (!first.stale) {
    return first;
  }
  return request(version());
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

/** validName says why a name cannot be declared, or nothing when it can. */
export function validName(name: string): string | undefined {
  if (name.trim() === "") {
    return "A name is required.";
  }
  if (!/^(?:[A-Za-z_][A-Za-z0-9_]*|'[^']+')$/.test(name.trim())) {
    return "A name is an identifier, or any text in single quotes.";
  }
  return undefined;
}
