import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import type { RenderNode, RenderResult } from "../protocol";
import { drawCanvas } from "./canvas";
import { layoutCanvas } from "./layout";
import { FOCUS, HAS_FOCUS, markSelection, neighbourhood, RELATED, relatedByKind } from "./selection";

const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

function node(id: string, extra: Partial<RenderNode> = {}): RenderNode {
  return { id, kind: "part", name: id, type: "", detail: "", fqn: `M::${id}`, ...extra };
}

const result: RenderResult = {
  view: "M::V",
  kind: "interconnection",
  stated: "",
  form: "mermaid",
  artifact: "",
  nodes: [node("a"), node("b"), node("c"), node("d")],
  edges: [
    { from: "a", to: "b", label: "", kind: "connection" },
    { from: "c", to: "a", label: "", kind: "flow" },
    { from: "c", to: "d", label: "", kind: "connection" },
    { from: "a", to: "d", label: "", kind: "binding" },
  ],
  notices: [],
  version: 3,
};

test("neighbourhood is the node, the edges at it and the nodes at their other ends", () => {
  const around = neighbourhood(result, "a");
  assert.deepEqual([...around.nodes], ["a", "b", "c", "d"]);
  assert.deepEqual([...around.edges], [0, 1, 3]);
  const lone = neighbourhood(result, "b");
  assert.deepEqual([...lone.nodes], ["b", "a"]);
  assert.deepEqual([...lone.edges], [0]);
});

test("neighbourhood leaves out edges of a hidden kind and the nodes only they reach", () => {
  const around = neighbourhood(result, "a", new Set(["binding", "flow"]));
  assert.deepEqual([...around.nodes], ["a", "b"]);
  assert.deepEqual([...around.edges], [0]);
});

test("a node with no edges is a neighbourhood of itself", () => {
  const around = neighbourhood({ ...result, edges: [] }, "a");
  assert.deepEqual([...around.nodes], ["a"]);
  assert.equal(around.edges.size, 0);
});

test("relatedByKind counts a node's incident edges by kind, hidden or not", () => {
  assert.deepEqual([...relatedByKind(result.edges ?? [], "a")], [["connection", 1], ["flow", 1], ["binding", 1]]);
  assert.deepEqual([...relatedByKind(result.edges ?? [], "c")], [["flow", 1], ["connection", 1]]);
  assert.equal(relatedByKind(result.edges ?? [], "zz").size, 0);
});

test("markSelection emphasises the selected node, its neighbours and incident edges, and dims under has-focus", () => {
  const svg = drawCanvas(layoutCanvas(result));
  markSelection(svg, neighbourhood(result, "c"));
  assert.ok(svg.classList.contains(HAS_FOCUS));
  const classes = (selector: string): string[] =>
    [...svg.querySelectorAll<SVGGElement>(selector)].map((group) => `${group.dataset.opensysmlId ?? group.dataset.edge}:${group.classList.contains(FOCUS) ? "focus" : group.classList.contains(RELATED) ? "related" : "-"}`);
  assert.deepEqual(classes("g.opensysml-node"), ["a:related", "b:-", "c:focus", "d:related"]);
  assert.deepEqual(classes("g.opensysml-edge"), ["0:-", "1:related", "2:related", "3:-"]);
  // Clearing takes every mark off.
  markSelection(svg, undefined);
  assert.ok(!svg.classList.contains(HAS_FOCUS));
  assert.equal(svg.querySelectorAll(`.${FOCUS}, .${RELATED}`).length, 0);
});

test("markSelection marks a lone node as the focus with nothing related", () => {
  const svg = drawCanvas(layoutCanvas({ ...result, edges: [] }));
  markSelection(svg, neighbourhood({ ...result, edges: [] }, "b"));
  const focus = svg.querySelectorAll(`g.opensysml-node.${FOCUS}`);
  assert.equal(focus.length, 1);
  assert.equal((focus[0] as SVGGElement).dataset.opensysmlId, "b");
  assert.equal(svg.querySelectorAll(`.${RELATED}:not(.${FOCUS})`).length, 0);
});
