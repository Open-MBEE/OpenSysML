import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import { nothingToDraw, type RenderResult } from "../protocol";
import { renderSummary } from "../rendertrace";
import { nothingLaidOut } from "./empty";
import { layoutCanvas } from "./layout";

const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

// The server's rendering of `expose abcd; filter @PartUsage;`: three parts
// nested in the exposed one, no edge, a notice for the excluded interface.
const filtered: RenderResult = {
  view: "Model::fails",
  kind: "interconnection",
  stated: "view def InterconnectionView",
  form: "mermaid",
  artifact: "",
  nodes: [
    { id: "n0", kind: "part", name: "Model::abcd", type: "", detail: "" },
    { id: "n1", kind: "part", name: "a", type: "", detail: "", parent: "n0" },
    { id: "n2", kind: "part", name: "b", type: "", detail: "", parent: "n0" },
    { id: "n3", kind: "part", name: "c", type: "", detail: "", parent: "n0" },
  ],
  edges: [],
  notices: ["interface Model::abcd::i attaches to 'a.a', which the view's filter conditions exclude; no edge is drawn"],
  version: 1,
};

test("a filtered interconnection view with parts but no edge is drawn, its notice carried", () => {
  assert.equal(nothingToDraw(filtered), false);
  const layout = layoutCanvas(filtered);
  assert.equal(nothingLaidOut(layout), false);
  assert.ok(layout.width > 0 && layout.height > 0, `layout extent ${layout.width}x${layout.height}`);
  const laidOut = (roots: typeof layout.roots): number =>
    roots.reduce((n, root) => n + 1 + laidOut((root as { children?: typeof layout.roots }).children ?? []), 0);
  assert.equal(laidOut(layout.roots), 4);
  assert.equal(
    renderSummary("interconnection.sysml", { textDocument: { uri: "file:///interconnection.sysml" }, view: "Model::fails" }, filtered),
    "Rendered Model::fails of interconnection.sysml: interconnection (mermaid), 4 node(s), 0 edge(s), 1 notice(s):\n" +
      "  interface Model::abcd::i attaches to 'a.a', which the view's filter conditions exclude; no edge is drawn",
  );
});
