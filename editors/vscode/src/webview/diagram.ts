// The diagram panel's script: draws the server's rendering on an SVG canvas, reports
// clicks, menu choices and drags back, and highlights the cursor's node.
import {
  normalizeRender,
  nothingToDraw,
  type FromWebview,
  type PickerEntry,
  type RenderNode,
  type RenderPoint,
  type RenderResult,
  type ToWebview,
} from "../protocol";
import { cameoLook, type DiagramStyle, pilotLook, type Routing, routingOf, STYLE_LABELS, STYLES, styleOf } from "../style";
import { MenuCommand, nodeMenu } from "./actions";
import { autoLayout, type AutoLayout } from "./autolayout";
import { loadAvoid, reportAvoidFailure } from "./avoid";
import { cssEscape, drawCanvas, liftNode } from "./canvas";
import { dragHint, Drop, dropOn } from "./drop";
import { emptyElement, emptyMessage, nothingLaidOut } from "./empty";
import { markSelection, neighbourhood, relatedByKind } from "./selection";
import { tableOf } from "./table";
import {
  canvasSize,
  type ClientPoint,
  fitZoom,
  readViewport,
  sizeCanvas,
  type Viewport,
  wheelFactor,
  ZOOM_STEP,
  zoomAbout,
  zoomLabel,
} from "./viewport";
import {
  CanvasLayout,
  insertedWaypoint,
  layoutCanvas,
  movable,
  movedNode,
  movedWaypoint,
  nodeUnder,
  overridesOf,
  type Overrides,
  Placements,
  removedWaypoint,
} from "./layout";

interface WebviewApi {
  postMessage(message: FromWebview): void;
  setState(state: unknown): void;
  getState(): unknown;
}

declare function acquireVsCodeApi(): WebviewApi;

const vscode = acquireVsCodeApi();
const body = document.body;
const picker = document.getElementById("view") as HTMLSelectElement;
const styler = document.getElementById("style") as HTMLSelectElement;
const kindLabel = document.getElementById("kind") as HTMLElement;
const status = document.getElementById("status") as HTMLElement;
const diagram = document.getElementById("diagram") as HTMLElement;
const notices = document.getElementById("notices") as HTMLDetailsElement;
const noticeList = document.getElementById("notice-list") as HTMLElement;
const undrawable = document.getElementById("undrawable") as HTMLDetailsElement;
const undrawableList = document.getElementById("undrawable-list") as HTMLElement;
const menu = document.getElementById("menu") as HTMLUListElement;
const zoomReadout = document.getElementById("zoom") as HTMLElement;
const autoButton = document.getElementById("auto-layout") as HTMLButtonElement;
const resetButton = document.getElementById("reset-layout") as HTMLButtonElement;
const relations = document.getElementById("relations") as HTMLDetailsElement;
const relationList = document.getElementById("relation-list") as HTMLElement;

const state = JSON.parse(body.dataset.state ?? "{}") as { uri?: string; avoid?: string };
const documentURI = state.uri ?? "";
const saved = (vscode.getState() ?? {}) as { view?: string; last?: RenderResult; style?: string; routing?: string; viewport?: unknown; hidden?: string[] };
let selected = saved.view ?? "";
// A rendering saved by an older extension is normalized like a fresh one, and
// drawn in the look it was, or the default when it saved none.
let last: RenderResult | undefined = saved.last === undefined ? undefined : normalizeRender(saved.last);
let style: DiagramStyle = styleOf(saved.style);
let routing: Routing = routingOf(saved.routing);
let selectedNode: string | undefined;
/** How far the canvas is zoomed and where it is scrolled to; kept per panel across reloads. */
let viewport: Viewport = readViewport(saved.viewport);
/** The node selected by a click, whose neighbourhood is emphasised; undefined when nothing is. */
let focused: string | undefined;
/** How many applyViewport scrolls are still settling; while any is, the scroll listener leaves recording to them. */
let settling = 0;
/** The relationship kinds this panel hides, once its filter has been set; until then the workspace default applies. */
let hiddenKinds: Set<string> | undefined = Array.isArray(saved.hidden) ? new Set(saved.hidden.filter((kind) => typeof kind === "string")) : undefined;
/** The kinds the `hideRelations` setting hides, from the latest rendering. */
let defaultHidden: string[] = [];
/** The layout on screen, which gestures act on; undefined while a table or nothing is shown. */
let layout: CanvasLayout | undefined;
/** What ELK placed for the rendering on screen; undefined until it answers, and for kinds it does not lay out. */
let auto: AutoLayout | undefined;
/** ELK's layout of the rendering free of the model's placements, while the Auto layout command is in force. */
let freeAuto: AutoLayout | undefined;
let gesture: Gesture | undefined;
// The router loaded or the routing changed mid-gesture, so the layout on screen predates it.
let unrouted = false;
/** How far the pointer moves before a press becomes a drag rather than a click. */
const DRAG_THRESHOLD = 3;
/** How soon a second click on a waypoint must follow the first to remove it. */
const DOUBLE_CLICK_MS = 400;
// The waypoint last clicked; the pointer is captured while pressed, so a dblclick
// event would name the container rather than the handle, and clicks are paired here.
let clickedWaypoint: { edge: number; point: number; at: number } | undefined;
// The extension's number for the drawing shown; an action names the drawing its ids came from.
let drawn = 0;

