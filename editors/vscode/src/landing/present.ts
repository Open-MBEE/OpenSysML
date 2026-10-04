import type { RenderResult } from "../protocol";
import type { LandingModel } from "./model";

/**
 * presented is the landing model as the hero draws it: each project's box headed by its
 * label, with its kind as the «keyword» and its language and role as the detail line.
 * Connection names are left to the cards, so the wires carry no labels.
 */
export function presented(model: LandingModel): RenderResult {
  const byId = new Map([...model.parts.values()].map((part) => [part.id, part]));
  return {
    ...model.render,
    nodes: model.render.nodes.map((node) => {
      const part = byId.get(node.id);
      if (!part) {
        return node;
      }
      const { label, kind, lang, role } = part.attrs;
      return {
        ...node,
        name: label || part.feature,
        kind: kind || node.kind,
        type: "",
        detail: [lang, role].filter((value) => value).join(" · "),
      };
    }),
    edges: model.render.edges.map((edge) => ({ ...edge, label: "" })),
  };
}
