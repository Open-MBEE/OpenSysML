// The look a diagram is drawn in: the editor's theme, the pilot visualizer's black and
// white, or that look filled from a server palette. Free of the vscode module for the webview.

/** The setting that picks the look every diagram panel is drawn in. */
export const STYLE_SETTING = "opensysml.diagram.style";

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

/** The looks: `theme` follows VS Code, `pilot` is the pilot's Standard B&W, a palette is that look filled by family. */
export const STYLES = ["theme", "pilot", ...PALETTES] as const;

export type DiagramStyle = (typeof STYLES)[number];

export const DEFAULT_STYLE: DiagramStyle = "theme";

/** How each style is offered to the user. */
export const STYLE_LABELS: Record<DiagramStyle, string> = {
  theme: "VS Code theme",
  pilot: "Pilot (black and white)",
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

/** pilotLook reports whether a style draws the pilot's look rather than the editor's theme. */
export function pilotLook(style: DiagramStyle): boolean {
  return style !== "theme";
}
