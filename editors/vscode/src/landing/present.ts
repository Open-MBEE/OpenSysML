import type { RenderResult } from "../protocol";
import type { EngineQuantity, LandingModel, LandingPart } from "./model";
import type { Scene, SceneTone } from "./scene";

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

const STACK_TONES: Record<string, SceneTone> = { runtime: "primary", reference: "secondary", "model store": "accent" };

/** stackTone fills a project's box by the kind its model declares; a component stays plain. */
export function stackTone(part: LandingPart): SceneTone | undefined {
  return STACK_TONES[part.attrs.kind ?? ""];
}

/**
 * rollupTone fills a top-level part's box as the stack fills its projects, and accents any
 * part whose rolled-up feature the engine left undetermined; a nested, valued part stays plain.
 */
export function rollupTone(part: LandingPart, scene: Scene): SceneTone | undefined {
  const feature = rolledUp(scene);
  if (feature !== undefined && part.unvalued[feature] !== undefined) {
    return "accent";
  }
  return part.owner === undefined ? "primary" : undefined;
}

/** quantityText is a quantity as the hero prints it: the engine's digits grouped, then the unit. */
export function quantityText(quantity: EngineQuantity): string {
  const grouped = /^-?\d+$/u.test(quantity.magnitude)
    ? BigInt(quantity.magnitude).toLocaleString("en-US")
    : quantity.magnitude;
  return `${grouped} ${quantity.unit}`;
}

/** rolledUp is the feature a roll-up scene reports on every box; absent for another kind of scene. */
export function rolledUp(scene: Scene): string | undefined {
  return scene.program.kind === "rollup" ? scene.program.feature : undefined;
}

/**
 * valueLine is a box's detail line for one feature: the quantity the instance holds, or
 * the engine's reason when the instantiation could not value it; empty when it has neither.
 */
export function valueLine(model: LandingModel, id: string, feature: string): string {
  const part = [...model.parts.values()].find((candidate) => candidate.id === id);
  if (!part) {
    return "";
  }
  const quantity = part.quantities[feature];
  if (quantity !== undefined) {
    return `${feature} = ${quantityText(quantity)}`;
  }
  const reason = part.unvalued[feature];
  return reason === undefined ? "" : `${feature} undetermined: ${reason}`;
}

/**
 * presentedRollup is the landing model as the hero draws a roll-up: each part's box
 * headed by its feature name and type, with the rolled-up feature's value as the detail
 * line, nested as the composition nests. Binding wires carry no labels.
 */
export function presentedRollup(model: LandingModel, scene: Scene): RenderResult {
  const feature = rolledUp(scene);
  const byId = new Map([...model.parts.values()].map((part) => [part.id, part]));
  return {
    ...model.render,
    nodes: model.render.nodes.map((node) => {
      const part = byId.get(node.id);
      if (!part) {
        return node;
      }
      return {
        ...node,
        name: part.feature,
        type: part.type,
        detail: feature === undefined ? "" : valueLine(model, node.id, feature),
      };
    }),
    edges: model.render.edges.map((edge) => ({ ...edge, label: "" })),
  };
}