fillStyles();
applyStyle(style);

// The panel is torn down while it is hidden, so the rendering it last drew is
// put back — dimmed until the server answers — rather than showing nothing.
if (last) {
  draw(last);
  diagram.classList.add("stale");
}

// The WASM router loads after the first draw, so edges it would route stay
// straight until it answers; a drawn mermaid rendering is laid out again then.
loadAvoid(state.avoid ?? "").then(
  () => {
    if (gesture) {
      unrouted = true;
      return;
    }
    if (last && layout) {
      layout = layoutCanvas(last, panelOverrides(), currentAuto());
      show(layout);
    }
  },
  (error: unknown) => reportAvoidFailure("failed to load", error),
);

// The look changes at once; the extension keeps the choice and renders for its palette.
styler.addEventListener("change", () => {
  applyStyle(styleOf(styler.value));
  vscode.postMessage({ type: "style", style });
});

picker.addEventListener("change", () => {
  selected = picker.value;
  remember();
  vscode.postMessage({ type: "pick", view: selected });
});

window.addEventListener("click", () => hideMenu());
window.addEventListener("blur", () => hideMenu());
window.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    hideMenu();
    cancelGesture();
    select(undefined);
  } else if (event.key === "Shift" && !event.repeat) {
    drawDrag(true);
  } else if (!event.ctrlKey && !event.metaKey && !event.altKey && !inControl(event.target)) {
    // Bare +, - and 0 zoom the canvas; with a modifier they are the editor's own zoom.
    switch (event.key) {
      case "+":
      case "=":
        zoomBy(ZOOM_STEP);
        break;
      case "-":
        zoomBy(1 / ZOOM_STEP);
        break;
      case "0":
        resetZoom();
        break;
      default:
        return;
    }
    event.preventDefault();
  }
});
// Ctrl+wheel, which a trackpad pinch arrives as, zooms about the pointer; a plain
// wheel scrolls the canvas as any overflowing element.
diagram.addEventListener("wheel", (event) => {
  if (!(event.ctrlKey || event.metaKey) || !layout) {
    return;
  }
  event.preventDefault();
  const delta = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? event.deltaY * WHEEL_LINE_PX : event.deltaY;
  setViewport(zoomAbout(scrolled(), wheelFactor(delta), clientPoint(event)));
}, { passive: false });
// Scrolling, by wheel or scrollbar, is where the viewport is; it is remembered so a reload comes back there.
diagram.addEventListener("scroll", () => {
  // The scroll applyViewport asked for is not news; what it settled on is read once the layout has.
  if (settling > 0) {
    return;
  }
  viewport = scrolled();
  rememberSoon();
});
document.getElementById("zoom-in")?.addEventListener("click", () => zoomBy(ZOOM_STEP));
document.getElementById("zoom-out")?.addEventListener("click", () => zoomBy(1 / ZOOM_STEP));
document.getElementById("zoom-fit")?.addEventListener("click", () => fitCanvas());
document.getElementById("zoom-reset")?.addEventListener("click", () => resetZoom());
autoButton.addEventListener("click", () => autoLayoutPanel());
resetButton.addEventListener("click", () => resetLayout());
window.addEventListener("keyup", (event) => {
  if (event.key === "Shift") {
    drawDrag(false);
  }
});
// A right-click off a node offers nothing; the browser's own menu offers less.
diagram.addEventListener("contextmenu", (event) => event.preventDefault());

// A table has no gestures; a click on a row holding a declaration opens it as a
// node click does.
diagram.addEventListener("click", (event) => {
  if (layout || !last?.rows) {
    return;
  }
  const row = (event.target as Element | null)?.closest?.<HTMLTableRowElement>("tr.located[data-opensysml-row]");
  if (row?.dataset.opensysmlRow !== undefined) {
    vscode.postMessage({ type: "revealRow", row: Number(row.dataset.opensysmlRow), drawn });
  }
});

window.addEventListener("message", (event: MessageEvent<ToWebview>) => {
  // The extension posts into this frame, so its messages carry the frame's own
  // origin; anything from elsewhere is not the extension and is dropped.
  if (event.origin !== window.origin) {
    return;
  }
  const message = event.data;
  switch (message.type) {
    case "views":
      fillPicker(message.views, message.selected);
      return;
    case "render":
      // The number names what is on screen; a drawing that failed left the last one up.
      applyStyle(message.style);
      applyRouting(message.routing);
      defaultHidden = message.hideRelations ?? [];
      if (draw(message.result)) {
        drawn = message.drawn;
        if (message.hint !== undefined) {
          // A rendering with nothing to draw keeps its reason; the hint follows it.
          if (emptyReason !== undefined) {
            showStatus(`${emptyReason}\n${message.hint}`);
          } else {
            showHint(message.hint);
          }
        }
      }
      return;
    case "style":
      applyStyle(message.style);
      return;
    case "routing":
      applyRouting(message.routing);
      if (last && layout && !gesture) {
        layout = layoutCanvas(last, panelOverrides(), currentAuto());
        show(layout);
      }
      return;
    case "error":
      showError(message.message);
      return;
    case "highlight":
      highlight(message.id);
      return;
    case "revert":
      revert(message.message);
      return;
  }
});

