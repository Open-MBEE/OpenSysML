import assert from "node:assert/strict";
import path from "node:path";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import { PANEL_MARKUP } from "../markup";
import type { FromWebview, RenderResult, ToWebview } from "../protocol";
import { loadAvoid } from "./avoid";

const WASM = path.resolve("node_modules/libavoid-js/dist/libavoid.wasm");

// The panel script is loaded into the markup the extension serves it in, with the
// extension's API faked, so the messages and gestures it answers are driven here.
const dom = new JSDOM(`<!DOCTYPE html><body data-state='{"uri":"file:///m.sysml"}'>${PANEL_MARKUP}</body>`, {
  url: "https://webview.test/",
  pretendToBeVisual: true,
});
const { window } = dom;
globalThis.window = window;
globalThis.document = window.document;
globalThis.requestAnimationFrame = window.requestAnimationFrame;
const posted: FromWebview[] = [];
(globalThis as { acquireVsCodeApi?: unknown }).acquireVsCodeApi = () => ({
  postMessage: (message: FromWebview) => posted.push(message),
  setState: () => undefined,
  getState: () => undefined,
});
// jsdom captures no pointers and lays no SVG out: the panel captures one for every
// gesture, and with no screen matrix it reads the pointer in the event's own offsets.
window.HTMLElement.prototype.setPointerCapture = () => undefined;
(window.SVGSVGElement.prototype as { getScreenCTM: () => DOMMatrix | null }).getScreenCTM = () => null;

const diagram = window.document.getElementById("diagram")!;
const status = window.document.getElementById("status")!;

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };

