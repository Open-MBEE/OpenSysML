import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import type { Connection, DocumentEvent, FeatureValue, Instance, SysMLValue } from "@openmbee/opensysml/browser";
import { ExecutionError, renderedViewFromJson } from "@openmbee/opensysml/browser";
import type { RenderResult } from "../protocol";
import {
  EngineInstance,
  clockOf,
  debugSteps,
  landingModel,
  journey,
  readModel,
  renderResult,
  runClock,
  runJourney,
  stateIdOf,
  TraceRecord,
  journeyOf,
  sceneTarget,
} from "./model";
import { MISSION_PHASES_SCENE, STACK_SCENE } from "./scene";

interface LandingFixture {
  hash: string;
  /** The protojson RenderViewResponse the module answered. */
  render: unknown;
  instances: EngineInstance[];
}

const fixture = JSON.parse(readFileSync("src/landing/stack.json", "utf8")) as LandingFixture;
const fixtureView = renderedViewFromJson(fixture.render);
const fixtureRender: RenderResult = renderResult(fixtureView);
const phasesFixture = JSON.parse(readFileSync("src/landing/phases.json", "utf8")) as LandingFixture;
const phasesRender: RenderResult = renderResult(renderedViewFromJson(phasesFixture.render));

const PROJECTS = ["flexo", "opensysml", "pilot"];
const COMPONENTS: Record<string, string[]> = {
  opensysml: [
    "analyzers", "clients", "docgen", "editors", "engine", "interchange", "jupyter", "lsp", "migration", "oslc",
    "parser", "repl", "service", "solver", "stdlib", "validation",
  ],
  pilot: ["editors", "evaluator", "grammars", "jupyter", "plantuml", "stdlib", "validation", "xmi"],
  flexo: ["auth", "layer1", "quadstore", "sysmlv2"],
};
const COMPONENT_PATHS = Object.entries(COMPONENTS).flatMap(([project, features]) =>
  features.map((feature) => `${project}.${feature}`),
);

test("landingModel keeps the three ported project parts and their interface edges", () => {
  const model = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  assert.deepEqual([...model.parts.keys()].sort(), [...COMPONENT_PATHS, ...PROJECTS].sort());
  assert.ok(model.render.nodes.every((node) => node.kind !== "attribute"));
  const nodeOf = (feature: string) => model.render.nodes.find(({ id }) => id === model.parts.get(feature)?.id)!;
  for (const feature of PROJECTS) {
    assert.equal(nodeOf(feature).parent, undefined, `${feature} should be lifted to the top level`);
  }

  const ports = new Map<string, string>();
  for (const feature of PROJECTS) {
    const part = model.parts.get(feature)!;
    const node = model.render.nodes.find(({ id }) => id === part.id)!;
    const api = node.ports?.find(({ name }) => name === "api");
    assert.ok(api, `${part.feature} should keep its api port`);
    ports.set(part.feature, api.id);
  }
  assert.equal(model.render.edges.length, 2);
  assert.ok(model.render.edges.every((edge) => edge.toPort === ports.get("flexo")));
  assert.deepEqual(
    model.render.edges.map((edge) => edge.label).sort(),
    ["opensysml_flexo", "pilot_flexo"],
  );
  assert.equal(model.parts.get("opensysml")?.attrs.label, "OpenSysML Runtime Environment and Development Kit");
});

test("landingModel keeps each project's components inside it, with their own attributes", () => {
  const model = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  for (const [project, features] of Object.entries(COMPONENTS)) {
    const owner = model.parts.get(project)!;
    for (const feature of features) {
      const part = model.parts.get(`${project}.${feature}`)!;
      const node = model.render.nodes.find(({ id }) => id === part.id)!;
      assert.equal(node.parent, owner.id, `${feature} should stay inside ${project}`);
      assert.equal(part.feature, feature);
      assert.equal(part.owner, project);
      assert.equal(part.symbol, `OpenSysMLStack::stack::${project}::${feature}`);
      assert.equal(part.attrs.kind, "component");
      assert.ok(part.attrs.role, `${project}.${feature} should state its role`);
    }
  }
  // The same feature name in two projects names two parts.
  assert.equal(model.parts.get("opensysml.interchange")?.attrs.label, "Interchange");
  assert.equal(model.parts.get("pilot.xmi")?.attrs.role, "XMI");
  assert.equal(model.parts.get("opensysml.oslc")?.attrs.label, "OSLC query");
  assert.equal(model.parts.get("opensysml.docgen")?.attrs.label, "Document generation");
  assert.equal(model.parts.get("flexo.layer1")?.attrs.label, "Layer 1 service");
});

