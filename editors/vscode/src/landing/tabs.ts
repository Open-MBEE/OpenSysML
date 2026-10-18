/** The tab set over the hero's scenes: one `[role=tab][data-osml-scene]` button per scene. */
export interface TabSet {
  /** Marks the tab for `id` selected, moving focusability to it; the others step aside. */
  select(id: string): void;
  /** The ids of the tabs, in document order. */
  ids(): string[];
}

/**
 * wireTabs makes the tabs under `tablist` an accessible tab set with automatic activation:
 * a click or an arrow key (Left/Right, Home/End wrap and jump) selects a tab and reports it
 * to `onSelect`; `aria-selected` and roving `tabindex` follow the selection.
 */
export function wireTabs(
  tablist: HTMLElement,
  selected: string,
  onSelect: (id: string) => void,
  signal?: AbortSignal,
): TabSet {
  const tabs = [...tablist.querySelectorAll<HTMLElement>('[role="tab"][data-osml-scene]')];
  const idOf = (tab: HTMLElement): string => tab.dataset.osmlScene ?? "";
  const select = (id: string): void => {
    for (const tab of tabs) {
      const on = idOf(tab) === id;
      tab.setAttribute("aria-selected", String(on));
      tab.tabIndex = on ? 0 : -1;
      const panel = tab.getAttribute("aria-controls");
      if (on && tab.id && panel) {
        tab.ownerDocument.getElementById(panel)?.setAttribute("aria-labelledby", tab.id);
      }
    }
  };
  const activate = (tab: HTMLElement, focus: boolean): void => {
    select(idOf(tab));
    if (focus) {
      tab.focus();
    }
    onSelect(idOf(tab));
  };
  tabs.forEach((tab, index) => {
    tab.addEventListener("click", () => activate(tab, false), { signal });
    tab.addEventListener(
      "keydown",
      (event) => {
        const last = tabs.length - 1;
        const next: Record<string, number> = {
          ArrowRight: index === last ? 0 : index + 1,
          ArrowLeft: index === 0 ? last : index - 1,
          Home: 0,
          End: last,
        };
        const to = next[event.key];
        if (to === undefined) {
          return;
        }
        event.preventDefault();
        activate(tabs[to], true);
      },
      { signal },
    );
  });
  select(selected);
  return { select, ids: () => tabs.map(idOf) };
}
