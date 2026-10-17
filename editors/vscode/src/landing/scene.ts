// A scene is one model the landing page draws and runs: where its text comes
// from, which element it draws, and which state machine its controls drive.

export interface SceneJourney {
  /** The state machine the Run and Debug controls execute. */
  symbol: string;
  /** The events the Run button sends, in order; each is also a Debug send button. */
  events: readonly string[];
}

export interface Scene {
  id: string;
  /** The name the model text is parsed under; diagnostics and origins name it. */
  documentName: string;
  /** The part whose interconnection is drawn; its nested parts are the diagram's boxes. */
  rootSymbol: string;
  /** The view RenderView draws: a declared view's qualified name or a targeted pseudo-view. */
  view: string;
  journey: SceneJourney;
}

export const STACK_SCENE: Scene = {
  id: "stack",
  documentName: "opensysml-stack.sysml",
  rootSymbol: "OpenSysMLStack::stack",
  view: "#interconnection:OpenSysMLStack::stack",
  journey: {
    symbol: "OpenSysMLStack::ModelJourney",
    events: ["Commit", "Pull", "Push", "Check"],
  },
};