/** The fixture's instances as the typed client decodes them. */
function typedInstance(instance: EngineInstance): Instance {
  const features = new Map<string, FeatureValue>();
  for (const [name, feature] of Object.entries(instance.featureValues ?? {})) {
    const value: SysMLValue | undefined = feature.value?.stringValue !== undefined
      ? { kind: "string", value: feature.value.stringValue }
      : feature.value?.instanceId === undefined
        ? undefined
        : { kind: "instance", id: BigInt(feature.value.instanceId) };
    if (value !== undefined) {
      features.set(name, { kind: "single", value, materialized: true });
    }
  }
  return { id: BigInt(instance.id ?? 0), typeId: instance.typeSymbolId ?? "", features } as unknown as Instance;
}

function traceEvent(record: TraceRecord): DocumentEvent {
  const time = record.time === undefined
    ? undefined
    : { kind: "quantity", magnitude: { kind: "real", value: Number(record.time.magnitude) }, unit: record.time.unit };
  return {
    kind: record.kind,
    time,
    state: record.state ?? "",
    fromState: record.from ?? "",
    toState: record.to ?? "",
    event: record.event ?? "",
    alternatives: record.alternatives ?? [],
    taken: record.taken ?? "",
    text: record.text,
  } as unknown as DocumentEvent;
}

interface Call {
  method: string;
  params: unknown[];
}

interface FakeAnswers {
  diagnostics?: unknown[];
  statesVisited?: string[];
  trace?: TraceRecord[];
  finalTime?: number;
  executeError?: Error;
  /** Features of the root the instantiation could not value, by the error it gives each. */
  rootErrors?: Record<string, string>;
  /** The reason Evaluate gives for an undetermined feature. */
  evalReason?: string;
}

/** A connection answering as the fixture's engine would, recording every call. */
function fakeConnection(answers: FakeAnswers = {}): { connection: Connection; calls: Call[] } {
  const calls: Call[] = [];
  const instances = fixture.instances.map(typedInstance);
  for (const [name, error] of Object.entries(answers.rootErrors ?? {})) {
    (instances[0].features as Map<string, FeatureValue>).set(name, { kind: "error", error } as unknown as FeatureValue);
  }
  const connection = {
    parseSources(...params: unknown[]) {
      calls.push({ method: "ParseSources", params });
      return Promise.resolve({
        hash: fixture.hash,
        diagnostics: answers.diagnostics ?? [],
        instantiate(name: string) {
          calls.push({ method: "Instantiate", params: [fixture.hash, name] });
          return Promise.resolve({ root: instances[0], all: instances.slice(1) });
        },
        eval(...params: unknown[]) {
          calls.push({ method: "Eval", params });
          return Promise.resolve({ kind: "undetermined", reason: answers.evalReason ?? "", countLower: "1", countUpper: "1" });
        },
      });
    },
    renderView(...params: unknown[]) {
      calls.push({ method: "RenderView", params });
      return Promise.resolve(fixtureView);
    },
    executeState(...params: unknown[]) {
      calls.push({ method: "ExecuteState", params });
      if (answers.executeError) {
        return Promise.reject(answers.executeError);
      }
      return Promise.resolve({
        statesVisited: answers.statesVisited ?? [],
        trace: (answers.trace ?? []).map(traceEvent),
        finalTime: answers.finalTime ?? 0,
      });
    },
  };
  return { connection: connection as unknown as Connection, calls };
}

