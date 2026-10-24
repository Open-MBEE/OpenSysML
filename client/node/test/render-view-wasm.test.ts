import assert from "node:assert/strict";
import { test } from "node:test";

import { renderedViewFromJson } from "../src/core/index.js";
import { connectWasmHost, type WasmHost } from "../src/core/wasm.js";

const rendering = {
  view: "#interconnection:Demo::Item",
  kind: "interconnection",
  stated: "Demo::Item",
  nodes: [
    { id: "n0", kind: "part", name: "Demo::Item", origin: { file: "demo.sysml", startLine: 2, startCol: 5, endLine: 2, endCol: 9 } },
    {
      id: "n1",
      kind: "part",
      name: "inner",
      type: "Part",
      parent: "n0",
      ports: [{ id: "n1.0", name: "p", direction: "in" }],
      geometry: { x: 10, y: 20, width: 100, height: 40, hasSize: true },
      style: { fill: "#fff", line: "#000", bold: true },
    },
  ],
  edges: [{ from: "n1", to: "n0", fromPort: "n1.0", label: "flow", kind: "connection" }],
  notices: ["one notice"],
  shown: ["edge-labels"],
};

/** A module answering the handshake and one RenderView, recording what it was asked. */
function renderingHost(calls: Array<{ method: string; params: unknown }>): WasmHost {
  return {
    version: "render-test",
    call(method: string, params: string) {
      calls.push({ method, params: JSON.parse(params) });
      const result = method === "GetServerInfo"
        ? { version: "render-test", capabilities: ["file", "render_view"] }
        : rendering;
      return Promise.resolve(JSON.stringify({ jsonrpc: "2.0", id: null, result }));
    },
    close: () => Promise.resolve(),
  };
}

test("renderView reads a module's protojson rendering through the typed connection", async () => {
  const calls: Array<{ method: string; params: unknown }> = [];
  const connection = await connectWasmHost(renderingHost(calls), {});
  const view = await connection.renderView("h1", "#interconnection:Demo::Item", {
    ports: "minimal",
    show: ["edge-labels", "typings"],
  });
  assert.deepEqual(calls.map(({ method }) => method), ["GetServerInfo", "RenderView"]);
  assert.deepEqual(calls[1].params, {
    modelHash: "h1",
    view: "#interconnection:Demo::Item",
    show: ["edge-labels", "typings"],
  });
  assert.equal(view.kind, "interconnection");
  assert.deepEqual(view.notices, ["one notice"]);
  assert.deepEqual(view.nodes.map((node) => node.id), ["n0", "n1"]);
  assert.deepEqual(view.nodes[0].origin, { file: "demo.sysml", startLine: 2, startCol: 5, endLine: 2, endCol: 9 });
  assert.equal(view.nodes[0].geometry, undefined);
  assert.deepEqual(view.nodes[1].geometry, { x: 10, y: 20, width: 100, height: 40, hasSize: true, collapsed: false, unplaced: false });
  assert.deepEqual(view.nodes[1].ports, [{ id: "n1.0", name: "p", type: "", direction: "in" }]);
  assert.equal(view.nodes[1].style?.bold, true);
  assert.deepEqual(view.edges[0], {
    from: "n1",
    to: "n0",
    fromPort: "n1.0",
    toPort: "",
    label: "flow",
    name: "",
    kind: "connection",
    route: [],
  });
  await connection.close();
});

test("a module without render_view refuses renderView before calling it", async () => {
  const calls: Array<{ method: string; params: unknown }> = [];
  const host = renderingHost(calls);
  host.call = (method: string, params: string) => {
    calls.push({ method, params: JSON.parse(params) });
    return Promise.resolve(JSON.stringify({
      jsonrpc: "2.0",
      id: null,
      result: { version: "render-test", capabilities: ["file"] },
    }));
  };
  const connection = await connectWasmHost(host, {});
  await assert.rejects(connection.renderView("h1", "Demo::V"), /render_view/);
  assert.deepEqual(calls.map(({ method }) => method), ["GetServerInfo"]);
  await connection.close();
});

test("renderedViewFromJson decodes a saved protojson rendering", () => {
  const view = renderedViewFromJson(rendering);
  assert.equal(view.view, "#interconnection:Demo::Item");
  assert.deepEqual(view.shown, ["edge-labels"]);
  assert.deepEqual(view.rows, []);
  assert.equal(view.canvas, undefined);
  assert.deepEqual(view.edges[0].route, []);
  assert.throws(() => renderedViewFromJson({ nodes: "not a list" }), /nodes/);
});
