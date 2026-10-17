import assert from "node:assert/strict";
import { test } from "node:test";

import type { RenderParams, RenderResult } from "./protocol";
import { logsRender, renderSummary } from "./rendertrace";

const params: RenderParams = { textDocument: { uri: "file:///m.sysml" }, view: "M::v" };

const drawn: RenderResult = {
  view: "M::v",
  kind: "tree",
  stated: "",
  form: "mermaid",
  artifact: "",
  nodes: [{ id: "n0", kind: "part def", name: "M::A", type: "", detail: "" }],
  edges: [],
  notices: [],
  version: 1,
};

const empty: RenderResult = {
  ...drawn,
  nodes: [],
  notices: ["every element the view exposes is rejected by its filter `@ActionUsage`; the rendering is empty"],
};

test("a render is logged while the server is traced, and always when it has nothing to draw", () => {
  assert.equal(logsRender("off", drawn), false);
  assert.equal(logsRender(undefined, drawn), false);
  assert.equal(logsRender("messages", drawn), true);
  assert.equal(logsRender("verbose", drawn), true);
  assert.equal(logsRender("off", empty), true);
});

test("the summary names the view, the kind, the counts and every notice", () => {
  assert.equal(renderSummary("m.sysml", params, drawn), "Rendered M::v of m.sysml: tree (mermaid), 1 node(s), 0 edge(s), 0 notice(s)");
  assert.equal(
    renderSummary("m.sysml", params, empty),
    "Rendered M::v of m.sysml: tree (mermaid), 0 node(s), 0 edge(s), 1 notice(s); nothing to draw:\n" +
      "  every element the view exposes is rejected by its filter `@ActionUsage`; the rendering is empty",
  );
});

test("a table counts its rows, and a pseudo-view is named by the request", () => {
  const table: RenderResult = { ...drawn, view: "", kind: "table", form: "markdown", nodes: [], rows: [{ cells: ["a"] }], notices: [] };
  assert.equal(
    renderSummary("m.sysml", { textDocument: params.textDocument }, table),
    "Rendered the implied view of m.sysml: table (markdown), 0 node(s), 0 edge(s), 1 row(s), 0 notice(s)",
  );
});
