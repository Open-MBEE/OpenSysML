import assert from "node:assert/strict";
import { test } from "node:test";

import { normalizeRender, RenderResult } from "./protocol";
import { labelLines } from "./webview/layout";

// A rendering as a server predating the node `type` field writes it: the type
// sits in `detail`, and `type` is absent. The wire is untyped, hence the cast.
const olderServer = {
  view: "Demo::v",
  kind: "interconnection",
  stated: "",
  form: "mermaid",
  artifact: "",
  nodes: [
    { id: "n0", kind: "part def", name: "Demo::Vehicle", detail: "" },
    { id: "n1", kind: "part", name: "", detail: "Wheel", parent: "n0" },
    { id: "n2", kind: "part", name: "engine", detail: "Engine", parent: "n0" },
  ],
  edges: [{ from: "n1", to: "n2", kind: "connect" }],
  version: 3,
} as unknown as RenderResult;

test("normalizeRender fills the fields an older server omits", () => {
  const result = normalizeRender(olderServer);
  assert.deepEqual(
    result.nodes.map((node) => [node.name, node.type, node.detail]),
    [
      ["Demo::Vehicle", "", ""],
      ["", "", "Wheel"],
      ["engine", "", "Engine"],
    ],
  );
  assert.deepEqual(result.edges, [{ from: "n1", to: "n2", kind: "connect", label: "" }]);
  assert.deepEqual(result.notices, []);
  assert.equal(result.version, 3);
});

test("a label never spells a value the server left out", () => {
  const result = normalizeRender(olderServer);
  const lines = result.nodes.map(labelLines);
  assert.deepEqual(lines, [["Demo::Vehicle", "«part def»"], ["part", "Wheel"], ["engine", "«part»", "Engine"]]);
  for (const line of lines.flat()) {
    assert.doesNotMatch(line, /undefined/);
  }
});

test("normalizeRender leaves a current server's rendering as it is", () => {
  const current: RenderResult = {
    ...olderServer,
    nodes: [{ id: "n0", kind: "part", name: "engine", type: "Engine", detail: "", fqn: "Demo::engine" }],
    edges: [{ from: "n0", to: "n0", kind: "connect", label: "c" }],
    notices: ["a notice"],
  };
  assert.deepEqual(normalizeRender(current), current);
});
