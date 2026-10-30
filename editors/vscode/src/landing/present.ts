import type { RenderResult } from "../protocol";
import type { EngineQuantity, LandingModel, LandingPart } from "./model";
import type { SceneTone } from "./scene";

/**
 * presentedStack is the landing model as the hero draws the stack: each project's box
 * headed by its label, with its kind as the «keyword» and its language and role as the
 * detail line. Connection names are left to the cards, so the wires carry no labels.
 */
export function presentedStack(model: LandingModel): RenderResult {
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
        // A nested part shows its label alone; its role is on its card.
        detail: part.owner === undefined ? [lang, role].filter(Boolean).join(" · ") : "",
      };
    }),
    edges: model.render.edges.map((edge) => ({ ...edge, label: "" })),
  };
}

/**
 * presentedPhases is the landing model as the hero draws a state machine: the states as the
 * view drew them, each transition labelled with the wait that fires it.
 */
export function presentedPhases(model: LandingModel): RenderResult {
  return model.render;
}

const STACK_TONES: Record<string, SceneTone> = { runtime: "primary", reference: "secondary", "model store": "accent" };

/** stackTone fills a project's box by the kind its model declares; a component stays plain. */
export function stackTone(part: LandingPart): SceneTone | undefined {
  return STACK_TONES[part.attrs.kind ?? ""];
}

/** quantityText is a quantity as the hero prints it: the engine's digits grouped, then the unit. */
export function quantityText(quantity: EngineQuantity): string {
  const grouped = /^-?\d+$/u.test(quantity.magnitude)
    ? BigInt(quantity.magnitude).toLocaleString("en-US")
    : quantity.magnitude;
  return `${grouped} ${quantity.unit}`;
}
