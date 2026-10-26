// The look a diagram is drawn in: the editor's theme, the pilot visualizer's black and
// white, that look filled from a server palette, or the look of Cameo Systems Modeler.
// Free of the vscode module for the webview.

/** The setting that picks the look every diagram panel is drawn in. */
export const STYLE_SETTING = "opensysml.diagram.style";

/** The setting naming the optional notation elements every diagram draws besides its view's own. */
export const SHOW_SETTING = "opensysml.diagram.show";

/** The setting naming the relationship kinds every diagram panel hides until its own filter says otherwise. */
export const HIDE_RELATIONS_SETTING = "opensysml.diagram.hideRelations";

/** The relationship kinds a rendering's edges carry, as the server's view model names them. */
export const RELATION_KINDS = [
  "connection", "transition", "succession", "flow", "binding", "specialization", "typing", "composition",
  "reference", "containment", "import", "satisfy", "verify", "derive", "refine", "allocate", "association",
  "include", "anchor", "dependency", "subsetting", "redefinition", "perform", "exhibit",
] as const;

/** hiddenRelationsOf reads the hideRelations setting: the kinds it lists, each once, in order; anything else hides none. */
export function hiddenRelationsOf(value: unknown): string[] {
  return shownOf(value);
}

/** The setting bounding how many levels of nested elements a diagram draws below each exposed element. */
export const DEPTH_SETTING = "opensysml.diagram.depth";

/** The setting that picks whose routes the diagram panel draws its edges along. */
export const ROUTING_SETTING = "opensysml.diagram.routing";

/** The routings: `model` draws a stated `Route` verbatim, `router` sends every edge without an ELK route through libavoid. */
export const ROUTINGS = ["model", "router"] as const;

export type Routing = (typeof ROUTINGS)[number];

export const DEFAULT_ROUTING: Routing = "model";

/** routingOf reads the routing setting: the routing it names, or the default when it names none. */
export function routingOf(value: unknown): Routing {
  return typeof value === "string" && (ROUTINGS as readonly string[]).includes(value) ? (value as Routing) : DEFAULT_ROUTING;
}

/** The setting that says where a newly opened diagram panel goes. */
export const OPEN_IN_SETTING = "opensysml.diagram.openIn";

/** Where a new panel opens: in the diagram group beside the source, or moved out into a window of its own. */
export const OPEN_INS = ["beside", "newWindow"] as const;

export type OpenIn = (typeof OPEN_INS)[number];

export const DEFAULT_OPEN_IN: OpenIn = "beside";

/** openInOf reads the openIn setting: the place it names, or the default when it names none. */
export function openInOf(value: unknown): OpenIn {
  return typeof value === "string" && (OPEN_INS as readonly string[]).includes(value) ? (value as OpenIn) : DEFAULT_OPEN_IN;
}

/** depthOf reads the depth setting: a non-negative whole number, anything else 0, every level. */
export function depthOf(value: unknown): number {
  return typeof value === "number" && Number.isInteger(value) && value > 0 ? value : 0;
}

/** shownOf reads the show setting: the element names it lists, each once, in order; anything else names none. */
export function shownOf(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const names: string[] = [];
  for (const name of value) {
    if (typeof name === "string" && name !== "" && !names.includes(name)) {
      names.push(name);
    }
  }
  return names;
}

/** The palettes the server fills a rendering from, by keyword family, as `internal/ir/view/palette.go` names them. */
export const PALETTES = [
  "okabe-ito",
  "tol-bright",
  "tol-muted",
  "tol-light",
  "brewer-set2",
  "brewer-dark2",
  "viridis",
  "cividis",
] as const;

export type Palette = (typeof PALETTES)[number];

/** The drawing styles the server draws the DOT form in, as `internal/ir/view/style.go` names them; `pilot` is its default. */
export const DRAWING_STYLES = ["pilot", "cameo"] as const;

export type DrawingStyle = (typeof DRAWING_STYLES)[number];

/** The looks: `theme` follows VS Code, `pilot` is the pilot's Standard B&W, `cameo` is Cameo's, a palette is the pilot look filled by family. */
export const STYLES = ["theme", "pilot", "cameo", ...PALETTES] as const;

export type DiagramStyle = (typeof STYLES)[number];

export const DEFAULT_STYLE: DiagramStyle = "theme";

/** How each style is offered to the user. */
export const STYLE_LABELS: Record<DiagramStyle, string> = {
  theme: "VS Code theme",
  pilot: "Pilot (black and white)",
  cameo: "Cameo Systems Modeler",
  "okabe-ito": "Pilot, Okabe–Ito",
  "tol-bright": "Pilot, Tol bright",
  "tol-muted": "Pilot, Tol muted",
  "tol-light": "Pilot, Tol light",
  "brewer-set2": "Pilot, Brewer Set2",
  "brewer-dark2": "Pilot, Brewer Dark2",
  viridis: "Pilot, viridis",
  cividis: "Pilot, cividis",
};

/** isStyle reports whether a value names a style; a setting or saved state may hold anything. */
export function isStyle(value: unknown): value is DiagramStyle {
  return typeof value === "string" && (STYLES as readonly string[]).includes(value);
}

/** styleOf is the style a value names, or the default when it names none. */
export function styleOf(value: unknown): DiagramStyle {
  return isStyle(value) ? value : DEFAULT_STYLE;
}

/** paletteOf is the palette a render request asks for under a style; none under the theme or black and white. */
export function paletteOf(style: DiagramStyle): Palette | undefined {
  return (PALETTES as readonly string[]).includes(style) ? (style as Palette) : undefined;
}

/** drawingStyleOf is the drawing style a render request asks the server for under a style; none under the theme or the pilot's looks. */
export function drawingStyleOf(style: DiagramStyle): DrawingStyle | undefined {
  return style === "cameo" ? "cameo" : undefined;
}

/** pilotLook reports whether a style draws the pilot's look rather than the editor's theme; Cameo's rules ride on it. */
export function pilotLook(style: DiagramStyle): boolean {
  return style !== "theme";
}

/** cameoLook reports whether a style draws Cameo Systems Modeler's look. */
export function cameoLook(style: DiagramStyle): boolean {
  return style === "cameo";
}
