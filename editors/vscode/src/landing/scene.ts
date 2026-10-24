import type { RenderResult } from "../protocol";
import type { LandingModel, LandingPart } from "./model";
import { presentedRollup, presentedStack, rollupTone, stackTone } from "./present";
import mass from "./mass.json";
import stack from "./stack.json";

/** A scene whose Run executes a state machine and animates the states it visits. */
export interface SceneJourney {
  kind: "journey";
  /** The state machine the Run and Debug controls execute. */
  symbol: string;
  /** The events the Run button sends, in order; each is also a Debug send button. */
  events: readonly string[];
}

/** A scene whose Run instantiates the root and animates one feature rolling up through its parts. */
export interface SceneRollup {
  kind: "rollup";
  /** The feature every box reports and the animation rolls up, bottom-up: `totalMass`. */
  feature: string;
}

export type SceneProgram = SceneJourney | SceneRollup;

/** The hero's fill for a box, one of the three the page defines; a box with none is drawn plain. */
export type SceneTone = "primary" | "secondary" | "accent";

/** The controls under the stage a scene offers; the page hides the others. */
export type SceneControl = "run" | "edit" | "debug" | "download";

/** Another root the scene can switch to, offered as a toggle under the stage. */
export interface SceneVariant {
  id: string;
  /** The toggle's label while the variant is off. */
  label: string;
  /** The toggle's label while the variant is on. */
  activeLabel: string;
  rootSymbol: string;
  view: string;
  /** What the status line says when the variant is switched on. */
  hint: string;
  /** The variant this one builds on: its toggle is offered while that one is on, and switching it off returns there. */
  requires?: string;
}

/** What the Go landing test pins for a scene: the first paint before the module loads. */
export interface LandingSnapshot {
  hash: string;
  /** The protojson RenderViewResponse the module answered. */
  render: unknown;
  /** The protojson Instance messages Instantiate answered, the root first. */
  instances: unknown[];
}

export interface Scene {
  id: string;
  /** The tab's label. */
  title: string;
  /** What the drawing is, for assistive technology. */
  description: string;
  /** The name the model text is parsed under; diagnostics and origins name it. */
  documentName: string;
  /** The part whose composition is drawn; its nested parts are the diagram's boxes. */
  rootSymbol: string;
  /** The view RenderView draws: a declared view's qualified name or a targeted pseudo-view. */
  view: string;
  /** How many levels of nesting RenderView draws below the root; absent for the view's own depth. */
  depth?: number;
  /** The optional notation the drawing shows, as RenderView's `show` names it. */
  show: readonly string[];
  snapshot: LandingSnapshot;
  /** The file name Download PNG saves the drawing under. */
  pngName: string;
  controls: readonly SceneControl[];
  /** How the hero draws the model: the boxes' heads and detail lines. */
  present: (model: LandingModel, scene: Scene) => RenderResult;
  /** The tone a part's box is filled with; undefined leaves it plain. */
  tone: (part: LandingPart, scene: Scene) => SceneTone | undefined;
  program: SceneProgram;
  variants: readonly SceneVariant[];
}

export const STACK_SCENE: Scene = {
  id: "stack",
  title: "OpenSysML stack",
  description: "The OpenSysML stack, drawn from its SysML model",
  documentName: "opensysml-stack.sysml",
  rootSymbol: "OpenSysMLStack::stack",
  view: "#interconnection:OpenSysMLStack::stack",
  show: ["edge-labels", "typings", "keywords"],
  snapshot: stack,
  pngName: "opensysml-stack.png",
  controls: ["run", "edit", "debug", "download"],
  present: presentedStack,
  tone: stackTone,
  program: {
    kind: "journey",
    symbol: "OpenSysMLStack::ModelJourney",
    events: ["Commit", "Pull", "Push", "Check"],
  },
  variants: [],
};

export const MASS_ROLLUP_SCENE: Scene = {
  id: "mass",
  title: "Saturn V mass roll-up",
  description: "The Saturn V's stages and engines, with the masses the engine rolls up from its SysML model",
  documentName: "mass-rollup.sysml",
  rootSymbol: "MassRollup::saturnV",
  view: "#interconnection:MassRollup::saturnV",
  depth: 3,
  show: ["typings", "keywords"],
  snapshot: mass,
  pngName: "saturn-v-mass-rollup.png",
  controls: ["run", "edit", "download"],
  present: presentedRollup,
  tone: rollupTone,
  program: { kind: "rollup", feature: "totalMass" },
  variants: [
    {
      id: "iu",
      label: "Add the instrument unit",
      activeLabel: "Remove the instrument unit",
      rootSymbol: "MassRollup::saturnVWithIU",
      view: "#interconnection:MassRollup::saturnVWithIU",
      hint: "On purpose, the model gives the instrument unit a height but no mass: valid SysML, since a definition may leave a value to its usages. So the roll-up it joins is undetermined, and the engine says which value is missing rather than guessing one. Give it its mass to see the fix.",
    },
    {
      id: "iu-mass",
      label: "Give it its mass",
      activeLabel: "Take its mass away",
      rootSymbol: "MassRollup::saturnVComplete",
      view: "#interconnection:MassRollup::saturnVComplete",
      hint: "The fix is in the model, not the engine: the usage supplies the mass the definition left open (:>> mass = 2000 [kg] on instrumentUnit), and the same roll-up completes.",
      requires: "iu",
    },
  ],
};

export const SCENES: readonly Scene[] = [STACK_SCENE, MASS_ROLLUP_SCENE];

/** requiredVariant is the variant `variant` builds on; undefined when it stands alone. */
export function requiredVariant(scene: Scene, variant: SceneVariant): SceneVariant | undefined {
  return variant.requires === undefined ? undefined : scene.variants.find((candidate) => candidate.id === variant.requires);
}

/** variantChain is the variant switched on and the ones it builds on, that variant first; empty while none is on. */
export function variantChain(scene: Scene, variant: SceneVariant | undefined): SceneVariant[] {
  const chain: SceneVariant[] = [];
  for (let current = variant; current !== undefined && !chain.includes(current); current = requiredVariant(scene, current)) {
    chain.push(current);
  }
  return chain;
}

/** forcedSceneId is the scene a URL names, by `?scene=<id>` or `#scene=<id>`; absent when it names none. */
export function forcedSceneId(url: URL): string | undefined {
  const query = url.searchParams.get("scene");
  if (query) {
    return query;
  }
  const fragment = new URLSearchParams(url.hash.replace(/^#/u, "")).get("scene");
  return fragment || undefined;
}

/**
 * chooseScene is the scene a visit opens on: the one the URL forces when it names one of
 * `scenes`, else one drawn at random with `random`, a uniform draw from [0, 1).
 */
export function chooseScene(scenes: readonly Scene[], url: URL, random: () => number = Math.random): Scene {
  if (scenes.length === 0) {
    throw new Error("no scene to choose from");
  }
  const forced = forcedSceneId(url);
  const named = forced === undefined ? undefined : scenes.find((scene) => scene.id === forced);
  return named ?? scenes[Math.min(scenes.length - 1, Math.floor(random() * scenes.length))];
}
