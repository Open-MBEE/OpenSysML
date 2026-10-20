// The palette's and a node's context menu's entries, built from the palette the
// server sent with the rendering.
import { admits, declaredHere, type EditAction, type EditPalette, type RenderNode } from "../protocol";

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

/** paletteItems are the "Add …" entries of the toolbar, members then connections; a member no drawn node may own is left out. */
export function paletteItems(palette: EditPalette): MenuItem[] {
  return [
    ...palette.members.filter((memberKind) => (palette.owners?.[memberKind]?.length ?? 1) > 0).map((memberKind) => ({
      label: `Add ${memberKind}…`,
      command: { kind: "addMember", memberKind, typed: palette.typed.includes(memberKind) } as const,
    })),
    ...palette.connections.map((connectionKind) => ({
      label: `Add ${connectionKind}…`,
      command: { kind: "addConnection", connectionKind } as const,
    })),
  ];
}

/** What the panel can do besides editing: `related` when the server answers what relates to a node. */
export interface MenuOptions {
  related?: boolean;
}

/**
 * nodeMenu: a located node is revealed; a named node on a server answering related elements has them shown, and is hidden
 * again when the request drew it (the view's own content cannot be hidden, so that entry is greyed); one the document
 * declares is also added to, connected from, renamed, moved, deleted.
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
    const members = palette.members.filter((memberKind) => admits(palette, memberKind, node));
    if (members.length > 0) {
      items.push({ label: "", separator: true });
      for (const memberKind of members) {
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
    items.push({ label: "", separator: true }, { label: "Rename…", command: { kind: "rename", id: node.id } });
    if (node.notation) {
      items.push({ label: "Move to…", command: { kind: "move", id: node.id } });
    }
    items.push({ label: "Delete…", command: { kind: "delete", id: node.id } });
  }
  return items.length > 1 ? items : [];
}

function capitalize(word: string): string {
  return word.charAt(0).toUpperCase() + word.slice(1);
}