vscode.postMessage({ type: "ready" });

// fillPicker lists the document's views and the pseudo-views, marking the ones
// whose rendering kind the server does not produce with why.
function fillPicker(views: PickerEntry[], pick: string): void {
  selected = pick;
  remember();
  picker.replaceChildren();
  for (const entry of views) {
    const option = document.createElement("option");
    option.value = entry.value;
    option.textContent = entry.supported ? entry.label : `${entry.label} (not drawable)`;
    option.disabled = !entry.supported;
    if (entry.reason) {
      option.title = entry.reason;
    }
    picker.append(option);
  }
  picker.value = pick;
  showUndrawable(views);
}

// fillStyles lists the looks the diagram can be drawn in.
function fillStyles(): void {
  styler.replaceChildren();
  for (const entry of STYLES) {
    const option = document.createElement("option");
    option.value = entry;
    option.textContent = STYLE_LABELS[entry];
    styler.append(option);
  }
}

// applyStyle draws what is on screen in a look: the pilot's rules take over from the
// editor's theme under every look but `theme`, Cameo's ride on them, and a palette's fills ride on each shape.
function applyStyle(chosen: DiagramStyle): void {
  style = chosen;
  styler.value = chosen;
  diagram.classList.toggle("pilot", pilotLook(chosen));
  diagram.classList.toggle("cameo", cameoLook(chosen));
  remember();
}

// applyRouting picks whose routes the edges follow; the caller redraws where a drawing is
// up, unless a gesture holds the screen, in which case the gesture's end does.
function applyRouting(chosen: Routing): void {
  if (chosen === routing) {
    return;
  }
  routing = chosen;
  if (gesture) {
    unrouted = true;
  }
  remember();
}

// showUndrawable says why a listed view is not drawable, since the picker only
// says that it is not and holds the reason in a tooltip.
function showUndrawable(views: PickerEntry[]): void {
  const listed = views.filter((entry) => !entry.supported);
  undrawableList.replaceChildren();
  undrawable.hidden = listed.length === 0;
  if (listed.length === 0) {
    return;
  }
  (undrawable.firstElementChild as HTMLElement).textContent =
    listed.length === 1 ? "1 view not drawable" : `${listed.length} views not drawable`;
  for (const entry of listed) {
    const item = document.createElement("li");
    // The reason names the view itself, so it stands alone; the label is what is
    // left to say when the server gave no reason.
    item.textContent = entry.reason ?? entry.label;
    undrawableList.append(item);
  }
}

// draw replaces the diagram with the rendering and reports whether it did. A failure
// to draw leaves the last diagram up, dimmed, so a mid-keystroke parse error does not
// blank the panel.
/** The status line of a rendering with nothing to draw, while it is the one on screen. */
let emptyReason: string | undefined;

function draw(result: RenderResult): boolean {
  unrouted = false;
  cancelGesture();
  freeAuto = undefined;
  showLayoutMode();
  if (focused !== undefined && !result.nodes?.some((node) => node.id === focused)) {
    focused = undefined;
  }
  try {
    // A rendering with nothing to draw says why, where the drawing would be and
    // in the status line, rather than drawing an empty canvas.
    const empty = nothingToDraw(result);
    if (result.form === "mermaid") {
      // Mermaid is the machine form a diagram is exported in; the panel draws
      // the same nodes and edges itself, so their geometry is its own to edit.
      auto = undefined;
      layout = layoutCanvas(result, panelOverrides());
      if (empty || nothingLaidOut(layout)) {
        layout = undefined;
        diagram.replaceChildren(emptyElement(result));
      } else {
        show(layout);
      }
    } else {
      // A table is not drawn as geometry: it is drawn as a table from its rows.
      layout = undefined;
      diagram.replaceChildren(tableOf(result));
    }
    diagram.classList.remove("stale");
    emptyReason = empty || (result.form === "mermaid" && layout === undefined) ? emptyMessage(result) : undefined;
    showStatus(emptyReason ?? "");
    last = result;
    if (layout !== undefined) {
      // The grid answers at once; ELK's layered layout replaces it when it resolves.
      showStatus("Laying out…");
      void autoLayout(result).then((laid) => {
        if (last !== result) {
          return;
        }
        showStatus("");
        if (!laid) {
          return;
        }
        auto = laid;
        if (freeAuto) {
          return;
        }
        layout = layoutCanvas(result, panelOverrides(), auto);
        show(layout);
      });
    }
    remember();
    showNotices(result);
    // An open menu names nodes of the drawing just replaced.
    hideMenu();
    showRelations(result);
    kindLabel.textContent = describe(result);
    return true;
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    showError(message);
    vscode.postMessage({ type: "failed", message });
    return false;
  }
}