// Two placed parts joined by a connection with a stated diagonal route: `model` routing
// draws it verbatim with a waypoint handle, `router` routes orthogonally and hides the handle.
function statedDiagonal(): RenderResult {
  return {
    view: "M::V",
    kind: "interconnection",
    stated: "",
    form: "mermaid",
    artifact: "",
    nodes: [
      { id: "a", kind: "part", name: "a", type: "", detail: "", fqn: "M::a", origin, x: 0, y: 0, width: 100, height: 40 },
      { id: "b", kind: "part", name: "b", type: "", detail: "", fqn: "M::b", origin, x: 400, y: 200, width: 100, height: 40 },
      { id: "c", kind: "part", name: "c", type: "", detail: "", fqn: "M::c", origin, x: 180, y: -20, width: 60, height: 80 },
    ],
    edges: [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab", route: [{ x: 250, y: 150 }] }],
    notices: [],
    version: 7,
  };
}

function post(message: ToWebview): void {
  window.dispatchEvent(new window.MessageEvent("message", { data: message, origin: window.origin }));
}

function pointer(type: string, target: Element, init: MouseEventInit = {}): void {
  const event = new window.MouseEvent(type, { bubbles: true, cancelable: true, button: 0, ...init });
  Object.defineProperty(event, "pointerId", { value: 1 });
  target.dispatchEvent(event);
}

async function settled(): Promise<void> {
  while (status.textContent === "Laying out…") {
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

function waypointHandles(): number {
  return diagram.querySelectorAll("circle.waypoint").length;
}

function routes(): (string | null)[] {
  return [...diagram.querySelectorAll("g.edges polyline")].map((line) => line.getAttribute("points"));
}

function nodeElement(id: string): Element {
  return diagram.querySelector(`g.opensysml-node[data-opensysml-id="${id}"]`)!;
}

await loadAvoid(WASM);
await import("./diagram");
await new Promise((resolve) => setTimeout(resolve, 0));

test("a routing change during a drag is applied when the drag settles", async () => {
  post({ type: "render", result: statedDiagonal(), selected: "M::V", drawn: 1, style: "theme", routing: "model" });
  await settled();
  assert.equal(waypointHandles(), 1, "model routing draws the stated waypoint's handle");

  // The routing changes to the router while the pointer holds a node; the panel must
  // not redraw under the pointer, so the routes on screen are still the model's.
  pointer("pointerdown", nodeElement("a"), { clientX: 50, clientY: 20 });
  assert.equal(diagram.classList.contains("dragging"), false);
  post({ type: "routing", routing: "router" });
  assert.equal(waypointHandles(), 1, "a drag in progress keeps the layout it started on");

  // Released as a click, the routes the setting now asks for are drawn.
  pointer("pointerup", nodeElement("a"), { clientX: 50, clientY: 20 });
  assert.equal(waypointHandles(), 0, "the router hides the stated waypoint's handle once the drag settles");
  assert.ok(posted.some((message) => message.type === "reveal" && message.id === "a"), "the click still reveals the node");
});

test("a routing change during a drag is applied when the drag is cancelled", async () => {
  post({ type: "render", result: statedDiagonal(), selected: "M::V", drawn: 2, style: "theme", routing: "router" });
  await settled();
  assert.equal(waypointHandles(), 0);

  pointer("pointerdown", nodeElement("a"), { clientX: 50, clientY: 20 });
  post({ type: "routing", routing: "model" });
  assert.equal(waypointHandles(), 0, "a drag in progress keeps the layout it started on");
  window.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Escape" }));
  assert.equal(waypointHandles(), 1, "model routing draws the stated waypoint's handle once the drag is cancelled");
});

test("a routing change during a pan is applied when the pan settles", async () => {
  post({ type: "render", result: statedDiagonal(), selected: "M::V", drawn: 3, style: "theme", routing: "model" });
  await settled();
  assert.equal(waypointHandles(), 1);

  const canvas = diagram.querySelector("svg")!;
  pointer("pointerdown", canvas, { clientX: 300, clientY: 40 });
  post({ type: "routing", routing: "router" });
  assert.equal(waypointHandles(), 1, "a pan in progress keeps the layout it started on");
  pointer("pointerup", canvas, { clientX: 300, clientY: 40 });
  assert.equal(waypointHandles(), 0, "the router hides the stated waypoint's handle once the pan settles");
});

test("a routing message during a drag that leaves the routing as it is changes nothing", async () => {
  post({ type: "render", result: statedDiagonal(), selected: "M::V", drawn: 4, style: "theme", routing: "model" });
  await settled();
  const canvas = diagram.querySelector("svg");
  const before = routes();
  pointer("pointerdown", nodeElement("a"), { clientX: 50, clientY: 20 });
  post({ type: "routing", routing: "model" });
  pointer("pointerup", nodeElement("a"), { clientX: 50, clientY: 20 });
  assert.equal(diagram.querySelector("svg"), canvas, "the click settles nothing, so the canvas is not redrawn");
  assert.deepEqual(routes(), before);
});

test("a drag's preview keeps the routing the drag began under until the drag ends", async () => {
  post({ type: "render", result: statedDiagonal(), selected: "M::V", drawn: 5, style: "theme", routing: "model" });
  await settled();
  pointer("pointerdown", nodeElement("a"), { clientX: 50, clientY: 20 });
  pointer("pointermove", diagram, { clientX: 60, clientY: 30 });
  assert.equal(waypointHandles(), 1, "the preview draws the stated waypoint's handle under model routing");

  post({ type: "routing", routing: "router" });
  pointer("pointermove", diagram, { clientX: 70, clientY: 40 });
  assert.equal(waypointHandles(), 1, "the preview stays on the routing the drag began under");
  window.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Escape" }));
  assert.equal(waypointHandles(), 0, "the router takes over once the drag ends");
});

test("a routing change during a pan is applied where the pan left the canvas when the pan is cancelled", async () => {
  post({ type: "render", result: statedDiagonal(), selected: "M::V", drawn: 6, style: "theme", routing: "model" });
  await settled();
  const canvas = diagram.querySelector("svg")!;
  pointer("pointerdown", canvas, { clientX: 300, clientY: 40 });
  pointer("pointermove", canvas, { clientX: 260, clientY: 40 });
  assert.equal(diagram.scrollLeft, 40, "the pan scrolls the canvas by how far the pointer came");

  post({ type: "routing", routing: "router" });
  window.dispatchEvent(new window.KeyboardEvent("keydown", { key: "Escape" }));
  assert.equal(waypointHandles(), 0, "the router hides the stated waypoint's handle once the pan is cancelled");
  assert.equal(diagram.scrollLeft, 40, "the redraw keeps the canvas where the pan left it");
});
