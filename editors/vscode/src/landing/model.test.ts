import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import type { RenderResult } from "../protocol";
import {
  EngineClient,
  EngineInstance,
  JOURNEY_EVENTS,
  JOURNEY_SYMBOL,
  debugSteps,
  landingModel,
  journey,
  readModel,
  rpc,
  runJourney,
  TraceRecord,
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
        result: { statesVisited: ["start", "opensysml", "unknown", "flexo", "toolkit", "flexo", "pilot"] },
      });
    },
  };
  assert.deepEqual(
    journey(engine, model),
    ["opensysml", "flexo", "toolkit", "flexo", "pilot"].map((feature) => model.parts.get(feature)!.id),
  );
});

test("runJourney requests a trace and adds a seed only when defined", () => {
  const model = landingModel(fixture.hash, fixtureRender, fixture.instances);
  const calls: Array<{ method: string; params: Record<string, unknown> }> = [];
  const engine: EngineClient = {
    call(method, params) {
      calls.push({ method, params: JSON.parse(params) as Record<string, unknown> });
      return JSON.stringify({
        result: {
          statesVisited: ["start", "flexo"],
          trace: [
            {
              kind: "entry",
              state: "start",
              text: "entry start",
              machine: "ModelJourney",
              at: 0,
            },
            { kind: "accept", event: "Commit", text: "accept Commit" },
          ],
        },
      });
    },
  };
  const unseededEvents = ["Commit", "Pull", "Check", "Push"];
  const unseeded = runJourney(engine, model, unseededEvents);
  assert.deepEqual(calls[0], {
    method: "ExecuteState",
    params: {
      modelHash: fixture.hash,
      stateMachineSymbolId: JOURNEY_SYMBOL,
      events: unseededEvents,
      trace: true,
    },
  });
  assert.deepEqual(unseeded, {
    visited: ["start", "flexo"],
    trace: [
      { kind: "entry", state: "start", text: "entry start" },
      { kind: "accept", event: "Commit", text: "accept Commit" },
    ],
  });
  assert.equal(Object.hasOwn(unseeded.trace[1], "state"), false);

  const seededEvents = ["Commit", "Pull"];
  runJourney(engine, model, seededEvents, 2);
  assert.deepEqual(calls[1], {
    method: "ExecuteState",
    params: {
      modelHash: fixture.hash,
      stateMachineSymbolId: JOURNEY_SYMBOL,
      events: seededEvents,
      trace: true,
      schedule: "seed:2",
    },
  });
});

test("runJourney returns partial traces on result errors and throws envelope errors", () => {
  const model = landingModel(fixture.hash, fixtureRender, fixture.instances);
  const failed: EngineClient = {
    call: () => JSON.stringify({
      result: {
        statesVisited: ["start", "flexo"],
        trace: [
          { kind: "entry", state: "start", text: "entry start" },
          { kind: "accept", event: "Commit", text: "accept Commit" },
        ],
        error: "state machine execution failed: incomplete run",
      },
    }),
  };
  assert.deepEqual(runJourney(failed, model, ["Commit"]), {
    visited: ["start", "flexo"],
    trace: [
      { kind: "entry", state: "start", text: "entry start" },
      { kind: "accept", event: "Commit", text: "accept Commit" },
    ],
    error: "state machine execution failed: incomplete run",
  });

  const unavailable: EngineClient = {
    call: () => JSON.stringify({ error: { message: "engine unavailable" } }),
  };
  assert.throws(() => runJourney(unavailable, model, ["Commit"]), /engine unavailable/);
});

test("debugSteps tracks active state, transitions, accepts, and ignored events", () => {
  const trace: TraceRecord[] = [
    { kind: "entry", state: "flexo", text: "entry flexo" },
    { kind: "accept", event: "Commit", text: "accept Commit" },
    { kind: "accept", event: "Pull", text: "accept Pull" },
    {
      kind: "choice",
      alternatives: ["1->toolkit", "2->pilot"],
      taken: "1->toolkit",
      text: "choice",
    },
    { kind: "exit", state: "flexo", text: "exit flexo" },
    { kind: "transition", from: "flexo", to: "toolkit", text: "transition" },
    { kind: "entry", state: "toolkit", text: "entry toolkit" },
    { kind: "accept", event: "Check", text: "accept Check" },
  ];
  const steps = debugSteps(trace);
  assert.deepEqual(
    steps.map(({ state }) => state),
    ["flexo", "flexo", "flexo", "flexo", "flexo", "flexo", "toolkit", "toolkit"],
  );
  assert.deepEqual(steps.map(({ accepted }) => accepted), [0, 1, 2, 2, 2, 2, 2, 3]);
  assert.equal(steps[1].ignored, true);
  assert.equal(Object.hasOwn(steps[2], "ignored"), false);
  assert.equal(Object.hasOwn(steps[3], "ignored"), false);
  assert.equal(steps[4].state, "flexo");
  assert.deepEqual(steps[5].edge, { from: "flexo", to: "toolkit" });
  assert.equal(steps[7].ignored, true);
  assert.equal(Object.hasOwn(steps[5], "ignored"), false);
});

test("debugSteps marks the seed:2 Commit ignored and Pull fired", () => {
  const trace: TraceRecord[] = [
    { kind: "entry", state: "start", text: "entry start" },
    {
      kind: "choice",
      alternatives: ["1->opensysml", "2->flexo"],
      taken: "2->flexo",
      text: "choice",
    },
    { kind: "exit", state: "start", text: "exit start" },
    { kind: "entry", state: "flexo", text: "entry flexo" },
    { kind: "transition", from: "start", to: "flexo", text: "transition" },
    { kind: "accept", event: "Commit", text: "accept Commit" },
    { kind: "accept", event: "Pull", text: "accept Pull" },
    {
      kind: "choice",
      alternatives: ["1->toolkit", "2->pilot", "3->opensysml"],
      taken: "3->opensysml",
      text: "choice",
    },
    { kind: "exit", state: "flexo", text: "exit flexo" },
    { kind: "entry", state: "opensysml", text: "entry opensysml" },
    {
      kind: "transition",
      from: "flexo",
      to: "opensysml",
      event: "accept Pull",
      text: "transition",
    },
  ];
  const steps = debugSteps(trace);
  assert.equal(steps[5].ignored, true);
  assert.equal(Object.hasOwn(steps[6], "ignored"), false);
  assert.equal(steps[5].accepted, 1);
  assert.equal(steps[6].accepted, 2);
  assert.deepEqual(steps[10].edge, { from: "flexo", to: "opensysml" });
});
