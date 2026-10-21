import assert from "node:assert/strict";
import path from "node:path";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import { PANEL_MARKUP } from "../markup";
import type { FromWebview, RenderNode, RenderResult, ToWebview } from "../protocol";
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
const relationList = window.document.getElementById("relation-list")!;

const origin = { uri: "file:///m.sysml", range: { start: { line: 0, character: 0 }, end: { line: 0, character: 4 } }, digest: "d0" };

function part(id: string, x: number, y: number, extra: Partial<RenderNode> = {}): RenderNode {
  return { id, kind: "part", name: id, type: "", detail: "", fqn: `M::${id}`, origin, x, y, width: 100, height: 40, ...extra };
}

function rendering(nodes: RenderNode[], edges: RenderResult["edges"], version: number): RenderResult {
  return { view: "M::V", kind: "interconnection", stated: "", form: "mermaid", artifact: "", nodes, edges, notices: [], version };
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

function nodeElement(id: string): Element {
  return diagram.querySelector(`g.opensysml-node[data-opensysml-id="${id}"]`)!;
}

function related(): string[] {
  return [...diagram.querySelectorAll("g.opensysml-node.related")].map((group) => group.getAttribute("data-opensysml-id") ?? "").sort();
}

function relatedEdges(): string[] {
  return [...diagram.querySelectorAll("g.opensysml-edge.related")].map((group) => group.getAttribute("data-edge") ?? "").sort();
}

function drawnEdgesByKind(): Map<string, number> {
  const counts = new Map<string, number>();
  for (const group of diagram.querySelectorAll("g.opensysml-edge")) {
    const kind = [...group.classList].find((name) => name !== "opensysml-edge" && name !== "related") ?? "";
    counts.set(kind, (counts.get(kind) ?? 0) + 1);
  }
  return counts;
}

function listedByKind(): Map<string, string> {
  const counts = new Map<string, string>();
  for (const box of relationList.querySelectorAll<HTMLInputElement>("input[data-kind]")) {
    counts.set(box.dataset.kind ?? "", box.parentElement?.querySelector(".count")?.textContent ?? "");
  }
  return counts;
}

await loadAvoid(WASM);
await import("./diagram");
await new Promise((resolve) => setTimeout(resolve, 0));

test("a focused node's neighbourhood follows an edit that moves its relationship", async () => {
  const nodes = [part("a", 0, 0), part("b", 300, 0), part("c", 0, 200)];
  post({ type: "render", result: rendering(nodes, [{ from: "a", to: "b", label: "", kind: "connection", fqn: "M::ab" }], 1), selected: "M::V", drawn: 1, style: "theme", routing: "model" });
  await settled();
  pointer("pointerdown", nodeElement("a"), { clientX: 50, clientY: 20 });
  pointer("pointerup", nodeElement("a"), { clientX: 50, clientY: 20 });
  assert.deepEqual(related(), ["a", "b"], "focusing a marks its neighbour b");
  assert.deepEqual(relatedEdges(), ["0"]);

  post({ type: "render", result: rendering(nodes, [{ from: "c", to: "b", label: "", kind: "connection", fqn: "M::cb" }, { from: "a", to: "c", label: "", kind: "connection", fqn: "M::ac" }], 2), selected: "M::V", drawn: 2, style: "theme", routing: "model" });
  assert.deepEqual(related(), ["a", "c"], "the canvas drawn for the edit marks a's new neighbour c");
  assert.deepEqual(relatedEdges(), ["1"], "and a's new edge");
  await settled();
  assert.deepEqual(related(), ["a", "c"], "so does the one laid out afresh");
  assert.deepEqual(relatedEdges(), ["1"]);
});

test("the Relations list counts the edges the canvas draws under a collapsed owner", async () => {
  pointer("pointerdown", diagram, { clientX: 500, clientY: 500 });
  pointer("pointerup", diagram, { clientX: 500, clientY: 500 });
  const nodes = [
    part("a", 0, 0, { collapsed: true }),
    { id: "p", kind: "port", name: "p", type: "", detail: "", fqn: "M::a.p", origin, parent: "a" },
    part("b", 300, 0),
  ];
  const edges = [
    { from: "p", to: "b", label: "", kind: "connection", fqn: "M::pb" },
    { from: "a", to: "b", label: "", kind: "dependency", fqn: "M::ab" },
  ];
  post({ type: "render", result: rendering(nodes, edges, 3), selected: "M::V", drawn: 3, style: "theme", routing: "model" });
  await settled();
  const drawn = drawnEdgesByKind();
  const listed = listedByKind();
  assert.deepEqual([...listed.keys()].sort(), [...drawn.keys()].sort(), "the list names the kinds the canvas draws");
  for (const [kind, count] of drawn) {
    assert.equal(listed.get(kind), String(count), `${kind} counts what is drawn`);
  }
});
