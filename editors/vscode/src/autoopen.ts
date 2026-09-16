// Whether a model file's diagram opens on its own, and how a closed one stays
// closed. Kept free of the vscode module so it is testable under node.

import { isModelLanguage } from "./target";

/** The setting that lets a model file's diagram open on its own. */
export const AUTO_OPEN_SETTING = "opensysml.diagram.autoOpen";

/** The `workspaceState` key under which dismissed documents are kept. */
export const DISMISSED_KEY = "opensysml.diagram.dismissed";

/** What the active editor's tab holds: a plain text document, a diff, or something else. */
export type TabKind = "text" | "diff" | "other" | "none";

/** The active editor, as the auto-open decision sees it. */
export interface ActiveEditor {
  uri: string;
  languageId: string;
  scheme: string;
  /** Whether the editor sits in an editor group; a peek or hover editor does not. */
  inGroup: boolean;
  tab: TabKind;
}

/** Everything the auto-open decision is made from. */
export interface AutoOpenContext {
  /** The `opensysml.diagram.autoOpen` setting. */
  enabled: boolean;
  /** Whether a server that draws diagrams is attached. */
  available: boolean;
  /** Whether the document already has a panel, restored or opened. */
  hasPanel: boolean;
  /** Whether the user closed this document's diagram before. */
  dismissed: boolean;
  editor?: ActiveEditor;
}

/**
 * shouldAutoOpen reports whether the active editor's document gets a diagram
 * without being asked: the setting is on, a server draws, the document has no
 * panel and was not dismissed, and the editor shows a model file from disk in
 * an ordinary text tab.
 */
export function shouldAutoOpen(context: AutoOpenContext): boolean {
  const editor = context.editor;
  if (!context.enabled || !context.available || context.hasPanel || context.dismissed || !editor) {
    return false;
  }
  return editor.scheme === "file" && isModelLanguage(editor.languageId) && editor.inGroup && editor.tab === "text";
}

/** A store of string lists, as `vscode.Memento` offers it. */
export interface Store {
  get(key: string): string[] | undefined;
  update(key: string, value: string[] | undefined): Thenable<void>;
}

/**
 * Dismissals remembers the documents whose diagram the user closed, so it stays
 * closed until Open Diagram is asked for again. Keyed by document URI.
 */
export class Dismissals {
  constructor(private readonly store: Store) {}

  has(uri: string): boolean {
    return this.list().includes(uri);
  }

  /** record keeps the document closed; recording it twice is one entry. */
  record(uri: string): Thenable<void> {
    const list = this.list();
    if (list.includes(uri)) {
      return Promise.resolve();
    }
    return this.store.update(DISMISSED_KEY, [...list, uri]);
  }

  /** clear lets the document's diagram open on its own again. */
  clear(uri: string): Thenable<void> {
    const list = this.list();
    if (!list.includes(uri)) {
      return Promise.resolve();
    }
    const kept = list.filter((entry) => entry !== uri);
    return this.store.update(DISMISSED_KEY, kept.length === 0 ? undefined : kept);
  }

  /** rename carries a dismissal to the document's new name. */
  rename(from: string, to: string): Thenable<void> {
    const list = this.list();
    if (!list.includes(from)) {
      return Promise.resolve();
    }
    const kept = list.filter((entry) => entry !== from && entry !== to);
    return this.store.update(DISMISSED_KEY, [...kept, to]);
  }

  private list(): string[] {
    return this.store.get(DISMISSED_KEY) ?? [];
  }
}

/**
 * Lifecycle tells a panel disposed by the extension — a replacement, a restart,
 * deactivation — from one whose tab the user closed, which is a dismissal.
 */
export class Lifecycle {
  private state: "open" | "disposing" | "closed" = "open";

  get disposed(): boolean {
    return this.state !== "open";
  }

  /** dispose marks a close the extension asked for; false when already closing. */
  dispose(): boolean {
    if (this.state !== "open") {
      return false;
    }
    this.state = "disposing";
    return true;
  }

  /** closed records that the webview is gone, and reports whether the user closed it. */
  closed(): boolean {
    const byUser = this.state === "open";
    this.state = "closed";
    return byUser;
  }
}