test("readModel returns parse diagnostics without requesting a rendering", async () => {
  const { connection, calls } = fakeConnection({
    diagnostics: [{
      message: "unresolved name",
      severity: "error",
      file: "opensysml-stack.sysml", startLine: 3, startColumn: 5, endLine: 3, endColumn: 9,
    }],
  });
  assert.deepEqual(await readModel(connection, STACK_SCENE, "bad model"), {
    diagnostics: [{ message: "unresolved name", severity: "error", line: 3, column: 5 }],
  });
  assert.deepEqual(calls.map(({ method }) => method), ["ParseSources"]);
});

test("readModel requests a minimal interconnection render before instantiating the stack", async () => {
  const { connection, calls } = fakeConnection();
  const read = await readModel(connection, STACK_SCENE, "model source");
  assert.deepEqual(read.diagnostics, []);
  assert.ok(read.model);
  assert.deepEqual(calls.map(({ method }) => method), ["ParseSources", "RenderView", "Instantiate"]);
  assert.deepEqual(calls[0].params, [[["opensysml-stack.sysml", "model source"]]]);
  assert.deepEqual(calls[1].params, [
    fixture.hash,
    "#interconnection:OpenSysMLStack::stack",
    { ports: "minimal", show: ["edge-labels", "typings", "keywords"] },
  ]);
  assert.deepEqual(calls[2].params, [fixture.hash, "OpenSysMLStack::stack"]);
  // The typed rendering and instances read back to the same model the fixture does.
  const expected = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  assert.deepEqual([...read.model.parts.keys()], [...expected.parts.keys()]);
  assert.deepEqual(read.model.parts.get("opensysml.engine"), expected.parts.get("opensysml.engine"));
  assert.deepEqual(read.model.render.edges, expected.render.edges);
});

test("readModel asks the engine why the root's unvalued feature is so, on the root's own path", async () => {
  const { connection, calls } = fakeConnection({
    rootErrors: { totalMass: "feature value stack.totalMass: no value for feature mass" },
    evalReason: "mass has no value in the model",
  });
  const read = await readModel(connection, STACK_SCENE, "model source");
  assert.ok(read.model);
  assert.equal(read.model.root.feature, "stack");
  assert.deepEqual(read.model.root.unvalued, { totalMass: "mass has no value in the model" });
  assert.deepEqual(
    calls.filter(({ method }) => method === "Eval").map(({ params }) => params),
    [["OpenSysMLStack::stack.totalMass"]],
  );
  for (const part of read.model.parts.values()) {
    assert.deepEqual(part.unvalued, {}, `${part.path} is valued`);
  }
});

test("journey maps visited feature names to node ids and drops unknown states", async () => {
  const model = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  const { connection, calls } = fakeConnection({
    statesVisited: ["start", "opensysml", "unknown", "flexo", "pilot", "flexo", "pilot"],
  });
  assert.deepEqual(
    await journey(connection, STACK_SCENE, model),
    ["opensysml", "flexo", "pilot", "flexo", "pilot"].map((feature) => model.parts.get(feature)!.id),
  );
  assert.deepEqual(calls, [{
    method: "ExecuteState",
    params: [fixture.hash, journeyOf(STACK_SCENE).symbol, { events: journeyOf(STACK_SCENE).events }],
  }]);
});

test("journey adds a seed only when defined", async () => {
  const model = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  const { connection, calls } = fakeConnection();

  await journey(connection, STACK_SCENE, model, 42);
  await journey(connection, STACK_SCENE, model);

  assert.deepEqual(calls.map(({ params }) => params[2]), [
    { events: journeyOf(STACK_SCENE).events, schedule: "seed:42" },
    { events: journeyOf(STACK_SCENE).events },
  ]);
});