// show puts a layout on screen in place of what was there, the cursor's node marked.
function show(shown: CanvasLayout): void {
  diagram.replaceChildren(drawCanvas(shown));
  applyViewport();
  highlight(selectedNode);
  applySelection();
}

// select makes a node the selection, or clears it, and shows the result.
function select(id: string | undefined): void {
  if (id === focused) {
    return;
  }
  focused = id;
  applySelection();
}

// applySelection marks the selection's neighbourhood on the canvas on screen.
function applySelection(): void {
  const svg = diagram.querySelector("svg");
  if (!svg || !last) {
    return;
  }
  markSelection(svg, focused === undefined ? undefined : neighbourhood(last, focused, hiddenSet()));
  countRelations(last);
}

// hiddenSet is the relationship kinds hidden now: the panel's own filter, else the workspace default.
function hiddenSet(): Set<string> {
  return hiddenKinds ?? new Set(defaultHidden);
}

// panelOverrides is what the panel itself lays over every layout of the rendering; a
// gesture in progress keeps the routing it began under, and its end lays out with the new one.
function panelOverrides(): Overrides {
  return { hiddenKinds: hiddenSet(), unplaced: freeAuto !== undefined, routing: gesture?.routing ?? routing };
}

// currentAuto is the ELK layout in force: the free one while Auto layout is, else the one held to the model.
function currentAuto(): AutoLayout | undefined {
  return freeAuto ?? auto;
}

// autoLayoutPanel lays the rendering on screen out afresh with ELK, the model's Layout and
// Route annotations set aside for this panel until the next rendering or Reset to model.
function autoLayoutPanel(): void {
  const result = last;
  if (!result || !layout || gesture) {
    return;
  }
  showStatus("Laying out…");
  void autoLayout(result, { free: true }).then((laid) => {
    if (last !== result) {
      return;
    }
    showStatus(laid ? "" : "Auto layout is not available for this diagram.");
    if (!laid || !layout) {
      return;
    }
    freeAuto = laid;
    layout = layoutCanvas(result, panelOverrides(), laid);
    show(layout);
    showLayoutMode();
  });
}

// resetLayout puts the model's layout back after Auto layout.
function resetLayout(): void {
  if (freeAuto === undefined) {
    return;
  }
  freeAuto = undefined;
  showLayoutMode();
  if (last && layout) {
    layout = layoutCanvas(last, panelOverrides(), auto);
    show(layout);
  }
}

// showLayoutMode offers Reset to model only while Auto layout is in force.
function showLayoutMode(): void {
  resetButton.hidden = freeAuto === undefined;
  autoButton.disabled = freeAuto !== undefined;
}

// showRelations lists the relationship kinds the rendering draws, each with a box to
// show or hide it; a rendering with no edges, or a table, offers no list.
function showRelations(result: RenderResult): void {
  const kinds = new Map<string, number>();
  for (const edge of result.edges ?? []) {
    kinds.set(edge.kind, (kinds.get(edge.kind) ?? 0) + 1);
  }
  relations.hidden = result.form !== "mermaid" || kinds.size === 0;
  const hidden = hiddenSet();
  relationList.replaceChildren(...[...kinds.keys()].sort().map((kind) => {
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = !hidden.has(kind);
    box.dataset.kind = kind;
    const count = document.createElement("span");
    count.className = "count";
    const label = document.createElement("label");
    label.append(box, ` ${kind} `, count);
    return label;
  }));
  countRelations(result);
}

// countRelations writes each kind's count after its name: how many edges, and how many at the selected node.
function countRelations(result: RenderResult): void {
  const related = focused === undefined ? undefined : relatedByKind(result, focused);
  const total = new Map<string, number>();
  for (const edge of result.edges ?? []) {
    total.set(edge.kind, (total.get(edge.kind) ?? 0) + 1);
  }
  for (const box of relationList.querySelectorAll<HTMLInputElement>("input[data-kind]")) {
    const kind = box.dataset.kind ?? "";
    const count = box.parentElement?.querySelector(".count");
    if (count) {
      const at = related?.get(kind) ?? 0;
      count.textContent = related === undefined ? String(total.get(kind) ?? 0) : `${at} of ${total.get(kind) ?? 0} related`;
    }
  }
  relations.classList.toggle("filtered", [...hiddenSet()].some((kind) => total.has(kind)));
}

// A box toggled hides or shows its kind for this panel alone, and lays the rendering out again without it.
relationList.addEventListener("change", (event) => {
  const box = event.target as HTMLInputElement | null;
  const kind = box?.dataset.kind;
  if (!box || kind === undefined) {
    return;
  }
  const hidden = new Set(hiddenSet());
  if (box.checked) {
    hidden.delete(kind);
  } else {
    hidden.add(kind);
  }
  hiddenKinds = hidden;
  remember();
  if (last && layout) {
    layout = layoutCanvas(last, panelOverrides(), currentAuto());
    show(layout);
  }
});
// The list closes when the pointer goes elsewhere, as a menu does.
window.addEventListener("click", (event) => {
  if (!relations.contains(event.target as Node)) {
    relations.open = false;
  }
});

