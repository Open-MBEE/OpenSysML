import type { PickerEntry, Position, Range, ViewsResult } from "./protocol";

/** A declared view as the picker offers it, with what choosing it on open needs. */
export interface DeclaredView extends PickerEntry {
  kind: string;
  /** Where the view is declared; absent from servers that do not locate views. */
  range?: Range;
}

/** The quick pick's entry that opens every drawable view at once. */
export const ALL_VIEWS = "*";

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
export function declaredViewEntries(listing: ViewsResult | undefined): DeclaredView[] {
  return (listing?.views ?? []).map((info) => ({
    value: info.name,
    label: `${info.name} — ${info.kind}`,
    kind: info.kind,
    supported: info.supported,
    reason: info.reason,
    range: info.range,
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

/** viewAt is the drawable view whose declaration contains the cursor, if any. */
export function viewAt(declared: DeclaredView[], cursor: Position | undefined): string | undefined {
  if (!cursor) {
    return undefined;
  }
  return declared.find((entry) => entry.supported && entry.range !== undefined && contains(entry.range, cursor))?.value;
}

function contains(range: Range, at: Position): boolean {
  const afterStart = at.line > range.start.line || (at.line === range.start.line && at.character >= range.start.character);
  const beforeEnd = at.line < range.end.line || (at.line === range.end.line && at.character <= range.end.character);
  return afterStart && beforeEnd;
}

/**
 * rememberedView is the view last chosen for the document when it still exists:
 * a drawable declared view or a pseudo-view the server still offers. A remembered
 * view that is gone is undefined, so its record is dropped.
 */
export function rememberedView(
  declared: DeclaredView[],
  pseudo: PickerEntry[],
  remembered: string | undefined,
): string | undefined {
  if (!remembered) {
    return undefined;
  }
  const exists = [...declared.filter((entry) => entry.supported), ...pseudo].some((entry) => entry.value === remembered);
  return exists ? remembered : undefined;
}

/**
 * How Open Diagram settles on a view: the view to draw, or that it must ask.
 * `stale` is a remembered view that no longer exists, to be forgotten.
 */
export type ViewChoice = { view: string; stale?: undefined } | { view?: undefined; stale: string | undefined };

/**
 * chooseView is the view Open Diagram draws without asking: the one a document
 * with at most one drawable view implies, else the one under the cursor, else the
 * one last chosen for the document. Nothing when only the user can decide.
 */
export function chooseView(
  declared: DeclaredView[],
  pseudo: PickerEntry[],
  cursor: Position | undefined,
  remembered: string | undefined,
): ViewChoice {
  const implied = impliedView(declared);
  if (implied !== undefined) {
    return { view: implied };
  }
  const atCursor = viewAt(declared, cursor);
  if (atCursor !== undefined) {
    return { view: atCursor };
  }
  const kept = rememberedView(declared, pseudo, remembered);
  if (kept !== undefined) {
    return { view: kept };
  }
  return { stale: remembered };
}

/** One entry of the quick pick Open Diagram asks with. */
export interface ViewPickItem {
  label: string;
  detail?: string;
  description?: string;
  /** The view, or ALL_VIEWS. */
  value: string;
}

/**
 * viewPickItems lists what Open Diagram asks between: the drawable declared views
 * (name, then kind), "All views", and the pseudo-views last. A view that cannot be
 * drawn is left out; the panel's own picker still lists it with the reason.
 */
export function viewPickItems(declared: DeclaredView[], pseudo: PickerEntry[]): ViewPickItem[] {
  const drawable = declared.filter((entry) => entry.supported);
  return [
    ...drawable.map((entry) => ({ label: entry.value, detail: entry.kind, value: entry.value })),
    { label: "All views", detail: `Open the ${drawable.length} drawable views, one panel each`, value: ALL_VIEWS },
    ...pseudo.map((entry) => ({ label: entry.label, description: entry.value, value: entry.value })),
  ];
}

/** expandChoice is the views a pick opens: every drawable declared view for ALL_VIEWS. */
export function expandChoice(picked: string, declared: DeclaredView[]): string[] {
  if (picked !== ALL_VIEWS) {
    return [picked];
  }
  return declared.filter((entry) => entry.supported).map((entry) => entry.value);
}

/** panelKey identifies a panel by the document and view it draws. */
export function panelKey(uri: string, view: string): string {
  return JSON.stringify([uri, view]);
}

/** viewTitle is how a view is named in a panel title: its short name, or the pseudo-view spec. */
export function viewTitle(view: string): string {
  if (view.startsWith("#")) {
    return view;
  }
  const parts = view.split("::");
  return parts[parts.length - 1];
}
