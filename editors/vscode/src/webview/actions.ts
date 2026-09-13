// The palette's and a node's context menu's entries, built from the palette the
// server sent with the rendering.
import type { EditAction, EditPalette, RenderNode } from "../protocol";

/** What a menu entry does when chosen. */
export type MenuCommand = EditAction | { kind: "reveal"; id: string };

export interface MenuItem {
  label: string;
  command?: MenuCommand;
  /** A heading that names what the menu is for; not choosable. */
  heading?: boolean;
  separator?: boolean;
}

/** paletteItems are the "Add …" entries of the toolbar, members then connections. */
export function paletteItems(palette: EditPalette): MenuItem[] {
  return [
    ...palette.members.map((memberKind) => ({
      label: `Add ${memberKind}…`,
      command: { kind: "addMember", memberKind, typed: palette.typed.includes(memberKind) } as const,
    })),
    ...palette.connections.map((connectionKind) => ({
      label: `Add ${connectionKind}…`,
      command: { kind: "addConnection", connectionKind } as const,
    })),
  ];
}

/** nodeMenu: a located node is revealed; a declared one is also added to, connected from, renamed, deleted. */
export function nodeMenu(node: RenderNode, palette: EditPalette | undefined): MenuItem[] {
  const items: MenuItem[] = [{ label: node.name || node.kind, heading: true }];
  if (node.origin) {
    items.push({ label: "Go to declaration", command: { kind: "reveal", id: node.id } });
  }
  if (node.fqn && palette) {
    if (palette.members.length > 0) {
      items.push({ label: "", separator: true });
      for (const memberKind of palette.members) {
        items.push({
          label: `Add ${memberKind}…`,
          command: { kind: "addMember", memberKind, typed: palette.typed.includes(memberKind), owner: node.id },
        });
      }
    }
    if (palette.connections.length > 0) {
      items.push({ label: "", separator: true });
      for (const connectionKind of palette.connections) {
        items.push({
          label: `${capitalize(connectionKind)} from here…`,
          command: { kind: "addConnection", connectionKind, from: node.id },
        });
      }
    }
    items.push(
      { label: "", separator: true },
      { label: "Rename…", command: { kind: "rename", id: node.id } },
      { label: "Delete…", command: { kind: "delete", id: node.id } },
    );
  }
  return items.length > 1 ? items : [];
}

function capitalize(word: string): string {
  return word.charAt(0).toUpperCase() + word.slice(1);
}