/** A pixel per line of a wheel that reports lines rather than pixels. */
const WHEEL_LINE_PX = 16;

// inControl is whether a key went to a toolbar control, whose own keys the canvas leaves alone.
function inControl(target: EventTarget | null): boolean {
  return target instanceof HTMLSelectElement || target instanceof HTMLInputElement
    || target instanceof HTMLTextAreaElement || target instanceof HTMLButtonElement;
}

// clientPoint is where a pointer or wheel event is in the diagram's client area.
function clientPoint(event: MouseEvent): ClientPoint {
  const rect = diagram.getBoundingClientRect();
  return { x: event.clientX - rect.left, y: event.clientY - rect.top };
}

// applyViewport draws the canvas on screen at the viewport's zoom and scrolls to its offsets.
function applyViewport(): void {
  const svg = diagram.querySelector("svg");
  if (!svg) {
    return;
  }
  sizeCanvas(svg, viewport.zoom);
  zoomReadout.textContent = zoomLabel(viewport.zoom);
  const want = viewport;
  settling++;
  diagram.scrollLeft = want.scrollLeft;
  diagram.scrollTop = want.scrollTop;
  const stopped = { left: diagram.scrollLeft, top: diagram.scrollTop };
  // The status line under the canvas may have no height yet and clamp the offset: ask again once
  // the layout has settled, unless the user or a newer viewport has moved the container since.
  requestAnimationFrame(() => {
    if (viewport === want && diagram.scrollLeft === stopped.left && diagram.scrollTop === stopped.top) {
      diagram.scrollLeft = want.scrollLeft;
      diagram.scrollTop = want.scrollTop;
    }
    requestAnimationFrame(() => {
      settling--;
      if (settling === 0) {
        viewport = scrolled();
        rememberSoon();
      }
    });
  });
}

// scrolled is the viewport at the container's own scroll offsets, which may be ahead of
// what the scroll listener has recorded while an applied viewport settles.
function scrolled(): Viewport {
  return { ...viewport, scrollLeft: diagram.scrollLeft, scrollTop: diagram.scrollTop };
}

// setViewport moves the viewport and keeps it.
function setViewport(next: Viewport): void {
  viewport = next;
  applyViewport();
  rememberSoon();
}

// zoomBy zooms about the centre of what is on screen.
function zoomBy(factor: number): void {
  setViewport(zoomAbout(scrolled(), factor, { x: diagram.clientWidth / 2, y: diagram.clientHeight / 2 }));
}

// resetZoom shows the canvas at 1:1, keeping the centre of what is on screen in view.
function resetZoom(): void {
  zoomBy(1 / viewport.zoom);
}

// fitCanvas zooms so the whole canvas is in view, from its top-left corner.
function fitCanvas(): void {
  const svg = diagram.querySelector("svg");
  const size = svg && canvasSize(svg);
  if (!size) {
    return;
  }
  setViewport({ zoom: fitZoom(size, { width: diagram.clientWidth, height: diagram.clientHeight }), scrollLeft: 0, scrollTop: 0 });
}

let rememberTimer: ReturnType<typeof setTimeout> | undefined;

// rememberSoon keeps the state once a burst of scrolling or zooming has settled.
function rememberSoon(): void {
  if (rememberTimer !== undefined) {
    clearTimeout(rememberTimer);
  }
  rememberTimer = setTimeout(() => {
    rememberTimer = undefined;
    remember();
  }, 150);
}

/** A drag in progress: what was pressed, where, and what it has moved to so far. */
type Gesture =
  | NodeGesture
  | PanGesture
  | { kind: "waypoint"; edge: number; point: number; start: RenderPoint; pointer: number; routing: Routing; placements?: Placements }
  | { kind: "segment"; edge: number; segment: number; start: RenderPoint; pointer: number; routing: Routing; placements?: Placements };

// A press on empty canvas pans the view once it moves past the drag threshold;
// released where it was pressed, it is a click on nothing.
interface PanGesture {
  kind: "pan";
  pointer: number;
  routing: Routing;
  from: ClientPoint;
  scroll: { left: number; top: number };
  moved: boolean;
}

// A node drag also tracks what a release would do: `at` is where the pointer holds the
// node, `drop` the node under it while Shift is held. Every gesture keeps the routing it
// began under, so what it draws stays on the layout it started from until it ends.
interface NodeGesture {
  kind: "node";
  id: string;
  start: RenderPoint;
  pointer: number;
  routing: Routing;
  fixed: boolean;
  placements?: Placements;
  at?: RenderPoint;
  drop?: Drop;
}