test("runJourney requests a trace and adds a seed only when defined", async () => {
  const model = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  const { connection, calls } = fakeConnection({
    statesVisited: ["start", "flexo"],
    trace: [
      { kind: "entry", state: "start", text: "entry start" },
      { kind: "accept", event: "Commit", text: "accept Commit" },
    ],
  });
  const unseededEvents = ["Commit", "Pull", "Check", "Push"];
  const unseeded = await runJourney(connection, STACK_SCENE, model, unseededEvents);
  assert.deepEqual(calls[0], {
    method: "ExecuteState",
    params: [fixture.hash, journeyOf(STACK_SCENE).symbol, { events: unseededEvents, trace: true }],
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
  await runJourney(connection, STACK_SCENE, model, seededEvents, 2);
  assert.deepEqual(calls[1].params, [
    fixture.hash,
    journeyOf(STACK_SCENE).symbol,
    { events: seededEvents, trace: true, schedule: "seed:2" },
  ]);
});

test("runJourney returns the partial trace of a failed run and rethrows other errors", async () => {
  const model = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  const trace: TraceRecord[] = [
    { kind: "entry", state: "start", text: "entry start" },
    { kind: "accept", event: "Commit", text: "accept Commit" },
  ];
  const failed = fakeConnection({
    executeError: new ExecutionError(
      "state machine execution failed: incomplete run",
      "unspecified",
      [],
      trace.map(traceEvent),
    ),
  });
  assert.deepEqual(await runJourney(failed.connection, STACK_SCENE, model, ["Commit"]), {
    visited: [],
    trace,
    error: "state machine execution failed: incomplete run",
  });

  const unavailable = fakeConnection({ executeError: new Error("engine unavailable") });
  await assert.rejects(runJourney(unavailable.connection, STACK_SCENE, model, ["Commit"]), /engine unavailable/);
});

test("landingModel lifts a state view's states out of the machine's box and keys them by name", () => {
  const model = landingModel(sceneTarget(MISSION_PHASES_SCENE), phasesFixture.hash, phasesRender, phasesFixture.instances);
  assert.ok(model.machine, "a state view gives a machine");
  assert.equal(model.machine.symbol, "MissionSequence::MissionPhases");
  assert.deepEqual(
    [...model.machine.states.keys()],
    ["prelaunch", "translunarCoast", "lunarOrbit", "onSurface", "transEarthCoast", "recovered", "done"],
  );
  assert.equal(model.machine.states.get("prelaunch")?.initial, true);
  assert.equal(model.machine.states.get("onSurface")?.path, "onSurface");
  assert.equal(model.machine.states.get("onSurface")?.initial, false);
  assert.equal(model.machine.states.get("done")?.kind, "final");
  assert.equal(model.parts.size, 0, "a state is not a part");
  assert.equal(model.render.nodes.length, 8, "six states, the start and the final");
  assert.ok(model.render.nodes.every((node) => node.parent === undefined), "the states are drawn at the top level");
  assert.equal(model.render.edges.length, 7);
  assert.deepEqual(
    model.render.edges.map((edge) => edge.label),
    ["", "after 9000 [s]", "after 270000 [s]", "after 100000 [s]", "after 78000 [s]", "after 245000 [s]", ""],
  );
  assert.equal(model.root.symbol, "MissionSequence::mission", "the root is the object the machine runs on");
  assert.equal(model.root.feature, "mission");
  assert.equal(stateIdOf(model, "onSurface"), model.machine.states.get("onSurface")?.id);
  assert.equal(stateIdOf(model, "nowhere"), undefined);
  const stack = landingModel(sceneTarget(STACK_SCENE), fixture.hash, fixtureRender, fixture.instances);
  assert.equal(stack.machine, undefined);

  // A state nested in a composite stays under it, keyed by its path and, being the only `landing`, by name.
  const surface = model.machine.states.get("onSurface")!;
  const nested: RenderResult = {
    ...phasesRender,
    nodes: [
      ...phasesRender.nodes,
      { id: "n9", kind: "state", name: "landing", type: "", detail: "", parent: surface.id },
      { id: "n10", kind: "state", name: "prelaunch", type: "", detail: "", parent: surface.id },
    ],
    edges: [...phasesRender.edges, { from: surface.id, to: "n9", kind: "transition", label: "" }],
  };
  const composite = landingModel(sceneTarget(MISSION_PHASES_SCENE), phasesFixture.hash, nested, phasesFixture.instances);
  assert.equal(composite.render.nodes.find((node) => node.id === "n9")?.parent, surface.id, "a nested state keeps its parent");
  assert.equal(composite.render.edges.filter((edge) => edge.to === "n9").length, 1, "the transition into it is kept");
  assert.equal(composite.machine?.states.get("onSurface.landing")?.id, "n9");
  assert.equal(stateIdOf(composite, "landing"), "n9");
  assert.equal(composite.machine?.states.get("onSurface.prelaunch")?.id, "n10");
  assert.equal(stateIdOf(composite, "prelaunch"), model.machine.states.get("prelaunch")?.id, "a repeated name stays with the machine's own state");
  assert.equal(stateIdOf(stack, "flexo"), stack.parts.get("flexo")?.id, "a journey's states name parts");
  assert.throws(
    () => landingModel(sceneTarget(MISSION_PHASES_SCENE), phasesFixture.hash, { ...phasesRender, nodes: [] }, phasesFixture.instances),
    /draws 0 machines/u,
  );
});

test("runClock performs the machine on its object for the duration and keeps each record's instant", async () => {
  const model = landingModel(sceneTarget(MISSION_PHASES_SCENE), phasesFixture.hash, phasesRender, phasesFixture.instances);
  const trace: TraceRecord[] = [
    { kind: "entry", state: "prelaunch", time: { magnitude: "0", unit: "s" }, text: "enter: prelaunch" },
    {
      kind: "transition",
      from: "prelaunch",
      to: "translunarCoast",
      event: "time",
      time: { magnitude: "9000", unit: "s" },
      text: "transition: prelaunch -> translunarCoast (event: time)",
    },
  ];
  const { connection, calls } = fakeConnection({ statesVisited: ["prelaunch", "translunarCoast"], finalTime: 400000, trace });
  const program = clockOf(MISSION_PHASES_SCENE.program);
  const run = await runClock(connection, program, model);
  assert.deepEqual(calls[0], {
    method: "ExecuteState",
    params: [phasesFixture.hash, "MissionSequence::MissionPhases", { performer: "MissionSequence::mission", advance: 400000, trace: true }],
  });
  assert.deepEqual(run, { visited: ["prelaunch", "translunarCoast"], trace, finalTime: 400000 });
  assert.throws(() => clockOf(STACK_SCENE.program), /runs no clock/u);

  const failed = fakeConnection({
    executeError: new ExecutionError("state machine execution failed: budget", "unspecified", [], trace.map(traceEvent)),
  });
  assert.deepEqual(await runClock(failed.connection, program, model), {
    visited: [],
    trace,
    error: "state machine execution failed: budget",
  });
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

test("renderResult carries a node's Style colours as the fill and border the canvas paints", () => {
  const result = renderResult(
    renderedViewFromJson({
      view: "#interconnection:Demo::Item",
      kind: "interconnection",
      nodes: [
        { id: "n0", kind: "part", name: "Demo::Item" },
        { id: "n1", kind: "part", name: "red", parent: "n0", style: { fill: "#ff0000", line: "#800000", bold: true } },
        { id: "n2", kind: "part", name: "plain", parent: "n0" },
      ],
    }),
  );
  const [, red, plain] = result.nodes;
  assert.equal(red.fill, "#ff0000");
  assert.equal(red.border, "#800000");
  assert.equal(red.style?.bold, true);
  assert.equal(plain.fill, undefined);
  assert.equal(plain.border, undefined);
});

test("renderResult states no x and y for a geometry that only folds the node", () => {
  const result = renderResult(
    renderedViewFromJson({
      view: "#interconnection:Demo::Item",
      kind: "interconnection",
      nodes: [
        { id: "n0", kind: "part", name: "Demo::Item" },
        { id: "n1", kind: "part", name: "folded", parent: "n0", geometry: { collapsed: true, unplaced: true } },
        { id: "n2", kind: "part", name: "pinned", parent: "n0", geometry: { x: 10, y: 20 } },
      ],
    }),
  );
  const [, folded, pinned] = result.nodes;
  assert.equal(folded.collapsed, true);
  assert.equal(folded.x, undefined);
  assert.equal(folded.y, undefined);
  assert.deepEqual([pinned.x, pinned.y], [10, 20]);
});
