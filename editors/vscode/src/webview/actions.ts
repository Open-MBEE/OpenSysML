// A node's context menu's entries; the edits are offered only on a rendering the
// server sent a palette with, which is what makes it editable.
import { declaredHere, type EditAction, type EditPalette, type RenderNode } from "../protocol";

/** What a menu entry does when chosen. */
export type MenuCommand = EditAction | { kind: "reveal"; id: string };

export interface MenuItem {
  label: string;
  command?: MenuCommand;
  /** A heading that names what the menu is for; not choosable. */
  heading?: boolean;
  separator?: boolean;
}

/** nodeMenu: a located node is revealed; one the document declares is also renamed, moved, deleted. */
export function nodeMenu(node: RenderNode, palette: EditPalette | undefined): MenuItem[] {
  const items: MenuItem[] = [{ label: node.name || node.kind, heading: true }];
  if (node.origin) {
    items.push({ label: "Go to declaration", command: { kind: "reveal", id: node.id } });
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