// The pointer is listened to on the diagram's container, which outlives the SVG a
// drag redraws under it, so a gesture is followed across redraws.
diagram.addEventListener("pointerdown", beginGesture);
diagram.addEventListener("pointermove", moveGesture);
diagram.addEventListener("pointerup", endGesture);
diagram.addEventListener("pointercancel", () => cancelGesture());
diagram.addEventListener("contextmenu", (event) => {
  const node = last && nodeAt(event.target, last);
  if (node && last) {
    event.stopPropagation();
    cancelGesture();
    showMenu(node, last, event.clientX, event.clientY);
  }
});

// beginGesture starts a drag from what the pointer pressed: a waypoint or segment
// handle, else a node. A node no Layout can name answers a click and nothing else.
function beginGesture(event: PointerEvent): void {
  if (event.button !== 0 || gesture || !layout || !last) {
    return;
  }
  const start = canvasPoint(event);
  const handle = handleAt(event.target);
  if (handle) {
    gesture = handle.point !== undefined
      ? { kind: "waypoint", edge: handle.edge, point: handle.point, start, pointer: event.pointerId, routing }
      : { kind: "segment", edge: handle.edge, segment: handle.segment ?? 0, start, pointer: event.pointerId, routing };
  } else {
    const node = nodeAt(event.target, last);
    if (!node) {
      gesture = {
        kind: "pan",
        pointer: event.pointerId,
        routing,
        from: clientPoint(event),
        scroll: { left: diagram.scrollLeft, top: diagram.scrollTop },
        moved: false,
      };
    } else {
      const entry = layout.nodes.get(node.id);
      gesture = { kind: "node", id: node.id, start, pointer: event.pointerId, routing, fixed: !entry || !movable(layout, entry) };
    }
  }
  event.preventDefault();
  // Captured so the release is seen wherever the pointer goes, a fixed node's too;
  // focused so Shift and Escape reach the panel while the pointer is held.
  diagram.setPointerCapture(event.pointerId);
  diagram.focus({ preventScroll: true });
}

// moveGesture follows the pointer, showing where the drag would leave things.
function moveGesture(event: PointerEvent): void {
  if (!gesture || !layout || !last || event.pointerId !== gesture.pointer) {
    return;
  }
  if (gesture.kind === "node" && gesture.fixed) {
    return;
  }
  if (gesture.kind === "pan") {
    panTo(gesture, clientPoint(event));
    return;
  }
  const result = last;
  const at = canvasPoint(event);
  const dx = at.x - gesture.start.x;
  const dy = at.y - gesture.start.y;
  if (!gesture.placements && Math.hypot(dx, dy) < DRAG_THRESHOLD) {
    return;
  }
  switch (gesture.kind) {
    case "node":
      gesture.placements = movedNode(layout, gesture.id, dx, dy, freeAuto !== undefined);
      break;
    case "waypoint":
      gesture.placements = movedWaypoint(layout, gesture.edge, gesture.point, at);
      break;
    case "segment":
      gesture.placements = insertedWaypoint(layout, gesture.edge, gesture.segment, at);
      break;
  }
  if (!gesture.placements) {
    gesture = undefined;
    return;
  }
  if (gesture.kind === "node") {
    gesture.at = at;
    drawDrag(event.shiftKey);
    return;
  }
  showDragged(layoutCanvas(result, { ...panelOverrides(), ...overridesOf(gesture.placements), held: layout }, currentAuto()));
}

// showDragged puts the canvas a gesture has changed on screen. The pointer is captured by the
// diagram element, whose cursor is the one shown while it is, so the drag is marked there.
function showDragged(shown: CanvasLayout): SVGSVGElement {
  const svg = drawCanvas(shown);
  diagram.classList.add("dragging");
  sizeCanvas(svg, viewport.zoom);
  diagram.replaceChildren(svg);
  highlight(selectedNode);
  applySelection();
  return svg;
}

// drawDrag draws a node drag: laid out again around the node's new place, or, with Shift held,
// the model's layout left where it is with the dragged node floating over it, so the node
// under the pointer is the one the user sees there and no owner grows out to reclaim it.
function drawDrag(shift: boolean): void {
  if (gesture?.kind !== "node" || !gesture.placements || !gesture.at || !layout || !last) {
    return;
  }
  if (shift) {
    const svg = showDragged(layout);
    liftNode(svg, layout, gesture.id, gesture.at.x - gesture.start.x, gesture.at.y - gesture.start.y);
  } else {
    showDragged(layoutCanvas(last, { ...panelOverrides(), ...overridesOf(gesture.placements), held: layout }, currentAuto()));
  }
  previewDrop(shift);
}

