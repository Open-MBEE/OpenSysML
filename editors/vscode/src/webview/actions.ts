// A node's context menu's entries; the edits are offered only on a rendering the
// server sent a palette with, which is what makes it editable.
import { declaredHere, type EditAction, type EditPalette, type RenderNode } from "../protocol";

/** What a menu entry does when chosen. */
export type MenuCommand = EditAction | { kind: "reveal"; id: string } | { kind: "related"; id: string } | { kind: "hide"; id: string };

export interface MenuItem {
  label: string;
  command?: MenuCommand;
  /** Shown but not choosable: the entry says what the node cannot have done to it. */
  disabled?: boolean;
  /** A heading that names what the menu is for; not choosable. */
  heading?: boolean;
  separator?: boolean;
}

/** What the panel can do besides editing: `related` when the server answers what relates to a node. */
export interface MenuOptions {
  related?: boolean;
}

/**
 * nodeMenu: a located node is revealed; a named node on a server answering related elements has them shown, and is hidden
 * again when the request drew it (the view's own content cannot be hidden, so that entry is greyed); one the document
 * declares is also renamed, moved, deleted.
 */
export function nodeMenu(node: RenderNode, palette: EditPalette | undefined, options: MenuOptions = {}): MenuItem[] {
  const items: MenuItem[] = [{ label: node.name || node.kind, heading: true }];
  if (node.origin) {
    items.push({ label: "Go to declaration", command: { kind: "reveal", id: node.id } });
  }
  if (options.related && node.fqn) {
    items.push({ label: "", separator: true });
    items.push({ label: "Show related elements…", command: { kind: "related", id: node.id } });
    items.push({ label: "Hide from diagram", command: { kind: "hide", id: node.id }, disabled: node.included !== true });
  }
  if (declaredHere(node) && palette) {
    items.push({ label: "", separator: true }, { label: "Rename…", command: { kind: "rename", id: node.id } });
    if (node.notation) {
      items.push({ label: "Move to…", command: { kind: "move", id: node.id } });
    }
    items.push({ label: "Delete…", command: { kind: "delete", id: node.id } });
  }
  return items.length > 1 ? items : [];
}
