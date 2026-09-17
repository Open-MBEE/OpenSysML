// Which document a diagram command acts on, decided from what has focus. Kept
// free of the vscode module so it is testable under node.

/** The language ids the diagram commands accept. */
export const MODEL_LANGUAGES = ["sysml", "kerml"] as const;

/** The diagram panel's view type: what restores it, and its `activeWebviewPanelId`. */
export const PANEL_TYPE = "opensysml.diagram";

/** An open text editor, as the commands see it. */
export interface EditorInfo {
  uri: string;
  languageId: string;
}

/** Everything a diagram command may take its document from. */
export interface CommandContext {
  /** The resource a menu entry was invoked on, if any. */
  argument?: string;
  /** The document of the diagram panel that has focus, if one does. */
  focusedPanel?: string;
  activeEditor?: EditorInfo;
  visibleEditors: EditorInfo[];
}

/** The document a diagram command acts on. */
export type Target =
  | { kind: "document"; uri: string; fromPanel: boolean }
  | { kind: "none"; message: string };

/** isModelLanguage reports whether the language id is one the diagram draws. */
export function isModelLanguage(languageId: string): boolean {
  return (MODEL_LANGUAGES as readonly string[]).includes(languageId);
}

/** isModelPath reports whether the path ends in a model file extension. */
export function isModelPath(path: string): boolean {
  return /\.(sysml|kerml)$/i.test(path);
}

/**
 * resolveTarget picks the document for `verb` (e.g. "draw a diagram of"):
 * the resource a menu was invoked on, else the focused diagram panel's, else
 * the active model editor's, else the one visible model editor's.
 */
export function resolveTarget(context: CommandContext, verb: string): Target {
  if (context.argument !== undefined) {
    if (isModelPath(context.argument)) {
      return { kind: "document", uri: context.argument, fromPanel: false };
    }
    return { kind: "none", message: `${fileLabel(context.argument)} is not a .sysml or .kerml file.` };
  }
  if (context.focusedPanel !== undefined) {
    return { kind: "document", uri: context.focusedPanel, fromPanel: true };
  }
  if (context.activeEditor && isModelLanguage(context.activeEditor.languageId)) {
    return { kind: "document", uri: context.activeEditor.uri, fromPanel: false };
  }
  const visible = context.visibleEditors.filter((editor) => isModelLanguage(editor.languageId));
  const distinct = new Set(visible.map((editor) => editor.uri));
  if (distinct.size === 1) {
    return { kind: "document", uri: visible[0].uri, fromPanel: false };
  }
  if (distinct.size > 1) {
    return { kind: "none", message: `Several .sysml or .kerml files are open; focus the one to ${verb}.` };
  }
  return { kind: "none", message: `Open a .sysml or .kerml file to ${verb}.` };
}

// fileLabel is the last path segment of a URI or path, for messages.
function fileLabel(uri: string): string {
  const path = uri.replace(/[?#].*$/, "");
  const parts = path.split("/");
  return decodeURIComponent(parts[parts.length - 1] || uri);
}