// previewDrop shows what a release would do: with Shift held over another node, that node
// is marked as taking the dragged one or the status line says why not; else the line says how.
function previewDrop(shift: boolean): void {
  if (gesture?.kind !== "node" || !gesture.at || !layout || !last) {
    return;
  }
  const result = last;
  const dragged = gesture.id;
  const node = result.nodes?.find((candidate) => candidate.id === dragged);
  const under = shift ? nodeUnder(layout, gesture.at, dragged) : undefined;
  gesture.drop = node && under ? dropOn(node, under.node, result) : undefined;
  for (const marked of diagram.querySelectorAll(".opensysml-drop-target")) {
    marked.classList.remove("opensysml-drop-target");
  }
  diagram.classList.toggle("refused", gesture.drop?.admits === false);
  if (gesture.drop) {
    if (gesture.drop.admits) {
      diagram.querySelector(`[data-opensysml-id="${cssEscape(gesture.drop.target.id)}"]`)?.classList.add("opensysml-drop-target");
    }
    showHint(gesture.drop.message);
    return;
  }
  showHint(node ? dragHint(node, result) ?? "" : "");
}

// endGesture releases the pointer: a drag that moved something is one edit, a press
// that moved nothing is a click — on a node, opening its declaration; on a waypoint,
// the second of two in quick succession removes it.
function endGesture(event: PointerEvent): void {
  if (event.pointerId !== gesture?.pointer) {
    return;
  }
  if (gesture.kind === "node" && gesture.placements) {
    previewDrop(event.shiftKey);
  }
  const done = gesture;
  gesture = undefined;
  diagram.classList.remove("dragging", "refused", "panning");
  if (done.kind === "pan") {
    if (unrouted) {
      settle();
    }
    if (done.moved) {
      rememberSoon();
    } else {
      select(undefined);
    }
    return;
  }
  if (done.placements) {
    clickedWaypoint = undefined;
    showStatus("");
    if (done.kind === "node" && done.drop) {
      dropNode(done.id, done.drop, done.placements);
      return;
    }
    place(done.placements);
    return;
  }
  if (unrouted) {
    settle();
  }
  if (done.kind === "node") {
    clickedWaypoint = undefined;
    select(done.id);
    vscode.postMessage({ type: "reveal", id: done.id, drawn });
    return;
  }
  if (done.kind === "waypoint") {
    const again = clickedWaypoint?.edge === done.edge
      && clickedWaypoint.point === done.point && event.timeStamp - clickedWaypoint.at < DOUBLE_CLICK_MS;
    if (again && layout && last) {
      clickedWaypoint = undefined;
      place(removedWaypoint(layout, done.edge, done.point));
      return;
    }
    clickedWaypoint = { edge: done.edge, point: done.point, at: event.timeStamp };
  }
}

// cancelGesture drops a drag and puts the layout back as the model has it.
function cancelGesture(): void {
  if (!gesture) {
    return;
  }
  const moved = gesture.kind !== "pan" && gesture.placements !== undefined;
  gesture = undefined;
  diagram.classList.remove("dragging", "refused", "panning");
  if (moved || unrouted) {
    settle();
  }
  if (moved) {
    showStatus("");
  }
}

// panTo scrolls the canvas by how far the pointer has come since the press, once past the drag threshold.
function panTo(pan: PanGesture, at: ClientPoint): void {
  const dx = at.x - pan.from.x;
  const dy = at.y - pan.from.y;
  if (!pan.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) {
    return;
  }
  pan.moved = true;
  diagram.classList.add("panning");
  diagram.scrollLeft = pan.scroll.left - dx;
  diagram.scrollTop = pan.scroll.top - dy;
}

// settle shows the model's layout after a gesture that made no edit, laid out again if the
// router loaded or the routing changed meanwhile, where the gesture left the canvas scrolled.
function settle(): void {
  if (unrouted && last && layout) {
    layout = layoutCanvas(last, panelOverrides(), currentAuto());
  }
  unrouted = false;
  if (layout) {
    // The scroll listener may not have seen a pan's last offsets yet; the redraw must keep them.
    viewport = scrolled();
    show(layout);
  }
}

// dropNode ends a Shift-drag over another node: one edit moves the declaration into it,
// placed where it was released; a node that does not admit it takes the canvas back instead.
function dropNode(id: string, drop: Drop, placements: Placements): void {
  if (!drop.admits) {
    revert(drop.message);
    return;
  }
  vscode.postMessage({ type: "reparent", id, owner: drop.target.id, nodes: placements.nodes, edges: placements.edges, drawn });
}

// revert puts the model's layout back after a drop the model did not take, and says why when told.
function revert(message: string | undefined): void {
  cancelGesture();
  settle();
  if (message !== undefined) {
    showStatus(message);
  }
}

// place hands a gesture's outcome to the extension as one edit on the drawing it
// was made on; the panel redraws once the document has changed.
function place(placements: Placements | undefined): void {
  if (!placements || (placements.nodes.length === 0 && placements.edges.length === 0)) {
    settle();
    return;
  }
  vscode.postMessage({ type: "place", nodes: placements.nodes, edges: placements.edges, drawn });
}

// canvasPoint is where a pointer event is in the canvas's own coordinates.
function canvasPoint(event: PointerEvent): RenderPoint {
  const svg = diagram.querySelector("svg");
  const matrix = svg?.getScreenCTM();
  if (!matrix) {
    return { x: event.offsetX, y: event.offsetY };
  }
  const point = new DOMPoint(event.clientX, event.clientY).matrixTransform(matrix.inverse());
  return { x: point.x, y: point.y };
}

