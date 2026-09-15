// The diagram panel's script: draws the server's rendering on an SVG canvas, reports
// clicks, menu choices and drags back, and highlights the cursor's node.
import type { EditPalette, FromWebview, PickerEntry, RenderNode, RenderPoint, RenderResult, ToWebview } from "../protocol";
import { MenuCommand, MenuItem, nodeMenu, paletteItems } from "./actions";
import { drawCanvas } from "./canvas";
import { dragHint, Drop, dropOn } from "./drop";
import {
  CanvasLayout,
  insertedWaypoint,
  layoutCanvas,
  movable,
  movedNode,
  movedWaypoint,
  nodeUnder,
  overridesOf,
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
const kindLabel = document.getElementById("kind") as HTMLElement;
const status = document.getElementById("status") as HTMLElement;
const diagram = document.getElementById("diagram") as HTMLElement;
const notices = document.getElementById("notices") as HTMLDetailsElement;
const noticeList = document.getElementById("notice-list") as HTMLElement;
const undrawable = document.getElementById("undrawable") as HTMLDetailsElement;
const undrawableList = document.getElementById("undrawable-list") as HTMLElement;
const adder = document.getElementById("add") as HTMLSelectElement;
const menu = document.getElementById("menu") as HTMLUListElement;

const documentURI = (JSON.parse(body.dataset.state ?? "{}") as { uri?: string }).uri ?? "";
const saved = (vscode.getState() ?? {}) as { view?: string; last?: RenderResult };
let selected = saved.view ?? "";
let last: RenderResult | undefined = saved.last;
let selectedNode: string | undefined;
/** The layout on screen, which gestures act on; undefined while a table or nothing is shown. */
let layout: CanvasLayout | undefined;
let gesture: Gesture | undefined;
/** How far the pointer moves before a press becomes a drag rather than a click. */
const DRAG_THRESHOLD = 3;
/** How soon a second click on a waypoint must follow the first to remove it. */
const DOUBLE_CLICK_MS = 400;
// The waypoint last clicked; the pointer is captured while pressed, so a dblclick
// event would name the container rather than the handle, and clicks are paired here.
let clickedWaypoint: { edge: number; point: number; at: number } | undefined;
let paletteEntries: MenuItem[] = [];
let paletteVersion = 0;

// The panel is torn down while it is hidden, so the rendering it last drew is
// put back — dimmed until the server answers — rather than showing nothing.
if (last) {
  draw(last);
  diagram.classList.add("stale");
}

picker.addEventListener("change", () => {
  selected = picker.value;
  remember();
  vscode.postMessage({ type: "pick", view: selected });
});

// The list's entries are actions, so the prompt is reselected after one is chosen.
adder.addEventListener("change", () => {
  const item = paletteEntries[Number(adder.value)];
  adder.selectedIndex = 0;
  if (item?.command) {
    run(item.command, paletteVersion);
  }
});

window.addEventListener("click", () => hideMenu());
window.addEventListener("blur", () => hideMenu());
window.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    hideMenu();
    cancelGesture();
  } else if (event.key === "Shift") {
    previewDrop(true);
  }
});
window.addEventListener("keyup", (event) => {
  if (event.key === "Shift") {
    previewDrop(false);
  }
});
// A right-click off a node offers nothing; the browser's own menu offers less.
diagram.addEventListener("contextmenu", (event) => event.preventDefault());

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
      draw(message.result);
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

// draw replaces the diagram with the rendering. A failure to draw leaves the last
// diagram up, dimmed, so a mid-keystroke parse error does not blank the panel.
function draw(result: RenderResult): void {
  cancelGesture();
  try {
    if (result.form === "mermaid") {
      // Mermaid is the machine form a diagram is exported in; the panel draws
      // the same nodes and edges itself, so their geometry is its own to edit.
      layout = layoutCanvas(result);
      show(layout);
    } else {
      // A table is written as Markdown rather than drawn, so it is shown as the
      // artifact it is.
      layout = undefined;
      const pre = document.createElement("pre");
      pre.textContent = result.artifact;
      diagram.replaceChildren(pre);
    }
    diagram.classList.remove("stale");
    showStatus("");
    last = result;
    remember();
    showNotices(result);
    // An open menu names nodes of the drawing just replaced.
    hideMenu();
    showPalette(result.palette, result.version);
    kindLabel.textContent = describe(result);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    showError(message);
    vscode.postMessage({ type: "failed", message });
  }
}

// show puts a layout on screen in place of what was there, the cursor's node marked.
function show(shown: CanvasLayout): void {
  diagram.replaceChildren(drawCanvas(shown));
  highlight(selectedNode);
}

/** A drag in progress: what was pressed, where, and what it has moved to so far. */
type Gesture =
  | NodeGesture
  | { kind: "waypoint"; edge: number; point: number; start: RenderPoint; pointer: number; placements?: Placements }
  | { kind: "segment"; edge: number; segment: number; start: RenderPoint; pointer: number; placements?: Placements };

// A node drag also tracks what a release would do: `shown` is the layout drawn with the
// node where the pointer holds it, `at` the pointer, `drop` the node under it while Shift is held.
interface NodeGesture {
  kind: "node";
  id: string;
  start: RenderPoint;
  pointer: number;
  fixed: boolean;
  placements?: Placements;
  shown?: CanvasLayout;
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
      ? { kind: "waypoint", edge: handle.edge, point: handle.point, start, pointer: event.pointerId }
      : { kind: "segment", edge: handle.edge, segment: handle.segment ?? 0, start, pointer: event.pointerId };
  } else {
    const node = nodeAt(event.target, last);
    if (!node) {
      return;
    }
    const entry = layout.nodes.get(node.id);
    gesture = { kind: "node", id: node.id, start, pointer: event.pointerId, fixed: !entry || !movable(layout, entry) };
  }
  event.preventDefault();
  // Captured so the release is seen wherever the pointer goes, a fixed node's too.
  diagram.setPointerCapture(event.pointerId);
}

