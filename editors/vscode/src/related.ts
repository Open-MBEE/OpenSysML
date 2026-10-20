// The elements a diagram panel draws besides its view's own content: the list a
// render request includes, and the checklist of related elements it is chosen from.
import { RELATED_ELEMENTS_CAPABILITY, RelatedElementsResult, RENDER_INCLUDE_CAPABILITY } from "./protocol";

/** supportsInclude reports whether the server draws a render request's `include`. */
export function supportsInclude(experimental: Record<string, unknown> | undefined): boolean {
  return experimental?.[RENDER_INCLUDE_CAPABILITY] === true;
}

/** supportsRelated reports whether the server answers what relates to an element. */
export function supportsRelated(experimental: Record<string, unknown> | undefined): boolean {
  return Array.isArray(experimental?.[RELATED_ELEMENTS_CAPABILITY]);
}

/** One line of the related-elements checklist: a group heading with its count, or an element checked when the panel includes it. */
export interface RelatedPickItem {
  label: string;
  description?: string;
  detail?: string;
  picked?: boolean;
  /** A relationship kind's heading, shown as a separator; not choosable. */
  heading?: boolean;
  /** The element's qualified name, as `include` takes it; absent on a heading. */
  fqn?: string;
}

/**
 * relatedPickItems lists each group as a heading with its count, then its elements, checked when already included; one the
 * drawing already shows (nested under a drawn node, say) is marked drawn, since including it changes nothing on screen.
 */
export function relatedPickItems(result: RelatedElementsResult, include: readonly string[], drawn: ReadonlySet<string> = new Set()): RelatedPickItem[] {
  const items: RelatedPickItem[] = [];
  for (const group of result.groups) {
    if (group.elements.length === 0) {
      continue;
    }
    items.push({ label: `${group.label} (${group.elements.length})`, heading: true });
    for (const element of group.elements) {
      items.push({
        label: element.name,
        description: drawn.has(element.fqn) && !include.includes(element.fqn) ? `${element.kind} · drawn` : element.kind,
        detail: element.fqn,
        picked: include.includes(element.fqn),
        fqn: element.fqn,
      });
    }
  }
  return items;
}

/** includeAfterPick is the include list once a checklist closes: the listed elements checked are in, the listed ones unchecked are out, the rest stand. */
export function includeAfterPick(include: readonly string[], listed: readonly RelatedPickItem[], picked: readonly RelatedPickItem[]): string[] {
  const shown = new Set(listed.map((item) => item.fqn).filter((fqn): fqn is string => fqn !== undefined));
  const chosen = picked.map((item) => item.fqn).filter((fqn): fqn is string => fqn !== undefined);
  const out = include.filter((fqn) => !shown.has(fqn));
  for (const fqn of chosen) {
    if (!out.includes(fqn)) {
      out.push(fqn);
    }
  }
  return out;
}

/** hiddenFrom is the include list without one element. */
export function hiddenFrom(include: readonly string[], fqn: string): string[] {
  return include.filter((included) => included !== fqn);
}