// nodeAt is the rendering node whose group the event target is in, if any.
function nodeAt(target: EventTarget | null, result: RenderResult): RenderNode | undefined {
  const group = (target as Element | null)?.closest?.<SVGElement>("g.opensysml-node[data-opensysml-id]");
  const id = group?.dataset.opensysmlId;
  return id === undefined ? undefined : result.nodes?.find((node) => node.id === id);
}

// handleAt is the route handle the event target is, if any: an edge with the
// waypoint it stands on or the segment it splits.
function handleAt(target: EventTarget | null): { edge: number; point?: number; segment?: number } | undefined {
  const handle = (target as Element | null)?.closest?.<SVGElement>("circle.waypoint, circle.segment");
  if (!handle) {
    return undefined;
  }
  const edge = Number(handle.dataset.edge);
  if (handle.dataset.point !== undefined) {
    return { edge, point: Number(handle.dataset.point) };
  }
  return { edge, segment: Number(handle.dataset.segment) };
}

// describe is what the rendering is, and how its kind was decided when the server
// had to decide it.
function describe(result: RenderResult): string {
  const name = result.view || "no view";
  return result.stated ? `${name} — ${result.kind} (${result.stated})` : `${name} — ${result.kind}`;
}

function showNotices(result: RenderResult): void {
  const list = result.notices ?? [];
  noticeList.replaceChildren();
  notices.hidden = list.length === 0;
  if (list.length === 0) {
    return;
  }
  (notices.firstElementChild as HTMLElement).textContent =
    list.length === 1 ? "1 notice" : `${list.length} notices`;
  for (const notice of list) {
    const item = document.createElement("li");
    item.textContent = notice;
    noticeList.append(item);
  }
}

// showError dims what is on screen rather than clearing it: the diagram shown is
// the last one the model was drawable at, and saying so is more useful than a
// blank panel.
function showError(message: string): void {
  showStatus(message);
  if (diagram.childElementCount > 0) {
    diagram.classList.add("stale");
  }
}

// showStatus puts a failure in the status line; showHint guidance on the gesture in progress.
function showStatus(message: string): void {
  status.textContent = message;
  status.classList.remove("hint");
}

function showHint(message: string): void {
  status.textContent = message;
  status.classList.toggle("hint", message !== "");
}

// showMenu opens a node's context menu where it was clicked, kept on screen;
// its entries act on the rendering the node was drawn from.
function showMenu(node: RenderNode, result: RenderResult, x: number, y: number): void {
  const items = nodeMenu(node, result.palette);
  menu.replaceChildren();
  if (items.length === 0) {
    hideMenu();
    return;
  }
  for (const item of items) {
    const entry = document.createElement("li");
    entry.setAttribute("role", item.separator ? "separator" : "menuitem");
    if (item.separator) {
      entry.className = "separator";
    } else if (item.heading) {
      entry.className = "title";
      entry.textContent = item.label;
    } else {
      entry.textContent = item.label;
      const { command } = item;
      entry.addEventListener("click", (event) => {
        event.stopPropagation();
        hideMenu();
        if (command) {
          run(command);
        }
      });
    }
    menu.append(entry);
  }
  menu.hidden = false;
  const width = menu.offsetWidth;
  const height = menu.offsetHeight;
  menu.style.left = `${Math.max(0, Math.min(x, window.innerWidth - width))}px`;
  menu.style.top = `${Math.max(0, Math.min(y, window.innerHeight - height))}px`;
}

function hideMenu(): void {
  menu.hidden = true;
}

// run hands a chosen entry to the extension with the drawing it was offered
// on; the panel redraws once the document has changed.
function run(command: MenuCommand): void {
  if (command.kind === "reveal") {
    vscode.postMessage({ type: "reveal", id: command.id, drawn });
    return;
  }
  vscode.postMessage({ type: "edit", action: command, drawn });
}

// highlight marks the node the cursor is in, and only that one. The id is kept so
// a redraw can mark it again.
function highlight(id: string | undefined): void {
  selectedNode = id;
  for (const marked of diagram.querySelectorAll(".opensysml-selected")) {
    marked.classList.remove("opensysml-selected");
  }
  if (!id) {
    return;
  }
  if (!layout && id.startsWith("row:")) {
    diagram.querySelector(`tr[data-opensysml-row="${cssEscape(id.slice(4))}"]`)?.classList.add("opensysml-selected");
    return;
  }
  const element = diagram.querySelector(`[data-opensysml-id="${cssEscape(id)}"]`);
  element?.classList.add("opensysml-selected");
}

// remember keeps what the panel is showing, so a window reload restores it.
function remember(): void {
    vscode.setState({ uri: documentURI, view: selected, last, style, routing, viewport, hidden: hiddenKinds === undefined ? undefined : [...hiddenKinds] });
}