// moveGesture follows the pointer, showing where the drag would leave things.
function moveGesture(event: PointerEvent): void {
  if (!gesture || !layout || !last || event.pointerId !== gesture.pointer) {
    return;
  }
  if (gesture.kind === "node" && gesture.fixed) {
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
      gesture.placements = movedNode(layout, gesture.id, dx, dy);
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
  const shown = layoutCanvas(result, overridesOf(gesture.placements));
  const svg = drawCanvas(shown);
  svg.classList.add("dragging");
  diagram.replaceChildren(svg);
  highlight(selectedNode);
  if (gesture.kind === "node") {
    gesture.shown = shown;
    gesture.at = at;
    previewDrop(event.shiftKey);
  }
}

// previewDrop shows what a release would do: with Shift held over another node, that node
// is marked as taking the dragged one or the status line says why not; else the line says how.
function previewDrop(shift: boolean): void {
  if (gesture?.kind !== "node" || !gesture.shown || !gesture.at || !last) {
    return;
  }
  const result = last;
  const dragged = gesture.id;
  const node = result.nodes?.find((candidate) => candidate.id === dragged);
  const under = shift ? nodeUnder(gesture.shown, gesture.at, dragged) : undefined;
  gesture.drop = node && under ? dropOn(node, under.node, result) : undefined;
  for (const marked of diagram.querySelectorAll(".opensysml-drop-target")) {
    marked.classList.remove("opensysml-drop-target");
  }
  const svg = diagram.querySelector("svg");
  svg?.classList.toggle("refused", gesture.drop?.admits === false);
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
  if (done.placements) {
    clickedWaypoint = undefined;
    showStatus("");
    if (done.kind === "node" && done.drop) {
      dropNode(done.id, done.drop, done.placements, last?.version ?? 0);
      return;
    }
    place(done.placements, last?.version ?? 0);
    return;
  }
  if (done.kind === "node") {
    clickedWaypoint = undefined;
    vscode.postMessage({ type: "reveal", id: done.id });
    return;
  }
  if (done.kind === "waypoint") {
    const again = clickedWaypoint?.edge === done.edge
      && clickedWaypoint.point === done.point && event.timeStamp - clickedWaypoint.at < DOUBLE_CLICK_MS;
    if (again && layout && last) {
      clickedWaypoint = undefined;
      place(removedWaypoint(layout, done.edge, done.point), last.version);
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
  const moved = gesture.placements !== undefined;
  gesture = undefined;
  if (moved && layout) {
    show(layout);
    showStatus("");
  }
}

// dropNode ends a Shift-drag over another node: one edit moves the declaration into it,
// placed where it was released; a node that does not admit it takes the canvas back instead.
function dropNode(id: string, drop: Drop, placements: Placements, version: number): void {
  if (!drop.admits) {
    revert(drop.message);
    return;
  }
  vscode.postMessage({ type: "reparent", id, owner: drop.target.id, nodes: placements.nodes, edges: placements.edges, version });
}

// revert puts the model's layout back after a drop the model did not take, and says why when told.
function revert(message: string | undefined): void {
  cancelGesture();
  if (layout) {
    show(layout);
  }
  if (message !== undefined) {
    showStatus(message);
  }
}

// place hands a gesture's outcome to the extension as one edit on the rendering it
// was made on; the panel redraws once the document has changed.
function place(placements: Placements | undefined, version: number): void {
  if (!placements || (placements.nodes.length === 0 && placements.edges.length === 0)) {
    if (layout) {
      show(layout);
    }
    return;
  }
  vscode.postMessage({ type: "place", nodes: placements.nodes, edges: placements.edges, version });
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

// showPalette fills the toolbar's "Add" list, or hides it for a rendering that is not editable.
function showPalette(palette: EditPalette | undefined, version: number): void {
  paletteEntries = palette ? paletteItems(palette) : [];
  paletteVersion = version;
  adder.replaceChildren();
  adder.hidden = paletteEntries.length === 0;
  if (paletteEntries.length === 0) {
    return;
  }
  const prompt = document.createElement("option");
  prompt.value = "";
  prompt.textContent = "Add…";
  adder.append(prompt);
  paletteEntries.forEach((item, index) => {
    const option = document.createElement("option");
    option.value = String(index);
    option.textContent = item.label;
    adder.append(option);
  });
  adder.selectedIndex = 0;
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
          run(command, result.version);
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

// run hands a chosen entry to the extension with the rendering it was offered
// on; the panel redraws once the document has changed.
function run(command: MenuCommand, version: number): void {
  if (command.kind === "reveal") {
    vscode.postMessage({ type: "reveal", id: command.id });
    return;
  }
  vscode.postMessage({ type: "edit", action: command, version });
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
  const element = diagram.querySelector(`[data-opensysml-id="${cssEscape(id)}"]`);
  element?.classList.add("opensysml-selected");
}

// cssEscape quotes an id for an attribute selector, since CSS.escape is not in
// every webview host.
function cssEscape(value: string): string {
  return value.replace(/["\\]/g, String.raw`\$&`);
}

// remember keeps what the panel is showing, so a window reload restores it.
function remember(): void {
  vscode.setState({ uri: documentURI, view: selected, last });
}
