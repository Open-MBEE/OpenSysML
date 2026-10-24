import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import type { RenderResult } from "../protocol";
import { autoLayout } from "./autolayout";
import { drawCanvas } from "./canvas";
import { type CanvasLayout, layoutCanvas, type PlacedNode } from "./layout";

const dom = new JSDOM("<!DOCTYPE html><body></body>");
globalThis.document = dom.window.document;

// The fixture is what the server renders for testdata/collapsed-ports.sysml, where
// engine is collapsed: its two ports stay on its boundary and the connectors still
// end at them, while injector and injector's port are folded away. The server
// pins it, so the panel is measured here against the rendering the CLI writes.
const fixture = JSON.parse(
  readFileSync("src/webview/testdata/collapsed-ports.json", "utf8"),
) as Record<"tree" | "wiring", RenderResult>;

function named(layout: CanvasLayout, name: string): PlacedNode {
  const found = [...layout.nodes.values()].find((entry) => entry.node.name === name);
  assert.ok(found, `${name} is laid out`);
  return found;
}

function inside(inner: PlacedNode, outer: PlacedNode): boolean {
  const a = inner.box;
  const b = outer.box;
  return a.x >= b.x && a.y > b.y && a.x + a.width <= b.x + b.width && a.y + a.height <= b.y + b.height;
}

for (const view of ["tree", "wiring"] as const) {
  test(`the ${view} panel draws every node the server renders for a collapsed part, its ports included`, async () => {
    const result = fixture[view];
    const auto = await autoLayout(result);
    assert.ok(auto, "ELK lays the rendering out, as the panel does");
    const layout = layoutCanvas(result, {}, auto);
    const svg = drawCanvas(layout);
    const drawn = [...svg.querySelectorAll<SVGGElement>("g.opensysml-node")].map(
      (group) => group.dataset.opensysmlId,
    );
    assert.deepEqual(drawn.sort(), result.nodes.map((node) => node.id).sort());

    const engine = named(layout, "engine");
    assert.equal(engine.collapsed, true);
    assert.equal(svg.querySelector(`g[data-opensysml-id="${engine.node.id}"] text.collapsed`)?.textContent, "+");
    assert.deepEqual(
      engine.children.map((child) => `${child.node.kind} ${child.node.name}`),
      ["port fuelIn", "port shaftOut"],
    );
    for (const port of engine.children) {
      assert.ok(inside(port, engine), `${port.node.name} is drawn on engine`);
    }
    for (const name of ["injector", "feed"]) {
      assert.equal([...layout.nodes.values()].some((entry) => entry.node.name === name), false);
    }

    const edges = [...svg.querySelectorAll<SVGGElement>("g.opensysml-edge")].map((edge) => edge.dataset.edge);
    assert.deepEqual(edges, result.edges.map((_, index) => String(index)));
    for (const placed of layout.edges) {
      assert.ok(placed.points.length >= 2, `edge ${placed.edge.from} -> ${placed.edge.to} is routed`);
    }
  });
}

test("the wiring fixture's connectors end at the collapsed part's ports", () => {
  const result = fixture.wiring;
  const nameOf = new Map(result.nodes.map((node) => [node.id, node.name]));
  assert.deepEqual(
    result.edges.map((edge) => `${nameOf.get(edge.from)} -- ${nameOf.get(edge.to)}`),
    ["fuelOut -- fuelIn", "shaftOut -- hub", "engine -- fuelOut"],
  );
});
