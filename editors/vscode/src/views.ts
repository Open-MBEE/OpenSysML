import type { PickerEntry, ViewsResult } from "./protocol";

const PSEUDO_VIEW_LABELS: Record<string, string> = {
  tree: "Model tree",
  interconnection: "Interconnections",
  state: "State machines",
  action: "Action flows",
  table: "Element table",
  sequence: "Message sequence",
};

// This is the historical set for servers that predate the pseudoViews field; do not grow it.
const HISTORICAL_PSEUDO_VIEWS = ["#tree", "#interconnection", "#state", "#action", "#table"];

/** The pseudo-view a document declaring no drawable view is rendered as. */
export const DEFAULT_PSEUDO_VIEW = "#tree";

/** declaredViewEntries is the picker's entries for the views a document declares. */
export function declaredViewEntries(listing: ViewsResult | undefined): PickerEntry[] {
  return (listing?.views ?? []).map((info) => ({
    value: info.name,
    label: `${info.name} — ${info.kind}`,
    supported: info.supported,
    reason: info.reason,
  }));
}

// Pseudo-views are always offered because a document being written usually declares no view.
export function pseudoViewEntries(specs: string[] | undefined): PickerEntry[] {
  return (specs ?? HISTORICAL_PSEUDO_VIEWS).map((value) => {
    const kind = value.startsWith("#") ? value.slice(1) : value;
    return {
      value,
      label: `${PSEUDO_VIEW_LABELS[kind] ?? kind} (no view declared)`,
      supported: true,
    };
  });
}

/**
 * impliedView is the view a document is rendered as when none is named: its one
 * drawable view, or the model tree when it declares none. Undefined when several
 * are drawable, which only the user can choose between.
 */
export function impliedView(declared: PickerEntry[]): string | undefined {
  const drawable = declared.filter((entry) => entry.supported);
  switch (drawable.length) {
    case 0:
      return DEFAULT_PSEUDO_VIEW;
    case 1:
      return drawable[0].value;
    default:
      return undefined;
  }
}
