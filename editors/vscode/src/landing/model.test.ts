import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import type { RenderResult } from "../protocol";
import {
  EngineClient,
  EngineInstance,
  JOURNEY_EVENTS,
  JOURNEY_SYMBOL,
  landingModel,
  journey,
  readModel,
  rpc,
} from "./model";

interface LandingFixture {
  hash: string;
  render: Omit<RenderResult, "form" | "artifact" | "version">;
  instances: EngineInstance[];
}

const fixture = JSON.parse(readFileSync("src/landing/stack.json", "utf8")) as LandingFixture;
const fixtureRender = fixture.render as RenderResult;

test("landingModel keeps the four ported project parts and their interface edges", () => {
  const model = landingModel(fixture.hash, fixtureRender, fixture.instances);
  assert.equal(model.parts.size, 4);
  assert.deepEqual(
    [...model.parts.keys()].sort(),
    ["flexo", "opensysml", "pilot", "toolkit"],
  );
  assert.ok(model.render.nodes.every((node) => node.kind !== "attribute"));
  assert.ok(model.render.nodes.every((node) => node.parent === undefined));

  const ports = new Map<string, string>();
  for (const part of model.parts.values()) {
    const node = model.render.nodes.find(({ id }) => id === part.id)!;
    const api = node.ports?.find(({ name }) => name === "api");
    assert.ok(api, `${part.feature} should keep its api port`);
    ports.set(part.feature, api.id);
  }
  assert.equal(model.render.edges.length, 3);
  assert.ok(model.render.edges.every((edge) => edge.toPort === ports.get("flexo")));
  assert.deepEqual(
    model.render.edges.map((edge) => edge.label).sort(),
    ["opensysml_flexo", "pilot_flexo", "toolkit_flexo"],
  );
  assert.equal(model.parts.get("opensysml")?.attrs.label, "OpenSysML");
});

test("readModel returns parse diagnostics without requesting a rendering", () => {
  const calls: string[] = [];
  const engine: EngineClient = {
    call(method) {
      calls.push(method);
      return JSON.stringify({
        result: {
          diagnostics: [{
            message: "unresolved name",
            severity: "error",
            span: { startLine: 3, startCol: 5 },
          }],
        },
      });
    },
  };
  assert.deepEqual(readModel(engine, "bad model"), {
    diagnostics: [{ message: "unresolved name", severity: "error", line: 3, column: 5 }],
  });
  assert.deepEqual(calls, ["ParseSources"]);
});

test("rpc throws errors returned in the envelope or result", () => {
  assert.throws(
    () => rpc({ call: () => JSON.stringify({ error: { message: "engine unavailable" } }) }, "RenderView", {}),
    /engine unavailable/,
  );
  assert.throws(
    () => rpc({ call: () => JSON.stringify({ result: { error: "symbol not found" } }) }, "Instantiate", {}),
    /symbol not found/,
  );
});

test("readModel requests a minimal interconnection render before instantiating the stack", () => {
  const calls: Array<{ method: string; params: Record<string, unknown> }> = [];
  const engine: EngineClient = {
    call(method, params) {
      calls.push({ method, params: JSON.parse(params) as Record<string, unknown> });
      const result = method === "ParseSources"
        ? { modelHash: fixture.hash, diagnostics: [] }
        : method === "RenderView"
          ? fixture.render
          : { instances: fixture.instances };
      return JSON.stringify({ result });
    },
  };
  const read = readModel(engine, "model source");
  assert.deepEqual(read.diagnostics, []);
  assert.ok(read.model);
  assert.deepEqual(calls.map(({ method }) => method), ["ParseSources", "RenderView", "Instantiate"]);
  assert.deepEqual(calls[1].params, {
    modelHash: fixture.hash,
    view: "#interconnection:OpenSysMLStack::stack",
    ports: "minimal",
  });
});

test("journey maps visited feature names to node ids and drops unknown states", () => {
  const model = landingModel(fixture.hash, fixtureRender, fixture.instances);
  const engine: EngineClient = {
    call(method, params) {
      assert.equal(method, "ExecuteState");
      assert.deepEqual(JSON.parse(params), {
        modelHash: fixture.hash,
        stateMachineSymbolId: JOURNEY_SYMBOL,
        events: JOURNEY_EVENTS,
      });
      return JSON.stringify({
        result: { statesVisited: ["opensysml", "unknown", "flexo", "toolkit", "flexo", "pilot"] },
      });
    },
  };
  assert.deepEqual(
    journey(engine, model),
    ["opensysml", "flexo", "toolkit", "flexo", "pilot"].map((feature) => model.parts.get(feature)!.id),
  );
});
