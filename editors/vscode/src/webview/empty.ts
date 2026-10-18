// What the panel shows for a rendering with nothing to draw: the server says why
// (the view exposes nothing, its filter rejects everything, the document did not
// analyse cleanly), and the panel puts that where the drawing would be.
import type { RenderResult } from "../protocol";

const NO_REASON = "the rendering has no node, edge or row";

/** emptyMessage is the status line of a rendering with nothing to draw: its first notice, and how many more there are. */
export function emptyMessage(result: RenderResult): string {
  const notices = (result.notices ?? []).filter((notice) => notice !== "");
  if (notices.length === 0) {
    return `Nothing to draw: ${NO_REASON}.`;
  }
  return `Nothing to draw: ${notices[0]}${moreNotices(notices.length - 1)}`;
}

function moreNotices(more: number): string {
  if (more === 0) {
    return "";
  }
  return more === 1 ? " (1 more notice below)" : ` (${more} more notices below)`;
}

/** emptyElement is what stands where the drawing would be: every notice, one per line. */
export function emptyElement(result: RenderResult): HTMLElement {
  const empty = document.createElement("p");
  empty.className = "empty";
  const notices = (result.notices ?? []).filter((notice) => notice !== "");
  empty.textContent = notices.length === 0 ? `Nothing to draw: ${NO_REASON}.` : `Nothing to draw:\n${notices.join("\n")}`;
  return empty;
}

/** nothingLaidOut reports a layout no drawing would show: no node or edge placed, or a canvas without extent. */
export function nothingLaidOut(layout: { roots: unknown[]; edges: unknown[]; width: number; height: number }): boolean {
  return (layout.roots.length === 0 && layout.edges.length === 0) || layout.width <= 0 || layout.height <= 0;
}
