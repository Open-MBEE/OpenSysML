import type { RenderResult } from "../protocol";
import type { EngineQuantity, LandingModel } from "./model";
import type { SceneBox, SceneTone } from "./scene";

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

/** presentedMission is the landing model as the hero draws an action: its steps as the view drew them, each succession a wire. */
export function presentedMission(model: LandingModel): RenderResult {
  return model.render;
}

const STACK_TONES: Record<string, SceneTone> = { runtime: "primary", reference: "secondary", "model store": "accent" };

/** stackTone fills a project's box by the kind its model declares; a component stays plain. */
export function stackTone(box: SceneBox): SceneTone | undefined {
  return box.kind === "part" ? STACK_TONES[box.part.attrs.kind ?? ""] : undefined;
}

/** phaseTone fills a state's box: the one the machine enters first in the accent, the others primary; the start and final symbols stay plain. */
export function phaseTone(box: SceneBox): SceneTone | undefined {
  if (box.kind !== "state" || box.state.kind !== "state") {
    return undefined;
  }
  return box.state.initial ? "accent" : "primary";
}

/** missionTone fills an action's box: a step primary, the decide that picks its branch in the accent; the initial and final symbols stay plain. */
export function missionTone(box: SceneBox): SceneTone | undefined {
  if (box.kind !== "step") {
    return undefined;
  }
  switch (box.step.kind) {
    case "action":
      return "primary";
    case "decision":
      return "accent";
    default:
      return undefined;
  }
}

/** quantityText is a quantity as the hero prints it: the engine's digits grouped, then the unit. */
export function quantityText(quantity: EngineQuantity): string {
  const grouped = /^-?\d+$/u.test(quantity.magnitude)
    ? BigInt(quantity.magnitude).toLocaleString("en-US")
    : quantity.magnitude;
  return `${grouped} ${quantity.unit}`;
}
